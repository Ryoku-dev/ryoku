package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

const (
	// accountEvery paces account reads per provider, however many windows,
	// clients or restarts ask.
	accountEvery = 10 * time.Minute
	// accountTimeout bounds one provider's account read.
	accountTimeout = 15 * time.Second
)

// accountReading is what a provider's account endpoint said.
type accountReading struct {
	windows []protocol.LimitWindow
	credits *protocol.LimitCredits
}

// accountReader asks one provider's account endpoints with get, which sends
// the key and returns the body of a 2xx answer.
type accountReader func(get func(url string) ([]byte, error), base string, now time.Time) (accountReading, error)

// accountReaders are the providers with an account endpoint, each verified
// against the provider's documentation:
//   - OpenRouter GET /api/v1/key (openrouter.ai/docs/api_reference/limits)
//     and GET /api/v1/credits, which only a management key may read;
//   - DeepSeek GET /user/balance (api-docs.deepseek.com/api/get-user-balance);
//   - SiliconFlow GET /v1/user/info (docs.siliconflow.com, Retrieve user info);
//   - Pollinations GET /account/balance (gen.pollinations.ai/docs, Account).
var accountReaders = map[string]accountReader{
	"openrouter":   readOpenRouter,
	"deepseek":     readDeepSeek,
	"siliconflow":  readSiliconFlow,
	"pollinations": readPollinations,
}

// RefreshLimits reads the account endpoints of the providers that have one
// and a usable key, or only of providerID when it is set, where the last read
// is ten minutes old. It returns at once and reports the change when the
// reads are done, straight away when none was due, so the providers'
// rate-limit headers and today's counts reach the app as well.
func (g *Gateway) RefreshLimits(ctx context.Context, providerID string) error {
	var due []*provider
	if strings.TrimSpace(providerID) != "" {
		p, err := g.findProvider(ctx, providerID, false)
		if err != nil {
			return err
		}
		due = append(due, p)
	} else {
		for i := range g.catalog.Providers {
			due = append(due, &g.catalog.Providers[i])
		}
	}
	var start []*provider
	for _, p := range due {
		if g.claimAccountRead(p, false) {
			start = append(start, p)
		}
	}
	if len(start) == 0 {
		g.notifyChange()
		return nil
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		done := make(chan struct{}, len(start))
		for _, p := range start {
			go func() {
				g.readAccount(g.ctx, p)
				done <- struct{}{}
			}()
		}
		for range start {
			<-done
		}
		g.notifyChange()
	}()
	return nil
}

// checkAccount reads a provider's account now, whatever the pacing says; it
// is what Check does besides its test completion.
func (g *Gateway) checkAccount(p *provider) {
	if g.claimAccountRead(p, true) {
		g.readAccount(g.ctx, p)
	}
}

// claimAccountRead says whether p's account is to be read now and marks it
// read: a provider with an account endpoint and a key, no read running, and
// the last read ten minutes old unless force is set.
func (g *Gateway) claimAccountRead(p *provider, force bool) bool {
	if accountReaders[p.ID] == nil {
		return false
	}
	if _, ok := g.accountKey(p); !ok {
		return false
	}
	g.limits.mu.Lock()
	defer g.limits.mu.Unlock()
	if g.limits.reading[p.ID] {
		return false
	}
	record := g.limitRecordLocked(p.ID)
	now := g.now()
	if !force && !record.Read.IsZero() && now.Sub(record.Read) < accountEvery {
		return false
	}
	if g.limits.reading == nil {
		g.limits.reading = map[string]bool{}
	}
	g.limits.reading[p.ID] = true
	record.Read = now
	_ = g.saveLimitRecordLocked(p.ID, record)
	return true
}

// accountKey is the key an account read sends: the provider's first one
// that is not known to be refused.
func (g *Gateway) accountKey(p *provider) (string, bool) {
	records, err := g.store.listKeys(p.ID)
	if err != nil {
		return "", false
	}
	for _, record := range records {
		if record.Status != protocol.KeyError {
			return record.ID, true
		}
	}
	return "", false
}

// readAccount asks p's account endpoints and keeps what they said. A failed
// read keeps the last figures; it is never logged, and its error carries no
// key and no body.
func (g *Gateway) readAccount(ctx context.Context, p *provider) {
	defer func() {
		g.limits.mu.Lock()
		delete(g.limits.reading, p.ID)
		g.limits.mu.Unlock()
	}()
	keyID, ok := g.accountKey(p)
	if !ok {
		return
	}
	secret, _, err := g.store.reveal(keyID)
	if err != nil || secret == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, accountTimeout)
	defer cancel()
	get := func(url string) ([]byte, error) { return g.accountGet(ctx, url, secret) }
	reading, err := accountReaders[p.ID](get, strings.TrimRight(p.BaseURL, "/"), g.now())
	if err != nil {
		return
	}
	g.limits.mu.Lock()
	defer g.limits.mu.Unlock()
	record := g.limitRecordLocked(p.ID)
	record.Account = reading.windows
	if reading.credits != nil {
		record.Credits = reading.credits
	}
	_ = g.saveLimitRecordLocked(p.ID, record)
}

// accountGet sends one authorised GET and returns a 2xx body.
func (g *Gateway) accountGet(ctx context.Context, url, secret string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.New("account request")
	}
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Accept", "application/json")
	response, err := g.client.Do(request)
	if err != nil {
		return nil, errors.New("account endpoint unreachable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return nil, fmt.Errorf("account endpoint answered %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 256<<10))
}

// withoutVersion drops a trailing /v1 from a provider's API base, where its
// account endpoints live beside the versioned API.
func withoutVersion(base string) string {
	return strings.TrimSuffix(base, "/v1")
}

func readOpenRouter(get func(string) ([]byte, error), base string, now time.Time) (accountReading, error) {
	body, err := get(base + "/key")
	if err != nil {
		return accountReading{}, err
	}
	reading, err := parseOpenRouterKey(body, now)
	if err != nil {
		return accountReading{}, err
	}
	// only a management key may read the balance; an ordinary key keeps what /key said
	if body, err := get(base + "/credits"); err == nil {
		if credits, err := parseOpenRouterCredits(body, now); err == nil {
			credits.FreeTier = reading.credits != nil && reading.credits.FreeTier
			reading.credits = credits
		}
	}
	return reading, nil
}

// parseOpenRouterKey reads GET /api/v1/key: free_model_daily_requests is
// the free-model request window of the UTC day, and limit, limit_remaining
// and usage the key's credit cap and spend in dollars.
func parseOpenRouterKey(body []byte, now time.Time) (accountReading, error) {
	var answer struct {
		Data *struct {
			Limit          *float64 `json:"limit"`
			LimitRemaining *float64 `json:"limit_remaining"`
			Usage          float64  `json:"usage"`
			IsFreeTier     bool     `json:"is_free_tier"`
			FreeDaily      *struct {
				Limit     *int64 `json:"limit"`
				Remaining *int64 `json:"remaining"`
			} `json:"free_model_daily_requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || answer.Data == nil {
		return accountReading{}, errors.New("unexpected OpenRouter key answer")
	}
	data := answer.Data
	seen := now
	reading := accountReading{credits: &protocol.LimitCredits{Limit: data.Limit, Remaining: data.LimitRemaining, Used: data.Usage, Currency: "USD", FreeTier: data.IsFreeTier, Source: protocol.LimitFromAccount, Seen: now}}
	if daily := data.FreeDaily; daily != nil && daily.Limit != nil {
		reset := nextUTCDay(now)
		reading.windows = append(reading.windows, protocol.LimitWindow{Span: protocol.LimitSpanDay, Unit: protocol.LimitUnitRequests, Limit: *daily.Limit, Remaining: daily.Remaining, Reset: &reset, Source: protocol.LimitFromAccount, Seen: &seen})
	}
	return reading, nil
}

// parseOpenRouterCredits reads GET /api/v1/credits: total_credits bought
// and total_usage spent, in dollars.
func parseOpenRouterCredits(body []byte, now time.Time) (*protocol.LimitCredits, error) {
	var answer struct {
		Data *struct {
			TotalCredits *float64 `json:"total_credits"`
			TotalUsage   float64  `json:"total_usage"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || answer.Data == nil || answer.Data.TotalCredits == nil {
		return nil, errors.New("unexpected OpenRouter credits answer")
	}
	total := *answer.Data.TotalCredits
	left := total - answer.Data.TotalUsage
	return &protocol.LimitCredits{Limit: &total, Remaining: &left, Used: answer.Data.TotalUsage, Currency: "USD", Source: protocol.LimitFromAccount, Seen: now}, nil
}

func readDeepSeek(get func(string) ([]byte, error), base string, now time.Time) (accountReading, error) {
	body, err := get(withoutVersion(base) + "/user/balance")
	if err != nil {
		return accountReading{}, err
	}
	credits, err := parseDeepSeekBalance(body, now)
	return accountReading{credits: credits}, err
}

// parseDeepSeekBalance reads GET /user/balance: one balance per currency as
// decimal strings, the granted part included in total_balance. USD is
// preferred when both are there.
func parseDeepSeekBalance(body []byte, now time.Time) (*protocol.LimitCredits, error) {
	var answer struct {
		BalanceInfos []struct {
			Currency     string `json:"currency"`
			TotalBalance string `json:"total_balance"`
		} `json:"balance_infos"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || len(answer.BalanceInfos) == 0 {
		return nil, errors.New("unexpected DeepSeek balance answer")
	}
	pick := answer.BalanceInfos[0]
	for _, info := range answer.BalanceInfos {
		if info.Currency == "USD" {
			pick = info
		}
	}
	left, err := strconv.ParseFloat(pick.TotalBalance, 64)
	if err != nil {
		return nil, errors.New("unexpected DeepSeek balance answer")
	}
	return &protocol.LimitCredits{Remaining: &left, Currency: pick.Currency, Source: protocol.LimitFromAccount, Seen: now}, nil
}

func readSiliconFlow(get func(string) ([]byte, error), base string, now time.Time) (accountReading, error) {
	body, err := get(base + "/user/info")
	if err != nil {
		return accountReading{}, err
	}
	// the Chinese site bills in yuan, the international one in dollars
	currency := "USD"
	if strings.Contains(base, "siliconflow.cn") {
		currency = "CNY"
	}
	credits, err := parseSiliconFlowInfo(body, currency, now)
	return accountReading{credits: credits}, err
}

// parseSiliconFlowInfo reads GET /v1/user/info: totalBalance is the granted
// balance and the charged one together, as a decimal string.
func parseSiliconFlowInfo(body []byte, currency string, now time.Time) (*protocol.LimitCredits, error) {
	var answer struct {
		Data *struct {
			TotalBalance string `json:"totalBalance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || answer.Data == nil {
		return nil, errors.New("unexpected SiliconFlow user answer")
	}
	left, err := strconv.ParseFloat(answer.Data.TotalBalance, 64)
	if err != nil {
		return nil, errors.New("unexpected SiliconFlow user answer")
	}
	return &protocol.LimitCredits{Remaining: &left, Currency: currency, Source: protocol.LimitFromAccount, Seen: now}, nil
}

func readPollinations(get func(string) ([]byte, error), base string, now time.Time) (accountReading, error) {
	body, err := get(withoutVersion(base) + "/account/balance")
	if err != nil {
		return accountReading{}, err
	}
	credits, err := parsePollinationsBalance(body, now)
	return accountReading{credits: credits}, err
}

// parsePollinationsBalance reads GET /account/balance: balance is what the
// key may still spend, in pollen.
func parsePollinationsBalance(body []byte, now time.Time) (*protocol.LimitCredits, error) {
	var answer struct {
		Balance *float64 `json:"balance"`
	}
	if err := json.Unmarshal(body, &answer); err != nil || answer.Balance == nil {
		return nil, errors.New("unexpected Pollinations balance answer")
	}
	left := *answer.Balance
	return &protocol.LimitCredits{Remaining: &left, Currency: "pollen", Source: protocol.LimitFromAccount, Seen: now}, nil
}

func nextUTCDay(now time.Time) time.Time {
	utc := now.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC)
}

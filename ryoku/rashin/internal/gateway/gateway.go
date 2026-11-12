// Package gateway provides Rashin's in-process model router: free, paid and
// local providers behind one OpenAI-compatible chat client.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

const (
	maxAttempts     = 6
	firstByteBudget = 45 * time.Second
)

// Gateway is Rashin's provider catalogue, credential vault and chat router.
type Gateway struct {
	catalog  catalog
	store    *store
	client   *http.Client
	local    localServers
	settings func() protocol.Settings
	now      func() time.Time

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	close    sync.Once
	changeMu sync.RWMutex
	onChange func()

	apiMu  sync.Mutex
	apiKey string
}

// Open opens the gateway database and embedded provider catalogue. settings
// gives the addresses of the local providers each time they are needed.
func Open(stateDir string, settings func() protocol.Settings) (*Gateway, error) {
	catalog, err := loadCatalog()
	if err != nil {
		return nil, err
	}
	store, err := openStore(stateDir)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Gateway{
		catalog: catalog,
		store:   store,
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          32,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: firstByteBudget,
		}},
		local:    newLocalServers(),
		settings: settings,
		now:      time.Now,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// OnChange installs the daemon hook for asynchronous provider-state changes.
func (g *Gateway) OnChange(fn func()) {
	g.changeMu.Lock()
	g.onChange = fn
	g.changeMu.Unlock()
}

func (g *Gateway) notifyChange() {
	g.changeMu.RLock()
	fn := g.onChange
	g.changeMu.RUnlock()
	if fn != nil {
		fn()
	}
}

// providers lists every provider: the local ones first, then the catalogue.
// fresh asks the local servers now instead of trusting a recent answer.
func (g *Gateway) providers(ctx context.Context, fresh bool) []*provider {
	local := g.localProviders(ctx, fresh)
	all := make([]*provider, 0, len(local)+len(g.catalog.Providers))
	for i := range local {
		all = append(all, &local[i])
	}
	for i := range g.catalog.Providers {
		all = append(all, &g.catalog.Providers[i])
	}
	return all
}

// findProvider finds a local or catalogue provider by id. fresh asks the
// local servers now.
func (g *Gateway) findProvider(ctx context.Context, id string, fresh bool) (*provider, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if slices.Contains(protocol.LocalProviders, id) {
		for _, p := range g.localProviders(ctx, fresh) {
			if p.ID == id {
				return &p, nil
			}
		}
	}
	if p, ok := g.catalog.findProvider(id); ok {
		return p, nil
	}
	return nil, fmt.Errorf(i18n.T("Rashin has no provider %s"), id)
}

// Providers returns every provider and its local key state without exposing
// a secret.
func (g *Gateway) Providers(ctx context.Context) ([]protocol.ProviderInfo, error) {
	providers := g.providers(ctx, false)
	infos := make([]protocol.ProviderInfo, 0, len(providers))
	for _, p := range providers {
		info, err := g.providerInfo(ctx, p)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (g *Gateway) providerInfo(_ context.Context, p *provider) (protocol.ProviderInfo, error) {
	enabled, err := g.store.providerEnabled(p.ID)
	if err != nil {
		return protocol.ProviderInfo{}, err
	}
	records, err := g.store.listKeys(p.ID)
	if err != nil {
		return protocol.ProviderInfo{}, err
	}
	info := protocol.ProviderInfo{ID: p.ID, Name: p.Name, Tier: p.Tier, Keyless: p.Keyless, Enabled: enabled, Configured: p.Keyless || len(records) > 0, Models: len(p.Models), Signup: p.Signup, KeyHint: p.KeyHint, Note: p.Note}
	if p.Tier == protocol.TierLocal {
		info.Local, info.Address, info.Running = true, p.BaseURL, p.running
	}
	if until, ok, err := g.store.cooldown(providerScope(p.ID), g.now()); err != nil {
		return protocol.ProviderInfo{}, err
	} else if ok {
		info.CooldownUntil = &until
	}
	usable := p.Keyless && enabled && info.CooldownUntil == nil
	for _, record := range records {
		secret, _, err := g.store.reveal(record.ID)
		if err != nil {
			return protocol.ProviderInfo{}, err
		}
		key := protocol.KeyInfo{ID: record.ID, Label: record.Label, Status: record.Status, Masked: maskSecret(secret), LastError: record.LastError}
		if !record.Checked.IsZero() {
			checked := record.Checked
			key.Checked = &checked
		}
		info.Keys = append(info.Keys, key)
		if record.Status != protocol.KeyError {
			if _, cooling, err := g.store.cooldown(keyScope(record.ID), g.now()); err != nil {
				return protocol.ProviderInfo{}, err
			} else if !cooling {
				usable = true
			}
		}
	}
	if usable && enabled {
		for _, m := range p.Models {
			if _, cooling, err := g.store.cooldown(modelScope(p.ID, m.ID), g.now()); err != nil {
				return protocol.ProviderInfo{}, err
			} else if !cooling {
				info.Available++
			}
		}
	}
	return info, nil
}

// AddKey stores a credential as unknown, then tests it with real inference.
func (g *Gateway) AddKey(ctx context.Context, params protocol.AddKeyParams) (protocol.ProviderInfo, error) {
	p, err := g.findProvider(ctx, params.Provider, false)
	if err != nil {
		return protocol.ProviderInfo{}, err
	}
	secret := strings.TrimSpace(params.Key)
	if p.Keyless {
		return protocol.ProviderInfo{}, errors.New(i18n.T("This provider works without a key"))
	}
	if secret == "" {
		return protocol.ProviderInfo{}, errors.New(i18n.T("Enter a provider key"))
	}
	record, _, err := g.store.addKey(p.ID, params.Label, secret)
	if err != nil {
		return protocol.ProviderInfo{}, fmt.Errorf("store provider key: %w", err)
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		probeCtx, cancel := context.WithTimeout(g.ctx, firstByteBudget)
		defer cancel()
		_, _ = g.testKey(probeCtx, p, record.ID)
	}()
	return g.providerInfo(ctx, p)
}

// RemoveKey removes one credential by its opaque id.
func (g *Gateway) RemoveKey(ctx context.Context, params protocol.RemoveKeyParams) (protocol.ProviderInfo, error) {
	p, err := g.findProvider(ctx, params.Provider, false)
	if err != nil {
		return protocol.ProviderInfo{}, err
	}
	if err := g.store.removeKey(params.Key, p.ID); err != nil {
		return protocol.ProviderInfo{}, err
	}
	return g.providerInfo(ctx, p)
}

// EnableProvider changes whether a provider participates in routing.
func (g *Gateway) EnableProvider(ctx context.Context, params protocol.EnableProviderParams) (protocol.ProviderInfo, error) {
	p, err := g.findProvider(ctx, params.Provider, false)
	if err != nil {
		return protocol.ProviderInfo{}, err
	}
	if err := g.store.setProviderEnabled(p.ID, params.Enabled); err != nil {
		return protocol.ProviderInfo{}, err
	}
	return g.providerInfo(ctx, p)
}

// Test runs a real one-token completion on the provider's best tool model. A
// local provider's server is asked for its models first.
func (g *Gateway) Test(ctx context.Context, providerID string) (protocol.ProviderTestResult, error) {
	p, err := g.findProvider(ctx, providerID, true)
	if err != nil {
		return protocol.ProviderTestResult{}, err
	}
	if p.Tier == protocol.TierLocal && !p.running {
		return protocol.ProviderTestResult{Provider: p.ID, Message: i18n.Tf("Not running at %s.", p.BaseURL)}, nil
	}
	if p.Keyless {
		return g.testKey(ctx, p, "")
	}
	records, err := g.store.listKeys(p.ID)
	if err != nil {
		return protocol.ProviderTestResult{}, err
	}
	if len(records) == 0 {
		return protocol.ProviderTestResult{Provider: p.ID, Message: i18n.T("Add a provider key first")}, nil
	}
	var last protocol.ProviderTestResult
	for _, record := range records {
		last, err = g.testKey(ctx, p, record.ID)
		if err != nil {
			return last, err
		}
		if last.OK {
			return last, nil
		}
	}
	return last, nil
}

func (g *Gateway) testKey(ctx context.Context, p *provider, keyID string) (protocol.ProviderTestResult, error) {
	defer g.notifyChange()
	m, ok := bestToolModel(p)
	if !ok {
		return protocol.ProviderTestResult{Provider: p.ID, Message: i18n.T("Provider has no tool-capable chat model")}, nil
	}
	started := g.now()
	result := protocol.ProviderTestResult{Provider: p.ID, Model: m.ID}
	stream, status, body, err := g.startAttempt(ctx, routeCandidate{provider: p, model: m, keyID: keyID}, probeRequest(m.ID))
	if err != nil {
		if keyID != "" {
			_ = g.applyHealthQuiet(keyID, status, body, false)
		}
		result.Message = g.redactKeyMessage(keyID, safeProviderError(status, body, err))
		return result, nil
	}
	defer stream.Close()
	produced := false
	for {
		delta, nextErr := stream.Next()
		err = nextErr
		if delta.Text != "" || delta.Reasoning != "" || len(delta.ToolCalls) > 0 {
			produced = true
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			result.Message = i18n.T("Provider test timed out")
			return result, nil
		}
		if err != nil {
			break
		}
	}
	if !errors.Is(err, io.EOF) {
		result.Message = g.redactKeyMessage(keyID, safeProviderError(0, "", err))
		return result, nil
	}
	// a reasoning model can spend the one token thinking: stopping at the limit is still an answer
	if !produced && stream.finish != llm.FinishLength {
		result.Message = i18n.T("Provider returned no completion content")
		return result, nil
	}
	result.OK = true
	result.LatencyMs = g.now().Sub(started).Milliseconds()
	result.Message = i18n.T("Provider completed a real chat request")
	if keyID != "" {
		_ = g.applyHealthQuiet(keyID, 200, "", true)
	}
	return result, nil
}

// bestToolModel is the model a provider is tested on: the best tool model it
// serves without billing, or its best paid one when it has nothing else. A
// key on a free plan is refused a paid model, which says nothing of the key.
func bestToolModel(p *provider) (*model, bool) {
	var models []*model
	for i := range p.Models {
		if p.Models[i].Tools {
			models = append(models, &p.Models[i])
		}
	}
	if len(models) == 0 {
		return nil, false
	}
	sort.Slice(models, func(i, j int) bool {
		left, right := p.tierOf(models[i]) == protocol.TierPaid, p.tierOf(models[j]) == protocol.TierPaid
		if left != right {
			return right
		}
		return models[i].QualityRank > models[j].QualityRank
	})
	return models[0], true
}

func probeRequest(modelID string) chatRequest {
	return chatRequest{Model: modelID, Messages: []wireMessage{{Role: "user", Content: "Reply with OK."}}, Tools: []wireTool{{Type: "function", Function: wireFunction{Name: "noop", Description: "No operation", Parameters: []byte(`{"type":"object","properties":{}}`)}}}, ToolChoice: "auto", MaxTokens: 1, Stream: true, StreamOptions: wireStreamOptions{IncludeUsage: true}}
}

func (g *Gateway) applyHealth(keyID string, status int, body string, success bool) error {
	return g.updateHealth(keyID, status, body, success, true)
}

func (g *Gateway) applyHealthQuiet(keyID string, status int, body string, success bool) error {
	return g.updateHealth(keyID, status, body, success, false)
}

func (g *Gateway) updateHealth(keyID string, status int, body string, success, notify bool) error {
	secret, record, err := g.store.reveal(keyID)
	if err != nil {
		return err
	}
	oldStatus := record.Status
	record.Checked = g.now().UTC()
	if success {
		record.Status = protocol.KeyHealthy
		record.LastError = ""
	} else if invalidCredential(status, body) {
		record.Status = protocol.KeyError
		record.LastError = safeProviderError(status, body, nil)
		if secret != "" {
			record.LastError = strings.ReplaceAll(record.LastError, secret, "[redacted]")
		}
	} else {
		return nil
	}
	if err := g.store.updateKey(record); err != nil {
		return err
	}
	if notify && record.Status != oldStatus {
		g.notifyChange()
	}
	return nil
}

func (g *Gateway) redactKeyMessage(keyID, message string) string {
	if keyID == "" || message == "" {
		return message
	}
	secret, _, err := g.store.reveal(keyID)
	if err == nil && secret != "" {
		return strings.ReplaceAll(message, secret, "[redacted]")
	}
	return message
}

// Redact replaces every form of every stored provider key, and of the local
// API's key, in text with its name, such as ‹OPENROUTER_API_KEY›.
func (g *Gateway) Redact(text string) string {
	if text == "" {
		return text
	}
	return g.Scrubber()(text)
}

// Scrubber opens every stored key once and returns what Redact does with
// them, for a pass over many texts. The daemon registers it with the redact
// package; it holds the keys, so it is dropped when the pass ends.
func (g *Gateway) Scrubber() func(string) string {
	return vault.ValuesScrubber(g.store.secrets())
}

// Models returns every chat model with its tier, the local ones a running
// server serves first, and the three routing choices, which stay free.
func (g *Gateway) Models(ctx context.Context, providerID string) (protocol.ModelsResult, error) {
	result := protocol.ModelsResult{Routes: []protocol.Choice{
		{ID: protocol.RouteAuto, Name: i18n.T("Auto"), Description: i18n.T("Best free model available, one on this computer first")},
		{ID: protocol.RouteFast, Name: i18n.T("Fast"), Description: i18n.T("Free model with the lowest measured time to first token")},
		{ID: protocol.RouteSmart, Name: i18n.T("Smart"), Description: i18n.T("Free model with the highest quality rank")},
	}}
	var providers []*provider
	if strings.TrimSpace(providerID) == "" {
		providers = g.providers(ctx, false)
	} else {
		p, err := g.findProvider(ctx, providerID, false)
		if err != nil {
			return protocol.ModelsResult{}, err
		}
		providers = []*provider{p}
	}
	for _, p := range providers {
		enabled, err := g.store.providerEnabled(p.ID)
		if err != nil {
			return protocol.ModelsResult{}, err
		}
		usable, err := g.hasUsableKey(p)
		if err != nil {
			return protocol.ModelsResult{}, err
		}
		_, providerCooling, err := g.store.cooldown(providerScope(p.ID), g.now())
		if err != nil {
			return protocol.ModelsResult{}, err
		}
		for i := range p.Models {
			m := &p.Models[i]
			until, modelCooling, err := g.store.cooldown(modelScope(p.ID, m.ID), g.now())
			if err != nil {
				return protocol.ModelsResult{}, err
			}
			info := protocol.ModelInfo{ID: p.ID + "/" + m.ID, Provider: p.ID, Name: m.Name, Tier: p.tierOf(m), Context: m.Context, Tools: m.Tools, Reasoning: m.Reasoning, Vision: m.Vision, Rank: m.QualityRank, Available: enabled && usable && !providerCooling && !modelCooling}
			if modelCooling {
				info.CooldownUntil = &until
			}
			result.Models = append(result.Models, info)
		}
	}
	return result, nil
}

func (g *Gateway) hasUsableKey(p *provider) (bool, error) {
	if p.Keyless {
		return true, nil
	}
	records, err := g.store.listKeys(p.ID)
	if err != nil {
		return false, err
	}
	for _, record := range records {
		if record.Status == protocol.KeyError {
			continue
		}
		if _, cooling, err := g.store.cooldown(keyScope(record.ID), g.now()); err != nil {
			return false, err
		} else if !cooling {
			return true, nil
		}
	}
	return false, nil
}

// Usage returns persisted daily usage totals.
func (g *Gateway) Usage(_ context.Context, days int) (protocol.GatewayUsage, error) {
	if days <= 0 {
		days = 7
	}
	rows, err := g.store.usage(days, g.now())
	return protocol.GatewayUsage{Days: days, Rows: rows}, err
}

// Status summarizes capacity available at this moment.
func (g *Gateway) Status(ctx context.Context) protocol.GatewayStatus {
	providers, err := g.Providers(ctx)
	if err != nil {
		return protocol.GatewayStatus{}
	}
	var status protocol.GatewayStatus
	for _, p := range providers {
		if p.Enabled && p.Available > 0 {
			status.Providers++
			status.Models += p.Available
			if p.Keyless {
				status.Keyless++
			}
		}
	}
	return status
}

// Close waits for background key probes and closes the database.
func (g *Gateway) Close() error {
	var err error
	g.close.Do(func() {
		g.cancel()
		g.wg.Wait()
		if transport, ok := g.client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
		err = g.store.db.Close()
	})
	return err
}

func providerScope(providerID string) string       { return "provider:" + providerID }
func modelScope(providerID, modelID string) string { return "model:" + providerID + "/" + modelID }
func keyScope(keyID string) string                 { return "key:" + keyID }

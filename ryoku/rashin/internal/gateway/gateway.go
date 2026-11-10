// Package gateway provides Rashin's in-process free-model router.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const (
	maxAttempts     = 6
	firstByteBudget = 45 * time.Second
)

// Gateway is Rashin's provider catalogue, credential vault and chat router.
type Gateway struct {
	catalog catalog
	store   *store
	client  *http.Client
	now     func() time.Time

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	close    sync.Once
	changeMu sync.RWMutex
	onChange func()

	apiMu  sync.Mutex
	apiKey string
}

// Open opens the gateway database and embedded free-provider catalogue.
func Open(stateDir string) (*Gateway, error) {
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
		now:    time.Now,
		ctx:    ctx,
		cancel: cancel,
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

// Providers returns catalogue and local key state without exposing a secret.
func (g *Gateway) Providers(ctx context.Context) ([]protocol.ProviderInfo, error) {
	infos := make([]protocol.ProviderInfo, 0, len(g.catalog.Providers))
	for i := range g.catalog.Providers {
		info, err := g.providerInfo(ctx, &g.catalog.Providers[i])
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
	p, ok := g.catalog.findProvider(params.Provider)
	if !ok {
		return protocol.ProviderInfo{}, fmt.Errorf(i18n.T("Provider is not in Rashin's free-model catalogue: %s"), params.Provider)
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
	p, ok := g.catalog.findProvider(params.Provider)
	if !ok {
		return protocol.ProviderInfo{}, fmt.Errorf(i18n.T("Provider is not in Rashin's free-model catalogue: %s"), params.Provider)
	}
	if err := g.store.removeKey(params.Key, p.ID); err != nil {
		return protocol.ProviderInfo{}, err
	}
	return g.providerInfo(ctx, p)
}

// EnableProvider changes whether a provider participates in routing.
func (g *Gateway) EnableProvider(ctx context.Context, params protocol.EnableProviderParams) (protocol.ProviderInfo, error) {
	p, ok := g.catalog.findProvider(params.Provider)
	if !ok {
		return protocol.ProviderInfo{}, fmt.Errorf(i18n.T("Provider is not in Rashin's free-model catalogue: %s"), params.Provider)
	}
	if err := g.store.setProviderEnabled(p.ID, params.Enabled); err != nil {
		return protocol.ProviderInfo{}, err
	}
	return g.providerInfo(ctx, p)
}

// Test runs a real one-token completion on the provider's best tool model.
func (g *Gateway) Test(ctx context.Context, providerID string) (protocol.ProviderTestResult, error) {
	p, ok := g.catalog.findProvider(providerID)
	if !ok {
		return protocol.ProviderTestResult{}, fmt.Errorf(i18n.T("Provider is not in Rashin's free-model catalogue: %s"), providerID)
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
	if !produced {
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
	sort.Slice(models, func(i, j int) bool { return models[i].QualityRank > models[j].QualityRank })
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

// Models returns the curated free chat catalogue and the three routing choices.
func (g *Gateway) Models(_ context.Context, providerID string) (protocol.ModelsResult, error) {
	providerID = strings.ToLower(strings.TrimSpace(providerID))
	result := protocol.ModelsResult{Routes: []protocol.Choice{
		{ID: protocol.RouteAuto, Name: i18n.T("Auto"), Description: i18n.T("Best available free model")},
		{ID: protocol.RouteFast, Name: i18n.T("Fast"), Description: i18n.T("Lowest measured time to first token")},
		{ID: protocol.RouteSmart, Name: i18n.T("Smart"), Description: i18n.T("Highest quality rank")},
	}}
	for i := range g.catalog.Providers {
		p := &g.catalog.Providers[i]
		if providerID != "" && p.ID != providerID {
			continue
		}
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
		for _, m := range p.Models {
			until, modelCooling, err := g.store.cooldown(modelScope(p.ID, m.ID), g.now())
			if err != nil {
				return protocol.ModelsResult{}, err
			}
			info := protocol.ModelInfo{ID: p.ID + "/" + m.ID, Provider: p.ID, Name: m.Name, Context: m.Context, Tools: m.Tools, Reasoning: m.Reasoning, Vision: m.Vision, Rank: m.QualityRank, Available: enabled && usable && !providerCooling && !modelCooling}
			if modelCooling {
				info.CooldownUntil = &until
			}
			result.Models = append(result.Models, info)
		}
	}
	if providerID != "" {
		if _, ok := g.catalog.findProvider(providerID); !ok {
			return protocol.ModelsResult{}, fmt.Errorf(i18n.T("Provider is not in Rashin's free-model catalogue: %s"), providerID)
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

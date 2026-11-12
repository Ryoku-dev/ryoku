package memory

import (
	"context"
	"slices"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// learnModel is the model a conversation is learned with: a provider/model
// or a route, whether it runs on this computer, and its context in tokens (0
// when unknown).
type learnModel struct {
	Model   string
	Local   bool
	Context int
}

// defaultContext is the context assumed for a model that does not say.
const defaultContext = 8192

// inputShare is the part of the model's context the conversation may take.
const inputShare = 0.7

// budget is how many bytes of conversation the model is given.
func (m learnModel) budget() int {
	context := m.Context
	if context <= 0 {
		context = defaultContext
	}
	return int(float64(context)*inputShare) * 4
}

// pickLocal is the first model of the first local provider that is enabled,
// running and lists an available model, in protocol.LocalProviders order.
func pickLocal(ctx context.Context, models func(context.Context) []protocol.ProviderInfo, providerModels func(context.Context, string) []protocol.ModelInfo) (protocol.ModelInfo, bool) {
	if models == nil || providerModels == nil {
		return protocol.ModelInfo{}, false
	}
	providers := models(ctx)
	for _, id := range protocol.LocalProviders {
		index := slices.IndexFunc(providers, func(p protocol.ProviderInfo) bool { return p.ID == id })
		if index < 0 || !providers[index].Enabled || !providers[index].Running || providers[index].Models == 0 {
			continue
		}
		for _, model := range providerModels(ctx, id) {
			if model.Available {
				return model, true
			}
		}
	}
	return protocol.ModelInfo{}, false
}

// pickModel is memory's one policy for which model learns, per choice
// (memory.model): a local model first; then, unless choice is local only,
// the model that answered the conversation when the gateway has its
// provider, or Ask's own route under LearnAny. When none fits, the model is
// empty and the reason says why learning waits. Hindsight's learning model
// is chosen with it too.
func pickModel(ctx context.Context, choice, answered string, models func(context.Context) []protocol.ProviderInfo, providerModels func(context.Context, string) []protocol.ModelInfo) (learnModel, string) {
	if local, ok := pickLocal(ctx, models, providerModels); ok {
		return learnModel{Model: local.ID, Local: true, Context: local.Context}, ""
	}
	switch choice {
	case protocol.LearnLocal:
		return learnModel{}, i18n.T("Waiting for a model on this computer: start Ollama or llama.cpp.")
	case protocol.LearnAny:
		return learnModel{Model: protocol.RouteAuto}, ""
	}
	if model, ok := gatewayModel(ctx, answered, models, providerModels); ok {
		return model, ""
	}
	if answered == "" {
		return learnModel{}, i18n.T("Waiting for a model on this computer, since the conversation does not say which model answered it.")
	}
	return learnModel{}, i18n.Tf("Waiting for a model on this computer, since Rashin's router does not have %s, which answered the conversation.", answered)
}

// chooseModel is pickModel with the engine's settings and providers.
func (e *Engine) chooseModel(ctx context.Context, answered string) (learnModel, string) {
	return pickModel(ctx, e.settings().Model, answered, e.deps.Models, e.deps.ProviderModels)
}

// gatewayModel is model as the gateway serves it, when the gateway has its
// provider in use.
func gatewayModel(ctx context.Context, model string, models func(context.Context) []protocol.ProviderInfo, providerModels func(context.Context, string) []protocol.ModelInfo) (learnModel, bool) {
	provider, _, ok := strings.Cut(model, "/")
	if !ok || provider == "" || models == nil {
		return learnModel{}, false
	}
	providers := models(ctx)
	index := slices.IndexFunc(providers, func(p protocol.ProviderInfo) bool { return p.ID == provider })
	if index < 0 || !providers[index].Enabled || !(providers[index].Configured || providers[index].Keyless) {
		return learnModel{}, false
	}
	chosen := learnModel{Model: model, Local: providers[index].Local}
	if providerModels != nil {
		for _, info := range providerModels(ctx, provider) {
			if info.ID == model {
				chosen.Context = info.Context
			}
		}
	}
	return chosen, true
}

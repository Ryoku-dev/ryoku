package memory

import (
	"context"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
	"slices"
	"strings"
)

// learnModel is a learning route, its local-only restriction, and the
// model's known context in tokens (0 when the route has not resolved yet).
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

// localAvailable includes installed models whose Rashin-owned server is resting.
func localAvailable(ctx context.Context, providers []protocol.ProviderInfo, providerModels func(context.Context, string) []protocol.ModelInfo) bool {
	if providerModels == nil {
		return false
	}
	for _, id := range protocol.LocalProviders {
		index := slices.IndexFunc(providers, func(p protocol.ProviderInfo) bool { return p.ID == id })
		if index < 0 || !providers[index].Enabled || !(providers[index].Running || providers[index].OnDemand) || providers[index].Models == 0 {
			continue
		}
		for _, model := range providerModels(ctx, id) {
			if model.Available {
				return true
			}
		}
	}
	return false
}

// pickModel keeps learning on the same remote model only when no local model
// can answer, so an automatic route never sends an earlier answer elsewhere.
func pickModel(ctx context.Context, choice, answered string, models func(context.Context) []protocol.ProviderInfo, providerModels func(context.Context, string) []protocol.ModelInfo) (learnModel, string) {
	if choice == protocol.LearnAny {
		return learnModel{Model: protocol.RouteAuto}, ""
	}
	var providers []protocol.ProviderInfo
	if models != nil {
		providers = models(ctx)
	}
	if localAvailable(ctx, providers, providerModels) {
		return learnModel{Model: protocol.RouteAuto, Local: true}, ""
	}
	waiting := i18n.T("Waiting for a Hyakki model on this computer: install one in Hyakki, or start Ollama or llama.cpp.")
	if choice == protocol.LearnLocal || answered == "" {
		return learnModel{}, waiting
	}
	provider, _, ok := strings.Cut(answered, "/")
	if !ok {
		return learnModel{}, waiting
	}
	if slices.Contains(protocol.LocalProviders, provider) {
		return learnModel{Model: answered, Local: true}, ""
	}
	if providerModels == nil {
		return learnModel{}, waiting
	}
	index := slices.IndexFunc(providers, func(p protocol.ProviderInfo) bool { return p.ID == provider })
	if index < 0 || !providers[index].Enabled {
		return learnModel{}, waiting
	}
	for _, candidate := range providerModels(ctx, provider) {
		if candidate.ID != answered {
			continue
		}
		tier := candidate.Tier
		if tier == "" {
			tier = providers[index].Tier
		}
		if candidate.Available && (tier == protocol.TierFree || tier == protocol.TierCredits) {
			return learnModel{Model: answered, Context: candidate.Context}, ""
		}
		break
	}
	return learnModel{}, waiting
}

// hindsightModel supplies a concrete local model when Hindsight requires
// local-only learning: its HTTP gateway cannot carry llm.Request.LocalOnly.
func hindsightModel(ctx context.Context, choice, answered string, models func(context.Context) []protocol.ProviderInfo, providerModels func(context.Context, string) []protocol.ModelInfo) (learnModel, string) {
	if choice != protocol.LearnLocal {
		return pickModel(ctx, choice, answered, models, providerModels)
	}
	if models != nil && providerModels != nil {
		providers := models(ctx)
		var best protocol.ModelInfo
		for _, id := range protocol.LocalProviders {
			index := slices.IndexFunc(providers, func(p protocol.ProviderInfo) bool { return p.ID == id })
			if index < 0 || !providers[index].Enabled || !(providers[index].Running || providers[index].OnDemand) || providers[index].Models == 0 {
				continue
			}
			for _, candidate := range providerModels(ctx, id) {
				if !candidate.Available || !candidate.Tools {
					continue
				}
				light := candidate.Light || candidate.Params <= protocol.LightParams
				bestLight := best.Light || best.Params <= protocol.LightParams
				if best.ID == "" || light && !bestLight || light == bestLight && candidate.Params < best.Params {
					best = candidate
				}
			}
		}
		if best.ID != "" {
			return learnModel{Model: best.ID, Local: true, Context: best.Context}, ""
		}
	}
	return learnModel{}, i18n.T("Waiting for a Hyakki model on this computer: install one in Hyakki, or start Ollama or llama.cpp.")
}

// chooseModel is pickModel with the engine's settings and providers.
func (e *Engine) chooseModel(ctx context.Context, answered string) (learnModel, string) {
	return pickModel(ctx, e.settings().Model, answered, e.deps.Models, e.deps.ProviderModels)
}

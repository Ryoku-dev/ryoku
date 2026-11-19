package teams

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// modelCheckTimeout bounds the model listing a task start waits on.
const modelCheckTimeout = 10 * time.Second

// SetModels hands the service the gateway's model listing, which a task's
// model must resolve in before its session starts; the daemon calls it once,
// before Bind.
func (s *Service) SetModels(models func(ctx context.Context, p protocol.ModelsParams) (protocol.ModelsResult, error)) {
	s.models = models
}

// taskEngine is the model and options a task starts with: its member's
// engine through the teams defaults and the Agents model
// (protocol.ResolveEngine), then the model asked for this task alone and its
// options, each one over the member's.
func (s *Service) taskEngine(member protocol.Member, task protocol.TeamTask) (string, map[string]string) {
	model, options := protocol.ResolveEngine(member, s.settings())
	if task.Model != "" {
		model = task.Model
	}
	options = maps.Clone(options)
	if len(task.Options) > 0 {
		if options == nil {
			options = make(map[string]string, len(task.Options))
		}
		maps.Copy(options, task.Options)
	}
	return model, options
}

// modelProblem says why a model cannot run now. The gateway's listing is
// authoritative: otherwise a misspelled model or a failed listing would
// silently create a task that cannot do any work.
func (s *Service) modelProblem(ctx context.Context, model string) string {
	problem, _ := s.modelStatus(ctx, model)
	return problem
}

func (s *Service) modelStatus(ctx context.Context, model string) (string, string) {
	if model == "" || !protocol.ValidEngineModel(model) {
		return i18n.Tf("%q is not a model Rashin can run: give a route such as auto, or a provider's model such as provider/model.", model), ""
	}
	if s.models == nil {
		return i18n.T("The model list is unavailable. Retry the task after Rashin reconnects to its providers."), ""
	}
	ctx, cancel := context.WithTimeout(ctx, modelCheckTimeout)
	defer cancel()
	listed, err := s.models(ctx, protocol.ModelsParams{})
	if err != nil {
		return i18n.Tf("Rashin could not check which models are ready: %v.", err), ""
	}
	usable := func(m protocol.ModelInfo) bool { return m.Available || m.CooldownUntil != nil }
	if protocol.IsRoute(model) {
		if slices.ContainsFunc(listed.Models, func(m protocol.ModelInfo) bool {
			return usable(m) && m.Tools && (m.Tier == protocol.TierFree || m.Tier == protocol.TierCredits || m.Tier == protocol.TierLocal)
		}) {
			return "", ""
		}
		return i18n.Tf("No model is ready for %s: Rashin has no provider key or local model to use.", s.engineWords(model)), ""
	}
	provider, _, _ := strings.Cut(model, "/")
	known, open := false, false
	for _, m := range listed.Models {
		if m.ID == model {
			if usable(m) && m.ToolsKnown && !m.Tools {
				return i18n.Tf("%s cannot use tools, which a team task needs.", model), ""
			}
			if usable(m) {
				return "", m.Tier
			}
			return i18n.Tf("%s is turned off or has no key.", model), ""
		}
		if m.Provider == provider {
			known, open = true, open || usable(m)
		}
	}
	switch {
	case !known:
		return i18n.Tf("Rashin has no provider %s for %s.", provider, model), ""
	case !open:
		return i18n.Tf("%s is turned off or has no key.", model), ""
	}
	return i18n.Tf("Rashin could not find %s among %s's models.", model, provider), ""
}

// checkEngine says what is wrong with a member's engine: a model id that is
// neither a route nor a provider's model, or options past the limits.
func checkEngine(m protocol.Member) error {
	if !protocol.ValidEngineModel(m.Model) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A model is a route such as auto, or a provider's model such as provider/model, without spaces."))
	}
	if !protocol.ValidEngineOptions(m.Options) {
		return protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("A model takes at most %d options, each with an id and a value.", protocol.EngineOptionsMax))
	}
	return nil
}

// engineWords says a model the way the ledger reads: a route by its name,
// any other model by its id, "the teams default" when it names none.
func (s *Service) engineWords(model string) string {
	switch model {
	case "":
		return i18n.T("the teams default")
	case protocol.RouteAuto:
		return i18n.T("Auto")
	case protocol.RouteFast:
		return i18n.T("Fast")
	case protocol.RouteSmart:
		return i18n.T("Smart")
	}
	return model
}

// memberEngineWords says a member's engine with its options, as a team
// change records it.
func (s *Service) memberEngineWords(m protocol.Member) string {
	words := s.engineWords(m.Model)
	if len(m.Options) == 0 {
		return words
	}
	parts := make([]string, 0, len(m.Options))
	for _, option := range slices.Sorted(maps.Keys(m.Options)) {
		parts = append(parts, option+" "+m.Options[option])
	}
	return i18n.Tf("%s (%s)", words, strings.Join(parts, ", "))
}

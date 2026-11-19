package teamsui

import (
	"maps"
	"slices"
	"strings"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the model a member or a task runs on, in words, kept in parts so a tight row shows only what fits
type Engine struct {
	// the route or provider/model the daemon takes
	ID string
	// "LFM3 2B", "Auto"; the id's own name while no list names it
	Model string
	// the words after the name that say where it runs, "on this computer"; empty for a route
	Where string
	// the options in words, "effort high, plans first"; empty without any
	Extra   string
	Options map[string]string
	// it follows the default, not a pick of its own
	Default bool
	// nothing can run it now: its provider is gone, off or without a key, its server is down, or it calls no tools
	Missing bool

	place string
}

// "LFM3 2B on this computer, effort high"
func (e Engine) Line() string {
	if e.Extra == "" {
		return e.place
	}
	return i18n.Tf("%s, %s", e.place, e.Extra)
}

// "LFM3 2B on this computer", for a line with no room for the options
func (e Engine) Place() string { return e.place }

// the model alone, for rows with no room for where it runs
func (e Engine) Short() string { return e.Model }

// what a member's next task starts on, through the chain the daemon starts it with
func MemberEngine(snapshot state.Snapshot, member protocol.Member) Engine {
	model, options := protocol.ResolveEngine(member, snapshot.Settings)
	engine := ModelEngine(snapshot, model, options)
	engine.Default = member.Model == ""
	return engine
}

// a started task keeps what it ran on even after its session and member are gone; before
// it starts it reads as its member's engine with what was asked for this task alone
func TaskEngine(snapshot state.Snapshot, task protocol.TeamTask) Engine {
	model, options := TaskEngineIDs(snapshot, task)
	engine := ModelEngine(snapshot, model, options)
	engine.Default = task.Model == "" && taskMember(snapshot, task).Model == ""
	return engine
}

// the same chain by the ids the daemon takes, for details that show a model's own id
func TaskEngineIDs(snapshot state.Snapshot, task protocol.TeamTask) (model string, options map[string]string) {
	model, options = protocol.ResolveEngine(taskMember(snapshot, task), snapshot.Settings)
	if task.Model != "" {
		model = task.Model
	}
	if len(task.Options) > 0 {
		options = task.Options
	}
	return model, options
}

// what a member that names no model runs: the teams default, else what Agents use
func DefaultEngine(snapshot state.Snapshot) Engine {
	return MemberEngine(snapshot, protocol.Member{})
}

// what Agents use, which the teams default falls back to
func AgentsEngine(snapshot state.Snapshot) Engine {
	model := snapshot.Settings.Agent.Model
	engine := ModelEngine(snapshot, model, snapshot.Settings.Agent.Options)
	engine.Default = model == ""
	return engine
}

// any model in words by the id the daemon takes; an empty one is the route every chain ends on
func ModelEngine(snapshot state.Snapshot, model string, options map[string]string) Engine {
	if model == "" {
		model = protocol.RouteAuto
	}
	engine := Engine{ID: model, Options: options, Extra: optionWords(options), place: env.ModelWordsOf(model)}
	info, listed := env.ModelInfoOf(model)
	// before the daemon said what it has, nothing is known to be missing
	ready := snapshot.Connection == state.Ready
	switch {
	case protocol.IsRoute(model):
		engine.Model = engine.place
		engine.Missing = ready && !env.AgentModelsReadyOf()
	case listed && info.Name != "":
		engine.Model = info.Name
		engine.Missing = !info.Available || info.ToolsKnown && !info.Tools
	default:
		// a model nothing lists reads by its own id, without the provider that leads it
		_, engine.Model, _ = strings.Cut(model, "/")
		engine.Missing = ready && (env.ModelsLoaded() || !providerUp(snapshot, model))
	}
	if where, cut := strings.CutPrefix(engine.place, engine.Model); cut {
		engine.Where = strings.TrimSpace(where)
	}
	return engine
}

// the provider a model id names is on, has what it needs to answer, and a server of its own is up or starts on demand
func providerUp(snapshot state.Snapshot, model string) bool {
	id, _, _ := strings.Cut(model, "/")
	for _, provider := range snapshot.Providers {
		if provider.ID != id {
			continue
		}
		if provider.Local {
			return provider.Enabled && (provider.Running || provider.OnDemand)
		}
		return provider.Enabled && (provider.Keyless || provider.Configured)
	}
	return false
}

// how hard it thinks first, then whether it plans, then any other option by id
func optionWords(options map[string]string) string {
	if len(options) == 0 {
		return ""
	}
	ids := slices.Sorted(maps.Keys(options))
	slices.SortStableFunc(ids, func(a, b string) int { return optionRank(a) - optionRank(b) })
	words := make([]string, 0, len(ids))
	for _, id := range ids {
		if word := optionWord(id, options[id]); word != "" {
			words = append(words, word)
		}
	}
	return strings.Join(words, ", ")
}

func optionRank(id string) int {
	switch id {
	case protocol.OptionEffort:
		return 0
	case protocol.OptionPlan:
		return 1
	}
	return 2
}

// an option inside a sentence; a plan that is off says nothing
func optionWord(id, value string) string {
	switch id {
	case protocol.OptionEffort:
		switch value {
		case effortOff:
			return i18n.T("thinking off")
		case effortLow:
			return i18n.T("effort low")
		case effortMedium:
			return i18n.T("effort medium")
		case effortHigh:
			return i18n.T("effort high")
		}
	case protocol.OptionPlan:
		if value == protocol.OptionOn {
			return i18n.T("plans first")
		}
		return ""
	}
	return i18n.Tf("%s %s", id, value)
}

// the values of protocol.OptionEffort, as the engine sends them to a model that reasons
const (
	effortOff    = "off"
	effortLow    = "low"
	effortMedium = "medium"
	effortHigh   = "high"
)

// one option as a fact with its label, for a details list: "Thinking" and "High", "Plan first" and "On"
func OptionFact(id, value string) (name, word string) {
	switch id {
	case protocol.OptionEffort:
		name = i18n.T("Thinking")
		switch value {
		case effortOff:
			return name, i18n.T("Off")
		case effortLow:
			return name, i18n.T("Low")
		case effortMedium:
			return name, i18n.T("Medium")
		case effortHigh:
			return name, i18n.T("High")
		}
		return name, value
	case protocol.OptionPlan:
		if value == protocol.OptionOn {
			return i18n.T("Plan first"), i18n.T("On")
		}
		return i18n.T("Plan first"), i18n.T("Off")
	}
	return id, value
}

// a choice's name as its list gives it, else its id
func ChoiceName(choices []protocol.Choice, id string) string {
	for _, choice := range choices {
		if choice.ID == id && choice.Name != "" {
			return choice.Name
		}
	}
	return id
}

// a task without a member goes to the lead; a removed member leaves the zero member, which follows the defaults
func taskMember(snapshot state.Snapshot, task protocol.TeamTask) protocol.Member {
	team, _ := TeamByID(snapshot.Teams, task.Team)
	for _, member := range team.Members {
		if member.ID == task.Member || task.Member == "" && member.Lead {
			return member
		}
	}
	return protocol.Member{}
}

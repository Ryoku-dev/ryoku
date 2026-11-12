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

// the harness and model a member or a task runs on, in words, kept in parts so a tight row shows only what fits
type Engine struct {
	// "Claude Code", "Claude Code (default)" when it comes from the defaults, ", not installed" added when missing
	Harness string
	// empty when unknown
	Version string
	// "Haiku 4.5", "Sonnet 4.5 (its default)" or "Its default model"
	Model string
	// the options a member or task sets, as "effort high", empty without any
	Extra   string
	Missing bool
}

// "Haiku 4.5 on Claude Code, effort high"
func (e Engine) Line() string {
	line := i18n.Tf("%s on %s", e.Model, e.Harness)
	if e.Extra != "" {
		line = i18n.Tf("%s, %s", line, e.Extra)
	}
	return line
}

// the model alone, for rows with no room for the harness
func (e Engine) Short() string { return e.Model }

// what a member's next task starts on, through the chain the daemon starts it with
func MemberEngine(snapshot state.Snapshot, member protocol.Member) Engine {
	harness, model, options := protocol.ResolveEngine(member, snapshot.Settings, registryDefault(snapshot.Harnesses))
	return describeEngine(snapshot.Harnesses, harness, model, options, member.Harness == "")
}

// a started task keeps what it ran on even after its session and member are gone; before
// it starts it reads as its member's engine with what was asked for this task alone
func TaskEngine(snapshot state.Snapshot, task protocol.TeamTask) Engine {
	harness, model, options := TaskEngineIDs(snapshot, task)
	fromDefaults := task.Harness == "" && taskMember(snapshot, task).Harness == ""
	return describeEngine(snapshot.Harnesses, harness, model, options, fromDefaults)
}

// the same chain by the ids the harness takes, for details that show a model's own id
func TaskEngineIDs(snapshot state.Snapshot, task protocol.TeamTask) (harness, model string, options map[string]string) {
	if task.Harness != "" {
		return task.Harness, task.Model, task.Options
	}
	harness, model, options = protocol.ResolveEngine(taskMember(snapshot, task), snapshot.Settings, registryDefault(snapshot.Harnesses))
	if task.Model != "" {
		model = task.Model
	}
	if len(task.Options) > 0 {
		options = task.Options
	}
	return harness, model, options
}

func describeEngine(harnesses []protocol.HarnessInfo, id, model string, options map[string]string, fromDefaults bool) Engine {
	info, found := env.HarnessByID(harnesses, id)
	engine := Engine{Missing: !found || !info.Installed}
	switch {
	case id == "":
		// the chain ends empty only when no harness is installed at all
		engine.Harness = i18n.T("No harness installed")
	case found:
		engine.Harness, engine.Version = info.Name, info.Version
	default:
		engine.Harness = id
	}
	if id != "" && fromDefaults {
		engine.Harness = i18n.Tf("%s (default)", engine.Harness)
	}
	if id != "" && engine.Missing {
		engine.Harness = i18n.Tf("%s, not installed", engine.Harness)
	}
	engine.Model = modelWords(info, model)
	engine.Extra = optionWords(info.Capabilities, options)
	return engine
}

// an empty model is the harness's own default, named when the harness reported it or its settings say it
func modelWords(info protocol.HarnessInfo, model string) string {
	if model != "" {
		return env.ModelName(info.Capabilities, model)
	}
	if model := env.HarnessDefaultModel(info); model != "" {
		return i18n.Tf("%s (its default)", env.ModelName(info.Capabilities, model))
	}
	return i18n.T("Its default model")
}

// options read in the order the harness offers them, any it no longer offers after them by id
func optionWords(capabilities *protocol.HarnessCapabilities, options map[string]string) string {
	if len(options) == 0 {
		return ""
	}
	words := make([]string, 0, len(options))
	named := make(map[string]bool, len(options))
	if capabilities != nil {
		for _, option := range capabilities.Options {
			if value, ok := options[option.ID]; ok {
				words = append(words, optionWord(option.Name, option.Choices, value))
				named[option.ID] = true
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(options)) {
		if !named[id] {
			words = append(words, optionWord(id, nil, options[id]))
		}
	}
	return strings.Join(words, ", ")
}

func optionWord(name string, choices []protocol.Choice, value string) string {
	return i18n.Tf("%s %s", strings.ToLower(name), ChoiceName(choices, value))
}

// the harness the registry starts when nothing names one, marked Default in the list
func registryDefault(harnesses []protocol.HarnessInfo) string {
	for _, harness := range harnesses {
		if harness.Default {
			return harness.ID
		}
	}
	return ""
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

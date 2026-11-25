package settings

import (
	"maps"
	"slices"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the settings that apply as they change and have a default to go back to, each true while it has left it;
// what a group saves on its own bar, a list or a secret has no single default and is not here
type changes struct {
	askModel, askApprovals, habits, resume                                          bool
	agentModel, plan, agentApprovals, idle, notices                                 bool
	system, semantic                                                                bool
	paused, wake, teamApprovals, teamNotify, keep                                   bool
	memoryUse, memoryLearn, sessions, agents, sensitive, suggestions, learner, fade bool
	apiEnabled, port, zoom                                                          bool
	features                                                                        int
	oni                                                                             int
}

func changesOf(now, stock protocol.Settings, e *env.Env) changes {
	return changes{
		askModel:     routed(now.Ask.Model) != routed(stock.Ask.Model),
		askApprovals: now.Ask.Approvals != stock.Ask.Approvals,
		habits:       now.Ask.Habits != stock.Ask.Habits,
		resume:       now.Ask.ResumeMinutes != stock.Ask.ResumeMinutes,

		agentModel:     routed(now.Agent.Model) != routed(stock.Agent.Model) || !maps.Equal(now.Agent.Options, stock.Agent.Options),
		plan:           now.Agent.Plan != stock.Agent.Plan,
		agentApprovals: now.Agent.Approvals != stock.Agent.Approvals,
		idle:           now.Agent.IdleStopMinutes != stock.Agent.IdleStopMinutes,
		notices:        !e.Notices,

		system:   now.Index.System != stock.Index.System,
		semantic: now.Index.Semantic != stock.Index.Semantic,

		paused:        now.Teams.Paused != stock.Teams.Paused,
		wake:          now.Teams.RoutinesWake != stock.Teams.RoutinesWake,
		teamApprovals: now.Teams.Defaults.Approvals != stock.Teams.Defaults.Approvals,
		teamNotify:    now.Teams.Defaults.Notify != stock.Teams.Defaults.Notify,
		keep:          now.Teams.KeepDays != stock.Teams.KeepDays,

		memoryUse:   now.Memory.Use != stock.Memory.Use || now.Memory.Asked != stock.Memory.Asked,
		memoryLearn: now.Memory.Learn != stock.Memory.Learn,
		sessions:    now.Memory.Sessions != stock.Memory.Sessions,
		agents:      now.Memory.Agents != stock.Memory.Agents,
		sensitive:   now.Memory.Sensitive != stock.Memory.Sensitive,
		suggestions: now.Memory.TeamSuggestions != stock.Memory.TeamSuggestions,
		learner:     now.Memory.Model != stock.Memory.Model,
		fade:        now.Memory.FadeMonths != stock.Memory.FadeMonths,

		apiEnabled: now.API.Enabled != stock.API.Enabled,
		port:       now.API.Enabled && now.API.Port != stock.API.Port,
		features:   featuresOn(now.Features),
		oni:        oniChanges(now.Oni, stock.Oni),
		zoom:       e.Zoom != state.DefaultZoom,
	}
}

// an empty model is the route that picks for you
func routed(model string) string {
	if model == "" {
		return protocol.RouteAuto
	}
	return model
}

func (c changes) count() int {
	count := 0
	for _, changed := range [...]bool{
		c.askModel, c.askApprovals, c.habits, c.resume,
		c.agentModel, c.plan, c.agentApprovals, c.idle, c.notices,
		c.system, c.semantic,
		c.paused, c.wake, c.teamApprovals, c.teamNotify, c.keep,
		c.memoryUse, c.memoryLearn, c.sessions, c.agents, c.sensitive, c.suggestions, c.learner, c.fade,
		c.apiEnabled, c.port, c.zoom,
	} {
		if changed {
			count++
		}
	}
	return count + c.features + c.oni
}

// Count each independently editable Oni value once, including children of
// disabled features: switching a parent off does not reset their choices.
func oniChanges(now, stock protocol.OniSettings) int {
	count := 0
	for _, changed := range [...]bool{
		now.Master != stock.Master, now.Character != stock.Character,
		now.Inspect != stock.Inspect, now.Disclosure != stock.Disclosure,
		now.Voice != stock.Voice, now.Chord != stock.Chord, now.Hands != stock.Hands,
		now.Reflex != stock.Reflex, now.Feeds.Guard != stock.Feeds.Guard,
		!slices.Equal(now.Feeds.Words, stock.Feeds.Words),
		now.Size != stock.Size, now.ReduceMotion != stock.ReduceMotion,
		now.Courtesy != stock.Courtesy, now.Hidden != stock.Hidden,
		now.Sounds != stock.Sounds, now.PerchFollowsFocus != stock.PerchFollowsFocus,
		now.VoiceSettings.Wake.Enabled != stock.VoiceSettings.Wake.Enabled,
		now.VoiceSettings.Wake.Phrase != stock.VoiceSettings.Wake.Phrase,
		now.VoiceSettings.OpenMic != stock.VoiceSettings.OpenMic,
		now.VoiceSettings.SpeakReplies != stock.VoiceSettings.SpeakReplies,
		now.VoiceSettings.Mic != stock.VoiceSettings.Mic,
		now.VoiceSettings.STT.Engine != stock.VoiceSettings.STT.Engine,
		now.VoiceSettings.STT.Model != stock.VoiceSettings.STT.Model,
		now.VoiceSettings.STT.Language != stock.VoiceSettings.STT.Language,
		now.VoiceSettings.TTS.Engine != stock.VoiceSettings.TTS.Engine,
		now.VoiceSettings.Dictation.Enabled != stock.VoiceSettings.Dictation.Enabled,
		now.VoiceSettings.Dictation.Mode != stock.VoiceSettings.Dictation.Mode,
		now.VoiceSettings.Dictation.Polish != stock.VoiceSettings.Dictation.Polish,
		now.Guide.Enabled != stock.Guide.Enabled, now.Guide.Keep != stock.Guide.Keep,
		now.Guide.Hints != stock.Guide.Hints,
		now.ReflexSettings.Routines != stock.ReflexSettings.Routines,
		now.ReflexSettings.Reminders != stock.ReflexSettings.Reminders,
		now.ReflexSettings.Notes != stock.ReflexSettings.Notes,
		now.ReflexSettings.Decisions != stock.ReflexSettings.Decisions,
		now.Recall.Enabled != stock.Recall.Enabled, now.Recall.Vault != stock.Recall.Vault,
		now.Rewrite.Enabled != stock.Rewrite.Enabled, now.Rewrite.Style != stock.Rewrite.Style,
		now.Rewrite.Voice != stock.Rewrite.Voice,
		now.Assignment.Enabled != stock.Assignment.Enabled,
		now.Assignment.Style != stock.Assignment.Style, now.Assignment.Model != stock.Assignment.Model,
		now.MCP.Enabled != stock.MCP.Enabled, !slices.Equal(now.MCP.Clients, stock.MCP.Clients),
	} {
		if changed {
			count++
		}
	}
	for _, id := range protocol.OniMCPTools {
		if now.MCP.Tools[id] != stock.MCP.Tools[id] {
			count++
		}
	}
	return count
}

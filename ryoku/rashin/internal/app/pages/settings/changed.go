package settings

import (
	"maps"

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
	return count + c.features
}

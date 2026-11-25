package agent

import (
	"gioui.org/layout"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// a session uses the same model form as a new session, seeded from the daemon's events
type sessionControls struct {
	form     env.ModelForm
	id       string
	model    string
	mode     string
	plan     string
	revision uint64
}

func (c *sessionControls) sync(e *env.Env, attached state.Attached) {
	info := attached.Info
	revision := attached.Model.ChoiceRevision()
	if info.ID != c.id || info.Model != c.model || revision != c.revision {
		options := make(map[string]string)
		for _, offered := range attached.Model.Options() {
			if offered.Current != "" && offered.ID != protocol.OptionPlan {
				options[offered.ID] = offered.Current
			}
		}
		c.form.Set(info.Model, options)
		c.model, c.id, c.revision = info.Model, info.ID, revision
	}
	mode := info.Mode
	if info.Team == "" {
		mode = e.Snapshot().Settings.Agent.Approvals
	}
	if mode != c.mode || info.ID != c.id {
		c.form.SetApprovals(mode)
		c.mode = mode
	}
	c.form.Offered = attached.Model.Options()
	for _, option := range c.form.Offered {
		if option.ID == protocol.OptionPlan && option.Current != c.plan {
			c.form.SetPlan(option.Current == protocol.OptionOn)
			c.plan = option.Current
		}
	}
	c.form.Parts, c.form.Agents = env.ModelOptions|env.ModelPlan|env.ModelAsking, true
}

func (c *sessionControls) widgets(e *env.Env, home any) []layout.Widget {
	return c.form.Controls(e, home)
}

func (c *sessionControls) update(e *env.Env, info protocol.SessionInfo) {
	for {
		change, ok := c.form.Changed()
		if !ok {
			return
		}
		switch change.Kind {
		case env.ModelPicked:
			e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: info.ID, Model: change.Value}, new(protocol.Empty), e.Report)
		case env.OptionPicked:
			e.Call(protocol.MSessionSetOption, protocol.SetOptionParams{ID: info.ID, Option: change.Option, Value: change.Value}, new(protocol.Empty), e.Report)
		case env.PlanPicked:
			e.Call(protocol.MSessionSetOption, protocol.SetOptionParams{ID: info.ID, Option: protocol.OptionPlan, Value: change.Value}, new(protocol.Empty), e.Report)
		case env.ApprovalsPicked:
			if info.Team != "" {
				e.Call(protocol.MSessionSetMode, protocol.SetModeParams{ID: info.ID, Mode: change.Value}, new(protocol.Empty), e.Report)
			} else {
				e.SetSettings(map[string]any{"agent": map[string]any{"approvals": change.Value}}, e.Report)
			}
		}
	}
}

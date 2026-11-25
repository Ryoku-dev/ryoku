package teamsui

import (
	"gioui.org/layout"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the model a member, the teams defaults or one task runs on: the app's one model form, its first row the chain a team follows
type EngineForm struct {
	// Agents makes the first row what Agents use, as the teams defaults resolve, instead of the teams default
	Agents bool
	// Keep is the member one task is changed for: the first row keeps what that member runs
	Keep *protocol.Member

	form    env.ModelForm
	inherit env.ModelInherit
}

func (f *EngineForm) Set(model string, options map[string]string) { f.form.Set(model, options) }

// model "" follows the first row, options hold only what was picked
func (f *EngineForm) Value() (model string, options map[string]string) { return f.form.Value() }

func (f *EngineForm) Changed() bool {
	_, changed := f.form.Changed()
	return changed
}

// what a task would start on: the picks over what the first row stands for
func (f *EngineForm) Effective(snapshot state.Snapshot) (model string, options map[string]string) {
	f.follow(snapshot)
	return f.form.Effective()
}

// the model it would start on can run now, true while the list still loads
func (f *EngineForm) Ready(e *env.Env) bool {
	f.follow(e.Snapshot())
	return f.form.Ready(e)
}

// the first row is read from the settings each frame, so a default changed elsewhere already shows here
func (f *EngineForm) follow(snapshot state.Snapshot) {
	switch {
	case f.Keep != nil:
		f.inherit.Model, f.inherit.Options = protocol.ResolveEngine(*f.Keep, snapshot.Settings)
		f.inherit.Label = i18n.Tf("Same as %s", f.Keep.Name)
	case f.Agents:
		f.inherit.Model, f.inherit.Options = snapshot.Settings.Agent.Model, snapshot.Settings.Agent.Options
		f.inherit.Label = i18n.T("Same as Agents")
	default:
		f.inherit.Model, f.inherit.Options = protocol.ResolveEngine(protocol.Member{}, snapshot.Settings)
		f.inherit.Label = i18n.T("Teams default")
	}
	// a team's member calls tools, so a model that cannot is listed but never picked
	f.form.Parts, f.form.Agents, f.form.Inherit = env.ModelOptions|env.ModelCheck, true, &f.inherit
}

func (f *EngineForm) Layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	f.follow(e.Snapshot())
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return f.form.Layout(gtx, e)
}

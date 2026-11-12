package ask

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// what answers a conversation a harness answers, with the harness's own models to pick from
type engine struct {
	pick look.Select
	// what the list was built from: a model list runs to hundreds, so it is rebuilt only when that moves
	session  string
	revision uint64
	current  string
	learned  *protocol.HarnessCapabilities
	models   env.ChoiceList
	// the model picked for the next ask on harness, "" leaves the harness on its own
	draft, harness string
}

// the harness an open conversation runs on, or the one Settings names for the next
func answeringHarness(snapshot state.Snapshot, attached state.Attached, open bool) string {
	if open {
		return attached.Info.Harness
	}
	return snapshot.Settings.Ask.Harness
}

// an open conversation lists what its harness offered; a new one what the harness offered when it last ran
func (g *engine) sync(snapshot state.Snapshot, attached state.Attached, open bool) {
	if open {
		info := attached.Info
		revision := attached.Model.ChoiceRevision()
		if g.session == info.ID && g.revision == revision && g.current == info.Model && g.learned == nil {
			return
		}
		g.session, g.revision, g.current, g.learned = info.ID, revision, info.Model, nil
		model := attached.Model.ModelChoice()
		if model.Current == "" {
			model.Current = info.Model
		}
		g.models = env.ListOf(model.Options, model.Current)
		return
	}
	want := snapshot.Settings.Ask.Harness
	if g.harness != want {
		g.harness, g.draft = want, ""
	}
	var learned *protocol.HarnessCapabilities
	for _, harness := range snapshot.Harnesses {
		if harness.ID == want {
			learned = harness.Capabilities
		}
	}
	if g.session == "" && g.learned == learned && g.current == g.draft && learned != nil {
		return
	}
	g.session, g.learned, g.current = "", learned, g.draft
	if learned == nil {
		g.models = env.ChoiceList{}
		return
	}
	current := g.draft
	if current == "" {
		current = learned.DefaultModel
	}
	g.models = env.ListOf(learned.ModelList, current)
}

// the harness's models as the composer's model select, none until the harness has said what it offers
func (g *engine) control(e *env.Env, home any) layout.Widget {
	if len(g.models.IDs) == 0 {
		return nil
	}
	g.pick.Form, g.pick.Label, g.pick.Return, g.pick.Searched = look.SelectQuiet, i18n.T("Model"), home, true
	return func(gtx layout.Context) layout.Dimensions {
		return g.pick.LayoutOptions(gtx, e.Theme, e.Overlays, g.models.Options, g.models.Selected)
	}
}

// a pick in an open conversation goes to its harness, before one it waits for the first message
func (g *engine) update(e *env.Env, id string, open bool) {
	index, changed := g.pick.Changed()
	if !changed || index < 0 || index >= len(g.models.IDs) {
		return
	}
	if open {
		e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: id, Model: g.models.IDs[index]}, new(protocol.Empty), e.Report)
		return
	}
	g.draft = g.models.IDs[index]
}

// says in the composer what answers, so a harness answering is never mistaken for Rashin's providers
func engineLabel(e *env.Env, name string) layout.Widget {
	th := e.Theme
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Left: 10, Right: 10}.Layout(gtx, kit.RawText(th.Theme, th.T.Label, th.R.Fg3, i18n.Tf("Answered by %s", name)))
	}
}

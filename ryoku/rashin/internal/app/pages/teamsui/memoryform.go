package teamsui

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// one form for a team and for new teams, so the two can never word the same setting differently
type TeamMemoryForm struct {
	remember, carry, suggest kit.Toggle
	reads                    look.Select
	held                     Held[protocol.TeamMemory]
}

// the whole value goes on every change, since a merge patch would keep a field the user just turned off
func (f *TeamMemoryForm) Rows(e *env.Env, value protocol.TeamMemory, save func(protocol.TeamMemory, func(error))) []look.Setting {
	th := e.Theme
	shown := f.held.Show(value)
	f.remember.Value, f.carry.Value, f.suggest.Value = shown.Remember, shown.CarryReport, shown.Suggest
	send := func(wanted protocol.TeamMemory) {
		f.held.Hold(wanted)
		save(wanted, f.held.Drop)
	}
	toggle := func(name string, control *kit.Toggle, apply func(*protocol.TeamMemory, bool)) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			dimensions := look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return control.Layout(gtx, th.Theme) })
			if control.Changed() {
				wanted := shown
				apply(&wanted, control.Value)
				send(wanted)
			}
			return dimensions
		}
	}
	suggest := i18n.T("Members may suggest things about you or your projects. Each waits in the Inbox.")
	never := e.Snapshot().Settings.Memory.TeamSuggestions == protocol.TeamSuggestNever
	if never {
		suggest = i18n.T("Settings, Memory turns down every suggestion from teams, so none reaches you.")
	}
	return []look.Setting{
		{Title: i18n.T("Remember between runs"), Detail: i18n.T("Members keep what they learn in a run and get it in the next one."),
			Control: toggle(i18n.T("Remember between runs"), &f.remember, func(m *protocol.TeamMemory, on bool) { m.Remember = on })},
		{Title: i18n.T("Carry the last report"), Detail: i18n.T("The next run starts with the last run's report."),
			Control: toggle(i18n.T("Carry the last report"), &f.carry, func(m *protocol.TeamMemory, on bool) { m.CarryReport = on })},
		{Title: i18n.T("Reads your memory"), Detail: readsSentence(shown.ReadsYou), Control: func(gtx layout.Context) layout.Dimensions {
			f.reads.Form, f.reads.Label = look.SelectQuiet, i18n.T("Reads your memory")
			options := []look.SelectOption{
				{Label: i18n.T("Nothing"), Detail: i18n.T("Members know only what the team learned.")},
				{Label: i18n.T("About you"), Detail: i18n.T("Members also get the card about you.")},
			}
			selected := 0
			if shown.ReadsYou {
				selected = 1
			}
			dimensions := f.reads.LayoutOptions(gtx, th, e.Overlays, options, selected)
			if index, changed := f.reads.Changed(); changed && (index == 1) != shown.ReadsYou {
				wanted := shown
				wanted.ReadsYou = index == 1
				send(wanted)
			}
			return dimensions
		}},
		{Title: i18n.T("Suggest to your memory"), Detail: suggest, Control: func(gtx layout.Context) layout.Dimensions {
			if never {
				gtx = gtx.Disabled()
			}
			return toggle(i18n.T("Suggest to your memory"), &f.suggest, func(m *protocol.TeamMemory, on bool) { m.Suggest = on })(gtx)
		}},
	}
}

func readsSentence(readsYou bool) string {
	if readsYou {
		return i18n.T("Members also get the card about you.")
	}
	return i18n.T("Members know only what the team learned.")
}

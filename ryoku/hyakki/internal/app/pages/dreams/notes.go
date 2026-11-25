package dreams

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// what the CEO wants changed in a presentation; sending it puts the team back to work
type notesDialog struct {
	dialog       kit.Dialog
	dream        protocol.Team
	note         look.MultiLine
	send, cancel kit.Plaque
}

func (d *notesDialog) open(dream protocol.Team) {
	d.dream = dream
	d.dialog.Open, d.dialog.Width = true, 560
	d.note.SetText("")
	d.note.Focus()
}

func (d *notesDialog) layout(gtx layout.Context, e *env.Env) {
	if d.dream.ID == "" {
		return
	}
	th := e.Theme
	note := strings.TrimSpace(d.note.Text())
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.send.Kind, d.send.Label = kit.PlaqueGold, i18n.T("Send")
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.send.Clicked(gtx) && note != "" {
		answer(e, d.dream, protocol.DreamNotes, note, func(error) {})
		d.dialog.Dismiss(gtx)
	}
	d.note.Label, d.note.Lines = i18n.T("Your notes"), 5
	d.note.Placeholder = i18n.T("What to change, add or fix before you accept it.")
	d.note.Help = i18n.T("The manager reads them, the team goes back to work, and it presents again.")
	d.dialog.Layout(gtx, th.Theme, i18n.Tf("Send notes on %s", d.dream.Name),
		func(gtx layout.Context) layout.Dimensions { return d.note.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if note == "" {
				gtx = gtx.Disabled()
			}
			return d.send.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.dream = protocol.Team{}
	}
}

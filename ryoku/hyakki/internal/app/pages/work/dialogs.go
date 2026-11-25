package work

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

// running a task again takes a note, so the team hears what to do differently
type retryDialog struct {
	id         string
	dialog     kit.Dialog
	note       kit.TextField
	do, cancel kit.Plaque
}

func (d *retryDialog) open(id string) {
	d.id, d.dialog.Open, d.dialog.Width = id, true, 480
	d.note.SetText("")
	d.note.Editor.Submit = true
}

func (d *retryDialog) layout(gtx layout.Context, e *env.Env) {
	if d.id == "" {
		return
	}
	th := e.Theme
	submitted := false
	for {
		event, ok := d.note.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	d.note.Label, d.note.Placeholder = i18n.T("A note for the team (optional)"), i18n.T("What to do differently this time")
	d.do.Kind, d.do.Label = kit.PlaqueGold, i18n.T("Run again")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	if d.do.Clicked(gtx) || submitted {
		params := protocol.TeamTaskRetryParams{ID: d.id, Note: strings.TrimSpace(d.note.Text())}
		e.Call(protocol.MTeamsTaskRetry, params, new(protocol.TeamTask), e.Report)
		d.dialog.Dismiss(gtx)
	}
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Run this task again"),
		func(gtx layout.Context) layout.Dimensions { return d.note.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.do.Layout(gtx, th.Theme) },
	)
	if d.dialog.Closed() {
		d.id = ""
	}
}

type removeDialog struct {
	id      string
	confirm kit.Confirm
}

func (d *removeDialog) open(id string) {
	d.id, d.confirm.Dialog.Open, d.confirm.Danger = id, true, true
}

func (d *removeDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if d.id == "" {
		return
	}
	title := e.Snapshot().TeamTasks[d.id].Title
	body := i18n.Tf("“%s”, its report and the pieces it handed out are removed. Its conversations and files stay.", title)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete this task?"), body, i18n.T("Delete task"), "") {
		id := d.id
		e.Call(protocol.MTeamsTaskDelete, protocol.TeamTaskRef{ID: id}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if page.selected == id {
				e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabWork})
			}
		})
	}
	if d.confirm.Closed() {
		d.id = ""
	}
}

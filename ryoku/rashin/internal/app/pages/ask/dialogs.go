package ask

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

type renameDialog struct {
	id         string
	dialog     kit.Dialog
	field      kit.TextField
	do, cancel kit.Plaque
}

func (d *renameDialog) open(info protocol.SessionInfo) {
	d.id, d.dialog.Open = info.ID, true
	d.field.SetText(info.Title)
	d.field.Editor.Submit = true
}

func (d *renameDialog) layout(gtx layout.Context, e *env.Env) {
	if d.id == "" {
		return
	}
	submitted := false
	for {
		event, ok := d.field.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	d.field.Label = i18n.T("Name")
	d.do.Kind, d.do.Label = kit.PlaqueGold, i18n.T("Rename")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	title := strings.TrimSpace(d.field.Text())
	if (d.do.Clicked(gtx) || submitted) && title != "" {
		renameSession(e, d.id, title)
		d.dialog.Dismiss(gtx)
	}
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, e.Theme.Theme, i18n.T("Rename ask"),
		func(gtx layout.Context) layout.Dimensions { return d.field.Layout(gtx, e.Theme.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, e.Theme.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.do.Layout(gtx, e.Theme.Theme) },
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
	title := sessionTitle(e.Snapshot().Sessions[d.id])
	body := i18n.Tf("“%s” and its answers are removed from this machine.", title)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete this ask?"), body, i18n.T("Delete"), "") {
		id := d.id
		e.Call(protocol.MSessionDelete, protocol.SessionRef{ID: id}, new(protocol.Empty), func(err error) {
			if err != nil {
				e.Notify(err.Error())
				return
			}
			if page.selected == id {
				e.Navigate(protocol.TabAsk, "")
			}
		})
	}
	if d.confirm.Closed() {
		d.id = ""
	}
}

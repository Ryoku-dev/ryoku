package ask

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
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
	id         string
	dialog     kit.Dialog
	cancel, do kit.Plaque
	forget     kit.Toggle
	// the memories learned from the conversation, asked for as the dialog opens
	learned []string
}

func (d *removeDialog) open(e *env.Env, id string) {
	d.id, d.dialog.Open, d.learned, d.forget.Value = id, true, nil, true
	if e.Snapshot().Memory == nil {
		return
	}
	e.ListMemory(protocol.MemoryListParams{Session: id, Limit: forgetLimit}, func(result protocol.MemoryListResult, err error) {
		if err != nil || d.id != id {
			return
		}
		d.learned = d.learned[:0]
		for _, memory := range result.Memories {
			d.learned = append(d.learned, memory.ID)
		}
	})
}

// more than a conversation ever teaches, so one call lists all of it
const forgetLimit = 1000

func (d *removeDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if d.id == "" {
		return
	}
	th := e.Theme
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.do.Kind, d.do.Label = kit.PlaqueDanger, i18n.T("Delete")
	d.forget.Label = i18n.Tf("Also forget the %d memories learned from it.", len(d.learned))
	if len(d.learned) == 1 {
		d.forget.Label = i18n.T("Also forget the memory learned from it.")
	}
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.do.Clicked(gtx) {
		d.remove(e, page)
		d.dialog.Dismiss(gtx)
	}
	title := sessionTitle(e.Snapshot().Sessions[d.id])
	body := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.Tf("“%s” and its answers are removed from this machine.", title)))}
		if len(d.learned) > 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.forget.Layout(gtx, th.Theme) }),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Delete this ask?"), body,
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.do.Layout(gtx, th.Theme) },
	)
	if d.dialog.Closed() {
		d.id, d.learned = "", nil
	}
}

// what was learned goes first, so a conversation that is gone leaves nothing it taught behind
func (d *removeDialog) remove(e *env.Env, page *Page) {
	id := d.id
	deleteSession := func() {
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
	if !d.forget.Value || len(d.learned) == 0 {
		deleteSession()
		return
	}
	params := protocol.MemoryForgetParams{IDs: append([]string(nil), d.learned...)}
	e.ChangeMemory(protocol.MMemoryForget, params, func(_ protocol.MemoryChange, err error) {
		if err != nil {
			e.Report(err)
			return
		}
		deleteSession()
	})
}

type keepDialog struct {
	info    protocol.SessionInfo
	confirm kit.Confirm
}

func (d *keepDialog) open(info protocol.SessionInfo) {
	d.info, d.confirm.Dialog.Open = info, true
}

func (d *keepDialog) layout(gtx layout.Context, e *env.Env, page *Page) {
	if d.info.ID == "" {
		return
	}
	body := i18n.T("It moves to your Ask history. Rashin still won't learn from it.")
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Keep this conversation?"), body, i18n.T("Keep"), "") {
		temporary := d.info.ID
		e.KeepTemporary(d.info, func(kept protocol.SessionInfo) {
			page.kept = temporary
			e.Attach(kept.ID, func(error) {})
			e.Navigate(protocol.TabAsk, kept.ID)
			e.Notify(i18n.T("Kept in your Ask history."))
		})
	}
	if d.confirm.Closed() {
		d.info = protocol.SessionInfo{}
	}
}

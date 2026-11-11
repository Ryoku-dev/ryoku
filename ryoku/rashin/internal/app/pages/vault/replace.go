package vault

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// replaces a credential's value; the old one is kept until the next replace
type replaceDialog struct {
	credential   protocol.Credential
	dialog       kit.Dialog
	value        kit.TextField
	save, cancel kit.Plaque
	busy         bool
	problem      string
}

func (d *replaceDialog) open(credential protocol.Credential) {
	d.credential, d.dialog.Open, d.dialog.Width = credential, true, 600
	d.busy, d.problem = false, ""
	d.value.SetText("")
}

func (d *replaceDialog) layout(gtx layout.Context, e *env.Env) {
	if d.credential.ID == "" {
		return
	}
	th := e.Theme
	value := d.value.Text()
	ready := !d.busy && strings.TrimSpace(value) != ""
	d.value.Secret, d.value.Mono = true, true
	d.value.Label, d.value.Help = i18n.T("New value"), i18n.T("Stored once and never shown again.")
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.save.Kind, d.save.Label, d.save.Busy = kit.PlaqueGold, i18n.T("Replace value"), d.busy
	if d.cancel.Clicked(gtx) {
		d.value.SetText("")
		d.dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) && ready {
		d.submit(e, value)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.Tf("Replace the value of %s", d.credential.Name),
		func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("Teams use the new value from now on. Nothing they did with the old one changes."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := d.value.Layout(gtx, th.Theme)
					if d.dialog.Open && !gtx.Focused(&d.value.Editor) && d.value.Len() == 0 {
						d.value.Focus(gtx)
					}
					return dimensions
				}),
			}
			if d.problem != "" {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Amber, d.problem)),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.save.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.value.SetText("")
		d.credential = protocol.Credential{}
	}
}

func (d *replaceDialog) submit(e *env.Env, value string) {
	// the value leaves the field the moment it is sent; a refusal asks for it again
	d.value.SetText("")
	d.busy, d.problem = true, ""
	params, name := protocol.VaultUpdateParams{ID: d.credential.ID, Value: value}, d.credential.Name
	e.Call(protocol.MVaultUpdate, params, nil, func(err error) {
		d.busy = false
		if err != nil {
			d.problem = err.Error()
			return
		}
		d.dialog.Open = false
		e.Notify(i18n.Tf("%s has a new value.", name))
	})
}

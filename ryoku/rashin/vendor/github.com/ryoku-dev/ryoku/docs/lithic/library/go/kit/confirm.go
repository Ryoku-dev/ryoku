package kit

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Confirm composes a dialog with cancel, a named action, and optional typed proof.
type Confirm struct {
	Dialog  Dialog
	Danger  bool
	Preview State

	cancel, confirm Plaque
	field           TextField
	closed          bool
}

func confirmReady(input, required string) bool {
	return required == "" || input == required
}

// SetConfirmationText sets the typed proof value.
func (c *Confirm) SetConfirmationText(value string) { c.field.SetText(value) }

// ConfirmationText returns the typed proof value.
func (c *Confirm) ConfirmationText() string { return c.field.Text() }

// Layout draws the confirmation and reports an accepted action.
func (c *Confirm) Layout(gtx layout.Context, th *theme.Theme, title, body, action, typeToConfirm string) bool {
	c.Dialog.Preview = c.Preview
	enabled := confirmReady(c.field.Text(), typeToConfirm)
	confirmed := false
	if c.cancel.Clicked(gtx) {
		c.Dialog.Dismiss(gtx)
		c.field.SetText("")
	} else if enabled && c.confirm.Clicked(gtx) {
		confirmed = true
		c.Dialog.Dismiss(gtx)
		c.field.SetText("")
	}

	bodyWidget := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Style: th.T.Body, Color: th.R.Fg2}.Layout(gtx, th, body)
			}),
		}
		if typeToConfirm != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: tokens.SizeDialogConfirmFieldGap}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					c.field.Label = "Type " + typeToConfirm + " to confirm"
					c.field.Placeholder = typeToConfirm
					c.field.Preview = c.Preview
					return c.field.Layout(gtx, th)
				}),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	keep := func(gtx layout.Context) layout.Dimensions {
		c.cancel.Kind = PlaqueQuiet
		c.cancel.Label = "Cancel"
		c.cancel.Preview = c.Preview
		return c.cancel.Layout(gtx, th)
	}
	accept := func(gtx layout.Context) layout.Dimensions {
		if !enabled {
			gtx = gtx.Disabled()
		}
		c.confirm.Kind = PlaqueGold
		if c.Danger {
			c.confirm.Kind = PlaqueDanger
		}
		c.confirm.Label = action
		c.confirm.Preview = c.Preview
		return c.confirm.Layout(gtx, th)
	}
	c.Dialog.Layout(gtx, th, title, bodyWidget, keep, accept)
	if c.Dialog.Closed() {
		c.field.SetText("")
		c.closed = true
	}
	return confirmed
}

// Closed reports one completed dismissal.
func (c *Confirm) Closed() bool {
	closed := c.closed
	c.closed = false
	return closed
}

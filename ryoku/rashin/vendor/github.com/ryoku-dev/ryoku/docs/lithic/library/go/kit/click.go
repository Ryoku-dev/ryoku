package kit

import (
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

// Click keeps events drained during Layout until Update or Clicked takes them.
type Click struct {
	widget.Clickable
	pending       int
	pendingDouble int
}

// Update reports one click and invalidates when handing it to the caller.
func (click *Click) Update(gtx layout.Context) (widget.Click, bool) {
	if click.pending > 0 {
		click.pending--
		event := widget.Click{NumClicks: 1}
		if click.pendingDouble > 0 {
			click.pendingDouble--
			event.NumClicks = 2
		}
		gtx.Execute(op.InvalidateCmd{})
		return event, true
	}
	if click.keypadEnter(gtx) {
		gtx.Execute(op.InvalidateCmd{})
		return widget.Click{NumClicks: 1}, true
	}
	event, ok := click.Clickable.Update(gtx)
	if ok {
		gtx.Execute(op.InvalidateCmd{})
	}
	return event, ok
}

// keypadEnter reports one press of the keypad Enter while focused; Clickable
// itself answers Return and Space.
func (click *Click) keypadEnter(gtx layout.Context) bool {
	for {
		value, ok := gtx.Event(key.Filter{Focus: &click.Clickable, Name: key.NameEnter})
		if !ok {
			return false
		}
		if pressed, ok := value.(key.Event); ok && pressed.State == key.Press {
			return true
		}
	}
}

// Clicked reports one latched click.
func (click *Click) Clicked(gtx layout.Context) bool {
	_, ok := click.Update(gtx)
	return ok
}

// Layout drains current events into the latch before laying out widget.
func (click *Click) Layout(gtx layout.Context, widget layout.Widget) layout.Dimensions {
	caught := false
	for click.keypadEnter(gtx) {
		click.pending++
		caught = true
	}
	for {
		event, ok := click.Clickable.Update(gtx)
		if !ok {
			break
		}
		click.pending++
		if event.NumClicks >= 2 {
			click.pendingDouble++
		}
		caught = true
	}
	if caught {
		gtx.Execute(op.InvalidateCmd{})
	}
	noteOverlayFocus(gtx, &click.Clickable)
	return click.Clickable.Layout(gtx, widget)
}

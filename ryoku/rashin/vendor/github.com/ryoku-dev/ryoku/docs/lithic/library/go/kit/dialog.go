package kit

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Dialog is a protected, deferred decision surface.
type Dialog struct {
	Open    bool
	Width   unit.Dp
	Opener  any
	Preview State

	scrimTag, blockerTag int
	enter                anim.Tween
	wasOpen, visible     bool
	closed               bool
	slabRect             image.Rectangle
	focusState           overlayFocus
	focusPending         bool
}

// Dismiss closes the dialog and returns focus to its opener.
func (d *Dialog) Dismiss(gtx layout.Context) {
	d.Open = false
	if d.Opener != nil {
		gtx.Execute(key.FocusCmd{Tag: d.Opener})
	}
}

func (d *Dialog) beginExit(gtx layout.Context) {
	d.Dismiss(gtx)
	d.wasOpen = false
	d.enter.Duration = tokens.MotionDialogOut
	d.enter.Ease = anim.EaseChisel
	d.enter.Set(gtx, 0)
}

// Layout draws a title, body, and start-aligned action row above a swallowing scrim.
func (d *Dialog) Layout(gtx layout.Context, th *theme.Theme, title string, body layout.Widget, actions ...layout.Widget) layout.Dimensions {
	window := gtx.Constraints.Max
	opening := d.Open && (!d.wasOpen || !d.visible)
	closing := !d.Open && d.wasOpen
	if opening {
		d.visible = true
		d.closed = false
		d.focusPending = d.Preview == StateRest
		d.enter = anim.Tween{Duration: tokens.MotionDialogIn, Ease: anim.EaseChisel}
		d.enter.Set(gtx, 0)
		d.enter.Set(gtx, 1)
	}
	if closing {
		d.enter.Duration = tokens.MotionDialogOut
		d.enter.Ease = anim.EaseChisel
		d.enter.Set(gtx, 0)
		if d.Opener != nil {
			gtx.Execute(key.FocusCmd{Tag: d.Opener})
		}
	}
	d.wasOpen = d.Open
	if !d.visible {
		d.slabRect = image.Rectangle{}
		return layout.Dimensions{Size: window}
	}

	if d.Open {
		for {
			value, ok := gtx.Event(pointer.Filter{Target: &d.scrimTag, Kinds: pointer.Press})
			if !ok {
				break
			}
			press, ok := value.(pointer.Event)
			if ok && !press.Position.Round().In(d.slabRect) {
				d.beginExit(gtx)
			}
		}
	} else {
		for {
			if _, ok := gtx.Event(pointer.Filter{Target: &d.blockerTag, Kinds: pointer.Press | pointer.Release}); !ok {
				break
			}
		}
	}

	progress := d.enter.Value(gtx)
	if !d.Open && progress <= 0 {
		d.visible = false
		d.closed = true
		d.slabRect = image.Rectangle{}
		return layout.Dimensions{Size: window}
	}

	margin := gtx.Dp(tokens.SizeDialogMargin)
	dialogWidth := d.Width
	if dialogWidth <= 0 {
		dialogWidth = unit.Dp(tokens.SizeDialogWidth)
	}
	width := min(gtx.Dp(dialogWidth), max(0, window.X-2*margin))
	padding := gtx.Dp(tokens.SizeDialogPadding)
	innerWidth := max(0, width-2*padding)
	maxInnerHeight := max(0, window.Y-2*margin-2*padding)

	insideContext := overlayContentContext(gtx)
	if d.Open {
		d.focusState = overlayFocus{parent: overlayFocusOf(gtx)}
		insideContext.Values[overlayFocusValue] = &d.focusState
	} else {
		insideContext = insideContext.Disabled()
	}
	insideContext.Constraints.Min = image.Pt(innerWidth, 0)
	insideContext.Constraints.Max = image.Pt(innerWidth, maxInnerHeight)
	insideRecord := op.Record(gtx.Ops)
	children := make([]layout.FlexChild, 0, 5)
	if title != "" {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return Label{Style: th.T.Sentence, Color: th.R.Fg, MaxLines: 2}.Layout(gtx, th, title)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeDialogTitleGap)}.Layout),
		)
	}
	children = append(children, layout.Rigid(body))
	if len(actions) > 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: unit.Dp(tokens.SizeDialogActionsGap)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				row := make([]layout.FlexChild, 0, len(actions)*2-1)
				for index, action := range actions {
					if index > 0 {
						row = append(row, layout.Rigid(layout.Spacer{Width: unit.Dp(tokens.SizeDialogActionGap)}.Layout))
					}
					row = append(row, layout.Rigid(action))
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceStart}.Layout(gtx, row...)
			}),
		)
	}
	insideDimensions := layout.Flex{Axis: layout.Vertical}.Layout(insideContext, children...)
	insideCall := insideRecord.Stop()
	if d.Open {
		// a dialog that opened with nothing inside it focused hands focus to
		// its first control once the content has had a frame to take it
		if d.focusPending && !opening {
			d.focusPending = false
			if !d.focusState.within && d.focusState.first != nil {
				gtx.Execute(key.FocusCmd{Tag: d.focusState.first})
			}
		}
		// Escape is read after the content, so an open menu or a Search with
		// text inside the dialog takes it first
		for {
			value, ok := gtx.Event(key.Filter{Name: key.NameEscape})
			if !ok {
				break
			}
			if keyEvent, ok := value.(key.Event); ok && keyEvent.State == key.Press {
				d.beginExit(gtx)
			}
		}
	}
	height := min(window.Y-2*margin, insideDimensions.Size.Y+2*padding)
	height = max(0, height)
	x := (window.X - width) / 2
	baseY := (window.Y - height) / 2
	travel := gtx.Dp(tokens.SizeDialogEnterTravel)
	if !d.Open {
		travel = gtx.Dp(tokens.SizeDialogExitTravel)
	}
	y := baseY + int(float32(travel)*(1-progress))
	d.slabRect = image.Rect(x, y, x+width, y+height)

	overlay := op.Record(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	overlayBounds := image.Rect(-overlayExtent, -overlayExtent, overlayExtent, overlayExtent)
	if d.Preview != StateRest {
		overlayBounds = image.Rectangle{Max: window}
	}
	catcher := clip.Rect(overlayBounds).Push(gtx.Ops)
	if d.Open {
		event.Op(gtx.Ops, &d.scrimTag)
	} else {
		event.Op(gtx.Ops, &d.blockerTag)
	}
	catcher.Pop()
	FillCut(gtx.Ops, overlayBounds, 0, th.R.Scrim)
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	bounds := image.Rect(0, 0, width, height)
	cut := gtx.Dp(th.Cut.Large)
	FillCut(gtx.Ops, bounds, cut, th.R.Stone4)
	StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
	contentOffset := op.Offset(image.Pt(padding, padding)).Push(gtx.Ops)
	insideCall.Add(gtx.Ops)
	contentOffset.Pop()
	offset.Pop()
	opacity.Pop()
	presentOverlay(gtx, overlay.Stop(), d.Preview != StateRest, window)
	return layout.Dimensions{Size: window}
}

// Closed reports one completed close transition.
func (d *Dialog) Closed() bool {
	if !d.closed {
		return false
	}
	d.closed = false
	return true
}

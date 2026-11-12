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

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

const overlayExtent = 1 << 20

const overlayHostValue = "lithic.kit.overlay-host"

var activePopover *Popover

// Popover places deferred content beside an anchor and owns its click-away catcher.
type Popover struct {
	Open    bool
	Opener  any
	Preview State

	contentRect image.Rectangle
	focusTag    int
	outsideTag  int
	blockerTag  int
	wasOpen     bool
	visible     bool
	enter       anim.Tween
	focusState  overlayFocus
	parent      *Popover
}

// Close dismisses the popover and restores keyboard focus to its opener.
func (p *Popover) Close(gtx layout.Context) {
	p.Open = false
	if activePopover == p {
		activePopover = nil
		if p.parent != nil && p.parent.Open {
			activePopover = p.parent
		}
	}
	p.restoreFocus(gtx)
}

// popoverWithin reports whether child opened inside ancestor's content.
func popoverWithin(child, ancestor *Popover) bool {
	for current := child.parent; current != nil; current = current.parent {
		if current == ancestor {
			return true
		}
	}
	return false
}

func (p *Popover) restoreFocus(gtx layout.Context) {
	if p.Opener != nil {
		gtx.Execute(key.FocusCmd{Tag: p.Opener})
	}
}

func overlayContentContext(gtx layout.Context) layout.Context {
	values := make(map[string]any, len(gtx.Values)+1)
	for key, value := range gtx.Values {
		values[key] = value
	}
	values[overlayHostValue] = true
	gtx.Values = values
	return gtx
}

func deferOverlay(gtx layout.Context, call op.CallOp) {
	if hosted, _ := gtx.Values[overlayHostValue].(bool); hosted {
		call.Add(gtx.Ops)
		return
	}
	op.Defer(gtx.Ops, call)
}

func presentOverlay(gtx layout.Context, call op.CallOp, preview bool, bounds image.Point) {
	if preview {
		area := clip.Rect{Max: bounds}.Push(gtx.Ops)
		call.Add(gtx.Ops)
		area.Pop()
		return
	}
	deferOverlay(gtx, call)
}

func placePopover(viewport, anchor image.Rectangle, content image.Point, gap int) image.Point {
	maxX := viewport.Max.X - content.X
	maxY := viewport.Max.Y - content.Y
	x := clampCoordinate(anchor.Min.X, viewport.Min.X, maxX)
	below := anchor.Max.Y + gap
	above := anchor.Min.Y - gap - content.Y
	y := below
	switch {
	case below+content.Y <= viewport.Max.Y:
	case above >= viewport.Min.Y:
		y = above
	default:
		y = clampCoordinate(below, viewport.Min.Y, maxY)
	}
	return image.Pt(x, clampCoordinate(y, viewport.Min.Y, maxY))
}

func clampCoordinate(value, low, high int) int {
	if high < low {
		return low
	}
	return max(low, min(value, high))
}

// dismiss closes the popover from inside it, restoring the opener's focus and
// starting the exit.
func (p *Popover) dismiss(gtx layout.Context) {
	p.Close(gtx)
	p.wasOpen = false
	p.enter.Duration = tokens.MotionMenuOut
	p.enter.Ease = anim.EaseChisel
	p.enter.Set(gtx, 0)
}

// overlayFocus is how an open overlay and its content share keyboard focus:
// the overlay says whether it holds focus itself, and the content says
// whether one of its parts does and which part can take focus first. An
// overlay inside another's content links to it through parent, so focus in
// the inner one counts as within the outer one too.
type overlayFocus struct {
	idle   bool
	within bool
	first  event.Tag
	parent *overlayFocus
	owner  *Popover
}

const overlayFocusValue = "lithic.kit.overlay-focus"

func overlayFocusOf(gtx layout.Context) *overlayFocus {
	state, _ := gtx.Values[overlayFocusValue].(*overlayFocus)
	return state
}

// noteOverlayFocus tells the enclosing overlays about tag: the nearest one
// remembers the first enabled part, and every one up the chain learns when
// it holds focus.
func noteOverlayFocus(gtx layout.Context, tag event.Tag) {
	state := overlayFocusOf(gtx)
	if state == nil {
		return
	}
	if state.first == nil && gtx.Source.Enabled() {
		state.first = tag
	}
	if !gtx.Focused(tag) {
		return
	}
	for ; state != nil; state = state.parent {
		state.within = true
	}
}

func enclosingPopover(state *overlayFocus) *Popover {
	for ; state != nil; state = state.parent {
		if state.owner != nil {
			return state.owner
		}
	}
	return nil
}

// Layout draws anchor in flow and content in the deferred overlay layer.
func (p *Popover) Layout(gtx layout.Context, anchor, content layout.Widget) layout.Dimensions {
	anchorDimensions := anchor(gtx)
	opening := p.Open && (!p.wasOpen || !p.visible)
	closing := !p.Open && p.wasOpen

	outer := overlayFocusOf(gtx)
	p.parent = enclosingPopover(outer)
	if p.Preview == StateRest && p.Open {
		switch {
		case activePopover == nil, activePopover == p, popoverWithin(p, activePopover):
			activePopover = p
		case popoverWithin(activePopover, p):
			// a popover open inside this one stays the active one
		default:
			activePopover.Close(gtx)
			activePopover = p
		}
	}
	if opening {
		p.visible = true
		p.enter = anim.Tween{Duration: tokens.MotionMenuIn, Ease: anim.EaseChisel}
		p.enter.Set(gtx, 0)
		p.enter.Set(gtx, 1)
	}
	if closing {
		p.enter.Duration = tokens.MotionMenuOut
		p.enter.Ease = anim.EaseChisel
		p.enter.Set(gtx, 0)
		p.restoreFocus(gtx)
		if activePopover == p {
			activePopover = nil
		}
	}
	p.wasOpen = p.Open
	if !p.visible {
		p.contentRect = image.Rectangle{}
		return anchorDimensions
	}

	if p.Open {
		event.Op(gtx.Ops, &p.focusTag)
		p.focusState = overlayFocus{idle: opening || gtx.Focused(&p.focusTag), parent: outer, owner: p}
		if opening {
			gtx.Execute(key.FocusCmd{Tag: &p.focusTag})
		}
		if outer != nil && gtx.Focused(&p.focusTag) {
			for state := outer; state != nil; state = state.parent {
				state.within = true
			}
		}
		for {
			value, ok := gtx.Event(pointer.Filter{Target: &p.outsideTag, Kinds: pointer.Press})
			if !ok {
				break
			}
			press, ok := value.(pointer.Event)
			if ok && !press.Position.Round().In(p.contentRect) {
				p.dismiss(gtx)
			}
		}
	} else {
		for {
			if _, ok := gtx.Event(pointer.Filter{Target: &p.blockerTag, Kinds: pointer.Press | pointer.Release}); !ok {
				break
			}
		}
	}

	progress := p.enter.Value(gtx)
	if !p.Open && progress <= 0 {
		p.visible = false
		p.contentRect = image.Rectangle{}
		return anchorDimensions
	}

	window := gtx.Constraints.Max
	contentContext := overlayContentContext(gtx)
	if p.Open {
		contentContext.Values[overlayFocusValue] = &p.focusState
	} else {
		contentContext = contentContext.Disabled()
	}
	contentContext.Constraints.Min = image.Point{}
	contentContext.Constraints.Max = image.Pt(max(0, window.X), max(0, window.Y))
	contentRecord := op.Record(gtx.Ops)
	contentDimensions := content(contentContext)
	contentCall := contentRecord.Stop()
	if p.Open {
		// Escape is read after the content, so a focused part that uses it
		// (a Search with text, an inner popover) takes it first
		escape := key.Filter{Name: key.NameEscape}
		if p.Preview != StateRest {
			escape.Focus = &p.focusTag
		}
		for {
			value, ok := gtx.Event(escape)
			if !ok {
				break
			}
			if keyEvent, ok := value.(key.Event); ok && keyEvent.State == key.Press {
				p.dismiss(gtx)
			}
		}
		if p.Open && !opening && !p.focusState.within && !gtx.Focused(&p.focusTag) {
			// focus left the popover and its content, so bring it back
			gtx.Execute(key.FocusCmd{Tag: &p.focusTag})
		}
	}

	viewport := image.Rect(0, -window.Y, window.X, window.Y)
	if p.Preview != StateRest {
		viewport = image.Rectangle{Max: window}
	}
	anchorRect := image.Rectangle{Max: anchorDimensions.Size}
	position := placePopover(viewport, anchorRect, contentDimensions.Size, gtx.Dp(tokens.SizePopoverAnchorGap))
	p.contentRect = image.Rectangle{Min: position, Max: position.Add(contentDimensions.Size)}
	distance := gtx.Dp(tokens.SizePopoverEnterTravel)
	if !p.Open {
		distance = gtx.Dp(tokens.SizePopoverExitTravel)
	}
	slide := int(float32(distance) * (1 - progress))
	if position.Y >= anchorRect.Max.Y {
		slide = -slide
	}

	overlay := op.Record(gtx.Ops)
	catcherBounds := image.Rect(-overlayExtent, -overlayExtent, overlayExtent, overlayExtent)
	if p.Preview != StateRest {
		catcherBounds = image.Rectangle{Max: window}
	}
	catcher := clip.Rect(catcherBounds).Push(gtx.Ops)
	if p.Open {
		event.Op(gtx.Ops, &p.outsideTag)
	} else {
		event.Op(gtx.Ops, &p.blockerTag)
	}
	catcher.Pop()
	opacity := paint.PushOpacity(gtx.Ops, progress)
	offset := op.Offset(position.Add(image.Pt(0, slide))).Push(gtx.Ops)
	contentCall.Add(gtx.Ops)
	offset.Pop()
	opacity.Pop()
	presentOverlay(gtx, overlay.Stop(), p.Preview != StateRest, window)
	return anchorDimensions
}

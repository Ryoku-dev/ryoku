package kit

import (
	"image"
	"image/color"
	"math"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// drawScrollThumb draws the cut thumb against the right edge of a pane right
// pixels wide.
func drawScrollThumb(gtx layout.Context, right, top, height int, ink color.NRGBA) {
	width := max(1, gtx.Dp(tokens.SizeScrollThumb))
	x := right - width
	FillCut(gtx.Ops, image.Rect(x, top, x+width, top+height), min(width/2, height/2), ink)
}

// ScrollRail draws the Lithic scroll thumb for a vertical layout.List pane.
// Dragging the thumb scrolls the list and a press on the track pages it; the
// wheel stays with the pane.
type ScrollRail struct {
	Preview State

	tag      uint8
	hovered  bool
	dragging bool
	dragFrom float32
	dragBase float32
	hover    anim.Tween
}

// Dragging reports whether the thumb is held.
func (r *ScrollRail) Dragging() bool { return r.dragging }

// Layout draws the rail over the pane that gtx.Constraints.Max describes. Call
// it after list.Layout, in the same bounds, with the list's element count.
func (r *ScrollRail) Layout(gtx layout.Context, th *theme.Theme, list *layout.List, count int) layout.Dimensions {
	size := gtx.Constraints.Max
	viewport := size.Y
	content := list.Position.Length
	top, height, ok := rowListThumb(viewport, content, list.Position.First, list.Position.Offset, count, gtx.Dp(tokens.SizeScrollThumbMin))
	if !ok {
		r.dragging, r.hovered = false, false
		return layout.Dimensions{Size: size}
	}
	travel := viewport - height
	maximum := float32(content - viewport)
	scrolled := scrollRailOffset(list.Position, content, count)
	for {
		value, more := gtx.Event(pointer.Filter{Target: &r.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Enter | pointer.Leave})
		if !more {
			break
		}
		pressed, isPointer := value.(pointer.Event)
		if !isPointer {
			continue
		}
		y := pressed.Position.Y
		switch pressed.Kind {
		case pointer.Enter:
			r.hovered = true
		case pointer.Leave:
			r.hovered = false
		case pointer.Press:
			if pressed.Buttons != pointer.ButtonPrimary && pressed.Source == pointer.Mouse {
				continue
			}
			if y >= float32(top) && y < float32(top+height) {
				r.dragging, r.dragFrom, r.dragBase = true, y, scrolled
				continue
			}
			page := float32(viewport)
			if y < float32(top) {
				page = -page
			}
			target := min(max(scrolled+page, 0), maximum)
			scrollListBy(list, target-scrolled)
			scrolled = target
			gtx.Execute(op.InvalidateCmd{})
		case pointer.Drag:
			if !r.dragging || travel <= 0 {
				continue
			}
			target := r.dragBase + (y-r.dragFrom)*maximum/float32(travel)
			target = min(max(target, 0), maximum)
			scrollListBy(list, target-scrolled)
			scrolled = target
			gtx.Execute(op.InvalidateCmd{})
		case pointer.Release, pointer.Cancel:
			r.dragging = false
		}
	}

	lane := gtx.Dp(th.Sp.S3)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rect(size.X-lane, 0, size.X, size.Y)).Push(gtx.Ops)
	event.Op(gtx.Ops, &r.tag)
	area.Pop()
	pass.Pop()

	active := r.hovered || r.dragging || r.Preview == StatePressed
	hover := previewTransition(gtx, &r.hover, r.Preview, StateHover, active, th.Motion.HoverIn, th.Motion.HoverOut)
	if r.Preview == StatePressed {
		hover = 1
	}
	drawScrollThumb(gtx, size.X, top, height, palette.Mix(th.R.Edge, th.R.Fg3, hover))
	return layout.Dimensions{Size: size}
}

// scrollRailOffset estimates the scrolled distance the way the thumb places it.
func scrollRailOffset(position layout.Position, content, count int) float32 {
	if count <= 0 {
		return 0
	}
	return float32(position.First)*float32(content)/float32(count) + float32(position.Offset)
}

func scrollListBy(list *layout.List, delta float32) {
	whole := int(math.Round(float64(delta)))
	if whole == 0 {
		return
	}
	list.Position.Offset += whole
	list.Position.BeforeEnd = true
}

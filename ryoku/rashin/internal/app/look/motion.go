package look

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

// reports pointer presence without taking input from the child
type Hover struct {
	Preview kit.State

	tag     struct{}
	hovered bool
	ready   bool
	tween   anim.Tween
}

func (hover *Hover) Hovered() bool { return hover.hovered }

func (hover *Hover) Layout(gtx layout.Context, th *Theme, child layout.Widget) (layout.Dimensions, float32) {
	for {
		// a pointer that leaves the window cancels instead of leaving, and would leave the hover lit
		raw, ok := gtx.Event(pointer.Filter{Target: &hover.tag, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		e, ok := raw.(pointer.Event)
		if !ok {
			continue
		}
		hover.hovered = e.Kind == pointer.Enter
	}

	dimensions := child(gtx)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect(image.Rectangle{Max: dimensions.Size}).Push(gtx.Ops)
	event.Op(gtx.Ops, &hover.tag)
	area.Pop()
	pass.Pop()

	if hover.Preview != kit.StateRest {
		if hover.Preview == kit.StateHover || hover.Preview == kit.StatePressed {
			return dimensions, 1
		}
		return dimensions, 0
	}
	target := float32(0)
	duration := th.Motion.HoverOut
	if hover.hovered && gtx.Source.Enabled() {
		target, duration = 1, th.Motion.HoverIn
	}
	hover.tween.Duration = duration
	hover.tween.Ease = anim.EaseChisel
	if !hover.ready {
		hover.tween.Set(gtx, target)
		hover.ready = true
	}
	hover.tween.Set(gtx, target)
	return dimensions, hover.tween.Value(gtx)
}

type Swap struct {
	current, previous string
	ready, moving     bool
	tween             anim.Tween
}

func (swap *Swap) Layout(
	gtx layout.Context,
	_ *Theme,
	key string,
	rise unit.Dp,
	duration time.Duration,
	child func(layout.Context, string) layout.Dimensions,
) layout.Dimensions {
	if !swap.ready {
		swap.current, swap.ready = key, true
		return child(gtx, key)
	}
	if key != swap.current {
		swap.previous, swap.current = swap.current, key
		swap.tween = anim.Tween{Duration: duration, Ease: anim.EaseChisel}
		swap.tween.Set(gtx, 0)
		swap.tween.Set(gtx, 1)
		swap.moving = duration > 0 && !anim.Reduced()
	}
	if !swap.moving {
		return child(gtx, swap.current)
	}
	progress := swap.tween.Value(gtx)
	if progress >= 1 {
		swap.previous, swap.moving = "", false
		return child(gtx, swap.current)
	}
	oldRecord := op.Record(gtx.Ops)
	oldDimensions := child(gtx.Disabled(), swap.previous)
	oldCall := oldRecord.Stop()
	newRecord := op.Record(gtx.Ops)
	newDimensions := child(gtx, swap.current)
	newCall := newRecord.Stop()
	risePX := float32(gtx.Dp(rise))
	drawFadedOffset(gtx.Ops, oldCall, 1-progress, image.Pt(0, -int(math.Round(float64(risePX*progress)))))
	drawFadedOffset(gtx.Ops, newCall, progress, image.Pt(0, int(math.Round(float64(risePX*(1-progress))))))
	return maxDimensions(oldDimensions, newDimensions)
}

type Morph struct {
	Preview kit.State

	current, previous *widget.Icon
	ready, moving     bool
	tween             anim.Tween
}

func (morph *Morph) Layout(
	gtx layout.Context,
	th *Theme,
	icon *widget.Icon,
	ink color.NRGBA,
	size unit.Dp,
) layout.Dimensions {
	pixels := max(0, gtx.Dp(size))
	if !morph.ready {
		morph.current, morph.ready = icon, true
		return drawMorphIcon(gtx, icon, ink, pixels, 1, 1)
	}
	if icon != morph.current {
		morph.previous, morph.current = morph.current, icon
		morph.tween = anim.Tween{Duration: th.Motion.Notch, Ease: anim.EaseChisel}
		morph.tween.Set(gtx, 0)
		morph.tween.Set(gtx, 1)
		morph.moving = th.Motion.Notch > 0 && !anim.Reduced()
	}
	if morph.Preview != kit.StateRest || !morph.moving {
		return drawMorphIcon(gtx, morph.current, ink, pixels, 1, 1)
	}
	progress := morph.tween.Value(gtx)
	if progress >= 1 {
		morph.previous, morph.moving = nil, false
		return drawMorphIcon(gtx, morph.current, ink, pixels, 1, 1)
	}
	drawMorphIcon(gtx, morph.previous, ink, pixels, 1-.15*progress, 1-progress)
	return drawMorphIcon(gtx, morph.current, ink, pixels, .85+.15*progress, progress)
}

type Unfold struct {
	Preview kit.State

	open  bool
	ready bool
	tween anim.Tween
}

func (unfold *Unfold) Layout(
	gtx layout.Context,
	th *Theme,
	open bool,
	child layout.Widget,
) layout.Dimensions {
	unfold.tween.Duration = th.Motion.Unfold
	unfold.tween.Ease = anim.EaseChisel
	if !unfold.ready {
		initial := float32(0)
		if open {
			initial = 1
		}
		unfold.tween.Set(gtx, initial)
		unfold.open, unfold.ready = open, true
	} else if unfold.open != open {
		unfold.open = open
		if open {
			unfold.tween.Set(gtx, 1)
		} else {
			unfold.tween.Set(gtx, 0)
		}
	}
	progress := unfold.tween.Value(gtx)
	if unfold.Preview != kit.StateRest {
		progress = 1
	}
	if progress <= 0 {
		return layout.Dimensions{}
	}
	record := op.Record(gtx.Ops)
	contentGTX := gtx
	contentGTX.Constraints.Min.Y = 0
	if !open && progress < 1 {
		contentGTX = contentGTX.Disabled()
	}
	content := child(contentGTX)
	call := record.Stop()
	height := int(math.Round(float64(float32(content.Size.Y) * progress)))
	height = max(0, min(content.Size.Y, height))
	area := clip.Rect(image.Rect(0, 0, content.Size.X, height)).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	call.Add(gtx.Ops)
	opacity.Pop()
	area.Pop()
	return layout.Dimensions{Size: image.Pt(content.Size.X, height)}
}

type Rise struct {
	key    string
	ready  bool
	moving bool
	tween  anim.Tween
}

func (rise *Rise) Arrive() { rise.ready, rise.key = true, "\x00" }

func (rise *Rise) Layout(
	gtx layout.Context,
	th *Theme,
	key string,
	distance unit.Dp,
	child layout.Widget,
) layout.Dimensions {
	if !rise.ready {
		rise.key, rise.ready = key, true
		return child(gtx)
	}
	if key != rise.key {
		rise.key = key
		rise.tween = anim.Tween{Duration: th.Motion.Navigate, Ease: anim.EaseChisel}
		rise.tween.Set(gtx, 0)
		rise.tween.Set(gtx, 1)
		rise.moving = th.Motion.Navigate > 0 && !anim.Reduced()
	}
	if !rise.moving {
		return child(gtx)
	}
	progress := rise.tween.Value(gtx)
	if progress >= 1 {
		rise.moving = false
		return child(gtx)
	}
	record := op.Record(gtx.Ops)
	dimensions := child(gtx)
	call := record.Stop()
	offset := image.Pt(0, int(math.Round(float64(float32(gtx.Dp(distance))*(1-progress)))))
	drawFadedOffset(gtx.Ops, call, progress, offset)
	return dimensions
}

// a change of key travels, the same key moving with a scroll or a resize is followed at once so the slab never lags
type Selection struct {
	key           string
	from, current image.Rectangle
	ready, moving bool
	tween         anim.Tween
}

func (selection *Selection) Rect(gtx layout.Context, key string, target image.Rectangle, duration time.Duration) image.Rectangle {
	if !selection.ready || anim.Reduced() || duration <= 0 {
		selection.key, selection.current = key, target
		selection.ready, selection.moving = true, false
		return target
	}
	if key != selection.key {
		selection.key, selection.from, selection.moving = key, selection.current, true
		selection.tween = anim.Tween{Duration: duration, Ease: anim.EaseChisel}
		selection.tween.Set(gtx, 0)
		selection.tween.Set(gtx, 1)
	}
	progress := float32(1)
	if selection.moving {
		progress = selection.tween.Value(gtx)
		selection.moving = progress < 1
	}
	selection.current = interpolateRect(selection.from, target, progress)
	return selection.current
}

func (selection *Selection) Moving() bool { return selection.moving }

func interpolateRect(from, to image.Rectangle, progress float32) image.Rectangle {
	mix := func(a, b int) int { return a + int(math.Round(float64(float32(b-a)*progress))) }
	return image.Rect(
		mix(from.Min.X, to.Min.X), mix(from.Min.Y, to.Min.Y),
		mix(from.Max.X, to.Max.X), mix(from.Max.Y, to.Max.Y),
	)
}

func drawMorphIcon(
	gtx layout.Context,
	icon *widget.Icon,
	ink color.NRGBA,
	size int,
	scale, alpha float32,
) layout.Dimensions {
	dimensions := layout.Dimensions{Size: image.Pt(size, size)}
	if icon == nil || alpha <= 0 {
		return dimensions
	}
	child := gtx
	child.Constraints = layout.Exact(dimensions.Size)
	opacity := paint.PushOpacity(gtx.Ops, min(alpha, 1))
	centre := f32.Pt(float32(size)/2, float32(size)/2)
	transform := op.Affine(f32.AffineId().Scale(centre, f32.Pt(scale, scale))).Push(gtx.Ops)
	icon.Layout(child, ink)
	transform.Pop()
	opacity.Pop()
	return dimensions
}

// drawn even at nothing: without its handlers in the frame a page loses the keyboard it asked for while it arrives
func drawFadedOffset(ops *op.Ops, call op.CallOp, alpha float32, offset image.Point) {
	opacity := paint.PushOpacity(ops, min(max(alpha, 0), 1))
	move := op.Offset(offset).Push(ops)
	call.Add(ops)
	move.Pop()
	opacity.Pop()
}

func maxDimensions(first, second layout.Dimensions) layout.Dimensions {
	return layout.Dimensions{
		Size:     image.Pt(max(first.Size.X, second.Size.X), max(first.Size.Y, second.Size.Y)),
		Baseline: max(first.Baseline, second.Baseline),
	}
}

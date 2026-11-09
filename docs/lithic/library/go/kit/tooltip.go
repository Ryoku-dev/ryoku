package kit

import (
	"image"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Tooltip reveals a short description after sustained pointer hover.
type Tooltip struct {
	Preview State

	tag     struct{}
	hovered bool
	since   time.Time
	shown   bool
	enter   anim.Tween
}

// Layout draws widget in flow and the tooltip in the deferred overlay layer.
func (tooltip *Tooltip) Layout(gtx layout.Context, th *theme.Theme, tip string, widget layout.Widget) layout.Dimensions {
	for {
		value, ok := gtx.Event(pointer.Filter{Target: &tooltip.tag, Kinds: pointer.Enter | pointer.Leave})
		if !ok {
			break
		}
		pointerEvent, ok := value.(pointer.Event)
		if !ok {
			continue
		}
		switch pointerEvent.Kind {
		case pointer.Enter:
			tooltip.hovered = true
			tooltip.since = gtx.Now
		case pointer.Leave:
			tooltip.hovered = false
			tooltip.shown = false
		}
	}

	dimensions := widget(gtx)
	semantic.DescriptionOp(tip).Add(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	area := clip.Rect{Max: dimensions.Size}.Push(gtx.Ops)
	event.Op(gtx.Ops, &tooltip.tag)
	area.Pop()
	pass.Pop()
	preview := tooltip.Preview == StateHover || tooltip.Preview == StateFocus
	if tip == "" || (!tooltip.hovered && !preview) {
		return dimensions
	}
	readyAt := tooltip.since.Add(tokens.MotionTooltipDelay)
	if !preview && gtx.Now.Before(readyAt) {
		gtx.Execute(op.InvalidateCmd{At: readyAt})
		return dimensions
	}
	if !tooltip.shown {
		tooltip.enter = anim.Tween{Duration: tokens.MotionTooltipIn, Ease: anim.EaseChisel}
		tooltip.enter.Set(gtx, 0)
		tooltip.enter.Set(gtx, 1)
		tooltip.shown = true
	}
	progress := tooltip.enter.Value(gtx)

	tipContext := gtx
	tipContext.Constraints.Min = image.Point{}
	tipContext.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(tokens.SizeTooltipWidthMax))
	tipRecord := op.Record(gtx.Ops)
	tipDimensions := layout.Inset{
		Top: unit.Dp(tokens.SizeTooltipPaddingVertical), Bottom: unit.Dp(tokens.SizeTooltipPaddingVertical),
		Left: unit.Dp(tokens.SizeTooltipPaddingHorizontal), Right: unit.Dp(tokens.SizeTooltipPaddingHorizontal),
	}.Layout(tipContext, Text(th, th.T.Caption, th.R.Fg, tip))
	tipCall := tipRecord.Stop()

	viewport := image.Rect(0, -gtx.Constraints.Max.Y, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	if tooltip.Preview != StateRest {
		viewport = image.Rectangle{Max: gtx.Constraints.Max}
	}
	anchor := image.Rectangle{Max: dimensions.Size}
	position := placePopover(viewport, anchor, tipDimensions.Size, gtx.Dp(tokens.SizeTooltipAnchorGap))
	position.Y += int(float32(gtx.Dp(tokens.SizeTooltipTravel)) * (1 - progress))

	overlay := op.Record(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, progress)
	offset := op.Offset(position).Push(gtx.Ops)
	bounds := image.Rectangle{Max: tipDimensions.Size}
	FillCut(gtx.Ops, bounds, gtx.Dp(tokens.CutTooltip), th.R.Stone4)
	StrokeCut(gtx.Ops, bounds, gtx.Dp(tokens.CutTooltip), max(1, gtx.Dp(tokens.SizeHairline)), th.R.Vein)
	tipCall.Add(gtx.Ops)
	offset.Pop()
	opacity.Pop()
	presentOverlay(gtx, overlay.Stop(), tooltip.Preview != StateRest, gtx.Constraints.Max)
	return dimensions
}

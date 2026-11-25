package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Disclosure reveals optional content while making closing content inert.
type Disclosure struct {
	Open    bool
	Preview State
	reveal  anim.Tween
	ready   bool
}

// Layout grows and fades content using the unfold motion.
func (d *Disclosure) Layout(gtx layout.Context, th *theme.Theme, content layout.Widget) layout.Dimensions {
	d.reveal.Duration = th.Motion.Unfold
	d.reveal.Ease = anim.EaseChisel
	target := float32(0)
	if d.Open {
		target = 1
	}
	if !d.ready {
		d.ready = true
		d.reveal.Set(gtx, target)
	} else {
		d.reveal.Set(gtx, target)
	}
	progress := d.reveal.Value(gtx)
	if !d.Open && progress <= 0 {
		return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Point{})}
	}

	contentGTX := gtx
	contentGTX.Constraints.Min.Y = 0
	if !d.Open {
		contentGTX = contentGTX.Disabled()
	}
	record := op.Record(gtx.Ops)
	semantic.DescriptionOp("Expanded content").Add(gtx.Ops)
	dimensions := content(contentGTX)
	call := record.Stop()
	visible := min(dimensions.Size.Y, max(0, int(float32(dimensions.Size.Y)*progress+.5)))
	viewport := clip.Rect(image.Rect(0, 0, dimensions.Size.X, visible)).Push(gtx.Ops)
	if progress < 1 {
		opacity := paint.PushOpacity(gtx.Ops, progress)
		call.Add(gtx.Ops)
		opacity.Pop()
	} else {
		call.Add(gtx.Ops)
	}
	viewport.Pop()
	return layout.Dimensions{Size: image.Pt(dimensions.Size.X, visible), Baseline: dimensions.Baseline}
}

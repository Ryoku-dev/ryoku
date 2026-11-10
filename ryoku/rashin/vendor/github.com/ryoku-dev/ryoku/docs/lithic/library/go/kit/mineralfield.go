package kit

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// MineralField is the tone-aware plane behind a selected resource header.
type MineralField struct {
	tone        palette.Tone
	from, to    color.NRGBA
	blend       anim.Tween
	initialized bool
}

// Layout fades state changes and snaps palette changes.
func (field *MineralField) Layout(gtx layout.Context, th *theme.Theme, tone palette.Tone, body layout.Widget) layout.Dimensions {
	target := th.R.Field(tone)
	field.blend.Duration = th.Motion.Field
	field.blend.Ease = anim.EaseChisel
	if !field.initialized {
		field.tone, field.from, field.to, field.initialized = tone, target, target, true
		field.blend.Set(gtx, 1)
	} else if field.tone != tone {
		field.from = palette.Mix(field.from, field.to, field.blend.Value(gtx))
		field.to, field.tone = target, tone
		field.blend = anim.Tween{Duration: th.Motion.Field, Ease: anim.EaseChisel}
		field.blend.Set(gtx, 0)
		field.blend.Set(gtx, 1)
	} else if target != field.to {
		field.from, field.to = target, target
	}
	ink := palette.Mix(field.from, field.to, field.blend.Value(gtx))
	record := op.Record(gtx.Ops)
	if gtx.Constraints.Max.X > 0 {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
	}
	var dimensions layout.Dimensions
	if body != nil {
		dimensions = body(gtx)
	} else {
		dimensions.Size = gtx.Constraints.Min
	}
	call := record.Stop()
	FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, 0, ink)
	call.Add(gtx.Ops)
	return dimensions
}

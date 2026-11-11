package inbox

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

// a tablet that holds more than a row: a report's preview, a task at work
type tile struct {
	click kit.Click
	hover anim.Tween

	// a task at work's presence, whose caption cross-fades as it changes
	presence teamsui.Presence
}

func (t *tile) clicked(gtx layout.Context) bool { return t.click.Clicked(gtx) }

// notch is the tone's stone, or none when its alpha is zero
func (t *tile) layout(gtx layout.Context, th *look.Theme, notch color.NRGBA, body layout.Widget) layout.Dimensions {
	hover := look.Transition(gtx, &t.hover, t.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
	record := op.Record(gtx.Ops)
	dimensions := t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(13).Layout(gtx, body)
	})
	call := record.Stop()
	bounds := image.Rectangle{Max: dimensions.Size}
	kit.FillCut(gtx.Ops, bounds, gtx.Dp(th.Cut.Small), palette.Mix(th.R.Stone2, th.R.Stone3, hover))
	if notch.A > 0 {
		kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), notch)
	}
	call.Add(gtx.Ops)
	return dimensions
}

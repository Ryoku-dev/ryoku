package inbox

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"

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

// a tile lifts off the pane as a slab does; notch is the tone's stone, or none when its alpha is zero
func (t *tile) layout(gtx layout.Context, th *look.Theme, notch color.NRGBA, body layout.Widget) layout.Dimensions {
	hover := look.Transition(gtx, &t.hover, t.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
	record := op.Record(gtx.Ops)
	dimensions := t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.UniformInset(13).Layout(gtx, body)
	})
	call := record.Stop()
	bounds, cut := image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small)
	kit.FillCut(gtx.Ops, bounds, cut, palette.Mix(th.R.Slab, th.R.Fg, tokens.AlphaHoverPlane*hover))
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	if notch.A > 0 {
		kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), notch)
	}
	call.Add(gtx.Ops)
	return dimensions
}

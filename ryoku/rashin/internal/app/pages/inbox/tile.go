package inbox

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
)

// a row that holds more than a line: a report's preview, a task at work
type tile struct {
	click kit.Click
	hover anim.Tween

	// a task at work's presence, whose caption cross-fades as it changes
	presence teamsui.Presence
}

func (t *tile) clicked(gtx layout.Context) bool { return t.click.Clicked(gtx) }

// a section lies open on the pane, so its rows have no plane at rest and light only under the pointer
func (t *tile) layout(gtx layout.Context, th *look.Theme, body layout.Widget) layout.Dimensions {
	hover := look.Transition(gtx, &t.hover, t.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
	record := op.Record(gtx.Ops)
	dimensions := t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: th.Sp.S2, Right: th.Sp.S2}.Layout(gtx, body)
	})
	call := record.Stop()
	if hover > 0 {
		kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Fg, tokens.AlphaHoverPlane*hover))
	}
	call.Add(gtx.Ops)
	return dimensions
}

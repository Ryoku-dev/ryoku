package shell

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
)

// a menu in everything but its width, which a menu caps
type paletteList struct {
	rows []paletteRow
	// the row under the pointer, so moving onto a row moves the cursor once and the arrow keys stay free
	hovered int
}

type paletteRow struct {
	click kit.Click
	hover anim.Tween
}

func (list *paletteList) Layout(
	gtx layout.Context, th *look.Theme, actions []paletteAction, cursor int,
) (dimensions layout.Dimensions, pressed, entered int) {
	if len(list.rows) < len(actions) {
		list.rows = append(list.rows, make([]paletteRow, len(actions)-len(list.rows))...)
	}
	width := gtx.Constraints.Max.X
	pad, row, caption := gtx.Dp(6), gtx.Dp(40), gtx.Dp(28)
	pressed, entered = -1, -1
	hovered := -1

	record := op.Record(gtx.Ops)
	y := pad
	group := ""
	for index, action := range actions {
		if action.group != group {
			group = action.group
			look.Place(gtx, image.Rect(pad, y, width-pad, y+caption), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Point{}
				look.MiddleLeft(gtx, gtx.Dp(10), caption, kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, group))
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
			y += caption
		}
		state := &list.rows[index]
		if state.click.Clicked(gtx) {
			pressed = index
		}
		if state.click.Hovered() {
			hovered = index
		}
		look.Place(gtx, image.Rect(pad, y, width-pad, y+row), func(gtx layout.Context) layout.Dimensions {
			return state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				semantic.Button.Add(gtx.Ops)
				semantic.LabelOp(action.label).Add(gtx.Ops)
				drawPaletteRow(gtx, th, state, action, index == cursor)
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		})
		y += row
	}
	if len(actions) == 0 {
		look.Place(gtx, image.Rect(pad, y, width-pad, y+row), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = image.Point{}
			look.MiddleLeft(gtx, gtx.Dp(10), row, kit.RawText(th.Theme, th.T.Label, th.R.Fg3, i18n.T("Nothing matches")))
			return layout.Dimensions{Size: gtx.Constraints.Max}
		})
		y += row
	}
	y += pad
	call := record.Stop()

	bounds := image.Rect(0, 0, width, y)
	cut := gtx.Dp(th.Cut.Menu)
	kit.FillCut(gtx.Ops, bounds, cut, th.R.Stone4)
	kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
	call.Add(gtx.Ops)

	if hovered != list.hovered {
		list.hovered, entered = hovered, hovered
	}
	return layout.Dimensions{Size: bounds.Max}, pressed, entered
}

func drawPaletteRow(gtx layout.Context, th *look.Theme, state *paletteRow, action paletteAction, raised bool) {
	size := gtx.Constraints.Max
	hover := look.Transition(gtx, &state.hover, raised, th.Motion.HoverIn, th.Motion.HoverOut)
	if hover > 0 {
		kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.Mix(th.R.Stone4, th.R.Fg, .08*hover))
	}
	inset, glyph := gtx.Dp(10), gtx.Dp(20)
	look.PlaceIcon(gtx, action.icon, image.Pt(inset, (size.Y-glyph)/2), 20, palette.Mix(th.R.Fg2, th.R.Fg, hover))
	left := inset + glyph + gtx.Dp(12)
	content := gtx
	content.Constraints = layout.Constraints{Max: image.Pt(size.X-left-inset, size.Y)}
	look.MiddleLeft(content, left, size.Y, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.RawText(th.Theme, th.T.Label, th.R.Fg, action.label)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, action.kind)),
		)
	})
}

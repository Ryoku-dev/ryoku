package projects

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ryoku-rashin/internal/app/look"
)

const (
	listWidth    unit.Dp = 380
	listStacked  unit.Dp = 280
	detailNarrow unit.Dp = 560
)

type splitPane struct {
	List, Stacked unit.Dp
}

func (s splitPane) Layout(gtx layout.Context, th *look.Theme, list, detail layout.Widget) layout.Dimensions {
	width, stacked := s.List, s.Stacked
	if width == 0 {
		width = listWidth
	}
	if stacked == 0 {
		stacked = listStacked
	}
	if gtx.Constraints.Max.X < gtx.Dp(look.SplitFrom) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(stacked))
				return list(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
			layout.Flexed(1, detail),
		)
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			pixels := gtx.Dp(width)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = pixels, pixels
			return list(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(max(1, gtx.Dp(1)), gtx.Constraints.Max.Y)
			look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
			return layout.Dimensions{Size: size}
		}),
		layout.Flexed(1, detail),
	)
}

func detailColumn(gtx layout.Context, th *look.Theme, top, bottom unit.Dp, part layout.Widget) layout.Dimensions {
	inset := gtx.Dp(th.Sp.S5)
	if gtx.Constraints.Max.X < gtx.Dp(detailNarrow) {
		inset = gtx.Dp(th.Sp.S4)
	}
	full := gtx.Constraints.Max.X
	width := min(full-2*inset, gtx.Dp(look.ReadingColumnWidth))
	if width <= 0 {
		return layout.Dimensions{}
	}
	left := inset
	above, below := gtx.Dp(top), gtx.Dp(bottom)
	column := gtx
	column.Constraints = layout.Constraints{Max: image.Pt(width, gtx.Constraints.Max.Y)}
	offset := op.Offset(image.Pt(left, above)).Push(gtx.Ops)
	dimensions := part(column)
	offset.Pop()
	return layout.Dimensions{Size: image.Pt(full, above+dimensions.Size.Y+below)}
}

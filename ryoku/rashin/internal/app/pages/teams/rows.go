package teams

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

// a flat row that opens what it names, lit under the pointer
type linkRow struct {
	click kit.Click
	hover anim.Tween
}

type linkRows map[string]*linkRow

func (rows *linkRows) row(id string) *linkRow {
	if *rows == nil {
		*rows = make(linkRows)
	}
	row := (*rows)[id]
	if row == nil {
		row = new(linkRow)
		(*rows)[id] = row
	}
	return row
}

func (row *linkRow) layout(gtx layout.Context, th *look.Theme, child layout.Widget) layout.Dimensions {
	lit := look.Transition(gtx, &row.hover, row.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
	return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		record := op.Record(gtx.Ops)
		dimensions := layout.Inset{Top: th.Sp.S1, Bottom: th.Sp.S1, Left: th.Sp.S2, Right: th.Sp.S2}.Layout(gtx, child)
		call := record.Stop()
		if lit > 0 {
			kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Fg, .08*lit))
		}
		call.Add(gtx.Ops)
		return dimensions
	})
}

// rows of one section stand 4 apart
func stack(th *look.Theme, rows []layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 2*len(rows))
		for index, row := range rows {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout))
			}
			children = append(children, layout.Rigid(row))
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

package settings

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"

	"ryoku-rashin/internal/app/look"
)

type foldRow struct {
	click   kit.Click
	hover   look.Hover
	chevron look.Morph
	unfold  look.Unfold
}

type foldHead struct {
	Title, Detail string
	// sets the detail in mono, for a command or a path
	Mono  bool
	Trail layout.Widget
}

func (row *foldRow) toggled(gtx layout.Context) bool { return row.click.Clicked(gtx) }

func (row *foldRow) layout(gtx layout.Context, th *look.Theme, open bool, head foldHead, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.layoutHead(gtx, th, open, head) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.unfold.Layout(gtx, th, open, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S3, Right: 13, Bottom: th.Sp.S4, Left: 13}.Layout(gtx, body)
			})
		}),
	)
}

func (row *foldRow) layoutHead(gtx layout.Context, th *look.Theme, open bool, head foldHead) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(look.SideRowTall))
	gtx.Constraints = layout.Exact(size)
	dimensions, hover := row.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: size} })
	})
	fill := hover
	if open {
		fill = 1
	}
	if fill > 0 {
		kit.FillCut(gtx.Ops, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(palette.Mix(th.R.Slab, th.R.Fg, tokens.AlphaHoverPlane), fill))
	}
	inset := gtx.Dp(13)
	content := gtx
	content.Constraints = layout.Constraints{Max: image.Pt(size.X-2*inset, size.Y)}
	look.MiddleLeft(content, inset, size.Y, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		icon := look.Icon.ExpandMore
		if open {
			icon = look.Icon.ExpandLess
		}
		detail := th.T.Detail
		if head.Mono {
			detail = th.T.MonoSm
		}
		children := []layout.FlexChild{
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.RawText(th.Theme, th.T.Row, th.R.Fg, head.Title)),
					layout.Rigid(kit.RawText(th.Theme, detail, th.R.Fg2, head.Detail)),
				)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		}
		if head.Trail != nil {
			children = append(children, layout.Rigid(head.Trail), layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row.chevron.Layout(gtx, th, icon, th.R.Fg3, 18)
		}))
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
	return dimensions
}

// rows reach close to the slab's edges so the stone under the pointer reads as the row
func foldSlab(gtx layout.Context, th *look.Theme, count int, row func(layout.Context, int) layout.Dimensions) layout.Dimensions {
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(8)
	record := op.Record(gtx.Ops)
	y := pad
	for index := range count {
		offset := op.Offset(image.Pt(pad, y)).Push(gtx.Ops)
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(width-2*pad, 0), Max: image.Pt(width-2*pad, gtx.Constraints.Max.Y)}
		y += row(child, index).Size.Y
		offset.Pop()
	}
	y += pad
	call := record.Stop()
	kit.SlabSurface(gtx, th.Theme, image.Rect(0, 0, width, y), gtx.Dp(th.Cut.Mid), th.R.Slab)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, y)}
}

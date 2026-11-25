package settings

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

type foldRow struct {
	click   kit.Click
	hover   look.Hover
	chevron look.Morph
	unfold  kit.Disclosure
}

type foldHead struct {
	Title, Detail string
	// sets the detail in mono, for a command or a path
	Mono  bool
	Mark  look.Mark
	Trail layout.Widget
}

func (row *foldRow) toggled(gtx layout.Context) bool { return row.click.Clicked(gtx) }

func (row *foldRow) layout(gtx layout.Context, th *look.Theme, open bool, head foldHead, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.layoutHead(gtx, th, open, head) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row.unfold.Open = open
			return row.unfold.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S3, Right: th.Sp.S3, Bottom: th.Sp.S4, Left: th.Sp.S3}.Layout(gtx, body)
			})
		}),
	)
}

func (row *foldRow) layoutHead(gtx layout.Context, th *look.Theme, open bool, head foldHead) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(look.SideRowTall))
	gtx.Constraints = layout.Exact(size)
	dimensions, hover := row.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// The visible title is drawn after the hover surface. Associate
			// it with this actual click receiver, not only that text sibling.
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(head.Title).Add(gtx.Ops)
			semantic.SelectedOp(open).Add(gtx.Ops)
			return layout.Dimensions{Size: size}
		})
	})
	fill := hover
	if open {
		fill = 1
	}
	if fill > 0 {
		kit.Fill(gtx, th.Theme, image.Rectangle{Max: size}, gtx.Dp(th.Cut.Small), palette.WithAlpha(th.R.Hover(), fill))
	}
	inset := gtx.Dp(th.Sp.S3)
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
		var children []layout.FlexChild
		if head.Mark.ID != "" {
			ink := th.R.Fg2
			if open || row.click.Hovered() {
				ink = th.R.Fg
			}
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Logo(gtx, th, head.Mark, 20, ink) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			)
		}
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(kit.RawText(th.Theme, th.T.Row, th.R.Fg, head.Title)),
					layout.Rigid(kit.RawText(th.Theme, detail, th.R.Fg2, head.Detail)),
				)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		)
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
	pad := gtx.Dp(th.Sp.S2)
	record := op.Record(gtx.Ops)
	y := pad
	for index := range count {
		offset := op.Offset(image.Pt(pad, y)).Push(gtx.Ops)
		child := gtx
		// the expanded manager may be taller than one viewport; the outer scroller clips it
		child.Constraints = layout.Constraints{Min: image.Pt(width-2*pad, 0), Max: image.Pt(width-2*pad, 1<<20)}
		y += row(child, index).Size.Y
		offset.Pop()
	}
	y += pad
	call := record.Stop()
	kit.SlabSurface(gtx, th.Theme, image.Rect(0, 0, width, y), gtx.Dp(th.Cut.Mid), th.R.Slab)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, y)}
}

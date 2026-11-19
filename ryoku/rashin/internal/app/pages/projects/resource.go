package projects

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

type resourceRow struct {
	click kit.Click
	hover look.Hover
	press anim.Tween
}

type resourceContent struct {
	Title, Facts, Claim, Proof string
	Tone                       palette.Tone
	Mark                       *widget.Icon
	Action                     *kit.Plaque
}

func (row *resourceRow) Clicked(gtx layout.Context) bool { return row.click.Clicked(gtx) }

func (row *resourceRow) Layout(gtx layout.Context, th *look.Theme, content resourceContent, selected bool) layout.Dimensions {
	width, height := gtx.Constraints.Max.X, gtx.Dp(76)
	if width <= 0 {
		return layout.Dimensions{}
	}
	size := image.Pt(width, height)
	gtx.Constraints = layout.Exact(size)
	_, hover := row.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: size}
		})
	})
	press := look.Transition(gtx, &row.press, row.click.Pressed(), th.Motion.Press, th.Motion.Press)
	look.RowWash(gtx, th, image.Rectangle{Max: size}, hover, press, selected)
	kit.ToneMark(gtx, th.Theme, image.Rectangle{Max: size}, content.Tone)
	ink, quiet := look.RowInk(th, selected)
	left := gtx.Dp(th.Sp.S3)
	if content.Mark != nil {
		look.PlaceIcon(gtx, content.Mark, image.Pt(left, (height-gtx.Dp(18))/2), 18, quiet)
		left += gtx.Dp(18) + gtx.Dp(th.Sp.S2)
	}
	right := width - gtx.Dp(th.Sp.S3)
	if content.Action != nil {
		record := op.Record(gtx.Ops)
		button := gtx
		button.Constraints.Min = image.Point{}
		dimensions := content.Action.Layout(button, th.Theme)
		call := record.Stop()
		right -= dimensions.Size.X
		offset := op.Offset(image.Pt(right, (height-dimensions.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		right -= gtx.Dp(th.Sp.S2)
	}
	if content.Proof != "" {
		proof := gtx
		proof.Constraints = layout.Constraints{Max: image.Pt(max(0, right-left), height)}
		record := op.Record(gtx.Ops)
		dimensions := kit.RawText(th.Theme, th.T.MonoSm, quiet, content.Proof)(proof)
		call := record.Stop()
		right -= dimensions.Size.X
		offset := op.Offset(image.Pt(right, (height-dimensions.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		right -= gtx.Dp(th.Sp.S2)
	}
	text := gtx
	text.Constraints = layout.Constraints{Max: image.Pt(max(0, right-left), height)}
	look.MiddleLeft(text, left, height, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.RawText(th.Theme, th.T.Body, ink, content.Title)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, quiet, content.Facts)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if content.Claim == "" {
					return layout.Dimensions{}
				}
				return kit.RawText(th.Theme, th.T.Detail, quiet, content.Claim)(gtx)
			}),
		)
	})
	return layout.Dimensions{Size: size}
}

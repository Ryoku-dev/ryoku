package look

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

func Place(gtx layout.Context, rectangle image.Rectangle, widget layout.Widget) layout.Dimensions {
	if rectangle.Dx() <= 0 || rectangle.Dy() <= 0 {
		return layout.Dimensions{}
	}
	offset := op.Offset(rectangle.Min).Push(gtx.Ops)
	child := gtx
	child.Constraints = layout.Exact(rectangle.Size())
	dimensions := widget(child)
	offset.Pop()
	return dimensions
}

func PlaceIcon(gtx layout.Context, icon *widget.Icon, at image.Point, size unit.Dp, ink color.NRGBA) {
	if icon == nil {
		return
	}
	pixels := gtx.Dp(size)
	offset := op.Offset(at).Push(gtx.Ops)
	child := gtx
	child.Constraints = layout.Exact(image.Pt(pixels, pixels))
	icon.Layout(child, ink)
	offset.Pop()
}

func IconWidget(icon *widget.Icon, size unit.Dp, ink color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		pixels := gtx.Dp(size)
		if icon == nil {
			return layout.Dimensions{Size: image.Pt(pixels, pixels)}
		}
		gtx.Constraints = layout.Exact(image.Pt(pixels, pixels))
		return icon.Layout(gtx, ink)
	}
}

func FillRect(gtx layout.Context, rectangle image.Rectangle, ink color.NRGBA) {
	paint.FillShape(gtx.Ops, ink, clip.Rect(rectangle).Op())
}

func Paragraph(th *Theme, style theme.TextStyle, ink color.NRGBA, value string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return kit.Label{Style: style, Color: ink}.Layout(gtx, th.Theme, value)
	}
}

func MiddleLeft(gtx layout.Context, x, height int, widget layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	child := gtx
	child.Constraints.Min = image.Point{}
	dimensions := widget(child)
	call := record.Stop()
	offset := op.Offset(image.Pt(x, (height-dimensions.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return dimensions
}

func transition(gtx layout.Context, tween *anim.Tween, on bool, in, out time.Duration) float32 {
	tween.Ease = anim.EaseChisel
	if on {
		tween.Duration = in
		tween.Set(gtx, 1)
	} else {
		tween.Duration = out
		tween.Set(gtx, 0)
	}
	return tween.Value(gtx)
}

func Transition(gtx layout.Context, tween *anim.Tween, on bool, in, out time.Duration) float32 {
	return transition(gtx, tween, on, in, out)
}

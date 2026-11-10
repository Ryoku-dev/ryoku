package kit

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// BrandMark returns the Home mark in selection roles with a jasper pointer.
func BrandMark(th *theme.Theme, size unit.Dp) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		requested := gtx.Dp(size)
		dimensions := gtx.Constraints.Constrain(image.Pt(requested, requested))
		diameter := min(dimensions.X, dimensions.Y)
		if diameter <= 0 {
			return layout.Dimensions{Size: dimensions}
		}
		offset := image.Pt((dimensions.X-diameter)/2, (dimensions.Y-diameter)/2)
		bounds := image.Rectangle{Min: offset, Max: offset.Add(image.Pt(diameter, diameter))}
		cut := int(float32(diameter)*tokens.SizeBrandCutRatio + .5)
		FillCut(gtx.Ops, bounds, cut, th.R.Sel)

		center := f32.Pt(float32(bounds.Min.X)+float32(diameter)/2, float32(bounds.Min.Y)+float32(diameter)/2)
		apothem := float32(diameter) * tokens.SizeBrandOctagonApothemRatio
		points := brandOctagon(center, apothem)
		brandPolyline(gtx, points[:], true, float32(max(1, gtx.Dp(tokens.SizeBrandOutline))), th.R.OnSel)
		point := max(1, int(float32(diameter)*tokens.SizeBrandPointRatio))
		cx, cy := int(center.X), int(center.Y)
		paint.FillShape(gtx.Ops, th.R.OnSel, clip.Ellipse(image.Rect(cx-point, cy-point, cx+point+1, cy+point+1)).Op(gtx.Ops))

		scale := float32(diameter)
		var pointer clip.Path
		pointer.Begin(gtx.Ops)
		pointer.MoveTo(f32.Pt(center.X+scale*tokens.SizeBrandPointerNearRatio, center.Y+scale*tokens.SizeBrandPointerNearRatio))
		pointer.LineTo(f32.Pt(center.X+scale*tokens.SizeBrandPointerFarRatio, center.Y+scale*tokens.SizeBrandPointerMidRatio))
		pointer.LineTo(f32.Pt(center.X+scale*tokens.SizeBrandPointerMidRatio, center.Y+scale*tokens.SizeBrandPointerFarRatio))
		pointer.Close()
		paint.FillShape(gtx.Ops, th.R.Jasper, clip.Outline{Path: pointer.End()}.Op())
		return layout.Dimensions{Size: dimensions}
	}
}

func brandOctagon(center f32.Point, apothem float32) [8]f32.Point {
	corner := apothem * (float32(math.Sqrt2) - 1)
	return [8]f32.Point{
		f32.Pt(center.X-corner, center.Y-apothem), f32.Pt(center.X+corner, center.Y-apothem),
		f32.Pt(center.X+apothem, center.Y-corner), f32.Pt(center.X+apothem, center.Y+corner),
		f32.Pt(center.X+corner, center.Y+apothem), f32.Pt(center.X-corner, center.Y+apothem),
		f32.Pt(center.X-apothem, center.Y+corner), f32.Pt(center.X-apothem, center.Y-corner),
	}
}

func brandPolyline(gtx layout.Context, points []f32.Point, closed bool, width float32, ink color.NRGBA) {
	if len(points) < 2 || width <= 0 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(points[0])
	for _, point := range points[1:] {
		path.LineTo(point)
	}
	if closed {
		path.Close()
	}
	paint.FillShape(gtx.Ops, ink, clip.Stroke{Path: path.End(), Width: width}.Op())
}

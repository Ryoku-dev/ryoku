package kit

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// The style-aware painters. Each control hands them its Lithic cut, as the QML
// controls hand theirs to CutRect: Lithic cuts the corner by it, a round design
// (Material, Adwaita, Fluent) turns it into a radius through the design's rule
// (design.Style.Corner, Lithic.cornerRadius), so a shape whose shorter side is
// within the design's pill size becomes a pill. A cut of 0 stays a square
// corner in every design. In Lithic they delegate to the cut primitives, so
// Lithic output is the same as before. Angular marks (Notch, Blade, Flake,
// Hatch's lines, the tab blade) keep their own painters in every design.

// arcHandle places the cubic control points of a quarter circle.
const arcHandle = 0.5522847498

// cornerRadii returns the four corner radii, in pixels, that the cuts take in
// th's design, and whether the design rounds at all. A radius never exceeds
// half the shorter side, so a pill closes into a stadium even when the side is
// an odd number of pixels. The pill test compares pixels with the pill size as
// gtx.Dp rounds it, so a control laid out at exactly the pill size (a 22 dp
// toggle in Fluent at scale 1.75 is 39 px, 22.3 dp) stays a pill.
func cornerRadii(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, northwest, northeast, southeast, southwest int) ([4]float32, bool) {
	var radii [4]float32
	if th == nil || !th.Design.Profile().Round {
		return radii, false
	}
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}
	width, height := float32(max(0, bounds.Dx())), float32(max(0, bounds.Dy()))
	half := min(width, height) / 2
	pill := min(width, height) <= float32(gtx.Dp(unit.Dp(th.Design.Profile().Pill)))
	for index, cut := range [4]int{northwest, northeast, southeast, southwest} {
		switch {
		case cut <= 0:
		case pill:
			radii[index] = half
		default:
			radii[index] = min(th.Design.Corner(float32(cut)/scale, width/scale, height/scale)*scale, half)
		}
	}
	return radii, true
}

// appendRoundRect appends a closed rounded rectangle to path. Forward runs
// clockwise on screen; reverse runs the other way, so an inner ring cuts a hole
// under the non-zero rule.
func appendRoundRect(path *clip.Path, x0, y0, x1, y1 float32, radii [4]float32, reverse bool) {
	nw, ne, se, sw := radii[0], radii[1], radii[2], radii[3]
	k := float32(arcHandle)
	// the eight joints clockwise from the north-west corner's end, each with the
	// controls of the arc that ends there (a straight edge has none)
	type segment struct {
		to, c0, c1 f32.Point
		arc        bool
	}
	start := f32.Pt(x0+nw, y0)
	segments := [8]segment{
		{to: f32.Pt(x1-ne, y0)},
		{to: f32.Pt(x1, y0+ne), c0: f32.Pt(x1-ne+k*ne, y0), c1: f32.Pt(x1, y0+ne-k*ne), arc: ne > 0},
		{to: f32.Pt(x1, y1-se)},
		{to: f32.Pt(x1-se, y1), c0: f32.Pt(x1, y1-se+k*se), c1: f32.Pt(x1-se+k*se, y1), arc: se > 0},
		{to: f32.Pt(x0+sw, y1)},
		{to: f32.Pt(x0, y1-sw), c0: f32.Pt(x0+sw-k*sw, y1), c1: f32.Pt(x0, y1-sw+k*sw), arc: sw > 0},
		{to: f32.Pt(x0, y0+nw)},
		{to: start, c0: f32.Pt(x0, y0+nw-k*nw), c1: f32.Pt(x0+nw-k*nw, y0), arc: nw > 0},
	}
	path.MoveTo(start)
	if !reverse {
		for _, s := range segments {
			if s.arc {
				path.CubeTo(s.c0, s.c1, s.to)
			} else {
				path.LineTo(s.to)
			}
		}
		path.Close()
		return
	}
	for index := len(segments) - 1; index >= 0; index-- {
		from := start
		if index > 0 {
			from = segments[index-1].to
		}
		if s := segments[index]; s.arc {
			path.CubeTo(s.c1, s.c0, from)
		} else {
			path.LineTo(from)
		}
	}
	path.Close()
}

func roundPath(ops *op.Ops, bounds image.Rectangle, radii [4]float32) clip.PathSpec {
	var path clip.Path
	path.Begin(ops)
	appendRoundRect(&path, float32(bounds.Min.X), float32(bounds.Min.Y), float32(bounds.Max.X), float32(bounds.Max.Y), radii, false)
	return path.End()
}

// Shape returns the outline of bounds with the four corner cuts in th's
// design: CutPath's equivalent, for clips and for Hatch.
func Shape(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, northwest, northeast, southeast, southwest int) clip.PathSpec {
	radii, round := cornerRadii(gtx, th, bounds, northwest, northeast, southeast, southwest)
	if !round {
		return CutPath(gtx.Ops, bounds, northwest, northeast, southeast, southwest)
	}
	return roundPath(gtx.Ops, bounds, radii)
}

// Fill fills bounds with the same cut on every corner, in th's design.
func Fill(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut int, ink color.NRGBA) {
	FillCorners(gtx, th, bounds, cut, cut, cut, cut, ink)
}

// FillCorners fills bounds with independent corner cuts, in th's design.
func FillCorners(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, northwest, northeast, southeast, southwest int, ink color.NRGBA) {
	radii, round := cornerRadii(gtx, th, bounds, northwest, northeast, southeast, southwest)
	if !round {
		FillCutCorners(gtx.Ops, bounds, northwest, northeast, southeast, southwest, ink)
		return
	}
	if bounds.Empty() || ink.A == 0 {
		return
	}
	paint.FillShape(gtx.Ops, ink, clip.Outline{Path: roundPath(gtx.Ops, bounds, radii)}.Op())
}

// Stroke draws an even stroke wholly inside bounds with the same cut on every
// corner, in th's design.
func Stroke(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, cut, width int, ink color.NRGBA) {
	StrokeCorners(gtx, th, bounds, cut, cut, cut, cut, width, ink)
}

// StrokeCorners draws an even stroke wholly inside bounds with independent
// corner cuts, in th's design. A round stroke is the ring between the outline
// and the outline inset by width, whose radii shrink by width, as a QML
// Rectangle's border.
func StrokeCorners(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, northwest, northeast, southeast, southwest, width int, ink color.NRGBA) {
	radii, round := cornerRadii(gtx, th, bounds, northwest, northeast, southeast, southwest)
	if !round {
		StrokeCutCorners(gtx.Ops, bounds, northwest, northeast, southeast, southwest, width, ink)
		return
	}
	if bounds.Empty() || width <= 0 || ink.A == 0 {
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	appendRoundRect(&path, float32(bounds.Min.X), float32(bounds.Min.Y), float32(bounds.Max.X), float32(bounds.Max.Y), radii, false)
	if inner := bounds.Inset(width); !inner.Empty() {
		w := float32(width)
		var innerRadii [4]float32
		for index, radius := range radii {
			innerRadii[index] = max(0, radius-w)
		}
		appendRoundRect(&path, float32(inner.Min.X), float32(inner.Min.Y), float32(inner.Max.X), float32(inner.Max.Y), innerRadii, true)
	}
	paint.FillShape(gtx.Ops, ink, clip.Outline{Path: path.End()}.Op())
}

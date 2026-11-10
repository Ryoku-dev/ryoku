// Package kit provides the reusable Gio foundation and Lithic components.
package kit

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func clampCut(cut, width, height int) int {
	return min(max(cut, 0), min(width, height))
}

func cutPoints(bounds image.Rectangle, northwest, northeast, southeast, southwest int) [8]f32.Point {
	width, height := max(0, bounds.Dx()), max(0, bounds.Dy())
	northwest = clampCut(northwest, width, height)
	northeast = clampCut(northeast, width, height)
	southeast = clampCut(southeast, width, height)
	southwest = clampCut(southwest, width, height)
	x0, y0 := float32(bounds.Min.X), float32(bounds.Min.Y)
	x1, y1 := float32(bounds.Max.X), float32(bounds.Max.Y)
	return [8]f32.Point{
		f32.Pt(x0+float32(northwest), y0), f32.Pt(x1-float32(northeast), y0),
		f32.Pt(x1, y0+float32(northeast)), f32.Pt(x1, y1-float32(southeast)),
		f32.Pt(x1-float32(southeast), y1), f32.Pt(x0+float32(southwest), y1),
		f32.Pt(x0, y1-float32(southwest)), f32.Pt(x0, y0+float32(northwest)),
	}
}

func pathFromPoints(ops *op.Ops, points []f32.Point) clip.PathSpec {
	var path clip.Path
	path.Begin(ops)
	if len(points) > 0 {
		path.MoveTo(points[0])
		for _, point := range points[1:] {
			path.LineTo(point)
		}
		path.Close()
	}
	return path.End()
}

// CutPath returns a cut rectangle path with independent corner cuts.
func CutPath(ops *op.Ops, bounds image.Rectangle, northwest, northeast, southeast, southwest int) clip.PathSpec {
	points := cutPoints(bounds, northwest, northeast, southeast, southwest)
	return pathFromPoints(ops, points[:])
}

// FillCut fills a rectangle with the same cut on every corner.
func FillCut(ops *op.Ops, bounds image.Rectangle, cut int, ink color.NRGBA) {
	FillCutCorners(ops, bounds, cut, cut, cut, cut, ink)
}

// FillCutCorners fills a cut rectangle with independent corner cuts.
func FillCutCorners(ops *op.Ops, bounds image.Rectangle, northwest, northeast, southeast, southwest int, ink color.NRGBA) {
	if bounds.Empty() || ink.A == 0 {
		return
	}
	paint.FillShape(ops, ink, clip.Outline{Path: CutPath(ops, bounds, northwest, northeast, southeast, southwest)}.Op())
}

// FillCutTop fills a rectangle whose top corners alone are cut.
func FillCutTop(ops *op.Ops, bounds image.Rectangle, cut int, ink color.NRGBA) {
	FillCutCorners(ops, bounds, cut, cut, 0, 0, ink)
}

func polygonArea2(points []f32.Point) float32 {
	var area float32
	for index, point := range points {
		next := points[(index+1)%len(points)]
		area += point.X*next.Y - next.X*point.Y
	}
	return area
}

func appendPolygon(path *clip.Path, points []f32.Point, reverse bool) bool {
	if len(points) < 3 {
		return false
	}
	area := polygonArea2(points)
	if area == 0 {
		return false
	}
	forward := area > 0
	if reverse {
		forward = !forward
	}
	if forward {
		path.MoveTo(points[0])
		for _, point := range points[1:] {
			path.LineTo(point)
		}
	} else {
		path.MoveTo(points[len(points)-1])
		for index := len(points) - 2; index >= 0; index-- {
			path.LineTo(points[index])
		}
	}
	path.Close()
	return true
}

func insetCut(cut, width int) int {
	return max(0, int(math.Round(float64(cut)-float64(width)*(2-math.Sqrt2))))
}

// StrokeCut draws an even stroke wholly inside a cut rectangle.
func StrokeCut(ops *op.Ops, bounds image.Rectangle, cut, width int, ink color.NRGBA) {
	StrokeCutCorners(ops, bounds, cut, cut, cut, cut, width, ink)
}

// StrokeCutCorners draws an inside stroke with independent corner cuts.
func StrokeCutCorners(ops *op.Ops, bounds image.Rectangle, northwest, northeast, southeast, southwest, width int, ink color.NRGBA) {
	if bounds.Empty() || width <= 0 || ink.A == 0 {
		return
	}
	outer := cutPoints(bounds, northwest, northeast, southeast, southwest)
	var path clip.Path
	path.Begin(ops)
	appendPolygon(&path, outer[:], false)
	innerBounds := bounds.Inset(width)
	if !innerBounds.Empty() {
		inner := cutPoints(innerBounds, insetCut(northwest, width), insetCut(northeast, width), insetCut(southeast, width), insetCut(southwest, width))
		appendPolygon(&path, inner[:], true)
	}
	paint.FillShape(ops, ink, clip.Outline{Path: path.End()}.Op())
}

// Slab paints a cut background behind widget.
func Slab(gtx layout.Context, ink color.NRGBA, cut unit.Dp, widget layout.Widget) layout.Dimensions {
	record := op.Record(gtx.Ops)
	dimensions := widget(gtx)
	call := record.Stop()
	FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(cut), ink)
	call.Add(gtx.Ops)
	return dimensions
}

// Notch draws a right-triangle state mark from at.
func Notch(ops *op.Ops, at image.Point, size int, ink color.NRGBA) {
	if size <= 0 || ink.A == 0 {
		return
	}
	points := [...]f32.Point{
		f32.Pt(float32(at.X), float32(at.Y)),
		f32.Pt(float32(at.X+size), float32(at.Y)),
		f32.Pt(float32(at.X), float32(at.Y+size)),
	}
	paint.FillShape(ops, ink, clip.Outline{Path: pathFromPoints(ops, points[:])}.Op())
}

// Blade draws a bar with a 45 degree leading edge.
func Blade(ops *op.Ops, bounds image.Rectangle, ink color.NRGBA) {
	if bounds.Empty() || ink.A == 0 {
		return
	}
	cut := min(max(0, bounds.Dy()), max(0, bounds.Dx()))
	points := [...]f32.Point{
		f32.Pt(float32(bounds.Min.X), float32(bounds.Min.Y)),
		f32.Pt(float32(bounds.Max.X-cut), float32(bounds.Min.Y)),
		f32.Pt(float32(bounds.Max.X), float32(bounds.Max.Y)),
		f32.Pt(float32(bounds.Min.X), float32(bounds.Max.Y)),
	}
	paint.FillShape(ops, ink, clip.Outline{Path: pathFromPoints(ops, points[:])}.Op())
}

// Hatch engraves clipped 45 degree lines across bounds.
func Hatch(ops *op.Ops, area clip.PathSpec, bounds image.Rectangle, spacing, width int, ink color.NRGBA) {
	if bounds.Empty() || spacing <= 0 || width <= 0 || ink.A == 0 {
		return
	}
	stack := clip.Outline{Path: area}.Op().Push(ops)
	height := bounds.Dy()
	for x := bounds.Min.X - height; x <= bounds.Max.X; x += spacing {
		var path clip.Path
		path.Begin(ops)
		path.MoveTo(f32.Pt(float32(x), float32(bounds.Max.Y)))
		path.LineTo(f32.Pt(float32(x+height), float32(bounds.Min.Y)))
		paint.FillShape(ops, ink, clip.Stroke{Path: path.End(), Width: float32(width)}.Op())
	}
	stack.Pop()
}

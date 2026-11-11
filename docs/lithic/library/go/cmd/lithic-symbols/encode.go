package main

import (
	"golang.org/x/exp/shiny/iconvg"
	"golang.org/x/image/math/f32"
)

// encode writes the segments as IconVG in a 48 unit box centred on the origin, the box the
// shiny Material Design icons used, filled with the colour widget.Icon paints.
func encode(view [4]float64, paths [][]segment) ([]byte, error) {
	var encoder iconvg.Encoder
	encoder.Reset(iconvg.Metadata{
		ViewBox: iconvg.Rectangle{Min: f32.Vec2{-24, -24}, Max: f32.Vec2{24, 24}},
		Palette: iconvg.DefaultPalette,
	})
	scale := 48 / view[2]
	at := func(p point) (float32, float32) {
		return float32((p.x-view[0])*scale - 24), float32((p.y-view[1])*scale - 24)
	}
	// each SVG path stays its own IconVG path so overlapping paths paint over each other as they do in the SVG
	for _, segments := range paths {
		for index, step := range segments {
			switch step.op {
			case 'M':
				x, y := at(step.p[0])
				if index == 0 {
					encoder.StartPath(0, x, y)
				} else {
					// a move closes the subpath before it, which is how a filled SVG treats an open one too
					encoder.ClosePathAbsMoveTo(x, y)
				}
			case 'L':
				encoder.AbsLineTo(at(step.p[0]))
			case 'Q':
				x1, y1 := at(step.p[0])
				x, y := at(step.p[1])
				encoder.AbsQuadTo(x1, y1, x, y)
			case 'C':
				x1, y1 := at(step.p[0])
				x2, y2 := at(step.p[1])
				x, y := at(step.p[2])
				encoder.AbsCubeTo(x1, y1, x2, y2, x, y)
			}
		}
		encoder.ClosePathEndPath()
	}
	return encoder.Bytes()
}

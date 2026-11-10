package look

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/iconvg"
	"golang.org/x/image/math/f32"
)

// shapes wound the same way add, one wound the other way cuts a hole
type shape []float32

func box(x0, y0, x1, y1 float32) shape { return shape{x0, y0, x1, y0, x1, y1, x0, y1} }

func cut(x0, y0, x1, y1 float32) shape { return shape{x0, y0, x0, y1, x1, y1, x1, y0} }

// for the few meanings material has no icon for, drawn filled like that set so the two sit together in a row
func glyph(shapes ...shape) *widget.Icon {
	var encoder iconvg.Encoder
	encoder.Reset(iconvg.Metadata{
		ViewBox: iconvg.Rectangle{Min: f32.Vec2{-24, -24}, Max: f32.Vec2{24, 24}},
		Palette: iconvg.DefaultPalette,
	})
	// material icons use a 48 unit box centred on the origin
	at := func(value float32) float32 { return (value - 12) * 2 }
	// one path with every shape as a subpath, only then does a shape wound the other way cut a hole instead of painting over
	for index, outline := range shapes {
		if index == 0 {
			encoder.StartPath(0, at(outline[0]), at(outline[1]))
		} else {
			encoder.ClosePathAbsMoveTo(at(outline[0]), at(outline[1]))
		}
		for point := 2; point+1 < len(outline); point += 2 {
			encoder.AbsLineTo(at(outline[point]), at(outline[point+1]))
		}
	}
	encoder.ClosePathEndPath()
	data, err := encoder.Bytes()
	if err != nil {
		panic(err)
	}
	icon, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return icon
}

var (
	// a window with its left pane filled
	glyphSidebar = glyph(box(3, 4, 21, 20), cut(10, 6, 19, 18))
	// a filled screen with a prompt cut into it
	glyphTerminal = glyph(
		box(3, 4, 21, 20),
		shape{6.5, 9.4, 9.1, 12, 6.5, 14.6, 7.9, 16, 11.9, 12, 7.9, 8},
		cut(12.5, 14, 17.5, 16),
	)
	// a trunk with one branch leaving it
	glyphBranch = glyph(
		box(4, 3, 9, 8), box(4, 16, 9, 21), box(15, 5, 20, 10),
		box(5.5, 8, 7.5, 16), box(16.5, 10, 18.5, 13.5), box(7.5, 11.5, 16.5, 13.5),
	)
	// lines added over lines removed
	glyphDiff = glyph(box(11, 3.5, 13, 12.5), box(7.5, 7, 16.5, 9), box(7.5, 17, 16.5, 19))
	// a head with two eyes, ears and an antenna
	glyphAgent = glyph(
		box(4, 8, 20, 20), cut(8, 12, 10.5, 15.5), cut(13.5, 12, 16, 15.5),
		box(11, 4, 13, 8), box(2, 12, 4, 16), box(20, 12, 22, 16),
	)
)

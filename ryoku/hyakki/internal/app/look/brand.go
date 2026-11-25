package look

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"sync"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// The current Ryoku parent mark, shared with native packaging. Its original
// body and tip geometry is normalized to this viewBox without reference art.
//
//go:embed assets/ryoku.svg
var parentSVG []byte

var parentPaths = sync.OnceValue(func() [2][]svgPointOp {
	var asset struct {
		Paths []struct {
			ID   string `xml:"id,attr"`
			Data string `xml:"d,attr"`
		} `xml:"path"`
	}
	if err := xml.Unmarshal(parentSVG, &asset); err != nil {
		panic(fmt.Errorf("read embedded Ryoku mark: %w", err))
	}
	var paths [2][]svgPointOp
	for _, path := range asset.Paths {
		index := 0
		switch path.ID {
		case "body":
		case "tip":
			index = 1
		default:
			panic("unknown path in embedded Ryoku mark: " + path.ID)
		}
		parsed, err := parseSVGPath(path.Data)
		if err != nil {
			panic(fmt.Errorf("read embedded Ryoku %s path: %w", path.ID, err))
		}
		paths[index] = parsed
	}
	if len(paths[0]) == 0 || len(paths[1]) == 0 {
		panic("embedded Ryoku mark needs its body and tip")
	}
	return paths
})

func RyokuSeal(th *Theme, size unit.Dp) layout.Widget {
	return parentMark(size, th.R.Fg, th.R.Jasper)
}

func RyokuMark(size unit.Dp, ink color.NRGBA) layout.Widget {
	return parentMark(size, ink, ink)
}

func parentMark(size unit.Dp, body, tip color.NRGBA) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		pixels := max(0, gtx.Dp(size))
		box := image.Pt(pixels, pixels)
		if pixels == 0 {
			return layout.Dimensions{Size: box}
		}
		height := (pixels*482 + 278) / 556
		offset := op.Offset(image.Pt(0, (pixels-height)/2)).Push(gtx.Ops)
		paths := parentPaths()
		source, drawn := f32.Pt(556, 482), image.Pt(pixels, height)
		paint.FillShape(gtx.Ops, body, clip.Outline{Path: buildSVGPath(gtx.Ops, paths[0], source, drawn)}.Op())
		paint.FillShape(gtx.Ops, tip, clip.Outline{Path: buildSVGPath(gtx.Ops, paths[1], source, drawn)}.Op())
		offset.Pop()
		return layout.Dimensions{Size: box}
	}
}

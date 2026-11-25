package kit

import (
	"image"
	"image/color"
	"sync"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Registration sheet geometry, in dp (RegSheet.qml:10-44).
const (
	regCell   = 24
	regEvery  = 4
	regMargin = 14
	regPlus   = 3 // a plus's arm: 7 across
	regCross  = 6 // a register cross's arm: 13 across
)

// RegistrationSheet draws the sheet under a page as the Hub does (components/sheet/RegSheet.qml),
// read by no style: a faint 1 px dot every 24, a 7 px plus on every fourth crossing, and 13 px
// print-register crosses 14 in from each corner, all in fg (dots .07 dark / .10 light, pluses
// .13 / .16, crosses .32 / .36), each solved over the ground with palette.SRGBAlpha so it reads
// as faint as the Hub's. It fills gtx.Constraints.Max. The marks are recorded once per size,
// scale and inks and replayed, so a sheet at rest allocates nothing and asks for no frame.
func RegistrationSheet(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}
	key := regKey{size: size, scale: gtx.Metric.PxPerDp, ink: th.R.Fg, ground: th.R.Ground, dark: th.R.Dark}
	regSheets.Lock()
	sheet := regSheets.sheets[key]
	if sheet == nil {
		if regSheets.sheets == nil || len(regSheets.sheets) >= regSheetLimit {
			regSheets.sheets = make(map[regKey]*regSheet)
		}
		sheet = recordRegSheet(gtx, key)
		regSheets.sheets[key] = sheet
	}
	regSheets.Unlock()
	sheet.call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

type regKey struct {
	size   image.Point
	scale  float32
	ink    color.NRGBA
	ground color.NRGBA
	dark   bool
}

type regSheet struct {
	ops  op.Ops
	call op.CallOp
}

// regSheetLimit keeps the few sizes a window passes through while it is resized.
const regSheetLimit = 8

var regSheets struct {
	sync.Mutex
	sheets map[regKey]*regSheet
}

// regInks returns the dot, plus and register-cross inks (RegSheet.qml:24-25,39).
func regInks(key regKey) (dot, plus, cross color.NRGBA) {
	roles := palette.Roles{Fg: key.ink, Ground: key.ground, Dark: key.dark}
	return roles.Wash(0.07, 0.10), roles.Wash(0.13, 0.16), roles.Wash(0.32, 0.36)
}

func recordRegSheet(gtx layout.Context, key regKey) *regSheet {
	sheet := &regSheet{}
	ops := &sheet.ops
	macro := op.Record(ops)
	cell, hair := gtx.Dp(regCell), max(1, gtx.Dp(1))
	plusArm, crossArm, margin := gtx.Dp(regPlus), gtx.Dp(regCross), gtx.Dp(regMargin)
	dotInk, plusInk, crossInk := regInks(key)
	width, height := key.size.X, key.size.Y

	var dots, pluses clip.Path
	dots.Begin(ops)
	for gy := 1; cell > 0 && gy*cell < height; gy++ {
		for gx := 1; gx*cell < width; gx++ {
			x, y := gx*cell, gy*cell
			if gx%regEvery != 0 || gy%regEvery != 0 {
				addRect(&dots, image.Rect(x, y, x+hair, y+hair))
			}
		}
	}
	fillPath(ops, dots.End(), dotInk)

	pluses.Begin(ops)
	for gy := regEvery; cell > 0 && gy*cell < height; gy += regEvery {
		for gx := regEvery; gx*cell < width; gx += regEvery {
			x, y := gx*cell, gy*cell
			addRect(&pluses, image.Rect(x-plusArm, y, x+plusArm+hair, y+hair))
			addRect(&pluses, image.Rect(x, y-plusArm, x+hair, y+plusArm+hair))
		}
	}
	fillPath(ops, pluses.End(), plusInk)

	var crosses clip.Path
	crosses.Begin(ops)
	for _, at := range [4]image.Point{{margin, margin}, {width - margin, margin}, {margin, height - margin}, {width - margin, height - margin}} {
		addRect(&crosses, image.Rect(at.X-crossArm, at.Y, at.X+crossArm+hair, at.Y+hair))
		addRect(&crosses, image.Rect(at.X, at.Y-crossArm, at.X+hair, at.Y+crossArm+hair))
	}
	fillPath(ops, crosses.End(), crossInk)
	sheet.call = macro.Stop()
	return sheet
}

// fillPath paints path in ink under the non-zero rule.
func fillPath(ops *op.Ops, path clip.PathSpec, ink color.NRGBA) {
	paint.FillShape(ops, ink, clip.Outline{Path: path}.Op())
}

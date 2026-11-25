package render

import (
	"image"
	"math"

	"ryoku-rashin/internal/oni/strand"
)

// The mic mark: a small plate at the sprite's bottom-right corner carrying a
// microphone, drawn while the microphone is open between presses (the wake
// phrase or open mic). It is a fraction of the sprite, never smaller than
// micMarkMinPx, logical px.
const (
	micMarkFrac  = 0.3
	micMarkMinPx = 14.0
	// micMarkCorner is the plate's corner size before the design's corner
	// rule, as a fraction of its side
	micMarkCorner = 0.3
	// micMarkRoundBorderPx is the round designs' border; Lithic draws its
	// hairline
	micMarkRoundBorderPx = 1.5
)

// MicMarkRect is where the mic mark sits in a sprite ext logical px big:
// its bottom-right corner, sprite-local logical px.
func MicMarkRect(ext image.Point) strand.Rect {
	side := math.Round(max(micMarkMinPx, float64(min(ext.X, ext.Y))*micMarkFrac))
	side = min(side, float64(min(ext.X, ext.Y)))
	return strand.Rect{X: float64(ext.X) - side, Y: float64(ext.Y) - side, W: side, H: side}
}

// PaintMicMark draws the mic mark over dst at r (logical px, relative to
// dst's top-left) on a device scale of dpr, in the look's design shape:
// Lithic cuts the plate's corners and its microphone's, the round designs
// round them. The plate is White with an Ink border and the microphone wears
// the listening accent (Ink when the look has none), so every colour comes
// from the desktop's palette roles. It returns the device rectangle painted.
func PaintMicMark(dst *image.NRGBA, r strand.Rect, look Look, dpr float64) image.Rectangle {
	if dst.Bounds().Empty() || !(dpr > 0) || !(r.W > 0) {
		return image.Rectangle{}
	}
	b := dst.Bounds()
	card := strand.Rect{X: float64(b.Min.X) + r.X*dpr, Y: float64(b.Min.Y) + r.Y*dpr, W: r.W * dpr, H: r.H * dpr}
	round := look.Design.Profile().Round
	painted := fillPolys(dst, [][]point{plate(card, look.Design, r.W*micMarkCorner, dpr)}, look.White, 1)
	border, join := micMarkRoundBorderPx, "round"
	if !round {
		border, join = hairlinePx, "miter"
	}
	bw := border * dpr
	inset := strand.Rect{X: card.X + bw/2, Y: card.Y + bw/2, W: card.W - bw, H: card.H - bw}
	ring := strokePieces([]contour{{pts: plate(inset, look.Design, r.W*micMarkCorner-border/2, dpr), closed: true}}, strokeStyle{width: bw, join: join, cap: "butt", miterLimit: 4}, perchTol)
	fillPolys(dst, ring, look.Ink, 1)

	ink, ok := look.accent("listening")
	if !ok {
		ink = look.Ink
	}
	s := card.W
	cx := card.X + s/2
	// the capsule
	w, h := 0.26*s, 0.4*s
	capsule := strand.Rect{X: cx - w/2, Y: card.Y + 0.16*s, W: w, H: h}
	var head []point
	if round {
		head = roundRect(capsule, w/2)
	} else {
		head = cutRect(capsule, w*0.35)
	}
	fillPolys(dst, [][]point{head}, ink, 1)
	// the cup under it, the stem and the foot
	line := max(1.2*dpr, 0.07*s)
	rx := 0.21 * s
	top, bottom, foot := capsule.Y+h*0.55, capsule.Y+h+0.08*s, card.Y+0.84*s
	var cup []point
	style := strokeStyle{width: line, join: "round", cap: "round"}
	if round {
		const n = 16
		for i := 0; i <= n; i++ {
			sn, cs := math.Sincos(math.Pi * float64(i) / n)
			cup = append(cup, point{cx - rx*cs, top + (bottom-top)*sn})
		}
	} else {
		c := rx * 0.5
		cup = []point{{cx - rx, top}, {cx - rx, bottom - c}, {cx - rx + c, bottom}, {cx + rx - c, bottom}, {cx + rx, bottom - c}, {cx + rx, top}}
		style = strokeStyle{width: line, join: "miter", cap: "butt", miterLimit: 4}
	}
	pieces := strokePieces([]contour{
		{pts: cup},
		{pts: []point{{cx, bottom}, {cx, foot}}},
		{pts: []point{{cx - 0.12*s, foot}, {cx + 0.12*s, foot}}},
	}, style, perchTol)
	fillPolys(dst, pieces, ink, 1)
	return painted
}

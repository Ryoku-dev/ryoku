package kit

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

const (
	// toneMarkShare is the most of a plane's shorter side the notch takes, so a
	// 36 dp row wears a 9 dp notch rather than the tablet's 12.
	toneMarkShare = 4
	// toneDotShare turns the notch's size into the dot's diameter, and
	// toneDotMin keeps the dot at the Hub's smallest round mark, its 5 px
	// section node (design.md, "What is still round").
	toneDotShare = 2
	toneDotMin   = unit.Dp(5)
	// toneDotGap stands the dot clear of the corner's arc, and toneDotInset is
	// the least it stands in from both edges where the corner is nearly square.
	toneDotGap   = unit.Dp(1)
	toneDotInset = unit.Dp(3)
)

// ToneMark draws the state mark of the plane in bounds, at its north-west
// corner, in th's design: Lithic's notch, and in Material, Adwaita and Fluent
// a small dot that stands inside the corner's radius, where a notch would
// overshoot the round corner. The mark is the notch scale's 12 dp, less on a
// plane under 48 dp on its shorter side. Idle draws nothing. Draw it after the
// plane and before the plane's content.
func ToneMark(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, tone palette.Tone) {
	if tone == palette.Idle {
		return
	}
	ToneMarkInk(gtx, th, bounds, th.R.Stone(tone))
}

// ToneMarkInk is ToneMark in a caller's ink, for a mark that is not one of the
// tones: a pending change, or a notch cut in the colour of the plane beneath.
func ToneMarkInk(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, ink color.NRGBA) {
	least := max(0, min(bounds.Dx(), bounds.Dy()))
	toneMark(gtx, th, bounds, min(gtx.Dp(th.Cut.Notch), least/toneMarkShare), ink, 1)
}

// toneMark draws the mark at a notch size of size pixels, grown from nothing
// (0) to whole (1), so a control can ease it in when its tone changes.
func toneMark(gtx layout.Context, th *theme.Theme, bounds image.Rectangle, size int, ink color.NRGBA, grown float32) {
	grown = min(max(grown, 0), 1)
	if size <= 0 || grown <= 0 || ink.A == 0 || bounds.Empty() {
		return
	}
	profile := th.Design.Profile()
	if !profile.Round {
		Notch(gtx.Ops, bounds.Min, int(math.Round(float64(size)*float64(grown))), ink)
		return
	}
	diameter := float32(max(size/toneDotShare, gtx.Dp(toneDotMin)))
	// the dot clears the largest corner the design can give these bounds, so
	// it stays inside whatever cut the plane was drawn with
	least := float32(min(bounds.Dx(), bounds.Dy()))
	radius := least / 2
	if least > float32(gtx.Dp(unit.Dp(profile.Pill))) {
		radius = min(float32(gtx.Dp(unit.Dp(profile.Max))), radius)
	}
	// the centre of a dot tangent inside a corner of that radius lies
	// (1 - 1/sqrt2) radius + diameter/(2 sqrt2) in from both edges
	centre := (1-math.Sqrt2/2)*float64(radius) + float64(diameter)/(2*math.Sqrt2) + float64(gtx.Dp(toneDotGap))
	centre = max(centre, float64(diameter)/2+float64(gtx.Dp(toneDotInset)))
	half := float64(diameter) * float64(grown) / 2
	box := image.Rect(
		bounds.Min.X+int(math.Round(centre-half)), bounds.Min.Y+int(math.Round(centre-half)),
		bounds.Min.X+int(math.Round(centre+half)), bounds.Min.Y+int(math.Round(centre+half)),
	)
	if box.Empty() {
		return
	}
	paint.FillShape(gtx.Ops, ink, clip.Ellipse(box).Op(gtx.Ops))
}

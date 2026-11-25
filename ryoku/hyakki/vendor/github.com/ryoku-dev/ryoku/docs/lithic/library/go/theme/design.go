package theme

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

// NewFor creates a theme for what the desktop asks for: its roles and its design. It also
// sets anim's reduced motion from d, so swapping themes on a WatchDesktop callback carries
// the desktop's reduced-motion switch with it.
func NewFor(d palette.Desktop, options ...Option) *Theme {
	anim.SetReduce(d.ReduceMotion)
	t := New(d.Roles, options...)
	t.Design = d.Design
	return t
}

// Corner returns the corner a shape with this cut wears in the theme's design, in pixels:
// the cut itself in Lithic (and for cut <= 0), a radius in the round designs. It converts
// through gtx.Metric because the design's thresholds are in dp (design.Style.Corner,
// Lithic.cornerRadius). The pill test compares the shorter side in pixels with the pill
// threshold as gtx.Dp rounds it, so a control sized gtx.Dp(22) stays a pill in Fluent at
// any scale (39 px at 1.75 is 22.29 dp, past 22, but is what Dp(22) gives).
func (t *Theme) Corner(gtx layout.Context, cut int, b image.Rectangle) int {
	profile := t.Design.Profile()
	if cut <= 0 || !profile.Round {
		return cut
	}
	least := max(0, min(b.Dx(), b.Dy()))
	half := float32(least) / 2
	if least <= gtx.Dp(unit.Dp(profile.Pill)) {
		return int(half + 0.5)
	}
	scale := gtx.Metric.PxPerDp
	if scale <= 0 {
		scale = 1
	}
	radius := t.Design.Corner(float32(cut)/scale, float32(b.Dx())/scale, float32(b.Dy())/scale) * scale
	return int(min(radius, half) + 0.5)
}

// Planes returns the planes a window lays out in the theme's design (Lithic.planes):
// the side, the pane and the card with their strokes.
func (t *Theme) Planes() palette.Planes {
	return t.R.Planes(t.Design)
}

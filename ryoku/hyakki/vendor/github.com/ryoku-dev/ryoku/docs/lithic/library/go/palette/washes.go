package palette

import (
	"image/color"
	"math"
)

// The washes are the sheet's ornament inks: fixed alphas of the ink, a little stronger on a
// light sheet so they read the same. The Hub names them in Singletons/Tone.qml:23-32.
//
// Qt Quick blends in sRGB, Gio in linear light, so the same alpha reads three to four times
// stronger in Gio at the faint end (fg at .05 over a black ground: Qt #0c0c0b, Gio #312f2c).
// The washes are therefore solved with SRGBAlpha against the ground, the Hub's paper (the
// Lithic pane is the same tone), so they land on the Hub's values there.

// SRGBAlpha returns the colour to paint, in Gio, so that it lands over under where Qt's sRGB
// blend of ink at alpha a would: the alpha stays a (raised only when no colour at a can reach
// the target) and the colour is solved in linear light. Over under the result matches Qt
// within a level; over a nearby plane it stays close, and it is still a translucent ink.
func SRGBAlpha(ink, under color.NRGBA, a float32) color.NRGBA {
	a = min(max(a, 0), 1) * float32(ink.A) / 255
	if a <= 0 {
		return color.NRGBA{}
	}
	if a >= 1 {
		ink.A = 255
		return ink
	}
	share := float64(a)
	// target and under in linear light, per channel
	var target, base [3]float64
	inks := [3]uint8{ink.R, ink.G, ink.B}
	unders := [3]uint8{under.R, under.G, under.B}
	alpha := share
	for i := range 3 {
		mixed := share*float64(inks[i]) + (1-share)*float64(unders[i])
		target[i], base[i] = srgbToLinear(mixed/255), srgbToLinear(float64(unders[i])/255)
		// the least alpha whose colour stays within 0..1 for this channel
		switch {
		case target[i] < base[i] && base[i] > 0:
			alpha = max(alpha, (base[i]-target[i])/base[i])
		case target[i] > base[i] && base[i] < 1:
			alpha = max(alpha, (target[i]-base[i])/(1-base[i]))
		}
	}
	// Gio carries the alpha in a byte: take the byte at or above the alpha needed
	level := min(math.Ceil(alpha*255-1e-9), 255)
	alpha = level / 255
	var out [3]uint8
	for i := range 3 {
		solved := base[i] + (target[i]-base[i])/alpha
		out[i] = uint8(math.Round(255 * linearToSRGB(min(max(solved, 0), 1))))
	}
	return color.NRGBA{R: out[0], G: out[1], B: out[2], A: uint8(level)}
}

// srgbToLinear converts an sRGB component, 0..1, to linear light (EXT_sRGB, as Gio does).
func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// linearToSRGB converts a linear component, 0..1, to sRGB.
func linearToSRGB(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// Wash is fg at alpha dark or light, by the roles' own darkness, solved over the ground.
func (r Roles) Wash(dark, light float32) color.NRGBA {
	if r.Dark {
		return SRGBAlpha(r.Fg, r.Ground, dark)
	}
	return SRGBAlpha(r.Fg, r.Ground, light)
}

// Line is the faintest rule: the sheet's own lines and dividers (Tone.line, .08 dark, .10 light).
func (r Roles) Line() color.NRGBA { return r.Wash(0.08, 0.10) }

// Guide is a guide line at rest: the tree's rails and leader lines (Tone.guide, .16 / .20).
func (r Roles) Guide() color.NRGBA { return r.Wash(0.16, 0.20) }

// GuideLit is a guide line on the path being read (Tone.guideLit, .36 / .42).
func (r Roles) GuideLit() color.NRGBA { return r.Wash(0.36, 0.42) }

// Hover is the wash under a pointed row (Tone.hover, .06 / .07).
func (r Roles) Hover() color.NRGBA { return r.Wash(0.06, 0.07) }

// Press is the wash under a pressed row (Tone.press, .10 / .12).
func (r Roles) Press() color.NRGBA { return r.Wash(0.10, 0.12) }

// Select is the open page's wash, Tone.select, which is Lithic.container. On Lithic's ladder
// (Lithic, and every design on Ryoku's colours) that is the selection at .16 dark, .14 light
// (Lithic.qml _withMinerals), solved over the ground. A round design on the desktop's colours
// wears the palette's own secondary container (Material), an opaque tone, returned as it is.
// The ink on either is OnContainer (Tone.selectInk).
func (r Roles) Select() color.NRGBA {
	if r.Container.A == 255 {
		return r.Container
	}
	if r.Dark {
		return SRGBAlpha(r.Sel, r.Ground, 0.16)
	}
	return SRGBAlpha(r.Sel, r.Ground, 0.14)
}

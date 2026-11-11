package palette

import (
	"image/color"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Contrast selects how far the stones stand from the ground.
type Contrast uint8

const (
	Standard Contrast = iota
	High
)

// Tint selects how much of the palette's tint role the stones take.
type Tint uint8

const (
	TintOff Tint = iota
	TintLow
	TintMedium
)

// Look is the reader's choice of contrast and tint. The zero look is
// standard contrast with no tint.
type Look struct {
	Contrast Contrast
	Tint     Tint
}

// ParseLook reads a contrast ("standard", "high") and a tint ("off", "low",
// "medium"). An unknown string gives the zero value of its field.
func ParseLook(contrast, tint string) Look {
	var look Look
	if contrast == "high" {
		look.Contrast = High
	}
	switch tint {
	case "low":
		look.Tint = TintLow
	case "medium":
		look.Tint = TintMedium
	}
	return look
}

// apply runs the contrast step, then the tint step, on mapped roles. tint is
// the palette's tint role and is used only when tinted is true.
func (look Look) apply(r Roles, tint color.NRGBA, tinted bool) Roles {
	if look.Contrast == High {
		factor := 1 + float64(tokens.AlphaContrastHigh)
		for _, stone := range []*color.NRGBA{&r.Stone1, &r.Stone2, &r.Stone3, &r.Stone4, &r.Slab, &r.Field, &r.Vein} {
			*stone = away(*stone, r.Ground, factor)
		}
		r.Fg3 = Mix(r.Fg3, r.Fg, tokens.AlphaContrastHighFg3)
	}
	amount := float32(0)
	switch look.Tint {
	case TintLow:
		amount = tokens.AlphaTintLow
	case TintMedium:
		amount = tokens.AlphaTintMedium
	}
	if tinted && amount > 0 {
		for _, stone := range []*color.NRGBA{&r.Ground, &r.Stone1, &r.Stone2, &r.Stone3, &r.Stone4, &r.Slab, &r.Field} {
			*stone = Mix(*stone, tint, amount)
		}
	}
	return r
}

// away scales the distance of c from ground by factor per colour channel,
// rounded half up and clamped to a byte. Alpha is kept.
func away(c, ground color.NRGBA, factor float64) color.NRGBA {
	channel := func(x, from uint8) uint8 {
		value := float64(from) + factor*(float64(x)-float64(from))
		return uint8(min(max(value, 0), 255) + 0.5)
	}
	return color.NRGBA{R: channel(c.R, ground.R), G: channel(c.G, ground.G), B: channel(c.B, ground.B), A: c.A}
}

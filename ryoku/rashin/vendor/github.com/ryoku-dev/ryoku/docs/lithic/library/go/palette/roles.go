// Package palette resolves Lithic's colour roles: the signature, the
// ryoku.dev stock, by default, and the live Ryoku palette when the reader
// follows the wallpaper.
package palette

import (
	"image/color"
	"math"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Tone names a state without tying it to a colour.
type Tone uint8

const (
	Idle Tone = iota
	Alive
	Becoming
	Attention
	Paused
	Failed
)

// Roles are the colours consumed by Lithic components.
type Roles struct {
	Ground  color.NRGBA
	Stone1  color.NRGBA
	Stone2  color.NRGBA
	Stone3  color.NRGBA
	Stone4  color.NRGBA
	Slab    color.NRGBA
	Field   color.NRGBA
	Vein    color.NRGBA
	Edge    color.NRGBA
	Fg      color.NRGBA
	Fg2     color.NRGBA
	Fg3     color.NRGBA
	Sel     color.NRGBA
	OnSel   color.NRGBA
	OnSel2  color.NRGBA
	Gold    color.NRGBA
	OnGold  color.NRGBA
	GoldInk color.NRGBA
	Jade    color.NRGBA
	Lapis   color.NRGBA
	Amber   color.NRGBA
	Jasper  color.NRGBA
	Scrim   color.NRGBA
	Dark    bool
	Source  string

	mineralTone float64
}

// Signature returns the roles Lithic paints with no palette: the bone dye on
// the tone ladder for dark or light mode.
func Signature(dark bool) Roles {
	return FromPalette(Palette{Dark: dark, Source: "signature"}, Look{})
}

// Stone returns the mineral ink associated with tone.
func (r Roles) Stone(tone Tone) color.NRGBA {
	switch tone {
	case Alive:
		return r.Jade
	case Becoming:
		return r.Lapis
	case Attention, Paused:
		return r.Amber
	case Failed:
		return r.Jasper
	default:
		return r.Vein
	}
}

// MineralField returns the mineral field associated with tone: the mineral's
// hue at the ladder's mineral-field lightness and its field chroma.
func (r Roles) MineralField(tone Tone) color.NRGBA {
	name := ""
	switch tone {
	case Alive:
		name = "jade"
	case Becoming:
		name = "lapis"
	case Attention, Paused:
		name = "amber"
	case Failed:
		name = "jasper"
	default:
		return r.Stone3
	}
	return toned(r.mineralTone, dye{C: tokens.DyeMineral[name], H: dyeOf(r.Stone(tone)).H})
}

// WithAlpha returns c at a fraction of its current alpha.
func WithAlpha(c color.NRGBA, alpha float32) color.NRGBA {
	alpha = min(max(alpha, 0), 1)
	c.A = uint8(float32(c.A)*alpha + 0.5)
	return c
}

// Mix blends a toward b by t, including alpha.
func Mix(a, b color.NRGBA, t float32) color.NRGBA {
	t = min(max(t, 0), 1)
	lerp := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t + 0.5) }
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: lerp(a.A, b.A)}
}

// InkOn returns black or white, whichever is more readable on background.
func InkOn(background color.NRGBA) color.NRGBA {
	black := color.NRGBA{A: 0xff}
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	if ContrastRatio(black, background) >= ContrastRatio(white, background) {
		return black
	}
	return white
}

// ContrastRatio returns the WCAG contrast ratio of two opaque colours.
func ContrastRatio(a, b color.NRGBA) float64 {
	lighter, darker := relativeLuminance(a), relativeLuminance(b)
	if lighter < darker {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

func relativeLuminance(c color.NRGBA) float64 {
	channel := func(value uint8) float64 {
		x := float64(value) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

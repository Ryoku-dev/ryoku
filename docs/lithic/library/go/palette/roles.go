// Package palette resolves the live Ryoku palette into Lithic colour roles.
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
	Vein    color.NRGBA
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
}

// Signature returns the fallback roles for dark or light mode.
func Signature(dark bool) Roles {
	table := tokens.SignatureLight
	minerals := tokens.MineralsLight
	if dark {
		table = tokens.SignatureDark
		minerals = tokens.MineralsDark
	}
	r := Roles{
		Ground: tableColor(table, "ground"), Stone1: tableColor(table, "stone1"),
		Stone2: tableColor(table, "stone2"), Stone3: tableColor(table, "stone3"),
		Stone4: tableColor(table, "stone4"), Vein: tableColor(table, "vein"),
		Fg: tableColor(table, "fg"), Fg2: tableColor(table, "fg2"), Fg3: tableColor(table, "fg3"),
		Sel: tableColor(table, "sel"), OnSel: tableColor(table, "onSel"), OnSel2: tableColor(table, "onSel2"),
		Gold: tableColor(table, "gold"), OnGold: tableColor(table, "onGold"), GoldInk: tableColor(table, "goldInk"),
		Jasper: tableColor(table, "jasper"), Scrim: tableColor(table, "scrim"),
		Jade: tableColor(minerals, "jade"), Lapis: tableColor(minerals, "lapis"), Amber: tableColor(minerals, "amber"),
		Dark: dark, Source: "signature",
	}
	return r
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

// Field returns the mineral field associated with tone.
func (r Roles) Field(tone Tone) color.NRGBA {
	switch tone {
	case Alive:
		return Mix(r.Stone2, r.Jade, tokens.AlphaMineralJade)
	case Becoming, Attention, Paused, Failed:
		return Mix(r.Stone2, r.Stone(tone), tokens.AlphaMineral)
	default:
		return r.Stone3
	}
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

func contrastTone(ink, background color.NRGBA, lighter bool) color.NRGBA {
	if ContrastRatio(ink, background) >= float64(tokens.ContrastMinimum) {
		return ink
	}
	hue, saturation, lightness := rgbToHSL(ink)
	for step := 1; step <= 255; step++ {
		at := float64(step) / 255
		next := lightness * (1 - at)
		if lighter {
			next = lightness + (1-lightness)*at
		}
		candidate := hslToNRGBA(hue, saturation, next, ink.A)
		if ContrastRatio(candidate, background) >= float64(tokens.ContrastMinimum) {
			return candidate
		}
	}
	if lighter {
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: ink.A}
	}
	return color.NRGBA{A: ink.A}
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

func rgbToHSL(c color.NRGBA) (hue, saturation, lightness float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	maximum, minimum := max(r, g, b), min(r, g, b)
	lightness = (maximum + minimum) / 2
	if maximum == minimum {
		return 0, 0, lightness
	}
	delta := maximum - minimum
	if lightness > 0.5 {
		saturation = delta / (2 - maximum - minimum)
	} else {
		saturation = delta / (maximum + minimum)
	}
	switch maximum {
	case r:
		hue = (g - b) / delta
		if g < b {
			hue += 6
		}
	case g:
		hue = (b-r)/delta + 2
	default:
		hue = (r-g)/delta + 4
	}
	return hue / 6, saturation, lightness
}

func hslToNRGBA(hue, saturation, lightness float64, alpha uint8) color.NRGBA {
	if saturation == 0 {
		value := uint8(math.Round(lightness * 255))
		return color.NRGBA{R: value, G: value, B: value, A: alpha}
	}
	q := lightness * (1 + saturation)
	if lightness >= 0.5 {
		q = lightness + saturation - lightness*saturation
	}
	p := 2*lightness - q
	hueToRGB := func(t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		default:
			return p
		}
	}
	return color.NRGBA{
		R: uint8(math.Round(hueToRGB(hue+1.0/3) * 255)),
		G: uint8(math.Round(hueToRGB(hue) * 255)),
		B: uint8(math.Round(hueToRGB(hue-1.0/3) * 255)), A: alpha,
	}
}

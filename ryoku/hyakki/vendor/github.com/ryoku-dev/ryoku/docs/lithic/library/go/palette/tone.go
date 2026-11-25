package palette

import (
	"image/color"
	"math"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// dye is the hue (degrees) and chroma a role takes from its source colour.
// Lithic sets the lightness; the palette only dyes.
type dye struct {
	C, H float64
}

// dyeOf reads the OKLCH chroma and hue of an sRGB colour.
func dyeOf(c color.NRGBA) dye {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	r, g, b := linear(c.R), linear(c.G), linear(c.B)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	a := 1.9779984951*l - 2.4285922050*m + 0.4505937099*s
	bb := 0.0259040371*l + 0.7827717662*m - 0.8086757660*s
	hue := math.Atan2(bb, a) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return dye{C: math.Hypot(a, bb), H: hue}
}

func linearFromOKLCH(lightness, chroma, hue float64) (r, g, b float64) {
	a := chroma * math.Cos(hue*math.Pi/180)
	bb := chroma * math.Sin(hue*math.Pi/180)
	l := lightness + 0.3963377774*a + 0.2158037573*bb
	m := lightness - 0.1055613458*a - 0.0638541728*bb
	s := lightness - 0.0894841775*a - 1.2914855480*bb
	l, m, s = l*l*l, m*m*m, s*s*s
	r = 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	g = -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	b = -0.0041960863*l - 0.7034186147*m + 1.7076147010*s
	return r, g, b
}

func inGamut(lightness, chroma, hue float64) bool {
	const e = 1e-7
	r, g, b := linearFromOKLCH(lightness, chroma, hue)
	return r >= -e && r <= 1+e && g >= -e && g <= 1+e && b >= -e && b <= 1+e
}

// toned paints a role at a ladder lightness with a dye, keeping as much of the
// dye's chroma as sRGB can show at that lightness.
func toned(lightness float64, d dye) color.NRGBA {
	chroma := d.C
	if !inGamut(lightness, chroma, d.H) {
		low, high := 0.0, chroma
		for range 24 {
			mid := (low + high) / 2
			if inGamut(lightness, mid, d.H) {
				low = mid
			} else {
				high = mid
			}
		}
		chroma = low
	}
	r, g, b := linearFromOKLCH(lightness, chroma, d.H)
	encode := func(x float64) uint8 {
		x = min(max(x, 0), 1)
		if x <= 0.0031308 {
			x *= 12.92
		} else {
			x = 1.055*math.Pow(x, 1/2.4) - 0.055
		}
		return uint8(math.Floor(x*255 + 0.5))
	}
	return color.NRGBA{R: encode(r), G: encode(g), B: encode(b), A: 0xff}
}

// ladder returns the tone table for a mode and contrast.
func ladder(dark bool, contrast Contrast) map[string]float64 {
	switch {
	case dark && contrast == High:
		return tokens.ToneDarkHigh
	case dark:
		return tokens.ToneDark
	case contrast == High:
		return tokens.ToneLightHigh
	default:
		return tokens.ToneLight
	}
}

// accent dyes an action from a palette or edition colour: its hue, and its
// chroma boosted up to the gold cap.
func accent(c color.NRGBA) dye {
	d := dyeOf(c)
	return dye{C: min(tokens.DyeChroma["gold"], d.C*tokens.DyeBoost["gold"]), H: d.H}
}

func tintFactor(tint Tint) float64 {
	switch tint {
	case TintLow:
		return tokens.DyeTint["low"]
	case TintMedium:
		return tokens.DyeTint["medium"]
	default:
		return 1
	}
}

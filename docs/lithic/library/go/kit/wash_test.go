package kit

import (
	"image/color"
	"math"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func washToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func washToSRGB(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// gioOver is what Gio's renderer lands on: ink blended over under in linear
// light, written back to an sRGB framebuffer.
func gioOver(ink, under color.NRGBA) [3]float64 {
	a := float64(ink.A) / 255
	inks, unders := [3]uint8{ink.R, ink.G, ink.B}, [3]uint8{under.R, under.G, under.B}
	var out [3]float64
	for i := range 3 {
		mixed := a*washToLinear(float64(inks[i])/255) + (1-a)*washToLinear(float64(unders[i])/255)
		out[i] = math.Round(255 * washToSRGB(mixed))
	}
	return out
}

// qtOver is what the Hub's Qt Quick scene lands on: the same blend in sRGB.
func qtOver(ink, under color.NRGBA, a float64) [3]float64 {
	inks, unders := [3]uint8{ink.R, ink.G, ink.B}, [3]uint8{under.R, under.G, under.B}
	var out [3]float64
	for i := range 3 {
		out[i] = math.Round(a*float64(inks[i]) + (1-a)*float64(unders[i]))
	}
	return out
}

func assertQtBlend(t *testing.T, name string, painted, ink, under color.NRGBA, a float64) {
	t.Helper()
	got, want := gioOver(painted, under), qtOver(ink, under, a)
	for i := range 3 {
		if math.Abs(got[i]-want[i]) > 1 {
			t.Errorf("%s: Gio lands on %v, Qt on %v", name, got, want)
			return
		}
	}
}

// A kit hover or press wash, painted by Gio over the plane it lies on, lands
// within one level of Qt's sRGB blend of the same ink at the same alpha.
func TestKitWashesMatchQtBlend(t *testing.T) {
	var router input.Router
	for _, dark := range []bool{true, false} {
		th := theme.New(palette.Signature(dark))
		gtx := layout.Context{Source: router.Source()}
		// a clear plaque's hover over the ground
		for _, kind := range []PlaqueKind{PlaqueQuiet, PlaqueLine, PlaqueDangerLine} {
			colors := interactivePlaqueColors(gtx, th, kind, 1)
			ink := plaquePalette(th, kind).ink
			assertQtBlend(t, "plaque hover", colors.fill, ink, th.R.Ground, float64(tokens.AlphaHoverFill))
		}
		// a segment's hover over the field track, a hatch over a field, a tab press
		assertQtBlend(t, "segment hover", faintInk(gtx, th, th.R.Fg2, th.R.Field, tokens.AlphaHoverFill), th.R.Fg2, th.R.Field, float64(tokens.AlphaHoverFill))
		assertQtBlend(t, "hatch", faintInk(gtx, th, th.R.Fg, th.R.Field, tokens.AlphaHatch), th.R.Fg, th.R.Field, float64(tokens.AlphaHatch))
		assertQtBlend(t, "row press", faintInk(gtx, th, th.R.Fg, th.R.Ground, tokens.AlphaPressRow), th.R.Fg, th.R.Ground, float64(tokens.AlphaPressRow))
		// a disabled control fades over the ground, and a faint wash on it fades further
		disabled := gtx.Disabled()
		assertQtBlend(t, "disabled ink", disabledInk(disabled, th, th.R.Fg), th.R.Fg, th.R.Ground, float64(tokens.AlphaDisabled))
		assertQtBlend(t, "disabled hover", faintInk(disabled, th, th.R.Fg2, th.R.Field, tokens.AlphaHoverFill), th.R.Fg2, th.R.Field, float64(tokens.AlphaHoverFill*tokens.AlphaDisabled))
		if got := disabledInk(gtx, th, th.R.Fg); got != th.R.Fg {
			t.Errorf("enabled ink changed to %v", got)
		}
	}
}

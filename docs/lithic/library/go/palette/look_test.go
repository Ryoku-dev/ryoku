package palette

import (
	"image/color"
	"testing"
)

func TestParseLook(t *testing.T) {
	for _, test := range []struct {
		contrast, tint string
		want           Look
	}{
		{"standard", "off", Look{}},
		{"high", "low", Look{Contrast: High, Tint: TintLow}},
		{"standard", "medium", Look{Tint: TintMedium}},
		{"loud", "low", Look{Tint: TintLow}},
		{"high", "pink", Look{Contrast: High}},
		{"", "", Look{}},
	} {
		if got := ParseLook(test.contrast, test.tint); got != test.want {
			t.Fatalf("ParseLook(%q, %q) = %#v, want %#v", test.contrast, test.tint, got, test.want)
		}
	}
}

func TestAwayRoundsAndClamps(t *testing.T) {
	ground := color.NRGBA{R: 10, G: 10, B: 10, A: 255}
	got := away(color.NRGBA{R: 20, G: 5, B: 200, A: 200}, ground, 1.6)
	assertColor(t, "away", got, color.NRGBA{R: 26, G: 2, B: 255, A: 200})
	got = away(color.NRGBA{R: 3, G: 11, B: 10, A: 255}, ground, 1.6)
	assertColor(t, "away below", got, color.NRGBA{R: 0, G: 12, B: 10, A: 255})
}

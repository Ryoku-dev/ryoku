package kit

import (
	"image/color"
	"testing"
)

func TestParseHexColor(t *testing.T) {
	cases := []struct {
		text       string
		allowAlpha bool
		want       color.NRGBA
		ok         bool
	}{
		{"#ce5e32", false, color.NRGBA{R: 0xce, G: 0x5e, B: 0x32, A: 0xff}, true},
		{"CE5E32", false, color.NRGBA{R: 0xce, G: 0x5e, B: 0x32, A: 0xff}, true},
		{" #abc ", false, color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff}, true},
		{"#ce5e3280", true, color.NRGBA{R: 0xce, G: 0x5e, B: 0x32, A: 0x80}, true},
		{"#abc8", true, color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0x88}, true},
		{"#ce5e3280", false, color.NRGBA{}, false},
		{"#abc8", false, color.NRGBA{}, false},
		{"#ce5e3", false, color.NRGBA{}, false},
		{"#ce5e3g", false, color.NRGBA{}, false},
		{"#+e5e32", false, color.NRGBA{}, false},
		{"", false, color.NRGBA{}, false},
		{"#", true, color.NRGBA{}, false},
	}
	for _, test := range cases {
		got, ok := parseHexColor(test.text, test.allowAlpha)
		if ok != test.ok || got != test.want {
			t.Errorf("parseHexColor(%q, %v) = %v %v, want %v %v", test.text, test.allowAlpha, got, ok, test.want, test.ok)
		}
	}
}

func TestFormatHexColor(t *testing.T) {
	cases := []struct {
		value      color.NRGBA
		allowAlpha bool
		want       string
	}{
		{color.NRGBA{R: 0xce, G: 0x5e, B: 0x32, A: 0xff}, true, "#ce5e32"},
		{color.NRGBA{R: 0xce, G: 0x5e, B: 0x32, A: 0x80}, true, "#ce5e3280"},
		{color.NRGBA{R: 0xce, G: 0x5e, B: 0x32, A: 0x80}, false, "#ce5e32"},
		{color.NRGBA{A: 0xff}, false, "#000000"},
	}
	for _, test := range cases {
		if got := formatHexColor(test.value, test.allowAlpha); got != test.want {
			t.Errorf("formatHexColor(%v, %v) = %q, want %q", test.value, test.allowAlpha, got, test.want)
		}
		if parsed, ok := parseHexColor(formatHexColor(test.value, true), true); !ok || parsed != test.value {
			t.Errorf("round trip of %v gave %v %v", test.value, parsed, ok)
		}
	}
}

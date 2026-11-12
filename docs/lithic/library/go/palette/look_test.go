package palette

import "testing"

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

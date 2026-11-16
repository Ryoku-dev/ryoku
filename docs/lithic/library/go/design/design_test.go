package design

import (
	"math"
	"testing"
)

// TestProfilesAreLithicQML pins the designs table of Lithic.qml:49-52.
func TestProfilesAreLithicQML(t *testing.T) {
	for _, test := range []struct {
		style Style
		name  string
		want  Profile
	}{
		{Lithic, "lithic", Profile{Round: false, Scale: 1, Pill: 0, Max: 16, Desktop: false}},
		{Material, "material", Profile{Round: true, Scale: 2.6, Pill: 56, Max: 28, Desktop: true}},
		{Adwaita, "adwaita", Profile{Round: true, Scale: 1.6, Pill: 26, Max: 14, Desktop: true}},
		{Fluent, "fluent", Profile{Round: true, Scale: 0.75, Pill: 22, Max: 8, Desktop: true}},
	} {
		if got := test.style.Profile(); got != test.want {
			t.Errorf("%s profile = %+v, want %+v", test.name, got, test.want)
		}
		if got := test.style.String(); got != test.name {
			t.Errorf("String() = %q, want %q", got, test.name)
		}
		parsed, ok := Parse(test.name)
		if !ok || parsed != test.style {
			t.Errorf("Parse(%q) = %v, %v", test.name, parsed, ok)
		}
	}
	var zero Style
	if zero != Lithic {
		t.Fatal("the zero style is not Lithic")
	}
	for _, name := range []string{"", "Material", "cupertino", " lithic"} {
		if style, ok := Parse(name); ok || style != Lithic {
			t.Errorf("Parse(%q) = %v, %v; want lithic, false", name, style, ok)
		}
	}
	if Style(9).String() != "lithic" || Style(9).Profile() != Lithic.Profile() {
		t.Error("an unknown style does not read as Lithic")
	}
}

// TestCornerIsCornerRadius pins Lithic.cornerRadius (Lithic.qml:61-69) on the library's
// shapes: cuts from tokens, sizes from the controls.
func TestCornerIsCornerRadius(t *testing.T) {
	for _, test := range []struct {
		shape                       string
		cut, width, height          float32
		lithic, material, adw, flnt float32
	}{
		{"toggle track", 4, 40, 22, 4, 11, 11, 11},
		{"toggle knob", 2, 14, 14, 2, 7, 7, 7},
		{"plaque", 7, 120, 40, 7, 20, 11.2, 5.25},
		{"small plaque", 6, 96, 32, 6, 16, 9.6, 4.5},
		{"segmented track", 7, 240, 36, 7, 18, 11.2, 5.25},
		{"segment", 5, 80, 28, 5, 14, 8, 3.75},
		{"text field", 7, 320, 44, 7, 22, 11.2, 5.25},
		{"slider handle", 4, 8, 20, 4, 4, 4, 4},
		{"scroll thumb", 1.5, 3, 80, 1.5, 1.5, 1.5, 1.5},
		{"slab", 7, 300, 200, 7, 18.2, 11.2, 5.25},
		{"menu", 8, 200, 300, 8, 20.8, 12.8, 6},
		{"dialog", 16, 400, 300, 16, 28, 14, 8},
		{"focus ring", 6, 50, 30, 6, 15, 9.6, 4.5},
		{"pressed plaque", 11, 120, 40, 11, 20, 14, 8},
		{"square", 0, 100, 100, 0, 0, 0, 0},
		{"negative cut", -2, 100, 100, -2, -2, -2, -2},
		{"zero size", 7, 0, 0, 7, 0, 0, 0},
	} {
		for _, want := range []struct {
			style  Style
			radius float32
		}{{Lithic, test.lithic}, {Material, test.material}, {Adwaita, test.adw}, {Fluent, test.flnt}} {
			got := want.style.Corner(test.cut, test.width, test.height)
			if math.Abs(float64(got-want.radius)) > 1e-4 {
				t.Errorf("%s in %s: corner = %v, want %v", test.shape, want.style, got, want.radius)
			}
		}
	}
}

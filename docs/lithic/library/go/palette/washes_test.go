package palette

import (
	"image/color"
	"math"
	"testing"
)

// gioOver is what Gio paints for ink over under: a blend in linear light of the 8-bit ink and
// its 8-bit alpha, written back to an sRGB byte.
func gioOver(ink, under color.NRGBA) [3]float64 {
	a := float64(ink.A) / 255
	inks := [3]uint8{ink.R, ink.G, ink.B}
	unders := [3]uint8{under.R, under.G, under.B}
	var out [3]float64
	for i := range 3 {
		mixed := a*srgbToLinear(float64(inks[i])/255) + (1-a)*srgbToLinear(float64(unders[i])/255)
		out[i] = math.Round(255 * linearToSRGB(mixed))
	}
	return out
}

// qtOver is what Qt Quick paints for ink at alpha a over under: a blend of the sRGB bytes.
func qtOver(ink, under color.NRGBA, a float32) [3]float64 {
	inks := [3]uint8{ink.R, ink.G, ink.B}
	unders := [3]uint8{under.R, under.G, under.B}
	var out [3]float64
	for i := range 3 {
		out[i] = math.Round(float64(a)*float64(inks[i]) + float64(1-a)*float64(unders[i]))
	}
	return out
}

func within(a, b [3]float64, levels float64) bool {
	for i := range 3 {
		if math.Abs(a[i]-b[i]) > levels {
			return false
		}
	}
	return true
}

// TestWashesLandOnQt: every wash, painted by Gio over the ground, lands within one level of the
// Hub's sRGB blend (Singletons/Tone.qml:23-32), on both signatures; and the alpha stays the
// Hub's where the colour alone can reach the target.
func TestWashesLandOnQt(t *testing.T) {
	cases := []struct {
		name        string
		get         func(Roles) color.NRGBA
		ink         func(Roles) color.NRGBA
		dark, light float32
	}{
		{"line", Roles.Line, func(r Roles) color.NRGBA { return r.Fg }, 0.08, 0.10},
		{"guide", Roles.Guide, func(r Roles) color.NRGBA { return r.Fg }, 0.16, 0.20},
		{"guideLit", Roles.GuideLit, func(r Roles) color.NRGBA { return r.Fg }, 0.36, 0.42},
		{"hover", Roles.Hover, func(r Roles) color.NRGBA { return r.Fg }, 0.06, 0.07},
		{"press", Roles.Press, func(r Roles) color.NRGBA { return r.Fg }, 0.10, 0.12},
		{"select", Roles.Select, func(r Roles) color.NRGBA { return r.Sel }, 0.16, 0.14},
	}
	for _, dark := range []bool{true, false} {
		roles := Signature(dark)
		for _, test := range cases {
			share := test.light
			if dark {
				share = test.dark
			}
			got := test.get(roles)
			want := qtOver(test.ink(roles), roles.Ground, share)
			if painted := gioOver(got, roles.Ground); !within(painted, want, 1) {
				t.Errorf("%s dark=%v: Gio paints %v over the ground, Qt %v (ink %v)", test.name, dark, painted, want, got)
			}
			if got.A < uint8(255*share) {
				t.Errorf("%s dark=%v: alpha %d under the Hub's %v", test.name, dark, got.A, share)
			}
		}
	}
}

// TestSRGBAlpha covers the faint end on black and white, a full and an empty alpha, and an
// ink that is already translucent.
func TestSRGBAlpha(t *testing.T) {
	black := color.NRGBA{A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	bone := color.NRGBA{R: 0xe8, G: 0xe2, B: 0xd8, A: 255}
	ink := color.NRGBA{R: 0x1c, G: 0x1a, B: 0x17, A: 255}
	for _, test := range []struct {
		ink, under color.NRGBA
		a          float32
	}{
		{bone, black, 0.05}, {bone, black, 0.07}, {bone, black, 0.32}, {white, black, 0.01},
		{ink, white, 0.07}, {ink, white, 0.10}, {ink, bone, 0.36}, {white, color.NRGBA{R: 128, G: 128, B: 128, A: 255}, 0.05},
	} {
		got := SRGBAlpha(test.ink, test.under, test.a)
		if painted, want := gioOver(got, test.under), qtOver(test.ink, test.under, test.a); !within(painted, want, 1) {
			t.Errorf("%v at %v over %v: Gio %v, Qt %v (solved %v)", test.ink, test.a, test.under, painted, want, got)
		}
	}
	if got := SRGBAlpha(bone, black, 1); got != bone {
		t.Errorf("a full alpha changed the ink: %v", got)
	}
	if got := SRGBAlpha(bone, black, 0); got.A != 0 {
		t.Errorf("an empty alpha painted: %v", got)
	}
	half := bone
	half.A = 128
	if a, b := SRGBAlpha(half, black, 0.2), SRGBAlpha(bone, black, 0.2*128/255.0); a != b {
		t.Errorf("a translucent ink: %v, want %v", a, b)
	}
}

// TestWashesFollowRoles: the washes are tones of the roles' own ink.
func TestWashesFollowRoles(t *testing.T) {
	for _, dark := range []bool{true, false} {
		roles := Signature(dark)
		roles.Fg = color.NRGBA{R: 200, G: 40, B: 40, A: 255}
		if got := roles.Line(); got.R <= got.G {
			t.Errorf("dark=%v: line %v does not follow a red fg", dark, got)
		}
	}
}

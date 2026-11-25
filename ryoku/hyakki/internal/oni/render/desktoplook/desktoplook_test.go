package desktoplook_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/oni/render/desktoplook"
	"ryoku-rashin/internal/oni/spec"
)

// TestLookFollowsDesktopRoles: every colour the character wears is one of
// the desktop's roles, every spec colour id has an accent, and a palette
// change reaches the look.
func TestLookFollowsDesktopRoles(t *testing.T) {
	for _, dark := range []bool{true, false} {
		r := palette.Signature(dark)
		l := desktoplook.FromRoles(r)
		if l.Dark != dark {
			t.Errorf("dark=%v: look says dark=%v", dark, l.Dark)
		}
		roles := []color.NRGBA{r.Ground, r.Fg, r.Fg3, r.Gold, r.Jade, r.Lapis, r.Amber, r.Jasper}
		isRole := func(c color.NRGBA) bool {
			for _, x := range roles {
				if x == c {
					return true
				}
			}
			return false
		}
		for name, c := range map[string]color.NRGBA{"ink": l.Ink, "white": l.White, "body": l.Body} {
			if !isRole(c) {
				t.Errorf("dark=%v: %s %v is no desktop role", dark, name, c)
			}
		}
		for _, id := range spec.PaletteIDs {
			c, ok := l.Accent[id]
			if !ok || !isRole(c) {
				t.Errorf("dark=%v: accent %s %v is no desktop role", dark, id, c)
			}
			// Oni at rest must never wear a state's colour
			if id != spec.PaletteIDs[0] && apart(c, l.Body) < 48 {
				t.Errorf("dark=%v: body %v is close to the %s accent %v", dark, l.Body, id, c)
			}
		}
		if lum(l.Ink) >= lum(l.White) {
			t.Errorf("dark=%v: ink %v is not darker than white %v", dark, l.Ink, l.White)
		}

		changed := r
		changed.Lapis = color.NRGBA{R: 1, G: 2, B: 3, A: 255}
		if got := desktoplook.FromDesktop(palette.Desktop{Roles: changed}).Accent["speaking"]; got != changed.Lapis {
			t.Errorf("dark=%v: speaking did not follow the palette: %v", dark, got)
		}
	}
}

func lum(c color.NRGBA) int { return 2*int(c.R) + 7*int(c.G) + int(c.B) }

func apart(a, b color.NRGBA) float64 {
	dr, dg, db := float64(a.R)-float64(b.R), float64(a.G)-float64(b.G), float64(a.B)-float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

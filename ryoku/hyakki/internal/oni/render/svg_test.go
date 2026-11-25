package render_test

import (
	"image/color"
	"testing"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
)

// probeSVG exercises the admitted subset in separate regions of a 100-unit
// viewBox drawn one pixel per unit.
const probeSVG = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 100 100">
  <defs>
    <linearGradient id="ramp" gradientUnits="userSpaceOnUse" x1="55" y1="0" x2="95" y2="0">
      <stop offset="0" stop-color="#000000"/>
      <stop offset="1" stop-color="#ffffff"/>
    </linearGradient>
    <linearGradient id="ramp2" xlink:href="#ramp"/>
    <radialGradient id="dot" cx="0.5" cy="0.5" r="0.5">
      <stop offset="0" stop-color="#ffffff"/>
      <stop offset="1" stop-color="#000000"/>
    </radialGradient>
    <clipPath id="disc"><circle cx="75" cy="75" r="15"/></clipPath>
    <mask id="half" maskUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">
      <rect x="5" y="78" width="20" height="20" fill="#ffffff"/>
      <rect x="25" y="78" width="20" height="20" fill="#000000"/>
    </mask>
    <symbol id="sym" viewBox="0 0 10 10"><rect width="10" height="10" fill="#00ffff"/></symbol>
  </defs>
  <path d="M5 5 H45 V45 H5 Z M15 15 H35 V35 H15 Z" fill="#ff0000" fill-rule="evenodd"/>
  <path d="m55 5 h40 v40 h-40 z m10 10 h20 v20 h-20 z" fill="#0000ff"/>
  <line x1="48" y1="5" x2="48" y2="45" stroke="#00ff00" stroke-width="2"/>
  <path d="M5 75 A20 20 0 0 1 45 75 Z" fill="rgb(0,255,0)"/>
  <rect x="55" y="55" width="40" height="40" fill="url(#ramp2)" clip-path="url(#disc)"/>
  <rect x="5" y="78" width="40" height="20" fill="#ffff00" mask="url(#half)"/>
  <circle cx="50" cy="50" r="3" fill="url(#dot)"/>
  <g transform="translate(46 90)" opacity="0.5"><use href="#sym" width="4" height="4"/></g>
  <polygon points="96,96 99,96 99,99" fill="#ff00ff"/>
</svg>`

func probe(t *testing.T) (*render.Character, func(x, y int) color.NRGBA) {
	t.Helper()
	s := loadSpec(t)
	p := profile.Profile{ID: "probe", Artwork: profile.Artwork{Kind: profile.ArtworkSVG, Image: "a.svg"}}
	c, err := render.Load(p, map[string][]byte{"a.svg": []byte(probeSVG)}, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range c.Fallbacks() {
		if f.Reason == render.FallbackUnsupported {
			t.Errorf("probe artwork reported unsupported: %+v", f)
		}
	}
	scale := 100 / s.Character.ButtonPx
	img, _ := draw(c, motion.Pose{Scale: 1, Bright: 1}, darkLook(), scale, 1)
	off := (img.Bounds().Dx() - 100) / 2
	return c, func(x, y int) color.NRGBA { return img.NRGBAAt(off+x, off+y) }
}

func TestSVGSubset(t *testing.T) {
	_, at := probe(t)
	near := func(name string, got, want color.NRGBA, tol float64) {
		t.Helper()
		if colourDistance(got, want) > tol || abs(int(got.A)-int(want.A)) > int(tol) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	red, blue, green := color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 255}, color.NRGBA{0, 255, 0, 255}
	near("even-odd ring", at(10, 10), red, 2)
	if at(25, 25).A != 0 {
		t.Errorf("even-odd hole painted %v", at(25, 25))
	}
	near("nonzero inner square", at(75, 25), blue, 2)
	near("stroked line", at(48, 25), green, 2)
	near("arc segment", at(25, 62), green, 2)
	if at(25, 72).A == 0 || at(4, 55).A != 0 {
		t.Errorf("arc bounds: inside %v, outside %v", at(25, 72), at(4, 55))
	}
	if at(57, 57).A != 0 {
		t.Errorf("clip-path left %v outside the disc", at(57, 57))
	}
	l, r := at(62, 75), at(88, 75)
	if l.A != 255 || r.A != 255 || int(l.R)+40 > int(r.R) || l.R != l.G {
		t.Errorf("clipped gradient: left %v right %v", l, r)
	}
	near("mask white half", at(15, 88), color.NRGBA{255, 255, 0, 255}, 2)
	if at(35, 88).A != 0 {
		t.Errorf("mask black half shows %v", at(35, 88))
	}
	centre, edge := at(50, 50), at(48, 50)
	// the pixel centre sits 0.71 units from the gradient centre: t = 0.236
	if centre.R < 185 || edge.R >= centre.R {
		t.Errorf("radial gradient: centre %v, edge %v", centre, edge)
	}
	sym := at(48, 92)
	if sym.A < 120 || sym.A > 135 || sym.G < 250 || sym.R > 5 {
		t.Errorf("symbol through use at half opacity: %v", sym)
	}
	near("polygon", at(98, 97), color.NRGBA{255, 0, 255, 255}, 2)
	if at(96, 98).A != 0 {
		t.Errorf("polygon outside painted %v", at(96, 98))
	}
}

// TestShippedArtworkWearsRoles: Oni's artwork follows the look it is given,
// the same shape in any palette.
func TestShippedArtworkWearsRoles(t *testing.T) {
	s := loadSpec(t)
	c := shipped(t, s)
	dark, _ := draw(c, rest(), darkLook(), 1, 1)
	light, _ := draw(c, rest(), lightLook(), 1, 1)
	if !alphaEqual(dark, light) {
		t.Fatal("the look changed the shape")
	}
	if differing(dark, light, 8) < 200 {
		t.Fatal("the look did not change the colours")
	}
	for _, l := range []render.Look{darkLook(), lightLook()} {
		img, _ := draw(c, rest(), l, 2, 1)
		seen := map[string]int{}
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				px := img.NRGBAAt(x, y)
				if px.A != 255 {
					continue
				}
				for name, role := range map[string]color.NRGBA{"ink": l.Ink, "white": l.White, "body": l.Body} {
					if colourDistance(px, role) <= 2 {
						seen[name]++
					}
				}
			}
		}
		for _, name := range []string{"ink", "white", "body"} {
			if seen[name] < 20 {
				t.Errorf("dark=%v: %s role shows on only %d pixels", l.Dark, name, seen[name])
			}
		}
	}
}

package render_test

import (
	"fmt"
	"image"
	"image/color"
	imgdraw "image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/oni/strand"
)

const strandStep = 10 * time.Millisecond

var (
	sheetScreen = strand.Rect{W: 800, H: 450}
	strandEnv   = strand.Env{
		Anchor: spec.Point{X: 700, Y: 380},
		Centre: spec.Point{X: 400, Y: 225},
		Screen: sheetScreen,
	}
	pointBox  = strand.Rect{X: 120, Y: 90, W: 90, H: 28}
	swingBox  = strand.Rect{X: 420, Y: 60, W: 80, H: 24}
	perchWin  = strand.Rect{X: 60, Y: 40, W: 360, H: 240}
	lassoBox  = strand.Rect{X: 250, Y: 120, W: 120, H: 36}
	regionBox = strand.Rect{X: 140, Y: 120, W: 80, H: 30}
	region    = strand.Rect{X: 100, Y: 80, W: 300, H: 180}
)

// strandScene is one moment of the thread, reached by driving the engine.
type strandScene struct {
	name  string
	phase strand.Phase
	// point scenes end in a plain tip whose cap is the farthest thing drawn
	point bool
	run   func(t *testing.T, e *strand.Engine) strand.Frame
}

func stepFor(e *strand.Engine, d time.Duration) strand.Tick {
	var tk strand.Tick
	for range int(d / strandStep) {
		tk = e.Step(strandStep, strandEnv)
	}
	return tk
}

// stepUntil steps until the frame is in phase and ok accepts it.
func stepUntil(t *testing.T, e *strand.Engine, phase strand.Phase, ok func(strand.Frame) bool) strand.Frame {
	t.Helper()
	for range 2000 {
		tk := e.Step(strandStep, strandEnv)
		if tk.Frame.Phase == phase && (ok == nil || ok(tk.Frame)) {
			return tk.Frame
		}
	}
	t.Fatalf("the thread never reached %s", phase)
	return strand.Frame{}
}

func strandScenes() []strandScene {
	point := strand.Target{Seq: 1, Box: pointBox, Quiet: true}
	return []strandScene{
		{"reach", strand.PhaseReach, true, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(point)
			return stepFor(e, 300*time.Millisecond).Frame
		}},
		{"tap", strand.PhaseTap, true, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(point)
			return stepFor(e, 830*time.Millisecond).Frame
		}},
		{"hold", strand.PhaseHold, true, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(point)
			return stepFor(e, 1200*time.Millisecond).Frame
		}},
		{"swing", strand.PhaseSwing, true, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(point)
			stepFor(e, 1200*time.Millisecond)
			e.Target(strand.Target{Seq: 2, Box: swingBox, Quiet: true})
			return stepFor(e, 150*time.Millisecond).Frame
		}},
		{"perch", strand.PhaseCurl, false, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(strand.Target{Seq: 1, Box: perchWin, Frame: &perchWin, Quiet: true})
			stepUntil(t, e, strand.PhaseCurl, nil)
			return stepFor(e, 200*time.Millisecond).Frame
		}},
		{"lasso", strand.PhaseLasso, false, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(strand.Target{Seq: 1, Box: lassoBox, Mode: strand.ModeLasso, Quiet: true})
			return stepUntil(t, e, strand.PhaseLasso, func(f strand.Frame) bool { return f.U >= 1 && len(f.Strokes) >= 2 })
		}},
		{"region", strand.PhaseRegion, false, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(strand.Target{Seq: 1, Box: regionBox, Region: &region, Mode: strand.ModeRegion, Quiet: true})
			return stepUntil(t, e, strand.PhaseRegion, func(f strand.Frame) bool { return f.U >= 0.5 })
		}},
		{"rewind", strand.PhaseRewind, true, func(t *testing.T, e *strand.Engine) strand.Frame {
			e.Target(point)
			stepFor(e, 1200*time.Millisecond)
			e.Retract(1)
			return stepFor(e, 120*time.Millisecond).Frame
		}},
	}
}

// paintFrame paints f the way the surface does: a buffer over its padded
// bbox at dpr, origin at the bbox's top-left.
func paintFrame(f strand.Frame, look render.Look, dpr float64) (*image.NRGBA, image.Rectangle, image.Rectangle) {
	rect := image.Rect(int(math.Floor(f.Bbox.X)), int(math.Floor(f.Bbox.Y)), int(math.Ceil(f.Bbox.X+f.Bbox.W)), int(math.Ceil(f.Bbox.Y+f.Bbox.H)))
	img := image.NewNRGBA(image.Rect(0, 0, int(math.Ceil(float64(rect.Dx())*dpr)), int(math.Ceil(float64(rect.Dy())*dpr))))
	painted := render.PaintStrand(img, rect.Min, f, look, dpr)
	return img, rect, painted
}

type strandView struct {
	img    *image.NRGBA
	origin image.Point
	dpr    float64
}

func (v strandView) device(p spec.Point) spec.Point {
	return spec.Point{X: (p.X - float64(v.origin.X)) * v.dpr, Y: (p.Y - float64(v.origin.Y)) * v.dpr}
}

// at is the pixel under device point p.
func (v strandView) at(p spec.Point) color.NRGBA {
	x, y := int(math.Floor(p.X)), int(math.Floor(p.Y))
	if !(image.Point{x, y}.In(v.img.Bounds())) {
		return color.NRGBA{}
	}
	return v.img.NRGBAAt(x, y)
}

func near(c, want color.NRGBA, tol float64) bool {
	return c.A == 255 && colourDistance(c, want) <= tol
}

// across walks the normal through device point p and returns how far the
// stroke is painted and how far it is the white core, each edge taken where
// coverage crosses one half (alpha, or nearer white than ink).
func (v strandView) across(p, n spec.Point, ink, white color.NRGBA, reach float64) (inked, cored float64) {
	const ds = 0.125
	for s := -reach; s <= reach; s += ds {
		c := v.at(spec.Point{X: p.X + n.X*s, Y: p.Y + n.Y*s})
		if c.A >= 128 {
			inked += ds
		}
		if c.A >= 128 && colourDistance(c, white) < colourDistance(c, ink) {
			cored += ds
		}
	}
	return inked, cored
}

func segDist(p, a, b spec.Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l2))
	}
	return math.Hypot(p.X-a.X-t*dx, p.Y-a.Y-t*dy)
}

// TestStrandContactSheet paints the thread through reach, tap, hold, swing,
// perch, lasso, region and rewind in Ryoku's dark look at device scale 1
// and its light look at the strand's 1.5 cap, writes the sheet, and checks
// every frame: the tip is the white core, the line is ink with a white core
// of the spec's widths, nothing is painted farther from a stroke than its
// ink reaches, translucent strokes show at their alpha, and on a plain
// reach the farthest painted pixel is the tip's round cap.
func TestStrandContactSheet(t *testing.T) {
	s := loadSpec(t)
	outline := s.Strand.Stroke.OutlinePx
	type shade struct {
		name   string
		look   render.Look
		ground color.NRGBA
		dpr    float64
	}
	shades := []shade{
		{"dark", darkLook(), darkLook().Ink, 1},
		{"light", lightLook(), lightLook().White, s.Strand.Stroke.DPRCap},
	}
	scenes := strandScenes()
	cellW, cellH := int(sheetScreen.W), int(sheetScreen.H)
	sheet := image.NewNRGBA(image.Rect(0, 0, 4*cellW, 2*len(shades)*cellH))

	for vi, va := range shades {
		ink, white := va.look.Ink, va.look.White
		for si, sc := range scenes {
			name := fmt.Sprintf("%s/%s", va.name, sc.name)
			e := strand.New(s)
			if tk := e.Step(0, strandEnv); !tk.Stop {
				t.Fatalf("%s: a fresh engine is not at rest", name)
			}
			f := sc.run(t, e)
			if f.Phase != sc.phase {
				t.Fatalf("%s: caught the thread in %s", name, f.Phase)
			}
			img, rect, painted := paintFrame(f, va.look, va.dpr)
			if painted.Empty() || !painted.In(img.Bounds()) {
				t.Fatalf("%s: painted %v in a %v buffer", name, painted, img.Bounds())
			}
			v := strandView{img: img, origin: rect.Min, dpr: va.dpr}

			if c := v.at(v.device(f.Tip)); !near(c, white, 6) {
				t.Errorf("%s: the tip %v is %v, want the white core %v", name, f.Tip, c, white)
			}

			var lead strand.Stroke
			for _, st := range f.Strokes {
				if st.Alpha == 1 && len(st.Pts) >= 3 {
					lead = st
					break
				}
			}
			if lead.Pts == nil {
				t.Fatalf("%s: no opaque stroke", name)
			}
			inkW, coreW := (lead.Width+2*outline)*va.dpr, lead.Width*va.dpr
			i := len(lead.Pts) / 2
			mid := v.device(lead.Pts[i])
			a, b := v.device(lead.Pts[i-1]), v.device(lead.Pts[i+1])
			tl := math.Hypot(b.X-a.X, b.Y-a.Y)
			n := spec.Point{X: -(b.Y - a.Y) / tl, Y: (b.X - a.X) / tl}
			inked, cored := v.across(mid, n, ink, white, inkW)
			if math.Abs(inked-inkW) > 1.5 || math.Abs(cored-coreW) > 1.5 {
				t.Errorf("%s: across the line %.2f px painted and %.2f px white core, want %.2f and %.2f", name, inked, cored, inkW, coreW)
			}
			edge := (lead.Width/2 + outline/2) * va.dpr
			for _, side := range []float64{-1, 1} {
				q := spec.Point{X: mid.X + n.X*edge*side, Y: mid.Y + n.Y*edge*side}
				if c := v.at(q); !near(c, ink, 12) {
					t.Errorf("%s: the line's edge at %v is %v, want ink %v", name, q, c, ink)
				}
			}

			// nothing farther from every stroke than half its ink width
			var dev [][]spec.Point
			reach := []float64{}
			for _, st := range f.Strokes {
				pts := make([]spec.Point, len(st.Pts))
				for k, p := range st.Pts {
					pts[k] = v.device(p)
				}
				dev = append(dev, pts)
				reach = append(reach, (st.Width/2+outline)*va.dpr+1)
			}
			stray := 0
			for y := painted.Min.Y; y < painted.Max.Y; y++ {
				for x := painted.Min.X; x < painted.Max.X; x++ {
					if img.NRGBAAt(x, y).A == 0 {
						continue
					}
					p := spec.Point{X: float64(x) + 0.5, Y: float64(y) + 0.5}
					inside := false
					for k, pts := range dev {
						for j := 0; j+1 < len(pts) && !inside; j++ {
							inside = segDist(p, pts[j], pts[j+1]) <= reach[k]
						}
						if inside {
							break
						}
					}
					if !inside {
						stray++
					}
				}
			}
			if stray > 0 {
				t.Errorf("%s: %d pixels painted beyond any stroke's ink", name, stray)
			}

			for _, st := range f.Strokes {
				if st.Alpha <= 0 || st.Alpha >= 1 || len(st.Pts) < 3 {
					continue
				}
				q := v.device(st.Pts[len(st.Pts)/2])
				apart := true
				for _, o := range f.Strokes {
					if o.Alpha != 1 {
						continue
					}
					for j := 0; j+1 < len(o.Pts); j++ {
						if segDist(q, v.device(o.Pts[j]), v.device(o.Pts[j+1])) < (o.Width/2+outline)*va.dpr+st.Width*va.dpr+2 {
							apart = false
						}
					}
				}
				if !apart {
					continue
				}
				c := v.at(q)
				if math.Abs(float64(c.A)-st.Alpha*255) > 4 || colourDistance(c, white) > 12 {
					t.Errorf("%s: a stroke at alpha %.2f shows %v, want the white core at that alpha", name, st.Alpha, c)
				}
			}

			if sc.point {
				ap := f.Approach()
				if ap == (spec.Point{}) {
					t.Fatalf("%s: no approach direction", name)
				}
				tip := v.device(f.Tip)
				capPx := (lead.Width/2 + outline) * va.dpr
				walked := 0.0
				for v.at(spec.Point{X: tip.X + ap.X*walked, Y: tip.Y + ap.Y*walked}).A >= 128 && walked < 4*capPx {
					walked += 0.125
				}
				if math.Abs(walked-capPx) > 1.5 {
					t.Errorf("%s: the line ends %.2f px past the tip, want the round cap's %.2f", name, walked, capPx)
				}
				if sc.name != "swing" {
					far := math.Inf(-1)
					for y := painted.Min.Y; y < painted.Max.Y; y++ {
						for x := painted.Min.X; x < painted.Max.X; x++ {
							if img.NRGBAAt(x, y).A >= 128 {
								far = math.Max(far, (float64(x)+0.5-tip.X)*ap.X+(float64(y)+0.5-tip.Y)*ap.Y)
							}
						}
					}
					if math.Abs(far-capPx) > 2 {
						t.Errorf("%s: the farthest painted pixel lies %.2f px past the tip, want %.2f", name, far, capPx)
					}
				}
			}

			cell := image.Rect(0, 0, cellW, cellH).Add(image.Pt((si%4)*cellW, (vi*2+si/4)*cellH))
			imgdraw.Draw(sheet, cell, &image.Uniform{va.ground}, image.Point{}, imgdraw.Src)
			scaled := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
			for y := range rect.Dy() {
				for x := range rect.Dx() {
					scaled.SetNRGBA(x, y, img.NRGBAAt(int(float64(x)*va.dpr), int(float64(y)*va.dpr)))
				}
			}
			imgdraw.Draw(sheet, scaled.Bounds().Add(cell.Min).Add(rect.Min), scaled, image.Point{}, imgdraw.Over)
		}
	}

	dirs := []string{t.TempDir()}
	if d := os.Getenv("HYAKKI_ONI_SHEET_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(d, "oni-strand-sheet.png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, sheet); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("strand sheet: %s (rows dark@1 ×2, light@%.1f ×2; reach tap hold swing / perch lasso region rewind)", path, s.Strand.Stroke.DPRCap)
	}
}

// TestPaintStrandClearsAndIsDeterministic: a frame paints the same pixels
// into a dirty buffer as into a fresh one, and an empty frame clears it.
func TestPaintStrandClearsAndIsDeterministic(t *testing.T) {
	s := loadSpec(t)
	e := strand.New(s)
	e.Step(0, strandEnv)
	e.Target(strand.Target{Seq: 1, Box: pointBox, Quiet: true})
	f := stepFor(e, 500*time.Millisecond).Frame
	fresh, rect, _ := paintFrame(f, darkLook(), 1)
	dirty := image.NewNRGBA(fresh.Bounds())
	for i := range dirty.Pix {
		dirty.Pix[i] = 200
	}
	render.PaintStrand(dirty, rect.Min, f, darkLook(), 1)
	if n := differing(fresh, dirty, 0); n != 0 {
		t.Fatalf("painting over an old frame left %d pixels different", n)
	}
	if r := render.PaintStrand(dirty, rect.Min, strand.Frame{}, darkLook(), 1); !r.Empty() {
		t.Fatalf("an empty frame painted %v", r)
	}
	for i := 3; i < len(dirty.Pix); i += 4 {
		if dirty.Pix[i] != 0 {
			t.Fatal("an empty frame left pixels behind")
		}
	}
}

// TestAnchorStaysOnTheDrawing: the strand's root is a painted pixel of the
// character at rest, in every state and through every gesture, at two sizes.
func TestAnchorStaysOnTheDrawing(t *testing.T) {
	s := loadSpec(t)
	c := shipped(t, s)
	poses := []motion.Pose{rest()}
	for _, id := range spec.StateIDs {
		poses = append(poses, statePose(s, id))
	}
	for _, id := range spec.GestureIDs {
		poses = append(poses, gesturePoses(t, s, id)...)
	}
	for _, scale := range []float64{1, 1.6} {
		for i, pose := range poses {
			img, _ := draw(c, pose, darkLook(), scale, 1)
			a := c.Anchor(pose, scale)
			if px := img.NRGBAAt(int(a.X), int(a.Y)); px.A < 200 {
				t.Fatalf("scale %v pose %d (%s %s): the anchor %v is off the drawing (alpha %d)", scale, i, pose.State, pose.Gesture, a, px.A)
			}
		}
	}
}

// TestAnchorFollowsThePose: the root floats with the hover offset, grows
// with the size factor and turns with a lean about the character's base.
func TestAnchorFollowsThePose(t *testing.T) {
	s := loadSpec(t)
	c := shipped(t, s)
	a0 := c.Anchor(rest(), 1)
	hover := rest()
	hover.HoverY = 6
	if a := c.Anchor(hover, 1); math.Abs(a.X-a0.X) > 1e-9 || math.Abs(a.Y-a0.Y-6) > 1e-9 {
		t.Fatalf("hovering 6 px moved the anchor from %v to %v", a0, a)
	}
	ext := c.Extent(2)
	if a := c.Anchor(rest(), 2); math.Abs(a.X-float64(ext.X)/2-2*(a0.X-float64(c.Extent(1).X)/2)) > 1e-6 {
		t.Fatalf("at size 2 the anchor sits %v from centre, want twice %v", a.X-float64(ext.X)/2, a0.X-float64(c.Extent(1).X)/2)
	}
	lean := rest()
	lean.LeanX = 0.2
	a := c.Anchor(lean, 1)
	if a.X >= a0.X-0.5 && a.X <= a0.X+0.5 {
		t.Fatalf("leaning did not move the anchor (%v vs %v)", a, a0)
	}
}

// TestProfileAnchorMovesOnlyTheRoot: a profile's artwork.anchor puts the
// root on that spot of its own drawing and changes nothing drawn.
func TestProfileAnchorMovesOnlyTheRoot(t *testing.T) {
	s := loadSpec(t)
	plainDir := variant(t, "testmoth", "plainmoth", nil, nil)
	anchoredDir := variant(t, "testmoth", "anchoredmoth", func(p map[string]any) {
		p["artwork"].(map[string]any)["anchor"] = map[string]any{"x": 0.5, "y": 0.5}
	}, nil)
	plain := fromDir(t, s, plainDir, "plainmoth")
	anchored := fromDir(t, s, anchoredDir, "anchoredmoth")
	ext := anchored.Extent(1)
	if a := anchored.Anchor(rest(), 1); math.Abs(a.X-float64(ext.X)/2) > 1e-9 || math.Abs(a.Y-float64(ext.Y)/2) > 1e-9 {
		t.Fatalf("anchor at the art's centre is %v, want the middle of %v", a, ext)
	}
	if plain.Anchor(rest(), 1) == anchored.Anchor(rest(), 1) {
		t.Fatal("the profile anchor did not move the root")
	}
	for _, id := range []string{"idle", "listening"} {
		pa, _ := draw(plain, statePose(s, id), darkLook(), 1, 1)
		pb, _ := draw(anchored, statePose(s, id), darkLook(), 1, 1)
		if n := differing(pa, pb, 0); n != 0 {
			t.Fatalf("%s: the anchor changed %d drawn pixels", id, n)
		}
	}
}

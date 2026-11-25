package gio_test

import (
	"image"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/render/desktoplook"
	rendergio "ryoku-rashin/internal/oni/render/gio"
	"ryoku-rashin/internal/oni/spec"
)

// TestLayoutMatchesRaster: the preview shows the pixels the native surface
// commits for the same character, pose and look.
func TestLayoutMatchesRaster(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	d := profile.Default()
	c, err := render.Load(d.Profile, d.Files, s)
	if err != nil {
		t.Fatal(err)
	}
	const side = 216
	win, err := headless.NewWindow(side, side)
	if err != nil {
		t.Fatalf("headless GPU: %v", err)
	}
	defer win.Release()

	e := motion.New(s)
	e.SetState("listening", "")
	var pose motion.Pose
	for range 120 {
		pose = e.Step(16*time.Millisecond, motion.Input{}).Pose
	}
	for _, look := range []render.Look{desktoplook.FromRoles(palette.Signature(true)), desktoplook.FromRoles(palette.Signature(false))} {
		ops := new(op.Ops)
		gtx := layout.Context{Ops: ops, Constraints: layout.Exact(image.Pt(side, side))}
		dims := rendergio.Layout(gtx, c, pose, look, side)
		if dims.Size != image.Pt(side, side) {
			t.Fatalf("Layout returned %v", dims.Size)
		}
		if err := win.Frame(ops); err != nil {
			t.Fatal(err)
		}
		shot := image.NewRGBA(image.Rect(0, 0, side, side))
		if err := win.Screenshot(shot); err != nil {
			t.Fatal(err)
		}

		want := image.NewNRGBA(image.Rect(0, 0, side, side))
		painted := c.Draw(want, pose, look, float64(side)/float64(c.Extent(1).X), 1)
		if painted.Empty() {
			t.Fatal("raster drew nothing")
		}

		worst, off := 0, 0
		for i := 0; i < len(shot.Pix); i += 4 {
			a := int(want.Pix[i+3])
			for ch := range 4 {
				w := a
				if ch < 3 {
					w = (int(want.Pix[i+ch])*a + 127) / 255
				}
				diff := w - int(shot.Pix[i+ch])
				if diff < 0 {
					diff = -diff
				}
				worst = max(worst, diff)
				if diff > 3 {
					off++
					break
				}
			}
		}
		t.Logf("dark=%v: worst channel difference %d, %d of %d pixels beyond 3", look.Dark, worst, off, side*side)
		if off > side*side/200 || worst > 24 {
			t.Errorf("dark=%v: Gio preview and raster disagree: %d pixels beyond tolerance, worst %d", look.Dark, off, worst)
		}
	}
}

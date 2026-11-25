package render_test

import (
	"image"
	"testing"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
)

func rest() motion.Pose { return motion.Pose{State: "idle", Scale: 1, Bright: 1} }

// TestPoseTransforms: each pose channel the renderer honours changes the
// drawing the way its name says.
func TestPoseTransforms(t *testing.T) {
	s := loadSpec(t)
	c := shipped(t, s)
	look := darkLook()
	base, br := draw(c, rest(), look, 1, 1)

	t.Run("scale", func(t *testing.T) {
		p := rest()
		p.Scale = 1.2
		_, r := draw(c, p, look, 1, 1)
		if r.Dx() <= br.Dx() || r.Dy() <= br.Dy() {
			t.Errorf("scale 1.2 drew %v, rest %v", r, br)
		}
	})
	t.Run("squash", func(t *testing.T) {
		p := rest()
		p.Squash = 0.2
		_, r := draw(c, p, look, 1, 1)
		if r.Dx() <= br.Dx() || r.Dy() >= br.Dy() {
			t.Errorf("squash drew %v, rest %v: want wider and shorter", r, br)
		}
		if d := r.Max.Y - br.Max.Y; d < -1 || d > 1 {
			t.Errorf("squash moved the base by %d", d)
		}
	})
	t.Run("lean", func(t *testing.T) {
		p := rest()
		p.LeanX = 0.3
		img, _ := draw(c, p, look, 1, 1)
		top := func(im *image.NRGBA, r image.Rectangle) float64 {
			var sum, n float64
			for y := r.Min.Y; y < r.Min.Y+r.Dy()/3; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					a := float64(im.NRGBAAt(x, y).A)
					sum += a * float64(x)
					n += a
				}
			}
			return sum / n
		}
		_, lr := draw(c, p, look, 1, 1)
		if top(img, lr) <= top(base, br)+2 {
			t.Error("leaning right did not move the top of the figure right")
		}
	})
	t.Run("hover", func(t *testing.T) {
		p := rest()
		p.HoverY = 6
		_, r := draw(c, p, look, 1, 1)
		if r != br.Add(image.Pt(0, 6)) {
			t.Errorf("hover 6 drew %v, rest %v", r, br)
		}
	})
	t.Run("brightness", func(t *testing.T) {
		p := rest()
		p.Bright = 0.5
		img, _ := draw(c, p, look, 1, 1)
		if !alphaEqual(img, base) {
			t.Error("brightness changed the shape")
		}
		if luma(img) >= luma(base)*0.75 {
			t.Errorf("brightness 0.5: luma %.1f, rest %.1f", luma(img), luma(base))
		}
	})
	t.Run("accent only while active", func(t *testing.T) {
		for _, lap := range []motion.Lap{
			{From: "idle", To: "idle", Progress: 1, Glow: 1},
			{From: "idle", To: "listening", Progress: 1, Glow: 0},
		} {
			p := rest()
			p.Lap = lap
			img, _ := draw(c, p, look, 1, 1)
			if differing(img, base, 0) != 0 {
				t.Errorf("lap %+v drew an accent", lap)
			}
		}
		p := rest()
		p.Lap = motion.Lap{From: "idle", To: "listening", Progress: 1, Glow: 1}
		_, r := draw(c, p, look, 1, 1)
		if r.Dx() <= br.Dx()+4 || r.Dy() <= br.Dy()+4 {
			t.Errorf("active accent drew %v, rest %v: want a halo", r, br)
		}
	})
	t.Run("lap hands over colour", func(t *testing.T) {
		p := rest()
		p.Lap = motion.Lap{From: "listening", To: "speaking", Progress: 0.5, Glow: 1}
		img, _ := draw(c, p, look, 1, 1)
		from, to := accentPixels(img, look.Accent["listening"], 20), accentPixels(img, look.Accent["speaking"], 20)
		if from < 20 || to < 20 {
			t.Errorf("half-way lap shows %d listening and %d speaking pixels", from, to)
		}
	})
}

// TestExtentHoldsEveryPose: at the biggest size on a 2x output the most
// extreme pose, glow included, stays inside the extent.
func TestExtentHoldsEveryPose(t *testing.T) {
	s := loadSpec(t)
	big, _ := s.Size("big")
	r := s.Springs.PoseRanges
	chars := map[string]*render.Character{"oni": shipped(t, s), "testmoth": fromDir(t, s, fixtures, "testmoth")}
	hover := 0.0
	for _, st := range s.States {
		hover = max(hover, st.Hover[0])
	}
	for _, sign := range []float64{-1, 1} {
		p := motion.Pose{
			State: "listening", Scale: r.Scale[1], Squash: r.Squash[1], LeanX: sign * r.Lean[1], LeanY: sign * r.Lean[1],
			HoverY: sign * hover, Bright: r.Bright[1],
			Lap: motion.Lap{From: "listening", To: "listening", Progress: 1, Glow: 1},
		}
		for name, ch := range chars {
			img, painted := draw(ch, p, darkLook(), big, 2)
			inner := img.Bounds().Inset(1)
			if painted.Empty() || !painted.In(inner) {
				t.Errorf("%s sign %v: painted %v outside %v", name, sign, painted, inner)
			}
		}
	}
}

func luma(img *image.NRGBA) float64 {
	var sum, n float64
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] == 255 {
			sum += 0.2126*float64(img.Pix[i]) + 0.7152*float64(img.Pix[i+1]) + 0.0722*float64(img.Pix[i+2])
			n++
		}
	}
	return sum / n
}

func BenchmarkDraw(b *testing.B) {
	s, err := spec.Load()
	if err != nil {
		b.Fatal(err)
	}
	d := profile.Default()
	c, err := render.Load(d.Profile, d.Files, s)
	if err != nil {
		b.Fatal(err)
	}
	for _, bc := range []struct {
		name  string
		state string
		dpr   float64
	}{{"idle-1x", "idle", 1}, {"listening-1x", "listening", 1}, {"listening-2x", "listening", 2}} {
		b.Run(bc.name, func(b *testing.B) {
			pose := statePose(s, bc.state)
			n := c.Extent(1)
			img := image.NewNRGBA(image.Rect(0, 0, int(float64(n.X)*bc.dpr), int(float64(n.Y)*bc.dpr)))
			look := darkLook()
			for b.Loop() {
				c.Draw(img, pose, look, 1, bc.dpr)
			}
		})
	}
}

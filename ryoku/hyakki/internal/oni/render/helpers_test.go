package render_test

import (
	"encoding/json"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/render/desktoplook"
	"ryoku-rashin/internal/oni/spec"
)

const fixtures = "../profile/testdata"

const frame = 16 * time.Millisecond

func loadSpec(t *testing.T) *spec.OniSpec {
	t.Helper()
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func darkLook() render.Look  { return desktoplook.FromRoles(palette.Signature(true)) }
func lightLook() render.Look { return desktoplook.FromRoles(palette.Signature(false)) }

func shipped(t *testing.T, s *spec.OniSpec) *render.Character {
	t.Helper()
	d := profile.Default()
	if !d.Valid {
		t.Fatalf("shipped oni invalid: %v", d.Report.Errors)
	}
	c, err := render.Load(d.Profile, d.Files, s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// fromDir loads character id the way the daemon does: the profile loader
// over a folder of characters, then its validated files.
func fromDir(t *testing.T, s *spec.OniSpec, dir, id string) *render.Character {
	t.Helper()
	l, _ := profile.Load(os.DirFS(dir), id)
	if !l.Valid {
		t.Fatalf("%s invalid: %v", id, l.Report.Errors)
	}
	c, err := render.Load(l.Profile, l.Files, s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// variant copies fixture from into a temporary characters folder as id,
// lets edit change its profile.json and adds extra files.
func variant(t *testing.T, from, id string, edit func(map[string]any), extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(fixtures, from)
	dst := filepath.Join(root, id)
	err := filepath.WalkDir(src, func(p string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if de.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	var prof map[string]any
	data, err := os.ReadFile(filepath.Join(dst, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &prof); err != nil {
		t.Fatal(err)
	}
	prof["id"] = id
	if edit != nil {
		edit(prof)
	}
	out, _ := json.Marshal(prof)
	if err := os.WriteFile(filepath.Join(dst, "profile.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range extra {
		if err := os.WriteFile(filepath.Join(dst, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// statePose is the engine's pose a while after entering state id, once its
// springs and accent lap have settled.
func statePose(s *spec.OniSpec, id string) motion.Pose {
	e := motion.New(s)
	e.SetState(id, "")
	var tk motion.Tick
	for range 150 {
		tk = e.Step(frame, motion.Input{})
	}
	return tk.Pose
}

// gesturePoses are the engine's poses through gesture id played from rest.
func gesturePoses(t *testing.T, s *spec.OniSpec, id string) []motion.Pose {
	t.Helper()
	e := motion.New(s)
	for range 30 {
		e.Step(frame, motion.Input{})
	}
	if !e.Play(id) {
		t.Fatalf("engine refused gesture %s", id)
	}
	dur := s.Gestures.DurationS[id]
	var out []motion.Pose
	for i := 0; float64(i)*frame.Seconds() < dur; i++ {
		tk := e.Step(frame, motion.Input{})
		if tk.Pose.Gesture == id {
			out = append(out, tk.Pose)
		}
	}
	if len(out) == 0 {
		t.Fatalf("gesture %s never showed in the pose", id)
	}
	return out
}

func draw(c *render.Character, pose motion.Pose, look render.Look, scale, dpr float64) (*image.NRGBA, image.Rectangle) {
	n := c.Extent(scale)
	side := int(math.Ceil(float64(n.X) * dpr))
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	r := c.Draw(img, pose, look, scale, dpr)
	return img, r
}

// differing counts pixels whose channels differ by more than tol.
func differing(a, b *image.NRGBA, tol int) int {
	n := 0
	for i := 0; i < len(a.Pix); i += 4 {
		pa := premulAt(a.Pix[i:])
		pb := premulAt(b.Pix[i:])
		for ch := range 4 {
			if abs(pa[ch]-pb[ch]) > tol {
				n++
				break
			}
		}
	}
	return n
}

func premulAt(p []uint8) [4]int {
	a := int(p[3])
	return [4]int{int(p[0]) * a / 255, int(p[1]) * a / 255, int(p[2]) * a / 255, a}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func colourDistance(a, b color.NRGBA) float64 {
	dr, dg, db := float64(a.R)-float64(b.R), float64(a.G)-float64(b.G), float64(a.B)-float64(b.B)
	return math.Sqrt(dr*dr + dg*dg + db*db)
}

// accentPixels counts the halo pixels (translucent, outside the opaque
// figure) whose colour is within tol of c.
func accentPixels(img *image.NRGBA, c color.NRGBA, tol float64) int {
	n := 0
	for i := 0; i < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		if a < 24 || a > 250 {
			continue
		}
		if colourDistance(color.NRGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}, c) <= tol {
			n++
		}
	}
	return n
}

func alphaEqual(a, b *image.NRGBA) bool {
	for i := 3; i < len(a.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] {
			return false
		}
	}
	return true
}

func hasFallback(fs []profile.Fallback, want profile.Fallback) bool {
	for _, f := range fs {
		if f == want {
			return true
		}
	}
	return false
}

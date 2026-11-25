package render_test

import (
	"image"
	"image/color"
	imgdraw "image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/profile"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
)

// TestContactSheet draws oni and testmoth in every state and through every
// gesture, in Ryoku's dark and light looks, writes the sheet, and checks
// every state looks different from every other and every gesture moves the
// character away from its rest.
func TestContactSheet(t *testing.T) {
	s := loadSpec(t)
	chars := []struct {
		name string
		c    *render.Character
	}{
		{"oni", shipped(t, s)},
		{"testmoth", fromDir(t, s, fixtures, "testmoth")},
	}
	looks := []struct {
		name   string
		look   render.Look
		ground color.NRGBA
	}{
		{"dark", darkLook(), darkLook().Ink},
		{"light", lightLook(), lightLook().White},
	}
	cell := chars[0].c.Extent(1).X
	cols := len(spec.StateIDs) + len(spec.GestureIDs)
	sheet := image.NewNRGBA(image.Rect(0, 0, cols*cell, len(chars)*len(looks)*cell))

	for ci, ch := range chars {
		states := map[string]*image.NRGBA{}
		for i, id := range spec.StateIDs {
			pose := statePose(s, id)
			img, r := draw(ch.c, pose, looks[0].look, 1, 1)
			if r.Empty() {
				t.Fatalf("%s %s: drew nothing", ch.name, id)
			}
			states[id] = img
			for li, l := range looks {
				shown := img
				if li > 0 {
					shown, _ = draw(ch.c, pose, l.look, 1, 1)
				}
				put(sheet, shown, l.ground, i*cell, (ci*len(looks)+li)*cell)
			}
		}
		for i, a := range spec.StateIDs {
			for _, b := range spec.StateIDs[i+1:] {
				if n := differing(states[a], states[b], 8); n < 20 {
					t.Errorf("%s: %s and %s differ in only %d pixels", ch.name, a, b, n)
				}
			}
		}
		rest, _ := draw(ch.c, statePose(s, "idle"), looks[0].look, 1, 1)
		for gi, id := range spec.GestureIDs {
			best, bestPose := -1, motion.Pose{}
			for _, pose := range gesturePoses(t, s, id) {
				img, _ := draw(ch.c, pose, looks[0].look, 1, 1)
				if n := differing(rest, img, 8); n > best {
					best, bestPose = n, pose
				}
			}
			if best < 20 {
				t.Errorf("%s: gesture %s moves only %d pixels from rest", ch.name, id, best)
			}
			for li, l := range looks {
				img, _ := draw(ch.c, bestPose, l.look, 1, 1)
				put(sheet, img, l.ground, (len(spec.StateIDs)+gi)*cell, (ci*len(looks)+li)*cell)
			}
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
		f, err := os.Create(filepath.Join(d, "oni-contact-sheet.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, sheet); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("contact sheet: %s (rows oni dark, oni light, testmoth dark, testmoth light; columns %s then gestures %s)",
			filepath.Join(d, "oni-contact-sheet.png"), strings.Join(spec.StateIDs, " "), strings.Join(spec.GestureIDs, " "))
	}
}

func put(sheet, img *image.NRGBA, ground color.NRGBA, x, y int) {
	r := img.Bounds().Add(image.Pt(x, y))
	imgdraw.Draw(sheet, r, &image.Uniform{ground}, image.Point{}, imgdraw.Src)
	imgdraw.Draw(sheet, r, img, image.Point{}, imgdraw.Over)
}

// TestStateAccents: the accent shows around the character while listening,
// thinking, speaking or in error, in that state's own colour, and never at
// idle; the four colours stay apart in Ryoku's dark and light looks.
func TestStateAccents(t *testing.T) {
	s := loadSpec(t)
	accented := []string{"listening", "thinking", "speaking", "error"}
	for _, l := range []struct {
		name string
		look render.Look
	}{{"dark", darkLook()}, {"light", lightLook()}} {
		for i, a := range accented {
			for _, b := range accented[i+1:] {
				ca, cb := l.look.Accent[s.Colour.StateColour[a]], l.look.Accent[s.Colour.StateColour[b]]
				if d := colourDistance(ca, cb); d < 48 {
					t.Errorf("%s: %s %v and %s %v accents are only %.0f apart", l.name, a, ca, b, cb, d)
				}
			}
		}
		for _, name := range []string{"oni", "testmoth"} {
			c := shipped(t, s)
			if name == "testmoth" {
				c = fromDir(t, s, fixtures, "testmoth")
			}
			idle, _ := draw(c, statePose(s, "idle"), l.look, 1, 1)
			for _, state := range accented {
				want := l.look.Accent[s.Colour.StateColour[state]]
				if n := accentPixels(idle, want, 20); n != 0 {
					t.Errorf("%s %s: idle shows %d pixels of the %s accent", l.name, name, n, state)
				}
				img, _ := draw(c, statePose(s, state), l.look, 1, 1)
				own := accentPixels(img, want, 20)
				if own < 60 {
					t.Errorf("%s %s: %s shows only %d accent pixels", l.name, name, state, own)
				}
				for _, other := range accented {
					oc := l.look.Accent[s.Colour.StateColour[other]]
					if other == state || oc == want {
						continue
					}
					if n := accentPixels(img, oc, 20); n*10 > own {
						t.Errorf("%s %s: %s shows %d pixels of the %s accent beside %d of its own", l.name, name, state, n, other, own)
					}
				}
			}
		}
	}
}

// TestSilhouetteFallback: a character with no artwork at all draws the
// neutral silhouette (Oni's static shape in ink and white, no body colour)
// in every state and reports each replaced slot.
func TestSilhouetteFallback(t *testing.T) {
	s := loadSpec(t)
	bare := fromDir(t, s, fixtures, "soft-bare")
	oni := shipped(t, s)
	fb := bare.Fallbacks()
	if !hasFallback(fb, profile.Fallback{Slot: "artwork", Reason: profile.FallbackMissing, Used: profile.UsedSilhouette}) {
		t.Errorf("artwork silhouette not reported: %v", fb)
	}
	for _, id := range spec.StateIDs {
		if !hasFallback(fb, profile.Fallback{Slot: "animations.states." + id, Reason: profile.FallbackMissing, Used: profile.UsedStaticPose}) {
			t.Errorf("state %s fallback not reported", id)
		}
	}
	for _, id := range spec.GestureIDs {
		if bare.HasGesture(id) {
			t.Errorf("silhouette claims gesture %s", id)
		}
		if !hasFallback(fb, profile.Fallback{Slot: "animations.gestures." + id, Reason: profile.FallbackMissing, Used: profile.UsedBasePose}) {
			t.Errorf("gesture %s fallback not reported", id)
		}
	}
	look := darkLook()
	pose := motion.Pose{State: "idle", Scale: 1, Bright: 1}
	got, r := draw(bare, pose, look, 1, 1)
	if r.Empty() {
		t.Fatal("silhouette drew nothing")
	}
	want, _ := draw(oni, pose, look, 1, 1)
	if !alphaEqual(got, want) {
		t.Error("silhouette is not Oni's static shape")
	}
	bodyPixels := func(img *image.NRGBA) int {
		n := 0
		for i := 0; i < len(img.Pix); i += 4 {
			if img.Pix[i+3] == 255 && colourDistance(color.NRGBA{img.Pix[i], img.Pix[i+1], img.Pix[i+2], 255}, look.Body) <= 2 {
				n++
			}
		}
		return n
	}
	if sil, own := bodyPixels(got), bodyPixels(want); own < 100 || sil*20 > own {
		t.Errorf("silhouette shows %d body-coloured pixels, Oni %d", sil, own)
	}
	for _, id := range spec.StateIDs {
		img, r := draw(bare, motion.Pose{State: id, Scale: 1, Bright: 1}, look, 1, 1)
		if r.Empty() || !alphaEqual(img, got) {
			t.Errorf("state %s does not draw the silhouette", id)
		}
	}
}

// TestMissingSlotFallback: a state slot left out draws the character's
// static image and is reported; the other slots keep their own poses.
func TestMissingSlotFallback(t *testing.T) {
	s := loadSpec(t)
	dropListening := func(p map[string]any) {
		delete(p["animations"].(map[string]any)["states"].(map[string]any), "listening")
	}
	missing := fromDir(t, s, variant(t, "testmoth", "moth-gap", dropListening, nil), "moth-gap")
	staticListening := func(p map[string]any) {
		p["animations"].(map[string]any)["states"].(map[string]any)["listening"] = "moth.svg"
	}
	explicit := fromDir(t, s, variant(t, "testmoth", "moth-static", staticListening, nil), "moth-static")
	full := fromDir(t, s, fixtures, "testmoth")

	if !hasFallback(missing.Fallbacks(), profile.Fallback{Slot: "animations.states.listening", Reason: profile.FallbackMissing, Used: profile.UsedStaticPose}) {
		t.Errorf("listening fallback not reported: %v", missing.Fallbacks())
	}
	if hasFallback(full.Fallbacks(), profile.Fallback{Slot: "animations.states.listening", Reason: profile.FallbackMissing, Used: profile.UsedStaticPose}) {
		t.Error("complete testmoth reports a listening fallback")
	}
	look := darkLook()
	pose := statePose(s, "listening")
	got, r := draw(missing, pose, look, 1, 1)
	if r.Empty() {
		t.Fatal("missing slot drew nothing")
	}
	want, _ := draw(explicit, pose, look, 1, 1)
	if differing(got, want, 0) != 0 {
		t.Error("missing listening slot does not draw the static image")
	}
	own, _ := draw(full, pose, look, 1, 1)
	if differing(got, own, 8) < 20 {
		t.Error("missing slot looks the same as the character's own listening pose")
	}
	thinking := statePose(s, "thinking")
	a, _ := draw(missing, thinking, look, 1, 1)
	b, _ := draw(full, thinking, look, 1, 1)
	if differing(a, b, 0) != 0 {
		t.Error("a missing listening slot changed the thinking pose")
	}
}

// TestUnsupportedSVGReported: artwork that asks for something this
// renderer cannot draw still draws the rest and says what it left out.
func TestUnsupportedSVGReported(t *testing.T) {
	s := loadSpec(t)
	odd := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 48 48">` +
		`<circle cx="24" cy="24" r="14" fill="#C9B79C" stroke="#3A3028" stroke-width="2em" paint-order="stroke"/></svg>`
	useOdd := func(p map[string]any) { p["artwork"].(map[string]any)["image"] = "odd.svg" }
	c := fromDir(t, s, variant(t, "testmoth", "moth-odd", useOdd, map[string]string{"odd.svg": odd}), "moth-odd")
	var got []string
	for _, f := range c.Fallbacks() {
		if f.Reason == render.FallbackUnsupported {
			if !strings.HasPrefix(f.Slot, "odd.svg: ") || f.Used != render.UsedNotDrawn {
				t.Errorf("unsupported entry %+v", f)
			}
			got = append(got, f.Slot)
		}
	}
	if len(got) != 2 {
		t.Errorf("unsupported entries %v, want paint-order and the em stroke width", got)
	}
	_, r := draw(c, motion.Pose{State: "nonexistent", Scale: 1, Bright: 1}, darkLook(), 1, 1)
	if r.Empty() {
		t.Error("the drawable part of the artwork was dropped")
	}
}

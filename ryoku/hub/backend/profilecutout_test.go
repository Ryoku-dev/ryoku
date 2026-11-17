package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testCutout is a w×h figure whose left clearCols columns are clear ground.
func testCutout(t *testing.T, w, h, clearCols int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x >= clearCols {
				img.SetNRGBA(x, y, color.NRGBA{200, 40, 120, 255})
			}
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// cutoutMachine is a machine with a custom w×h picture on its card.
func cutoutMachine(t *testing.T, holder bool, w, h int) {
	t.Helper()
	pictureMachine(t, holder)
	hero := image.NewRGBA(image.Rect(0, 0, w, h))
	var raw bytes.Buffer
	if err := png.Encode(&raw, hero); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(filepath.Join(profileHeroDir(), "hero.png"), raw.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(profileConfigPath(), []byte(`{"hero":{"kind":"custom","source":"hero.png"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProfileStandStyleNeedsItsOwner(t *testing.T) {
	pictureMachine(t, false)
	if _, err := validateProfile(json.RawMessage(`{"style":"stand"}`)); err == nil || err.Error() != "a card style comes with a team badge, and this machine holds none" {
		t.Fatal("a machine without a badge chose the Stand style", err)
	}
	for _, accepted := range []string{`{"style":""}`, `{"style":"classic"}`, `{"text":{"standName":"` + strings.Repeat("星", 64) + `"}}`, `{"hero":{"cutout":true}}`, `{"hero":{"cutout":false}}`} {
		if _, err := validateProfile(json.RawMessage(accepted)); err != nil {
			t.Fatal(accepted, err)
		}
	}
	for _, refused := range []string{`{"style":"jojo"}`, `{"style":null}`, `{"style":1}`, `{"text":{"standName":"` + strings.Repeat("星", 65) + `"}}`,
		`{"text":{"standName":"<b>Star</b>"}}`, `{"text":{"standName":7}}`, `{"hero":{"cutout":"yes"}}`, `{"hero":{"cutout":1}}`, `{"hero":{"cutout":null}}`} {
		if _, err := validateProfile(json.RawMessage(refused)); err == nil {
			t.Fatal("accepted", refused)
		}
	}
	pictureMachine(t, true)
	if model, err := validateProfile(json.RawMessage(`{"style":"stand","text":{"standName":"Star Platinum"},"hero":{"cutout":true}}`)); err != nil || model.Style != "stand" || !model.Hero.Cutout {
		t.Fatal("the Stand style's owner could not choose it", err)
	}
	// another badge holder makes their own look through a team design, never by taking this one
	otherHolder(t, "Friend")
	if _, err := validateProfile(json.RawMessage(`{"style":"stand"}`)); err == nil || err.Error() != "this card style belongs to another badge holder; make your own look through a team design" {
		t.Fatal("another badge holder chose the owner's style", err)
	}
	if _, err := validateProfile(json.RawMessage(`{"style":"classic","text":{"tagline":"my own words"}}`)); err != nil {
		t.Fatal(err)
	}
}

// otherHolder activates a badge for login on this machine, which owns no card style
func otherHolder(t *testing.T, login string) {
	t.Helper()
	owner, _, _ := badgeTestKeys(t)
	device, err := badgeDeviceSeed()
	if err != nil {
		t.Fatal(err)
	}
	if err := badgeActivate(badgeTestCode(t, 1, 2, owner, device, login)); err != nil || !badgeHolder() {
		t.Fatal("badge not held", err)
	}
}

func TestCutoutRefusals(t *testing.T) {
	cutoutMachine(t, false, 200, 100)
	good := writeChosen(t, "figure.png", testCutout(t, 200, 100, 20))
	if _, err := saveCutout(good); err == nil || err.Error() != "a cutout comes with a team badge" {
		t.Fatal("a machine without a badge kept a cutout", err)
	}
	// a badge whose login owns no card style keeps no cutout, but may still clear one
	otherHolder(t, "friend")
	if _, err := saveCutout(good); err == nil || err.Error() != "a cutout comes with a card style of your own; make your own look through a team design" {
		t.Fatal("a holder who owns no style kept a cutout", err)
	}
	if err := atomicWrite(filepath.Join(profileHeroDir(), "hero-cutout.png"), testCutout(t, 200, 100, 20), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCutout([]string{"--clear"}); err != nil {
		t.Fatal("a holder who owns no style could not clear a cutout", err)
	}
	if _, err := os.Stat(filepath.Join(profileHeroDir(), "hero-cutout.png")); !os.IsNotExist(err) {
		t.Fatal("--clear left the cutout", err)
	}
	pictureMachine(t, true)
	if _, err := saveCutout(good); err == nil || err.Error() != "choose your own picture first: the cutout is cut from it" {
		t.Fatal("a cutout was kept without the reader's own picture", err)
	}
	if err := atomicWrite(profileConfigPath(), []byte(`{"hero":{"kind":"gallery","source":"hawk.png"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := saveCutout(good); err == nil || !strings.HasPrefix(err.Error(), "choose your own picture first") {
		t.Fatal("a cutout was kept for a gallery picture", err)
	}

	cutoutMachine(t, true, 200, 100)
	var photo bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 200, 100)), nil); err != nil {
		t.Fatal(err)
	}
	oversized := writeChosen(t, "huge.png", nil)
	if err := os.Truncate(oversized, maxCutoutBytes+1); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		raw  []byte
		path string
		want string
	}{
		"jpeg":       {raw: photo.Bytes(), want: "a cutout is a PNG with a clear ground"},
		"gif":        {raw: testGIF(t, 1), want: "a cutout is a PNG with a clear ground"},
		"damaged":    {raw: testCutout(t, 200, 100, 20)[:60], want: "a cutout is a PNG with a clear ground"},
		"opaque":     {raw: testCutout(t, 200, 100, 0), want: "this picture has no clear ground; a cutout keeps only the figure"},
		"empty":      {raw: testCutout(t, 200, 100, 200), want: "this picture has no clear ground; a cutout keeps only the figure"},
		"barely cut": {raw: testCutout(t, 200, 100, 3), want: "this picture has no clear ground; a cutout keeps only the figure"},
		"small":      {raw: testCutout(t, 140, 70, 20), want: ""},
		"tiny":       {raw: testCutout(t, 60, 30, 10), want: "a cutout is 64 to 4096 pixels on each side; this one is 60x30"},
		"shape":      {raw: testCutout(t, 100, 100, 20), want: "the cutout needs your picture's shape (200x100); this one is 100x100"},
		"oversized":  {path: oversized, want: "this file is 8.0 MB; a cutout can be up to 8.0 MB"},
	} {
		path := tc.path
		if path == "" {
			path = writeChosen(t, "figure.png", tc.raw)
		}
		_, err := saveCutout(path)
		if tc.want == "" {
			if err != nil {
				t.Fatal(name, err)
			}
			continue
		}
		if err == nil || err.Error() != tc.want {
			t.Fatalf("%s: got %v, want %q", name, err, tc.want)
		}
	}
	if _, err := saveCutout(t.TempDir()); err == nil {
		t.Fatal("a directory was read as a cutout")
	}
}

func TestCutoutKeptAndCleared(t *testing.T) {
	cutoutMachine(t, true, 300, 200)
	chosen := append(testCutout(t, 303, 200, 40), "trailing secret"...)
	saved, err := saveCutout(writeChosen(t, "figure.png", chosen))
	if err != nil || saved != (savedCutout{Cutout: "hero-cutout.png", Width: 303, Height: 200}) {
		t.Fatal(saved, err)
	}
	if out, _ := json.Marshal(saved); string(out) != `{"cutout":"hero-cutout.png","width":303,"height":200}` {
		t.Fatal(string(out))
	}
	path := filepath.Join(profileHeroDir(), "hero-cutout.png")
	kept, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(kept, []byte("secret")) || !bytes.Equal(kept, testCutout(t, 303, 200, 40)) {
		t.Fatal("the cutout was kept with more than its pixels")
	}
	if profile, _ := os.ReadFile(profileConfigPath()); string(profile) != `{"hero":{"kind":"custom","source":"hero.png"}}` {
		t.Fatal("the cutout command edited profile.json", string(profile))
	}
	// a reader whose badge has gone still takes their cutout away
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if badgeHolder() {
		t.Fatal("badge still held")
	}
	for range 2 {
		if err := runCutout([]string{"--clear"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("cutout not cleared", err)
	}
	if out, _ := json.Marshal(savedCutout{}); string(out) != `{"cutout":""}` {
		t.Fatal(string(out))
	}
	if err := runCutout(nil); err == nil {
		t.Fatal("cutout ran without a path")
	}
}

func TestProfileCarriesCutout(t *testing.T) {
	cutoutMachine(t, true, 200, 100)
	const profile = `{"style":"stand","hero":{"kind":"custom","source":"hero.png","cutout":true,"zoom":1.5},"text":{"name":"Jotaro","standName":"Star Platinum"}}`
	if err := atomicWrite(profileConfigPath(), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := saveCutout(writeChosen(t, "figure.png", testCutout(t, 200, 100, 30))); err != nil {
		t.Fatal(err)
	}
	cutoutPath := filepath.Join(profileHeroDir(), "hero-cutout.png")
	cutout, _ := os.ReadFile(cutoutPath)
	share := filepath.Join(t.TempDir(), "share.ryoprofile")
	if err := exportProfile(share); err != nil {
		t.Fatal(err)
	}
	var env ryoEnvelope
	raw, _ := os.ReadFile(share)
	if err := json.Unmarshal(raw, &env); err != nil || env.Cutout == nil || env.Cutout.Name != "hero-cutout.png" {
		t.Fatal("export left the cutout out", err)
	}
	os.Remove(cutoutPath)
	os.Remove(profileConfigPath())
	if err := importProfile(share); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(profileConfigPath()); string(got) != profile {
		t.Fatalf("profile.json not restored exactly: %s", got)
	}
	if got, _ := os.ReadFile(cutoutPath); !bytes.Equal(got, cutout) {
		t.Fatal("cutout not restored")
	}

	// a cutout missing at export, or packed broken, leaves the card without one
	os.Remove(cutoutPath)
	if err := exportProfile(share); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(share)
	env = ryoEnvelope{}
	if err := json.Unmarshal(raw, &env); err != nil || env.Cutout != nil || bytes.Contains(raw, []byte(`"cutout":{`)) {
		t.Fatal("export packed a missing cutout", err)
	}
	broken := env
	broken.Cutout = &ryoHero{Name: "hero-cutout.png", B64: base64.StdEncoding.EncodeToString(testCutout(t, 200, 100, 0))}
	renamed := env
	renamed.Cutout = &ryoHero{Name: "../hero-cutout.png", B64: base64.StdEncoding.EncodeToString(cutout)}
	for _, sent := range []ryoEnvelope{env, broken, renamed} {
		out, _ := json.Marshal(sent)
		if err := os.WriteFile(share, out, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := importProfile(share); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(cutoutPath); !os.IsNotExist(err) {
			t.Fatal("a cutout that didn't travel was written", err)
		}
		var got, want map[string]any
		restored, _ := os.ReadFile(profileConfigPath())
		json.Unmarshal(restored, &got)
		json.Unmarshal([]byte(strings.Replace(profile, `"cutout":true`, `"cutout":false`, 1)), &want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("profile.json should only lose its cutout: %s", restored)
		}
	}

	// importing a Stand profile takes the style's owner: no badge, or another holder's, is refused
	pictureMachine(t, false)
	out, _ := json.Marshal(env)
	os.WriteFile(share, out, 0o644)
	if err := importProfile(share); err == nil || !strings.Contains(err.Error(), "a card style comes with a team badge") {
		t.Fatal("a machine without a badge imported the Stand style", err)
	}
	otherHolder(t, "friend")
	if err := importProfile(share); err == nil || !strings.Contains(err.Error(), "this card style belongs to another badge holder") {
		t.Fatal("another badge holder imported the owner's style", err)
	}
	if _, err := os.Stat(profileConfigPath()); !os.IsNotExist(err) {
		t.Fatal("a refused import wrote profile.json", err)
	}
}

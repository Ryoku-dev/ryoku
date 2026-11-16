package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// a compact profile.json: hero.kind=custom with a bare source name, plus a few
// other fields. Compact so the verbatim-carry round-trip is byte-exact.
const sampleProfile = `{"preset":"full","heroSide":"right","hero":{"kind":"custom","source":"hero.png","focalX":0.5,"focalY":0.4,"zoom":1,"invert":false},"text":{"name":"Nero","tagline":"力"},"vitals":["core","gpu"]}`

// Use a real PNG so the import exercises the bounded decoder.
var sampleHero = func() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{200, 100, 50, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}()

func TestProfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := os.MkdirAll(profileDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profileConfigPath(), []byte(sampleProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	heroPath := filepath.Join(profileHeroDir(), "hero.png")
	if err := os.MkdirAll(profileHeroDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(heroPath, sampleHero, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "share.ryoprofile")
	if err := exportProfile(dst); err != nil {
		t.Fatalf("exportProfile: %v", err)
	}

	// wipe the live state so import has to restore it from the envelope alone.
	if err := os.Remove(profileConfigPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(heroPath); err != nil {
		t.Fatal(err)
	}

	if err := importProfile(dst); err != nil {
		t.Fatalf("importProfile: %v", err)
	}

	got, err := os.ReadFile(profileConfigPath())
	if err != nil {
		t.Fatalf("read restored profile.json: %v", err)
	}
	if string(got) != sampleProfile {
		t.Fatalf("profile.json not restored exactly:\n want %s\n got  %s", sampleProfile, got)
	}
	gotHero, err := os.ReadFile(heroPath)
	if err != nil {
		t.Fatalf("read restored hero: %v", err)
	}
	if !bytes.Equal(gotHero, sampleHero) {
		t.Fatalf("hero bytes not restored exactly:\n want %v\n got  %v", sampleHero, gotHero)
	}
}

func TestProfileImportRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	garbage := filepath.Join(t.TempDir(), "bad.ryoprofile")
	if err := os.WriteFile(garbage, []byte(`{"profile":{"preset":"full"}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := importProfile(garbage); err == nil {
		t.Fatal("importProfile accepted a file lacking ryoprofile:1")
	}
	if _, err := os.Stat(profileConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("profile.json must not exist after a rejected import, stat err = %v", err)
	}
}

// TestProfileImportFailedSettingsWriteLeavesNoTrace: the hero lands before
// profile.json, so a profile.json that cannot be written puts the old hero back.
func TestProfileImportFailedSettingsWriteLeavesNoTrace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(profileDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profileConfigPath(), []byte(sampleProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	heroPath := filepath.Join(profileHeroDir(), "hero.png")
	if err := os.MkdirAll(profileHeroDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(heroPath, sampleHero, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "share.ryoprofile")
	if err := exportProfile(dst); err != nil {
		t.Fatalf("exportProfile: %v", err)
	}

	// a different box: its own hero, no profile.json, and a settings dir it cannot write
	oldHero := []byte("old hero")
	if err := os.WriteFile(heroPath, oldHero, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(profileConfigPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(profileDir(), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(profileDir(), 0o755) })

	if err := importProfile(dst); err == nil {
		t.Fatal("import succeeded with profile.json unwritable")
	}
	got, err := os.ReadFile(heroPath)
	if err != nil || !bytes.Equal(got, oldHero) {
		t.Fatalf("hero after a failed import = %q (%v), want %q", got, err, oldHero)
	}
	if _, err := os.Stat(profileConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("profile.json appeared after a failed import, stat err = %v", err)
	}

	if err := os.Chmod(profileDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := importProfile(dst); err != nil {
		t.Fatalf("import after the failure: %v", err)
	}
	if got, err := os.ReadFile(profileConfigPath()); err != nil || string(got) != sampleProfile {
		t.Fatalf("profile.json after a good import = %q (%v), want the sample", got, err)
	}
	if got, err := os.ReadFile(heroPath); err != nil || !bytes.Equal(got, sampleHero) {
		t.Fatalf("hero after a good import = %v (%v), want the sample", got, err)
	}
}

func TestCardPictureSharesOnlyTheRenderedPixels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Sharing does not need or export the profile or its original custom photo.
	rendered := filepath.Join(t.TempDir(), "rendered.png")
	writeTinyPNG(t, rendered)
	card := filepath.Join(t.TempDir(), "card.png")
	if err := writeCard(rendered, card); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(card)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("share changed the rendered PNG: %v", err)
	}
	if err := importProfile(card); err == nil {
		t.Fatal("profile import accepted a shared picture")
	}
	if _, err := os.Stat(profileConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("a refused picture changed the profile: %v", err)
	}
}

func writeTinyPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{200, 100, 50, 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProfileImportRejectsHostileFieldsWithoutWrites(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		hero    *ryoHero
	}{
		{name: "profile null", profile: `null`},
		{name: "profile array", profile: `[]`},
		{name: "unknown command", profile: `{"command":"sh -c id"}`},
		{name: "prototype", profile: `{"__proto__":{"command":"id"}}`},
		{name: "kind", profile: `{"hero":{"kind":"https"}}`},
		{name: "traversal", profile: `{"hero":{"kind":"custom","source":"../../private.png"}}`},
		{name: "encoded traversal", profile: `{"hero":{"kind":"gallery","source":"%2e%2e%2fprivate.png"}}`},
		{name: "url", profile: `{"hero":{"kind":"gallery","source":"https://example.com/a.png"}}`},
		{name: "query", profile: `{"hero":{"kind":"gallery","source":"david.png?x"}}`},
		{name: "gallery alphabet", profile: `{"hero":{"kind":"gallery","source":"David.png"}}`},
		{name: "external file override", profile: `{"hero":{"kind":"custom","source":"hero.png","file":"/etc/passwd"}}`},
		{name: "missing embedded image", profile: sampleProfile},
		{name: "mismatched image", profile: sampleProfile, hero: &ryoHero{Name: "../hero.png", B64: base64.StdEncoding.EncodeToString(sampleHero)}},
		{name: "invalid image", profile: sampleProfile, hero: &ryoHero{Name: "hero.png", B64: base64.StdEncoding.EncodeToString([]byte("not an image"))}},
		{name: "svg image", profile: sampleProfile, hero: &ryoHero{Name: "hero.png", B64: base64.StdEncoding.EncodeToString([]byte(`<svg><script>id</script></svg>`))}},
		{name: "invalid base64", profile: sampleProfile, hero: &ryoHero{Name: "hero.png", B64: "%%%"}},
		{name: "orphan image", profile: `{}`, hero: &ryoHero{Name: "hero.png", B64: base64.StdEncoding.EncodeToString(sampleHero)}},
		{name: "markup", profile: `{"text":{"name":"<img src='file:///etc/passwd'>"}}`},
		{name: "entity", profile: `{"text":{"tagline":"&#60;img src=x&#62;"}}`},
		{name: "terminal control", profile: `{"text":{"name":"a\u001bb"}}`},
		{name: "bidi control", profile: `{"text":{"name":"a\u202eb"}}`},
		{name: "invisible control", profile: `{"text":{"name":"a\u200bb"}}`},
		{name: "surrogate", profile: `{"text":{"name":"\ud800"}}`},
		{name: "long name", profile: `{"text":{"name":"` + strings.Repeat("a", 129) + `"}}`},
		{name: "long list", profile: `{"text":{"verse":[` + strings.Repeat(`"a",`, 16) + `"a"]}}`},
		{name: "name object", profile: `{"text":{"name":{"source":"file:///etc/passwd"}}}`},
		{name: "list type", profile: `{"text":{"epithets":"wrong"}}`},
		{name: "layout", profile: `{"preset":"../../x"}`},
		{name: "home layout", profile: `{"homeView":"remote"}`},
		{name: "hero side", profile: `{"heroSide":12}`},
		{name: "style", profile: `{"hero":{"style":"url"}}`},
		{name: "zoom", profile: `{"hero":{"zoom":4.01}}`},
		{name: "focal", profile: `{"hero":{"focalX":-0.1}}`},
		{name: "dither", profile: `{"hero":{"dither":0}}`},
		{name: "brightness", profile: `{"hero":{"brightness":0.61}}`},
		{name: "contrast", profile: `{"hero":{"contrast":-0.61}}`},
		{name: "saturation", profile: `{"hero":{"saturation":1.1}}`},
		{name: "numeric string", profile: `{"hero":{"zoom":"2"}}`},
		{name: "numeric null", profile: `{"hero":{"zoom":null}}`},
		{name: "boolean coercion", profile: `{"hero":{"invert":1}}`},
		{name: "unknown block", profile: `{"blocks":{"url":true}}`},
		{name: "combined blocks", profile: `{"blocks":{"epithets|telemetry":true}}`},
		{name: "block coercion", profile: `{"blocks":{"specs":"false"}}`},
		{name: "vital", profile: `{"vitals":["path"]}`},
		{name: "spec", profile: `{"specs":["command"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			if err := os.MkdirAll(profileHeroDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			prior := []byte(`{"text":{"name":"existing"}}`)
			if err := os.WriteFile(profileConfigPath(), prior, 0o600); err != nil {
				t.Fatal(err)
			}
			heroPath := filepath.Join(profileHeroDir(), "hero.png")
			if err := os.WriteFile(heroPath, sampleHero, 0o600); err != nil {
				t.Fatal(err)
			}
			env := ryoEnvelope{Ryoprofile: 1, Profile: json.RawMessage(tc.profile), Hero: tc.hero}
			raw, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			src := filepath.Join(t.TempDir(), "hostile.ryoprofile")
			if err := os.WriteFile(src, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := importProfile(src); err == nil {
				t.Fatal("hostile profile accepted")
			}
			if got, err := os.ReadFile(profileConfigPath()); err != nil || !bytes.Equal(got, prior) {
				t.Fatalf("rejected profile changed settings: %q, %v", got, err)
			}
			if got, err := os.ReadFile(heroPath); err != nil || !bytes.Equal(got, sampleHero) {
				t.Fatalf("rejected profile changed hero: %v", err)
			}
		})
	}
}

func TestProfileImportLimitsAndUTF8(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	src := filepath.Join(dir, "invalid.ryoprofile")
	for _, raw := range [][]byte{
		append([]byte(`{"ryoprofile":1,"profile":{"text":{"name":"`), append([]byte{0xff}, []byte(`"}}}`)...)...),
		[]byte(`{"ryoprofile":1,"profile":{"text":{"tagline":"` + strings.Repeat("a", maxProfileBytes) + `"}}}`),
	} {
		if err := os.WriteFile(src, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := importProfile(src); err == nil {
			t.Fatal("invalid UTF-8 or oversized settings accepted")
		}
	}
	if err := os.Truncate(src, maxProfileEnvelopeBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := importProfile(src); err == nil {
		t.Fatal("oversized envelope accepted")
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readProfileFile(fifo, maxProfileEnvelopeBytes); err == nil {
		t.Fatal("FIFO accepted as an envelope")
	}
	if _, err := os.Stat(profileConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("invalid input wrote settings: %v", err)
	}
}

func TestProfileImportCanonicalizesEmbeddedPixels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	raw, err := json.Marshal(ryoEnvelope{
		Ryoprofile: 1,
		Profile:    json.RawMessage(`{"hero":{"kind":"custom","source":"picture.gif"}}`),
		Hero:       &ryoHero{Name: "picture.gif", B64: base64.StdEncoding.EncodeToString(profilePNGWithMetadata(t))},
	})
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "profile.ryoprofile")
	if err := os.WriteFile(src, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := importProfile(src); err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(profileConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	var model profileModel
	if err := json.Unmarshal(settings, &model); err != nil || model.Hero.Source != "hero.png" {
		t.Fatalf("custom source not canonicalized: %s, %v", settings, err)
	}
	picture, err := os.ReadFile(filepath.Join(profileHeroDir(), "hero.png"))
	if err != nil {
		t.Fatal(err)
	}
	assertProfilePNGOnlyPixels(t, picture)
}

func profilePNGWithMetadata(t *testing.T) []byte {
	t.Helper()
	out := append([]byte(nil), sampleHero[:len(sampleHero)-12]...)
	for _, kind := range []string{"tEXt", "iTXt", "zTXt", "eXIf"} {
		payload := []byte("private-profile-secret")
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(payload)))
		out = append(out, n[:]...)
		chunk := append([]byte(kind), payload...)
		out = append(out, chunk...)
		binary.BigEndian.PutUint32(n[:], crc32.ChecksumIEEE(chunk))
		out = append(out, n[:]...)
	}
	out = append(out, sampleHero[len(sampleHero)-12:]...)
	return append(out, []byte("private-trailing-secret")...)
}

func assertProfilePNGOnlyPixels(t *testing.T, raw []byte) {
	t.Helper()
	if len(raw) < 8 || string(raw[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatal("not a PNG")
	}
	for pos := 8; ; {
		if pos+12 > len(raw) {
			t.Fatal("truncated PNG output")
		}
		n := int(binary.BigEndian.Uint32(raw[pos : pos+4]))
		if n > len(raw)-pos-12 {
			t.Fatal("invalid output chunk size")
		}
		kind := string(raw[pos+4 : pos+8])
		switch kind {
		case "IHDR", "IDAT", "IEND":
		default:
			t.Fatalf("non-pixel PNG chunk exported: %s", kind)
		}
		pos += n + 12
		if kind == "IEND" {
			if pos != len(raw) {
				t.Fatal("trailing payload exported")
			}
			break
		}
	}
	want, err := png.Decode(bytes.NewReader(sampleHero))
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatal("export changed pixel bounds")
	}
	for y := want.Bounds().Min.Y; y < want.Bounds().Max.Y; y++ {
		for x := want.Bounds().Min.X; x < want.Bounds().Max.X; x++ {
			r, g, b, a := want.At(x, y).RGBA()
			gr, gg, gb, ga := got.At(x, y).RGBA()
			if r != gr || g != gg || b != gb || a != ga {
				t.Fatalf("pixel changed at %d,%d", x, y)
			}
		}
	}
}

func TestCardPictureStripsAllMetadataAndTrailingPayload(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "rendered.png"), filepath.Join(dir, "shared.png")
	if err := os.WriteFile(src, profilePNGWithMetadata(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCard(src, dst); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	assertProfilePNGOnlyPixels(t, raw)
}

func TestCardPictureRejectsInvalidAndOversizedInputs(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "input"), filepath.Join(dir, "shared.png")
	if err := os.WriteFile(src, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCard(src, dst); err == nil {
		t.Fatal("non-image shared")
	}
	if err := os.Truncate(src, maxPictureBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := writeCard(src, dst); err == nil {
		t.Fatal("oversized image shared")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("invalid image created output: %v", err)
	}
}

func TestProfileImportRejectsEmbeddedImageResourceBombs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	wide := append([]byte(nil), sampleHero...)
	binary.BigEndian.PutUint32(wide[16:20], maxPictureDimension+1)
	binary.BigEndian.PutUint32(wide[29:33], crc32.ChecksumIEEE(wide[12:29]))
	for name, b64 := range map[string]string{
		"dimensions": base64.StdEncoding.EncodeToString(wide),
		"bytes":      strings.Repeat("A", base64.StdEncoding.EncodedLen(maxPictureBytes)+4),
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(ryoEnvelope{
				Ryoprofile: 1, Profile: json.RawMessage(sampleProfile),
				Hero: &ryoHero{Name: "hero.png", B64: b64},
			})
			if err != nil {
				t.Fatal(err)
			}
			src := filepath.Join(t.TempDir(), "bomb.ryoprofile")
			if err := os.WriteFile(src, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := importProfile(src); err == nil {
				t.Fatal("embedded image bomb accepted")
			}
			if _, err := os.Stat(profileConfigPath()); !os.IsNotExist(err) {
				t.Fatalf("image bomb changed settings: %v", err)
			}
		})
	}
}

func TestCardClipboardStripsMetadataWithoutLiveClipboard(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	captured := filepath.Join(dir, "captured.png")
	// This private wl-copy replacement never reaches the user's clipboard.
	script := "#!/bin/sh\n[ \"$1\" = --type ] && [ \"$2\" = image/png ] || exit 1\nexec /usr/bin/cat > \"$PROFILE_CAPTURE\"\n"
	if err := os.WriteFile(filepath.Join(bin, "wl-copy"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PROFILE_CAPTURE", captured)
	rendered := filepath.Join(dir, "rendered.png")
	if err := os.WriteFile(rendered, profilePNGWithMetadata(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCard(rendered, "--copy"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(captured)
	if err != nil {
		t.Fatal(err)
	}
	assertProfilePNGOnlyPixels(t, raw)
}

func TestProfileExportMissingSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dst := filepath.Join(t.TempDir(), "empty.ryoprofile")
	if err := exportProfile(dst); err != nil {
		t.Fatal(err)
	}
	if err := importProfile(dst); err != nil {
		t.Fatal(err)
	}
}

func TestProfileValidationAcceptsEditorRanges(t *testing.T) {
	for _, profile := range []string{
		`{}`,
		`{"preset":"minimal","heroSide":"left","homeView":"card","blocks":{"telemetry":false,"specs":true}}`,
		`{"hero":{"kind":"gallery","source":"rashin-hero.png","style":"dither","zoom":4,"focalX":0,"focalY":1,"brightness":-0.6,"contrast":0.6,"saturation":-1,"dither":6,"invert":true,"fit":false}}`,
		`{"hero":{"kind":"gallery","source":"animation.gif"},"text":{"name":"Nero","tagline":"力 美","epithets":["力","Beauty"],"verse":[],"watermarkGlyph":"","marginalia":""},"vitals":["core","gpu","mem","net","frac"],"specs":["resolution","compositor","uptime"]}`,
	} {
		if _, err := validateProfile(json.RawMessage(profile)); err != nil {
			t.Fatalf("editor profile rejected: %s: %v", profile, err)
		}
	}
}

func FuzzProfileValidation(f *testing.F) {
	for _, seed := range []string{`{}`, sampleProfile, `{"hero":{"kind":"gallery","source":"../../x.png"}}`, `{"text":{"name":"<img src=x>"}}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		model, err := validateProfile(raw)
		if err != nil {
			return
		}
		if !json.Valid(raw) {
			t.Fatal("accepted invalid JSON")
		}
		if model.Hero.Source != "" && (model.Hero.Source != filepath.Base(model.Hero.Source) || strings.ContainsAny(model.Hero.Source, ":?#%\\")) {
			t.Fatal("accepted a nonlocal hero source")
		}
	})
}

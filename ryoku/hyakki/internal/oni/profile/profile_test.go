package profile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/oni/spec"
)

var testdata = os.DirFS("testdata")

func TestShippedOniIsValid(t *testing.T) {
	d := Default()
	if !d.Valid || len(d.Report.Errors) > 0 || len(d.Report.Fallbacks) > 0 {
		t.Fatalf("shipped oni: valid=%v report=%+v", d.Valid, d.Report)
	}
	p := d.Profile
	if p.ID != ShippedID || !p.Shipped || p.Artwork.Kind != ArtworkParametric || p.Artwork.Fallback == "" {
		t.Fatalf("shipped oni profile: %+v", p)
	}
	if p.Persona == "" || len(p.Lines) == 0 {
		t.Fatalf("shipped oni has no persona or lines: %+v", p)
	}
	for _, id := range spec.StateIDs {
		if p.Animations.States[id] != PoseFromSpec {
			t.Errorf("state %s is %q", id, p.Animations.States[id])
		}
	}
	for _, id := range spec.GestureIDs {
		if p.Animations.Gestures[id] != PoseFromSpec {
			t.Errorf("gesture %s is %q", id, p.Animations.Gestures[id])
		}
	}
}

func TestResetReturnsUntouchedOni(t *testing.T) {
	want := Default()
	got := Default()
	got.Profile.Name = "Changed"
	got.Profile.Lines[0] = "changed"
	got.Profile.Animations.States[spec.StateIDs[0]] = "x.svg"
	got.Report.Fallbacks = append(got.Report.Fallbacks, Fallback{Slot: "x"})
	if again := Default(); !reflect.DeepEqual(again, want) {
		t.Fatalf("a caller's changes reached the next reset: %+v", again)
	}
}

func TestSecondCharacterLoads(t *testing.T) {
	l, r := Load(testdata, "testmoth")
	if !l.Valid || len(r.Errors) > 0 || len(r.Fallbacks) > 0 {
		t.Fatalf("testmoth: valid=%v report=%+v", l.Valid, r)
	}
	if !reflect.DeepEqual(r, l.Report) {
		t.Fatalf("returned report %+v differs from the loaded one %+v", r, l.Report)
	}
	p := l.Profile
	if p.ID != "testmoth" || p.Name != "Moth (test)" || p.Shipped || p.Artwork.Kind != ArtworkSVG {
		t.Fatalf("testmoth profile: %+v", p)
	}
	if !reflect.DeepEqual(p.Lines, []string{"Noted.", "Working on it."}) || p.Persona == "" {
		t.Fatalf("testmoth personality: lines=%q persona=%q", p.Lines, p.Persona)
	}
}

func TestBrokenProfilesAreRejected(t *testing.T) {
	cases := map[string]string{
		"broken-json":             ReasonBadJSON,
		"broken-version":          ReasonBadVersion,
		"broken-tools":            ReasonAuthorityKey,
		"broken-capabilities":     ReasonAuthorityKey,
		"broken-nested-grant":     ReasonAuthorityKey,
		"broken-unknown-key":      ReasonUnknownKey,
		"broken-id-mismatch":      ReasonIDMismatch,
		"broken-dotdot":           ReasonEscapingPath,
		"broken-absolute":         ReasonAbsolutePath,
		"broken-script-svg":       ReasonActiveSVG,
		"broken-external-svg":     ReasonActiveSVG,
		"broken-handler-svg":      ReasonActiveSVG,
		"broken-clips-unlicensed": ReasonClipUnlicensed,
		"broken-spec-pose":        ReasonBadValue,
		"oni":                     ReasonShadowsShipped,
		"nobody":                  ReasonMissingProfile,
		"../testmoth":             ReasonBadID,
		"Testmoth":                ReasonBadID,
		"":                        ReasonBadID,
	}
	for id, reason := range cases {
		l, r := Load(testdata, id)
		if l.Valid || !r.Has(reason) {
			t.Errorf("%q: want rejection %s, got valid=%v report=%+v", id, reason, l.Valid, r)
			continue
		}
		if l.Profile.ID != id || l.Profile.Name != "" || len(r.Fallbacks) > 0 {
			t.Errorf("%q: a rejected character keeps only its id and errors: %+v %+v", id, l.Profile, r)
		}
	}
}

// character writes testmoth into dir/id, with edit applied to its profile and
// files written over it (nil removes one).
func character(t *testing.T, dir, id string, edit func(map[string]any), files map[string][]byte) {
	t.Helper()
	dst := filepath.Join(dir, id)
	err := fs.WalkDir(testdata, "testmoth", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(testdata, p)
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(dst, strings.TrimPrefix(p, "testmoth/")), data)
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dst, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	p["id"] = id
	if edit != nil {
		edit(p)
	}
	if data, err = json.Marshal(p); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(dst, "profile.json"), data); err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		target := filepath.Join(dst, name)
		if data == nil {
			if err := os.RemoveAll(target); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := writeFile(target, data); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, data, 0o644)
}

func sub(p map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		next, ok := p[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			p[k] = next
		}
		p = next
	}
	return p
}

func svgOfSize(n int) []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg"><!--` + strings.Repeat("a", n) + `--></svg>`)
}

func linesOf(lines ...string) []byte {
	data, _ := json.Marshal(Lines{Acknowledge: lines})
	return data
}

func TestGeneratedProfilesAreRejected(t *testing.T) {
	outside := t.TempDir()
	art := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="4" height="4"/></svg>`)
	for _, name := range []string{"art.svg", "poses/rest.svg", "poses/alert.svg"} {
		if err := writeFile(filepath.Join(outside, name), art); err != nil {
			t.Fatal(err)
		}
	}
	svg := func(body string) []byte {
		return []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">` + body + `</svg>`)
	}
	manyLines := make([]string, MaxLines+1)
	for i := range manyLines {
		manyLines[i] = fmt.Sprintf("Line %d.", i)
	}
	bigFiles := map[string][]byte{}
	spreadBig := func(p map[string]any) {
		for _, id := range spec.StateIDs {
			sub(p, "animations", "states")[id] = "big/state-" + id + ".svg"
		}
		for _, id := range spec.GestureIDs {
			sub(p, "animations", "gestures")[id] = "big/gesture-" + id + ".svg"
		}
	}
	for _, id := range spec.StateIDs {
		bigFiles["big/state-"+id+".svg"] = svgOfSize(MaxSVGBytes - 1024)
	}
	for _, id := range spec.GestureIDs {
		bigFiles["big/gesture-"+id+".svg"] = svgOfSize(MaxSVGBytes - 1024)
	}
	deep := map[string]any{}
	for range maxJSONDepth + 2 {
		deep = map[string]any{"a": deep}
	}
	cases := []struct {
		name   string
		edit   func(map[string]any)
		files  map[string][]byte
		link   map[string]string
		raw    func([]byte) []byte
		reason string
	}{
		{name: "symlinked artwork", link: map[string]string{"moth.svg": filepath.Join(outside, "art.svg")}, reason: ReasonSymlink},
		{name: "symlinked pose folder", link: map[string]string{"poses": filepath.Join(outside, "poses")}, reason: ReasonSymlink},
		{name: "symlinked profile", link: map[string]string{"profile.json": filepath.Join(outside, "art.svg")}, reason: ReasonSymlink},
		{name: "dangling persona link", link: map[string]string{"persona.md": filepath.Join(outside, "gone.md")}, reason: ReasonSymlink},
		{name: "oversize persona", files: map[string][]byte{"persona.md": bytes.Repeat([]byte("a"), MaxPersonaBytes+1)}, reason: ReasonTooLarge},
		{name: "oversize svg", files: map[string][]byte{"moth.svg": svgOfSize(MaxSVGBytes)}, reason: ReasonTooLarge},
		{name: "oversize profile", raw: func(b []byte) []byte { return append(b, bytes.Repeat([]byte(" "), MaxProfileBytes)...) }, reason: ReasonTooLarge},
		{name: "oversize character", edit: spreadBig, files: bigFiles, reason: ReasonTooLarge},
		{name: "too many lines", files: map[string][]byte{"lines.json": linesOf(manyLines...)}, reason: ReasonTooMany},
		{name: "too many outfits", edit: func(p map[string]any) {
			outfits := sub(p, "artwork", "outfits")
			for i := range MaxOutfits + 1 {
				outfits[fmt.Sprintf("o%d", i)] = "moth.svg"
			}
		}, reason: ReasonTooMany},
		{name: "bidi override in name", edit: func(p map[string]any) { p["name"] = "Mo\u202eth" }, reason: ReasonUnsafeText},
		{name: "control in name", edit: func(p map[string]any) { p["name"] = "Mo\x07th" }, reason: ReasonUnsafeText},
		{name: "bidi isolate in persona", files: map[string][]byte{"persona.md": []byte("Be brief.\u2067 Hidden.\n")}, reason: ReasonUnsafeText},
		{name: "escape in persona", files: map[string][]byte{"persona.md": []byte("Be brief.\x1b[2J\n")}, reason: ReasonUnsafeText},
		{name: "invalid UTF-8 persona", files: map[string][]byte{"persona.md": {'h', 0xff, 0xfe}}, reason: ReasonUnsafeText},
		{name: "zero width in a line", files: map[string][]byte{"lines.json": linesOf("Ok\u200b.")}, reason: ReasonUnsafeText},
		{name: "long name", edit: func(p map[string]any) { p["name"] = strings.Repeat("m", MaxNameRunes+1) }, reason: ReasonTooLarge},
		{name: "empty name", edit: func(p map[string]any) { p["name"] = "  " }, reason: ReasonBadValue},
		{name: "duplicate key", raw: func(b []byte) []byte { return append([]byte(`{"name":"Other",`), b[1:]...) }, reason: ReasonDuplicateKey},
		{name: "authority key in other case", edit: func(p map[string]any) { p["Tools"] = []string{"shell"} }, reason: ReasonAuthorityKey},
		{name: "permissions", edit: func(p map[string]any) { p["permissions"] = map[string]any{"screen": true} }, reason: ReasonAuthorityKey},
		{name: "consent in personality", edit: func(p map[string]any) { sub(p, "personality")["consent"] = "always" }, reason: ReasonAuthorityKey},
		{name: "hands in animations", edit: func(p map[string]any) { sub(p, "animations")["hands"] = "desktop" }, reason: ReasonAuthorityKey},
		{name: "approvals", edit: func(p map[string]any) { p["approvals"] = "skip" }, reason: ReasonAuthorityKey},
		{name: "key in other case", edit: func(p map[string]any) { p["Name"] = "Other" }, reason: ReasonUnknownKey},
		{name: "nested too deep", edit: func(p map[string]any) { sub(p, "artwork")["outfits"] = deep }, reason: ReasonBadJSON},
		{name: "trailing data", raw: func(b []byte) []byte { return append(b, []byte(`{}`)...) }, reason: ReasonBadJSON},
		{name: "unknown state", edit: func(p map[string]any) { sub(p, "animations", "states")["dancing"] = "moth.svg" }, reason: ReasonUnknownSlot},
		{name: "unknown motif", edit: func(p map[string]any) { sub(p, "animations", "motifs")["fanfare"] = []int{60} }, reason: ReasonUnknownSlot},
		{name: "motif note out of range", edit: func(p map[string]any) { sub(p, "animations", "motifs")[spec.MotifIDs[0]] = []int{200} }, reason: ReasonBadValue},
		{name: "voice engine", edit: func(p map[string]any) { sub(p, "voice")["engine"] = "espeak" }, reason: ReasonBadValue},
		{name: "voice speed", edit: func(p map[string]any) { sub(p, "voice")["speed"] = 9 }, reason: ReasonBadValue},
		{name: "honorific", edit: func(p map[string]any) { sub(p, "voice")["honorific"] = "lord" }, reason: ReasonBadValue},
		{name: "artwork kind", edit: func(p map[string]any) { sub(p, "artwork")["kind"] = "gif" }, reason: ReasonBadValue},
		{name: "anchor off the artwork", edit: func(p map[string]any) { sub(p, "artwork")["anchor"] = map[string]any{"x": 0.5, "y": 1.2} }, reason: ReasonBadValue},
		{name: "anchor below zero", edit: func(p map[string]any) { sub(p, "artwork")["anchor"] = map[string]any{"x": -0.01, "y": 0.5} }, reason: ReasonBadValue},
		{name: "anchor without y", edit: func(p map[string]any) { sub(p, "artwork")["anchor"] = map[string]any{"x": 0.5} }, reason: ReasonBadValue},
		{name: "anchor as text", edit: func(p map[string]any) { sub(p, "artwork")["anchor"] = map[string]any{"x": "middle", "y": 0.5} }, reason: ReasonBadValue},
		{name: "anchor as a list", edit: func(p map[string]any) { sub(p, "artwork")["anchor"] = []float64{0.5, 0.5} }, reason: ReasonBadValue},
		{name: "anchor with a third key", edit: func(p map[string]any) { sub(p, "artwork")["anchor"] = map[string]any{"x": 0.5, "y": 0.5, "z": 1} }, reason: ReasonUnknownKey},
		{name: "grant inside the anchor", edit: func(p map[string]any) {
			sub(p, "artwork")["anchor"] = map[string]any{"x": 0.5, "y": 0.5, "permissions": true}
		}, reason: ReasonAuthorityKey},
		{name: "wrong type", edit: func(p map[string]any) { p["name"] = 7 }, reason: ReasonBadValue},
		{name: "svg doctype", files: map[string][]byte{"moth.svg": []byte(`<!DOCTYPE svg [<!ENTITY x "y">]><svg xmlns="http://www.w3.org/2000/svg"/>`)}, reason: ReasonActiveSVG},
		{name: "svg foreign object", files: map[string][]byte{"moth.svg": svg(`<foreignObject><div/></foreignObject>`)}, reason: ReasonActiveSVG},
		{name: "svg style element", files: map[string][]byte{"moth.svg": svg(`<style>@import url(https://example.org/a.css);</style>`)}, reason: ReasonActiveSVG},
		{name: "svg external url", files: map[string][]byte{"moth.svg": svg(`<rect style="fill:url( 'https://example.org/p.svg#a')"/>`)}, reason: ReasonActiveSVG},
		{name: "svg xlink file", files: map[string][]byte{"moth.svg": svg(`<use xlink:href="file:///etc/passwd"/>`)}, reason: ReasonActiveSVG},
		{name: "svg image", files: map[string][]byte{"moth.svg": svg(`<image href="#a"/>`)}, reason: ReasonActiveSVG},
		{name: "not svg", files: map[string][]byte{"moth.svg": []byte(`<html/>`)}, reason: ReasonBadSVG},
		{name: "not an svg path", edit: func(p map[string]any) { sub(p, "animations", "states")[spec.StateIDs[0]] = "poses/rest.png" }, reason: ReasonBadPath},
		{name: "backslash path", edit: func(p map[string]any) { sub(p, "animations", "states")[spec.StateIDs[0]] = `poses\rest.svg` }, reason: ReasonBadPath},
		{name: "home path", edit: func(p map[string]any) { sub(p, "artwork")["image"] = "~/moth.svg" }, reason: ReasonAbsolutePath},
		{name: "folder as asset", edit: func(p map[string]any) { sub(p, "personality")["promptPersona"] = "poses" }, reason: ReasonBadPath},
		{name: "clip pack without provenance", edit: clips, files: map[string][]byte{"voice/LICENSE": []byte("CC0-1.0\n")}, reason: ReasonClipNoSource},
		{name: "clip pack without license", edit: clips, files: map[string][]byte{"voice/PROVENANCE.md": []byte("Recorded for this test.\n")}, reason: ReasonClipUnlicensed},
		{name: "clip pack license link", edit: clips, files: map[string][]byte{"voice/PROVENANCE.md": []byte("Recorded for this test.\n")}, link: map[string]string{"voice/LICENSE": filepath.Join(outside, "art.svg")}, reason: ReasonSymlink},
		{name: "clip pack missing", edit: clips, reason: ReasonClipUnlicensed},
		{name: "clip pack escaping", edit: func(p map[string]any) { clips(p); sub(p, "voice")["clipPack"] = "../voice" }, reason: ReasonEscapingPath},
		{name: "clip pack without clips engine", edit: func(p map[string]any) { sub(p, "voice")["clipPack"] = "voice" }, reason: ReasonBadValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			character(t, dir, "moth", c.edit, c.files)
			if c.raw != nil {
				name := filepath.Join(dir, "moth", "profile.json")
				data, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, c.raw(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for name, target := range c.link {
				p := filepath.Join(dir, "moth", name)
				if err := os.RemoveAll(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, p); err != nil {
					t.Fatal(err)
				}
			}
			l, r := Load(os.DirFS(dir), "moth")
			if l.Valid || !r.Has(c.reason) {
				t.Fatalf("want rejection %s, got valid=%v report=%+v", c.reason, l.Valid, r)
			}
			if len(r.Fallbacks) > 0 {
				t.Fatalf("a rejected character reports fallbacks: %+v", r.Fallbacks)
			}
		})
	}
}

func clips(p map[string]any) {
	v := sub(p, "voice")
	v["engine"] = VoiceClips
	v["clipPack"] = "voice/"
}

func TestLicensedClipPackLoads(t *testing.T) {
	dir := t.TempDir()
	character(t, dir, "moth", clips, map[string][]byte{
		"voice/LICENSE":       []byte("CC0-1.0\n"),
		"voice/PROVENANCE.md": []byte("Recorded for this test.\n"),
	})
	l, r := Load(os.DirFS(dir), "moth")
	if !l.Valid || len(r.Fallbacks) > 0 || l.Profile.Voice.Engine != VoiceClips {
		t.Fatalf("licensed clip pack: valid=%v report=%+v voice=%+v", l.Valid, r, l.Profile.Voice)
	}
}

func TestMotifOverrideIsKept(t *testing.T) {
	dir := t.TempDir()
	motif := spec.MotifIDs[0]
	character(t, dir, "moth", func(p map[string]any) {
		sub(p, "animations", "motifs")[motif] = []int{62, 69}
	}, nil)
	l, r := Load(os.DirFS(dir), "moth")
	if !l.Valid || len(r.Fallbacks) > 0 || !reflect.DeepEqual(l.Profile.Animations.Motifs[motif], []int{62, 69}) {
		t.Fatalf("motif override: valid=%v report=%+v motifs=%v", l.Valid, r, l.Profile.Animations.Motifs)
	}
}

// An anchor moves where the strand leaves the drawing and nothing else: the
// character loads exactly as it would without one.
func TestArtworkAnchorChangesOnlyWhereTheStrandLeaves(t *testing.T) {
	dir := t.TempDir()
	character(t, dir, "moth", nil, nil)
	character(t, dir, "anchored", func(p map[string]any) {
		sub(p, "artwork")["anchor"] = map[string]any{"x": 0.25, "y": 1}
	}, nil)
	plain, _ := Load(os.DirFS(dir), "moth")
	anchored, r := Load(os.DirFS(dir), "anchored")
	if !anchored.Valid || len(r.Fallbacks) > 0 {
		t.Fatalf("anchored: valid=%v report=%+v", anchored.Valid, r)
	}
	if a := anchored.Profile.Artwork.Anchor; a == nil || *a != (Anchor{X: 0.25, Y: 1}) {
		t.Fatalf("anchor = %+v", a)
	}
	if plain.Profile.Artwork.Anchor != nil {
		t.Fatalf("a character without an anchor has %+v", plain.Profile.Artwork.Anchor)
	}
	got := anchored.Profile
	got.ID, got.Artwork.Anchor = plain.Profile.ID, nil
	if !reflect.DeepEqual(got, plain.Profile) || !reflect.DeepEqual(anchored.Report, plain.Report) {
		t.Fatalf("an anchor changed more than the anchor:\n%+v\n%+v", got, plain.Profile)
	}
	if !bytes.Contains(anchored.Files[ProfileFile], []byte(`"anchor"`)) {
		t.Fatal("the renderer's copy of profile.json lost the anchor")
	}

	// an anchor on artwork that fell back belongs to no drawing
	character(t, dir, "lost", func(p map[string]any) {
		sub(p, "artwork")["anchor"] = map[string]any{"x": 0.5, "y": 0.5}
	}, map[string][]byte{"moth.svg": nil})
	lost, _ := Load(os.DirFS(dir), "lost")
	if !lost.Valid || lost.Profile.Artwork.Kind != ArtworkSilhouette || lost.Profile.Artwork.Anchor != nil {
		t.Fatalf("a silhouette kept the anchor: %+v", lost.Profile.Artwork)
	}
}

func TestSoftFallbacks(t *testing.T) {
	var bare []Fallback
	bare = append(bare, Fallback{"artwork", FallbackMissing, UsedSilhouette})
	for _, id := range spec.StateIDs {
		bare = append(bare, Fallback{"animations.states." + id, FallbackMissing, UsedStaticPose})
	}
	for _, id := range spec.GestureIDs {
		bare = append(bare, Fallback{"animations.gestures." + id, FallbackMissing, UsedBasePose})
	}
	bare = append(bare,
		Fallback{"personality.voiceLines", FallbackMissing, UsedNeutralAcks},
		Fallback{"personality.promptPersona", FallbackMissing, UsedNoPersona})
	missingFiles := []Fallback{
		{"artwork", FallbackAssetMissing, UsedSilhouette},
		{"personality.voiceLines", FallbackAssetMissing, UsedNeutralAcks},
		{"personality.promptPersona", FallbackAssetMissing, UsedNoPersona},
	}
	cases := map[string][]Fallback{
		"soft-no-artwork":    {{"artwork", FallbackMissing, UsedSilhouette}},
		"soft-missing-files": missingFiles,
		"soft-bare":          bare,
	}
	acks := Default().Profile.Lines
	for id, want := range cases {
		l, r := Load(testdata, id)
		if !l.Valid || len(r.Errors) > 0 {
			t.Errorf("%s: valid=%v errors=%v", id, l.Valid, r.Errors)
			continue
		}
		if !reflect.DeepEqual(r.Fallbacks, want) {
			t.Errorf("%s fallbacks:\n got %+v\nwant %+v", id, r.Fallbacks, want)
		}
		if l.Profile.Artwork.Kind != ArtworkSilhouette || l.Profile.Artwork.Image != "" {
			t.Errorf("%s: artwork %+v, want the default silhouette", id, l.Profile.Artwork)
		}
		if id != "soft-no-artwork" && !reflect.DeepEqual(l.Profile.Lines, acks) {
			t.Errorf("%s: lines %q, want the neutral acknowledgements %q", id, l.Profile.Lines, acks)
		}
	}
	l, _ := Load(testdata, "soft-bare")
	v := l.Profile.Voice
	if v.Engine != VoiceSystem || v.Speed != 1 || v.Honorific != HonorificNone || l.Profile.Persona != "" {
		t.Fatalf("a bare character's voice and persona: %+v %q", v, l.Profile.Persona)
	}
}

func TestEachSlotFallsBackAlone(t *testing.T) {
	type drop struct {
		name  string
		edit  func(map[string]any)
		files map[string][]byte
		want  Fallback
	}
	var drops []drop
	for _, id := range spec.StateIDs {
		slot := "animations.states." + id
		drops = append(drops,
			drop{"state " + id, func(p map[string]any) { delete(sub(p, "animations", "states"), id) }, nil, Fallback{slot, FallbackMissing, UsedStaticPose}},
			drop{"state file " + id, func(p map[string]any) { sub(p, "animations", "states")[id] = "poses/gone.svg" }, nil, Fallback{slot, FallbackAssetMissing, UsedStaticPose}})
	}
	for _, id := range spec.GestureIDs {
		slot := "animations.gestures." + id
		drops = append(drops,
			drop{"gesture " + id, func(p map[string]any) { delete(sub(p, "animations", "gestures"), id) }, nil, Fallback{slot, FallbackMissing, UsedBasePose}},
			drop{"gesture file " + id, func(p map[string]any) { sub(p, "animations", "gestures")[id] = "poses/gone.svg" }, nil, Fallback{slot, FallbackAssetMissing, UsedBasePose}})
	}
	drops = append(drops,
		drop{"artwork", func(p map[string]any) { delete(p, "artwork") }, nil, Fallback{"artwork", FallbackMissing, UsedSilhouette}},
		drop{"artwork file", nil, map[string][]byte{"moth.svg": nil}, Fallback{"artwork", FallbackAssetMissing, UsedSilhouette}},
		drop{"parametric fallback", func(p map[string]any) {
			p["artwork"] = map[string]any{"kind": ArtworkParametric, "spec": SpecFile}
		}, nil, Fallback{"artwork.fallback", FallbackMissing, UsedSilhouette}},
		drop{"outfit file", func(p map[string]any) {
			sub(p, "artwork", "outfits")["winter"] = "winter.svg"
		}, nil, Fallback{"artwork.outfits.winter", FallbackAssetMissing, UsedBaseArtwork}},
		drop{"lines", func(p map[string]any) { delete(sub(p, "personality"), "voiceLines") }, nil, Fallback{"personality.voiceLines", FallbackMissing, UsedNeutralAcks}},
		drop{"lines file", nil, map[string][]byte{"lines.json": nil}, Fallback{"personality.voiceLines", FallbackAssetMissing, UsedNeutralAcks}},
		drop{"lines empty", nil, map[string][]byte{"lines.json": linesOf()}, Fallback{"personality.voiceLines", FallbackEmpty, UsedNeutralAcks}},
		drop{"persona", func(p map[string]any) { delete(sub(p, "personality"), "promptPersona") }, nil, Fallback{"personality.promptPersona", FallbackMissing, UsedNoPersona}},
		drop{"persona file", nil, map[string][]byte{"persona.md": nil}, Fallback{"personality.promptPersona", FallbackAssetMissing, UsedNoPersona}},
		drop{"persona blank", nil, map[string][]byte{"persona.md": []byte(" \n\t\n")}, Fallback{"personality.promptPersona", FallbackEmpty, UsedNoPersona}},
	)
	for _, d := range drops {
		t.Run(d.name, func(t *testing.T) {
			dir := t.TempDir()
			character(t, dir, "moth", d.edit, d.files)
			l, r := Load(os.DirFS(dir), "moth")
			if !l.Valid || len(r.Errors) > 0 {
				t.Fatalf("valid=%v errors=%v", l.Valid, r.Errors)
			}
			if !reflect.DeepEqual(r.Fallbacks, []Fallback{d.want}) {
				t.Fatalf("fallbacks %+v, want only %+v", r.Fallbacks, d.want)
			}
			if l.Profile.Name != "Moth (test)" {
				t.Fatalf("dropping %s changed the name to %q", d.name, l.Profile.Name)
			}
		})
	}
}

func TestListNeverShadowsShippedOni(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	list := List("testdata")
	if len(list) != len(entries)+1 {
		t.Fatalf("listed %d characters for %d folders", len(list), len(entries))
	}
	if !reflect.DeepEqual(list[0], Default()) {
		t.Fatalf("the list does not start with the shipped oni: %+v", list[0])
	}
	shipped, userOni, moth := 0, false, false
	for _, l := range list {
		switch {
		case l.Profile.ID == ShippedID && l.Valid:
			shipped++
			if !l.Profile.Shipped {
				t.Fatalf("a valid oni that is not the shipped one: %+v", l)
			}
		case l.Profile.ID == ShippedID:
			userOni = l.Report.Has(ReasonShadowsShipped)
		case l.Profile.ID == "testmoth":
			moth = l.Valid
		}
	}
	if shipped != 1 || !userOni || !moth {
		t.Fatalf("shipped oni %d times, user oni rejected %v, testmoth valid %v", shipped, userOni, moth)
	}
	for _, dir := range []string{"", filepath.Join(t.TempDir(), "none")} {
		if got := List(dir); len(got) != 1 || !got[0].Valid || got[0].Profile.ID != ShippedID {
			t.Fatalf("List(%q) = %+v, want only the shipped oni", dir, got)
		}
	}
}

func TestListRejectsLinkedCharacterFolders(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	character(t, dir, "moth", nil, nil)
	character(t, elsewhere, "linked", nil, nil)
	if err := os.Symlink(filepath.Join(elsewhere, "linked"), filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a character"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := List(dir)
	if len(list) != 3 || list[1].Profile.ID != "linked" || list[1].Valid || !list[1].Report.Has(ReasonSymlink) || list[2].Profile.ID != "moth" || !list[2].Valid {
		t.Fatalf("list: %+v", list)
	}
}

func TestListCapsCharacterCount(t *testing.T) {
	dir := t.TempDir()
	for i := range MaxCharacters + 1 {
		id := fmt.Sprintf("c%03d", i)
		data := fmt.Appendf(nil, `{"version":1,"id":%q,"name":"C"}`, id)
		if err := writeFile(filepath.Join(dir, id, "profile.json"), data); err != nil {
			t.Fatal(err)
		}
	}
	list := List(dir)
	if len(list) != MaxCharacters+2 {
		t.Fatalf("listed %d", len(list))
	}
	for _, l := range list[:MaxCharacters+1] {
		if !l.Valid {
			t.Fatalf("%s rejected: %v", l.Profile.ID, l.Report.Errors)
		}
	}
	if last := list[len(list)-1]; last.Valid || !last.Report.Has(ReasonTooMany) {
		t.Fatalf("the character over the cap: %+v", last)
	}
}

func TestCharactersCarryNoForeignMarks(t *testing.T) {
	marks := []string{"n\u016bs", "kn" + "ot", "ni" + "xi", "co" + "ucou"}
	shipped := os.DirFS("../characters")
	licensed := false
	err := fs.WalkDir(shipped, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(shipped, p)
		if err != nil {
			return err
		}
		text := strings.ToLower(string(data))
		for _, m := range marks {
			if strings.Contains(text, m) {
				t.Errorf("%s carries a foreign mark", p)
			}
		}
		if path.Base(p) == "LICENSE.artwork" {
			licensed = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !licensed {
		t.Fatal("no LICENSE.artwork under characters/")
	}
}

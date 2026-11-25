package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestOniIsOffOnAMachineThatNeverChangedASetting(t *testing.T) {
	isolateConfig(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	o := loaded.Oni
	if o.Master || o.Inspect || o.Voice || o.Reflex || o.Feeds.Guard || len(o.Feeds.Words) != 0 || o.PerchFollowsFocus {
		t.Fatalf("oni = %#v", o)
	}
	if o.Disclosure != protocol.OniDisclosureSession {
		t.Fatalf("disclosure = %q", o.Disclosure)
	}
	if o.Hands != protocol.OniHandsOff || o.Character != protocol.OniDefaultCharacter {
		t.Fatalf("oni = %#v", o)
	}
	if protocol.EffectiveOni(loaded).Master {
		t.Fatal("Oni is effectively on by default")
	}
}

func TestPerchFollowsFocusIsOffUntilTurnedOnAndSurvivesASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"perchFollowsFocus":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !next.Oni.PerchFollowsFocus || next.Oni.Inspect || next.Oni.Master {
		t.Fatalf("oni = %#v", next.Oni)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Oni.PerchFollowsFocus {
		t.Fatal("perchFollowsFocus was not kept")
	}
	if _, err := Merge(loaded, json.RawMessage(`{"oni":{"perchFollowsFocus":"yes"}}`)); err == nil {
		t.Fatal("a perchFollowsFocus that is no boolean was accepted")
	}
}

func TestHyakkiOffKeepsOniOffWhateverItsSwitchSays(t *testing.T) {
	s := Defaults()
	s.Oni.Master = true
	if protocol.EffectiveOni(s).Master {
		t.Fatal("Oni counts as on while Hyakki is off")
	}
	s.Enabled = true
	if !protocol.EffectiveOni(s).Master {
		t.Fatal("Oni's switch does not count while Hyakki is on")
	}
	if !s.Oni.Master {
		t.Fatal("EffectiveOni changed the stored switch")
	}
}

// settings.set stores each of the four stages, exactly spelled; anything
// else is refused as no stage, and a refused patch changes nothing. Who may
// store one is the gate's: only the app.
func TestSettingsSetTakesTheFourHandsStages(t *testing.T) {
	cur := Defaults()
	for _, stage := range []string{"", "everything", "OFF", "Browser", " browser", "App", "live", "whole-desktop"} {
		patch := json.RawMessage(`{"oni":{"master":true,"inspect":true,"hands":"` + stage + `"}}`)
		next, err := Merge(cur, patch)
		var perr *protocol.Error
		if !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
			t.Fatalf("hands %q: %v, %#v", stage, err, next.Oni)
		}
		if next.Oni.Master || next.Oni.Hands != protocol.OniHandsOff {
			t.Fatalf("a refused patch changed oni to %#v", next.Oni)
		}
	}
	for _, stage := range []string{protocol.OniHandsBrowser, protocol.OniHandsApp, protocol.OniHandsDesktop} {
		next, err := Merge(cur, json.RawMessage(`{"oni":{"master":true,"inspect":true,"hands":"`+stage+`"}}`))
		if err != nil || next.Oni.Hands != stage || !next.Oni.Master || !next.Oni.Inspect {
			t.Fatalf("hands %s = %#v, %v", stage, next.Oni, err)
		}
		next, err = Merge(next, json.RawMessage(`{"oni":{"hands":"off"}}`))
		if err != nil || next.Oni.Hands != protocol.OniHandsOff {
			t.Fatalf("hands off = %#v, %v", next.Oni, err)
		}
	}
}

// A stored stage loads as it was; anything else loads as off.
func TestAStoredHandsStageLoadsAsItWasOrOff(t *testing.T) {
	isolateConfig(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	for stage, want := range map[string]string{
		protocol.OniHandsBrowser: protocol.OniHandsBrowser,
		protocol.OniHandsOff:     protocol.OniHandsOff,
		protocol.OniHandsApp:     protocol.OniHandsApp,
		protocol.OniHandsDesktop: protocol.OniHandsDesktop,
		"BROWSER":                protocol.OniHandsOff,
		"whole-desktop":          protocol.OniHandsOff,
		"":                       protocol.OniHandsOff,
	} {
		file := `{"version":2,"enabled":true,"oni":{"master":true,"inspect":true,"hands":"` + stage + `"}}`
		if err := os.WriteFile(paths.ConfigFile(), []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Oni.Hands != want {
			t.Errorf("stored hands %q loaded as %q, want %q", stage, loaded.Oni.Hands, want)
		}
	}
}

func TestAStoredHandsStageLoadsAsOff(t *testing.T) {
	isolateConfig(t)
	file := `{"version":2,"enabled":true,"oni":{"master":true,"character":"../oni","inspect":true,"disclosure":"never","voice":true,"chord":"a","hands":"everywhere","reflex":true,"feeds":{"guard":true,"words":[" Reels ","reels","","Shorts\u202e","` + strings.Repeat("x", protocol.OniFeedWordMaxRunes+1) + `"]},"size":"huge","reduceMotion":"sometimes","courtesy":false,"hidden":true}}`
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigFile(), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := protocol.OniSettings{
		Master:        true,
		Character:     protocol.OniDefaultCharacter,
		Inspect:       true,
		Disclosure:    protocol.OniDisclosureSession,
		Voice:         true,
		Chord:         protocol.OniDefaultChord,
		Hands:         protocol.OniHandsOff,
		Reflex:        true,
		Feeds:         protocol.OniFeedSettings{Guard: true, Words: []string{"reels"}},
		Size:          protocol.OniSizeNormal,
		ReduceMotion:  protocol.OniReduceFollow,
		Courtesy:      false,
		Hidden:        true,
		VoiceSettings: protocol.DefaultOniSettings().VoiceSettings,
		Rewrite:       protocol.DefaultOniSettings().Rewrite,
		Assignment:    protocol.DefaultOniSettings().Assignment,
	}
	if !reflect.DeepEqual(loaded.Oni, want) {
		t.Fatalf("loaded oni = %#v\nwant %#v", loaded.Oni, want)
	}
}

func TestOniPresentationDefaultsAndSurvivesASave(t *testing.T) {
	isolateConfig(t)
	d := Defaults().Oni
	if d.Size != protocol.OniSizeNormal || d.ReduceMotion != protocol.OniReduceFollow || !d.Courtesy || d.Hidden {
		t.Fatalf("default presentation = %#v", d)
	}
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	// a file written before these settings existed keeps courtesy on
	if err := os.WriteFile(paths.ConfigFile(), []byte(`{"version":2,"oni":{"master":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Oni.Courtesy || loaded.Oni.Size != protocol.OniSizeNormal || loaded.Oni.ReduceMotion != protocol.OniReduceFollow {
		t.Fatalf("an older file loads oni as %#v", loaded.Oni)
	}
	next, err := Merge(loaded, json.RawMessage(`{"oni":{"size":"big","reduceMotion":"on","courtesy":false,"hidden":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	if loaded, err = Load(); err != nil {
		t.Fatal(err)
	}
	if o := loaded.Oni; o.Size != protocol.OniSizeBig || o.ReduceMotion != protocol.OniReduceOn || o.Courtesy || !o.Hidden || !o.Master {
		t.Fatalf("after a save and a load oni = %#v", o)
	}
}

func TestOniSettingsSurviveASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"master":true,"character":"testmoth","inspect":true,"disclosure":"always","chord":"Super + Alt + T","feeds":{"guard":true,"words":["reels"," Shorts "]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Oni.Chord != "alt+super+t" || !reflect.DeepEqual(next.Oni.Feeds.Words, []string{"reels", "shorts"}) {
		t.Fatalf("patched oni = %#v", next.Oni)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Oni, next.Oni) {
		t.Fatalf("after a save and a load oni = %#v\nwant %#v", loaded.Oni, next.Oni)
	}
	if loaded.Oni.Voice || loaded.Oni.Reflex {
		t.Fatalf("a patch that named neither turned on voice or reflex: %#v", loaded.Oni)
	}
}

func TestSettingsSetRefusesBadOniValues(t *testing.T) {
	words := make([]string, protocol.OniFeedWordsMax+1)
	for i := range words {
		words[i] = strings.Repeat("w", i+1)
	}
	tooMany, err := json.Marshal(map[string]any{"oni": map[string]any{"feeds": map[string]any{"words": words}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []string{
		`{"oni":{"character":"../evil"}}`,
		`{"oni":{"character":"Oni"}}`,
		`{"oni":{"character":""}}`,
		`{"oni":{"character":"` + strings.Repeat("a", protocol.OniCharacterIDMax+1) + `"}}`,
		`{"oni":{"disclosure":"never"}}`,
		`{"oni":{"chord":"a"}}`,
		`{"oni":{"chord":"shift+a"}}`,
		`{"oni":{"chord":"ctrl+ctrl"}}`,
		`{"oni":{"chord":"ctrl+a+b"}}`,
		`{"oni":{"chord":"ctrl+alt+shift+super+a"}}`,
		`{"oni":{"feeds":{"words":["reels\u202e"]}}}`,
		`{"oni":{"feeds":{"words":[""]}}}`,
		string(tooMany),
		`{"oni":{"master":"yes"}}`,
		`{"oni":{"size":"huge"}}`,
		`{"oni":{"size":""}}`,
		`{"oni":{"reduceMotion":"always"}}`,
		`{"oni":{"courtesy":"on"}}`,
	} {
		next, err := Merge(Defaults(), json.RawMessage(patch))
		if err == nil {
			t.Fatalf("patch %s accepted: %#v", patch, next.Oni)
		}
	}
}

func TestOniSoundsAreOffUntilTurnedOnAndSurviveASave(t *testing.T) {
	isolateConfig(t)
	if Defaults().Oni.Sounds {
		t.Fatal("Oni's sounds are on by default")
	}
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	// a file from before the setting, and one with a value of the wrong type
	for _, file := range []string{`{"version":2,"oni":{"master":true}}`, `{"version":2,"oni":{"master":true,"sounds":"loud"}}`} {
		if err := os.WriteFile(paths.ConfigFile(), []byte(file), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Oni.Sounds || !loaded.Oni.Master {
			t.Fatalf("%s loads oni as %#v", file, loaded.Oni)
		}
	}
	for _, patch := range []string{`{"oni":{"sounds":"on"}}`, `{"oni":{"sounds":1}}`} {
		if next, err := Merge(Defaults(), json.RawMessage(patch)); err == nil {
			t.Fatalf("patch %s accepted: %#v", patch, next.Oni)
		}
	}
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"sounds":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Oni.Sounds || !reflect.DeepEqual(loaded.Oni, next.Oni) {
		t.Fatalf("after a save and a load oni = %#v", loaded.Oni)
	}
	if next, err = Merge(loaded, json.RawMessage(`{"oni":{"sounds":null}}`)); err != nil || next.Oni.Sounds {
		t.Fatalf("clearing sounds left %#v, %v", next.Oni, err)
	}
}

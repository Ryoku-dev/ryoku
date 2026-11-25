package demo

import (
	"encoding/json"
	"errors"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestDemoOniAnswersAsTheDaemon(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	if s.data.snapshot.Settings.Features[FeatureOni] {
		t.Fatal("the Oni demo switch leaked into the feature flags")
	}
	status := call(t, s, protocol.MOniStatus, protocol.Empty{}).(protocol.OniStatus)
	if !status.Master || !status.Active || status.Character != protocol.OniDefaultCharacter || !status.Capabilities.Master {
		t.Fatalf("the Oni demo starts on with the shipped character, got %+v", status)
	}
	ids := map[string]protocol.OniCharacter{}
	for _, c := range status.Characters {
		ids[c.ID] = c
	}
	if status.Characters[0].ID != protocol.OniDefaultCharacter || !ids["testmoth"].Valid || !ids["paper"].Valid || len(ids["paper"].Fallbacks) == 0 || ids["lantern"].Valid || len(ids["lantern"].Errors) == 0 {
		t.Fatalf("characters = %+v", status.Characters)
	}

	status = call(t, s, protocol.MOniCharacterSelect, protocol.OniCharacterSelectParams{ID: "testmoth"}).(protocol.OniStatus)
	if status.Character != "testmoth" || s.data.snapshot.Settings.Oni.Character != "testmoth" {
		t.Fatalf("select left %q", status.Character)
	}
	if events := s.takeEvents(); len(events) != 1 || events[0].Event != protocol.EvtSettingsChanged {
		t.Fatalf("select told %+v, want settings.changed", events)
	}
	assets := call(t, s, protocol.MOniCharacterAssets, protocol.OniCharacterAssetsParams{ID: "testmoth"}).(protocol.OniCharacterAssets)
	for _, name := range []string{"profile.json", "moth.svg", "poses/rest.svg", "poses/alert.svg"} {
		if len(assets.Files[name]) == 0 {
			t.Fatalf("assets lack %s: %v", name, len(assets.Files))
		}
	}

	for id, code := range map[string]string{"lantern": protocol.CodeBadRequest, "nobody": protocol.CodeNotFound, "Bad Id": protocol.CodeBadRequest} {
		raw, _ := json.Marshal(protocol.OniCharacterSelectParams{ID: id})
		_, err := s.call(protocol.MOniCharacterSelect, raw)
		var wire *protocol.Error
		if !errors.As(err, &wire) || wire.Code != code {
			t.Fatalf("select %q: %v, want %s", id, err, code)
		}
	}
	if s.data.snapshot.Settings.Oni.Character != "testmoth" {
		t.Fatal("a refused select changed the character")
	}

	status = call(t, s, protocol.MOniCharacterReset, protocol.Empty{}).(protocol.OniStatus)
	if status.Character != protocol.OniDefaultCharacter {
		t.Fatalf("reset left %q", status.Character)
	}
	raw, _ := json.Marshal(protocol.OniPlaceParams{Corner: "middle"})
	if _, err := s.call(protocol.MOniPlace, raw); err == nil {
		t.Fatal("a corner that is not one was accepted")
	}
	call(t, s, protocol.MOniPlace, protocol.OniPlaceParams{Corner: "br"})
}

func TestDemoOniSettingsDefaultOff(t *testing.T) {
	settings := makeFixtures().snapshot.Settings.Oni
	if want := protocol.DefaultOniSettings(); settings.Master || settings.Sounds || settings.Size != want.Size || settings.ReduceMotion != want.ReduceMotion || !settings.Courtesy {
		t.Fatalf("demo Oni settings = %+v, want the defaults", settings)
	}
}

func TestDemoOniSoundsToggle(t *testing.T) {
	s := &server{data: makeFixtures(FeatureOni)}
	for _, on := range []bool{true, false} {
		raw, err := json.Marshal(map[string]any{"oni": map[string]any{"sounds": on}})
		if err != nil {
			t.Fatal(err)
		}
		settings := call(t, s, protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw}).(protocol.Settings)
		if settings.Oni.Sounds != on || s.data.snapshot.Settings.Oni.Sounds != on {
			t.Fatalf("sounds %v: settings.set answered %v, snapshot %v", on, settings.Oni.Sounds, s.data.snapshot.Settings.Oni.Sounds)
		}
	}
}

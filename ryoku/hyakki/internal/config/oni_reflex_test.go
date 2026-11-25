package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestOniReflexSettingsAreOffOnAMachineThatNeverChangedASetting(t *testing.T) {
	isolateConfig(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Oni.Reflex || loaded.Oni.ReflexSettings != (protocol.OniReflexSettings{}) || protocol.DefaultOniSettings().ReflexSettings != (protocol.OniReflexSettings{}) {
		t.Fatalf("reflex = %v, reflexSettings = %#v", loaded.Oni.Reflex, loaded.Oni.ReflexSettings)
	}
	b, err := json.Marshal(loaded.Oni.ReflexSettings)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"routines":false,"reminders":false,"notes":false,"decisions":false}` {
		t.Fatalf("the reflex block is %s", b)
	}
}

func TestOniReflexSettingsSurviveASave(t *testing.T) {
	isolateConfig(t)
	all := protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true, Decisions: true}
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"reflexSettings":{"routines":true,"reminders":true,"notes":true,"decisions":true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Oni.ReflexSettings != all {
		t.Fatalf("patched reflexSettings = %#v", next.Oni.ReflexSettings)
	}
	// a sub-switch never turns on what it builds on, nor the decision route
	base, err := Merge(Defaults(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Oni.Master || next.Oni.Reflex || next.Oni.Voice || !reflect.DeepEqual(next.Providers.Decisions, base.Providers.Decisions) {
		t.Fatalf("a reflex patch turned on a switch it never named: %#v %#v", next.Oni, next.Providers.Decisions)
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
	one, err := Merge(loaded, json.RawMessage(`{"oni":{"reflexSettings":{"decisions":false}}}`))
	if err != nil || one.Oni.ReflexSettings != (protocol.OniReflexSettings{Routines: true, Reminders: true, Notes: true}) {
		t.Fatalf("turning decisions off left %#v, %v", one.Oni.ReflexSettings, err)
	}
	cleared, err := Merge(loaded, json.RawMessage(`{"oni":{"reflexSettings":null}}`))
	if err != nil || cleared.Oni.ReflexSettings != (protocol.OniReflexSettings{}) {
		t.Fatalf("clearing the reflex block left %#v, %v", cleared.Oni.ReflexSettings, err)
	}
}

func TestSettingsSetRefusesBadOniReflexValues(t *testing.T) {
	cur := Defaults()
	cur.Oni.Reflex = true
	cur.Oni.ReflexSettings.Reminders = true
	for _, patch := range []string{
		`{"oni":{"reflexSettings":{"routines":"yes"}}}`,
		`{"oni":{"reflexSettings":{"reminders":0}}}`,
		`{"oni":{"reflexSettings":{"notes":"off"}}}`,
		`{"oni":{"reflexSettings":{"decisions":[true]}}}`,
		`{"oni":{"reflexSettings":{"routines":false,"decisions":{"hosted":true}}}}`,
		`{"oni":{"reflexSettings":true}}`,
		`{"oni":{"reflexSettings":"on"}}`,
		`{"oni":{"reflexSettings":[]}}`,
	} {
		next, err := Merge(cur, json.RawMessage(patch))
		var perr *protocol.Error
		if err == nil || !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
			t.Errorf("patch %s: %v", patch, err)
		}
		if next.Oni.ReflexSettings != cur.Oni.ReflexSettings || next.Oni.Reflex != cur.Oni.Reflex {
			t.Errorf("a refused patch %s changed reflex to %v %#v", patch, next.Oni.Reflex, next.Oni.ReflexSettings)
		}
	}
}

func TestAStoredBadOniReflexValueLoadsAsItsDefault(t *testing.T) {
	isolateConfig(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		file string
		want protocol.OniReflexSettings
	}{
		// each switch that is no boolean falls back on its own
		{`{"version":2,"oni":{"master":true,"reflex":true,"reflexSettings":{"routines":"yes","reminders":true,"notes":3,"decisions":true},"voice":true}}`, protocol.OniReflexSettings{Reminders: true, Decisions: true}},
		// a block that is no object falls back whole
		{`{"version":2,"oni":{"master":true,"reflex":true,"reflexSettings":"on","voice":true}}`, protocol.OniReflexSettings{}},
		{`{"version":2,"oni":{"master":true,"reflex":true,"reflexSettings":null,"voice":true}}`, protocol.OniReflexSettings{}},
		// a file from before the reflex switches loads them off
		{`{"version":2,"oni":{"master":true,"reflex":true,"voice":true}}`, protocol.OniReflexSettings{}},
		// a key no build knows is dropped
		{`{"version":2,"oni":{"master":true,"reflex":true,"reflexSettings":{"notes":true,"tabs":true},"voice":true}}`, protocol.OniReflexSettings{Notes: true}},
	} {
		if err := os.WriteFile(paths.ConfigFile(), []byte(c.file), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Oni.ReflexSettings != c.want {
			t.Errorf("%s loads reflexSettings as %#v, want %#v", c.file, loaded.Oni.ReflexSettings, c.want)
		}
		if !loaded.Oni.Master || !loaded.Oni.Voice || !loaded.Oni.Reflex {
			t.Errorf("%s lost the rest of the Oni block: %#v", c.file, loaded.Oni)
		}
	}
}

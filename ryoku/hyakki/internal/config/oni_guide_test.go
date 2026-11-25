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

func TestOniGuideIsOffOnAMachineThatNeverChangedASetting(t *testing.T) {
	isolateConfig(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Oni.Guide != (protocol.OniGuideSettings{}) || protocol.DefaultOniSettings().Guide != (protocol.OniGuideSettings{}) {
		t.Fatalf("guide = %#v", loaded.Oni.Guide)
	}
	b, err := json.Marshal(loaded.Oni.Guide)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"enabled":false,"keep":false,"hints":false}` {
		t.Fatalf("the guide block is %s", b)
	}
}

func TestOniGuideSettingsSurviveASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"guide":{"enabled":true,"keep":true,"hints":true}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Oni.Guide != (protocol.OniGuideSettings{Enabled: true, Keep: true, Hints: true}) {
		t.Fatalf("patched guide = %#v", next.Oni.Guide)
	}
	if next.Oni.Master || next.Oni.Inspect {
		t.Fatalf("a guide patch turned on a switch it never named: %#v", next.Oni)
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
	one, err := Merge(loaded, json.RawMessage(`{"oni":{"guide":{"hints":false}}}`))
	if err != nil || one.Oni.Guide != (protocol.OniGuideSettings{Enabled: true, Keep: true}) {
		t.Fatalf("turning hints off left %#v, %v", one.Oni.Guide, err)
	}
	cleared, err := Merge(loaded, json.RawMessage(`{"oni":{"guide":null}}`))
	if err != nil || cleared.Oni.Guide != (protocol.OniGuideSettings{}) {
		t.Fatalf("clearing the guide block left %#v, %v", cleared.Oni.Guide, err)
	}
}

func TestSettingsSetRefusesBadOniGuideValues(t *testing.T) {
	cur := Defaults()
	cur.Oni.Guide.Enabled = true
	for _, patch := range []string{
		`{"oni":{"guide":{"enabled":"yes"}}}`,
		`{"oni":{"guide":{"keep":1}}}`,
		`{"oni":{"guide":{"hints":"off"}}}`,
		`{"oni":{"guide":{"enabled":false,"keep":[true]}}}`,
		`{"oni":{"guide":true}}`,
		`{"oni":{"guide":"on"}}`,
		`{"oni":{"guide":[]}}`,
	} {
		next, err := Merge(cur, json.RawMessage(patch))
		var perr *protocol.Error
		if err == nil || !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
			t.Errorf("patch %s: %v", patch, err)
		}
		if next.Oni.Guide != cur.Oni.Guide {
			t.Errorf("a refused patch %s changed the guide to %#v", patch, next.Oni.Guide)
		}
	}
}

func TestAStoredBadOniGuideValueLoadsAsItsDefault(t *testing.T) {
	isolateConfig(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		file string
		want protocol.OniGuideSettings
	}{
		// each switch that is no boolean falls back on its own
		{`{"version":2,"oni":{"master":true,"guide":{"enabled":"yes","keep":true,"hints":3},"voice":true}}`, protocol.OniGuideSettings{Keep: true}},
		// a block that is no object falls back whole
		{`{"version":2,"oni":{"master":true,"guide":"on","voice":true}}`, protocol.OniGuideSettings{}},
		{`{"version":2,"oni":{"master":true,"guide":null,"voice":true}}`, protocol.OniGuideSettings{}},
		// a file from before the guide loads it off
		{`{"version":2,"oni":{"master":true,"voice":true}}`, protocol.OniGuideSettings{}},
		// a key no build knows is dropped
		{`{"version":2,"oni":{"master":true,"guide":{"enabled":true,"replay":true},"voice":true}}`, protocol.OniGuideSettings{Enabled: true}},
	} {
		if err := os.WriteFile(paths.ConfigFile(), []byte(c.file), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Oni.Guide != c.want {
			t.Errorf("%s loads the guide as %#v, want %#v", c.file, loaded.Oni.Guide, c.want)
		}
		if !loaded.Oni.Master || !loaded.Oni.Voice {
			t.Errorf("%s lost the rest of the Oni block: %#v", c.file, loaded.Oni)
		}
	}
}

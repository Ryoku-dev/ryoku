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

var (
	offRecall     = protocol.OniRecallSettings{}
	offRewrite    = protocol.OniRewriteSettings{Style: protocol.OniRewriteGrammar}
	offAssignment = protocol.OniAssignmentSettings{Style: protocol.OniAssignmentWalkthrough}
)

func TestPhase16SettingsAreOffOnAMachineThatNeverChangedASetting(t *testing.T) {
	isolateConfig(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Oni.Recall != offRecall || loaded.Oni.Rewrite != offRewrite || loaded.Oni.Assignment != offAssignment {
		t.Fatalf("recall %#v rewrite %#v assignment %#v", loaded.Oni.Recall, loaded.Oni.Rewrite, loaded.Oni.Assignment)
	}
	b, err := json.Marshal(struct {
		R protocol.OniRecallSettings     `json:"recall"`
		W protocol.OniRewriteSettings    `json:"rewrite"`
		A protocol.OniAssignmentSettings `json:"assignment"`
	}{loaded.Oni.Recall, loaded.Oni.Rewrite, loaded.Oni.Assignment})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"recall":{"enabled":false,"vault":false},"rewrite":{"enabled":false,"style":"grammar","voice":false},"assignment":{"enabled":false,"style":"walkthrough","model":""}}`; string(b) != want {
		t.Fatalf("the blocks are %s", b)
	}
}

func TestPhase16SettingsSurviveASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"recall":{"enabled":true,"vault":true},"rewrite":{"enabled":true,"style":"friendly","voice":true},"assignment":{"enabled":true,"style":"answers","model":"openrouter/some-model"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Oni.Recall != (protocol.OniRecallSettings{Enabled: true, Vault: true}) ||
		next.Oni.Rewrite != (protocol.OniRewriteSettings{Enabled: true, Style: protocol.OniRewriteFriendly, Voice: true}) ||
		next.Oni.Assignment != (protocol.OniAssignmentSettings{Enabled: true, Style: protocol.OniAssignmentAnswers, Model: "openrouter/some-model"}) {
		t.Fatalf("patched %#v %#v %#v", next.Oni.Recall, next.Oni.Rewrite, next.Oni.Assignment)
	}
	if next.Oni.Master || next.Oni.Inspect || next.Oni.Reflex {
		t.Fatalf("a Phase 16 patch turned on a switch it never named: %#v", next.Oni)
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
	cleared, err := Merge(loaded, json.RawMessage(`{"oni":{"recall":null,"rewrite":null,"assignment":null}}`))
	if err != nil || cleared.Oni.Recall != offRecall || cleared.Oni.Rewrite != offRewrite || cleared.Oni.Assignment != offAssignment {
		t.Fatalf("clearing the blocks left %#v %#v %#v, %v", cleared.Oni.Recall, cleared.Oni.Rewrite, cleared.Oni.Assignment, err)
	}
}

func TestSettingsSetRefusesBadPhase16Values(t *testing.T) {
	cur := Defaults()
	for _, patch := range []string{
		`{"oni":{"recall":{"enabled":"yes"}}}`,
		`{"oni":{"recall":{"vault":1}}}`,
		`{"oni":{"recall":true}}`,
		`{"oni":{"rewrite":{"style":"pirate"}}}`,
		`{"oni":{"rewrite":{"style":"custom"}}}`,
		`{"oni":{"rewrite":{"style":""}}}`,
		`{"oni":{"rewrite":{"voice":"on"}}}`,
		`{"oni":{"assignment":{"style":"hints"}}}`,
		`{"oni":{"assignment":{"model":" openrouter/x"}}}`,
		`{"oni":{"assignment":{"model":"not a model"}}}`,
		`{"oni":{"assignment":[]}}`,
	} {
		next, err := Merge(cur, json.RawMessage(patch))
		var perr *protocol.Error
		if err == nil || !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
			t.Errorf("patch %s: %v", patch, err)
		}
		if next.Oni.Recall != cur.Oni.Recall || next.Oni.Rewrite != cur.Oni.Rewrite || next.Oni.Assignment != cur.Oni.Assignment {
			t.Errorf("a refused patch %s changed a block", patch)
		}
	}
}

func TestAStoredBadPhase16ValueLoadsAsItsDefault(t *testing.T) {
	isolateConfig(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		file       string
		rewrite    protocol.OniRewriteSettings
		assignment protocol.OniAssignmentSettings
	}{
		{`{"version":2,"oni":{"master":true,"rewrite":{"enabled":true,"style":"pirate"},"assignment":{"enabled":true,"style":"hints","model":"a b"}}}`,
			protocol.OniRewriteSettings{Enabled: true, Style: protocol.OniRewriteGrammar},
			protocol.OniAssignmentSettings{Enabled: true, Style: protocol.OniAssignmentWalkthrough}},
		{`{"version":2,"oni":{"master":true}}`, offRewrite, offAssignment},
		{`{"version":2,"oni":{"master":true,"rewrite":{"style":"email","clipboard":true}}}`, protocol.OniRewriteSettings{Style: protocol.OniRewriteEmail}, offAssignment},
	} {
		if err := os.WriteFile(paths.ConfigFile(), []byte(c.file), 0o644); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Oni.Rewrite != c.rewrite || loaded.Oni.Assignment != c.assignment {
			t.Errorf("%s loads %#v %#v", c.file, loaded.Oni.Rewrite, loaded.Oni.Assignment)
		}
		if !loaded.Oni.Master {
			t.Errorf("%s lost the rest of the Oni block", c.file)
		}
	}
}

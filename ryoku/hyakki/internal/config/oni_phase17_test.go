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

func TestPhase17SettingsAreOffOnAMachineThatNeverChangedASetting(t *testing.T) {
	isolateConfig(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Oni.MCP, protocol.OniMCPSettings{}) || loaded.Onboarding != (protocol.OnboardingSettings{}) {
		t.Fatalf("mcp %#v onboarding %#v", loaded.Oni.MCP, loaded.Onboarding)
	}
}

func TestPhase17SettingsSurviveASaveAndClear(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"oni":{"mcp":{"enabled":true,"tools":{"notify":true,"reminders_list":true,"screen_ask":false},"clients":["Claude Code","Zed","Claude Code"]}},"onboarding":{"seen":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := protocol.OniMCPSettings{Enabled: true, Tools: map[string]bool{protocol.OniMCPNotify: true, protocol.OniMCPRemindersList: true}, Clients: []string{"Claude Code", "Zed"}}
	if !reflect.DeepEqual(next.Oni.MCP, want) || next.Onboarding != (protocol.OnboardingSettings{Seen: true}) {
		t.Fatalf("patched %#v %#v", next.Oni.MCP, next.Onboarding)
	}
	if next.Oni.Master || next.Oni.Reflex || next.Oni.ReflexSettings.Reminders {
		t.Fatalf("a Phase 17 patch turned on a switch it never named: %#v", next.Oni)
	}
	if err := Save(next); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Oni, next.Oni) || loaded.Onboarding != next.Onboarding {
		t.Fatalf("after a save and a load %#v %#v", loaded.Oni.MCP, loaded.Onboarding)
	}
	raw, err := os.ReadFile(paths.ConfigFile())
	if err != nil || !strings.Contains(string(raw), `"onboarding"`) {
		t.Fatalf("the file holds no onboarding block: %s %v", raw, err)
	}
	cleared, err := Merge(loaded, json.RawMessage(`{"oni":{"mcp":null},"onboarding":null}`))
	if err != nil || !reflect.DeepEqual(cleared.Oni.MCP, protocol.OniMCPSettings{}) || cleared.Onboarding != (protocol.OnboardingSettings{}) {
		t.Fatalf("clearing left %#v %#v, %v", cleared.Oni.MCP, cleared.Onboarding, err)
	}
	// turning one tool off drops it
	off, err := Merge(loaded, json.RawMessage(`{"oni":{"mcp":{"tools":{"notify":false}}}}`))
	if err != nil || !reflect.DeepEqual(off.Oni.MCP.Tools, map[string]bool{protocol.OniMCPRemindersList: true}) {
		t.Fatalf("turning notify off left %#v, %v", off.Oni.MCP.Tools, err)
	}
}

func TestSettingsSetRefusesBadPhase17Values(t *testing.T) {
	cur := Defaults()
	for _, patch := range []string{
		`{"oni":{"mcp":{"enabled":"yes"}}}`,
		`{"oni":{"mcp":true}}`,
		`{"oni":{"mcp":{"tools":{"tasks_add":true}}}}`,
		`{"oni":{"mcp":{"tools":{"notify":"on"}}}}`,
		`{"oni":{"mcp":{"tools":["notify"]}}}`,
		`{"oni":{"mcp":{"clients":[" Zed"]}}}`,
		`{"oni":{"mcp":{"clients":["two\nlines"]}}}`,
		`{"oni":{"mcp":{"clients":["` + strings.Repeat("a", protocol.OniMCPClientNameMax+1) + `"]}}}`,
		`{"oni":{"mcp":{"clients":"Zed"}}}`,
		`{"onboarding":{"seen":"yes"}}`,
		`{"onboarding":[]}`,
	} {
		next, err := Merge(cur, json.RawMessage(patch))
		var perr *protocol.Error
		if err == nil || !errors.As(err, &perr) || perr.Code != protocol.CodeBadRequest {
			t.Errorf("patch %s: %v", patch, err)
		}
		if !reflect.DeepEqual(next.Oni.MCP, cur.Oni.MCP) || next.Onboarding != cur.Onboarding {
			t.Errorf("a refused patch %s changed a block", patch)
		}
	}
	many := make([]string, protocol.OniMCPClientsMax+1)
	for i := range many {
		many[i] = strings.Repeat("p", i+1)
	}
	patch, _ := json.Marshal(map[string]any{"oni": map[string]any{"mcp": map[string]any{"clients": many}}})
	if _, err := Merge(cur, patch); err == nil {
		t.Error("too many approved programs passed")
	}
}

func TestAStoredBadPhase17ValueLoadsCleaned(t *testing.T) {
	isolateConfig(t)
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o755); err != nil {
		t.Fatal(err)
	}
	file := `{"version":2,"oni":{"master":true,"mcp":{"enabled":true,"tools":{"tasks_add":true,"notify":true,"screen_ask":false},"clients":["Zed"," bad","Zed","Claude Code"]}},"onboarding":{"seen":true,"done":true,"autostartOffers":9}}`
	if err := os.WriteFile(paths.ConfigFile(), []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := protocol.OniMCPSettings{Enabled: true, Tools: map[string]bool{protocol.OniMCPNotify: true}, Clients: []string{"Zed", "Claude Code"}}
	if !reflect.DeepEqual(loaded.Oni.MCP, want) || loaded.Onboarding != (protocol.OnboardingSettings{Seen: true, Done: true}) || !loaded.Oni.Master {
		t.Fatalf("%s loads %#v %#v", file, loaded.Oni.MCP, loaded.Onboarding)
	}
}

package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// A new session tells Rashin the harness's whole catalog: every model, the
// one it starts on, and every option with its choices.
func TestANewSessionReportsTheCatalog(t *testing.T) {
	r, sink := helperRunner(t, "options", protocol.ApprovalsAsk, nil)
	var reported protocol.HarnessCapabilities
	r.reportCapabilities = func(capabilities protocol.HarnessCapabilities) { reported = capabilities }
	if err := r.Start(context.Background(), sink); err != nil {
		t.Fatal(err)
	}
	wantModels := []protocol.Choice{{ID: "big", Name: "Big"}, {ID: "small", Name: "Small"}}
	if !reflect.DeepEqual(reported.ModelList, wantModels) || reported.DefaultModel != "big" {
		t.Fatalf("models %#v, default %q", reported.ModelList, reported.DefaultModel)
	}
	wantOptions := []protocol.SessionOption{{
		ID: "thinking", Name: "Thinking", Category: "thought_level", Current: "auto",
		Choices: []protocol.Choice{{ID: "auto", Name: "Auto"}, {ID: "high", Name: "High"}},
	}}
	if !reflect.DeepEqual(reported.Options, wantOptions) {
		t.Fatalf("options %#v", reported.Options)
	}
}

// A resumed session reports the model it was left on, which says nothing of
// the harness's default.
func TestAResumedSessionReportsNoDefaultModel(t *testing.T) {
	resume, err := json.Marshal(resumeState{ACPSession: "fake-session", Load: true})
	if err != nil {
		t.Fatal(err)
	}
	r, sink := helperRunner(t, "capabilities", protocol.ApprovalsAsk, resume)
	var reported protocol.HarnessCapabilities
	r.reportCapabilities = func(capabilities protocol.HarnessCapabilities) { reported = capabilities }
	if err := r.Start(context.Background(), sink); err != nil {
		t.Fatal(err)
	}
	if reported.DefaultModel != "" || len(reported.ModelList) != 1 || reported.ModelList[0].ID != "smart" {
		t.Fatalf("a resumed session reported %#v", reported)
	}
}

// Capabilities are kept per harness id and binary: two instances of one
// binary keep their own lists, a resumed session keeps the default a new one
// learned, a changed binary forgets them, and each record is announced.
func TestCapabilitiesAreKeptPerHarnessAndBinary(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	binary := writeTempBinary(t)
	registry := New()
	announced := 0
	registry.OnLearned(func() { announced++ })
	work := protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "opus", Name: "Opus"}}, DefaultModel: "opus"}
	home := protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "haiku", Name: "Haiku"}}, DefaultModel: "haiku"}
	registry.recordCapabilities("work", binary, work)
	registry.recordCapabilities("home", binary, home)
	registry.recordCapabilities("work", binary, protocol.HarnessCapabilities{ModelList: work.ModelList})
	if announced != 3 {
		t.Fatalf("%d records were announced, want 3", announced)
	}

	reloaded := New()
	gotWork, gotHome := reloaded.capabilitiesFor("work", binary), reloaded.capabilitiesFor("home", binary)
	if gotWork == nil || gotHome == nil {
		t.Fatalf("the cache lost an instance: work %v, home %v", gotWork, gotHome)
	}
	if gotWork.ModelList[0].ID != "opus" || gotWork.DefaultModel != "opus" || gotWork.Learned.IsZero() {
		t.Fatalf("work reads %#v", gotWork)
	}
	if gotHome.ModelList[0].ID != "haiku" || gotHome.DefaultModel != "haiku" {
		t.Fatalf("home reads %#v", gotHome)
	}
	if reloaded.capabilitiesFor("work", binary+"-other") != nil {
		t.Fatal("capabilities followed the id to another binary")
	}
	stamp := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(binary, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if reloaded.capabilitiesFor("work", binary) != nil {
		t.Fatal("capabilities outlived a changed binary")
	}
}

// A cache written before entries named their binary is dropped, not read as
// harness ids.
func TestAPathKeyedCacheIsDropped(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	binary := writeTempBinary(t)
	st, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	old := map[string]any{"entries": map[string]any{binary: map[string]any{"mtime": st.ModTime().UnixNano(), "capabilities": map[string]any{"models": true}}}}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := paths.EnsureDir(paths.StateDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(capabilitiesCachePath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
	registry := New()
	registry.mu.Lock()
	registry.loadCapabilitiesLocked()
	kept := len(registry.capabilities)
	registry.mu.Unlock()
	if kept != 0 {
		t.Fatalf("%d path-keyed entries were kept", kept)
	}
}

// Known names every built-in harness and every instance, installed or not;
// DefaultID and Name read without starting anything.
func TestKnownDefaultIDAndName(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	settings := protocol.Settings{Agent: protocol.AgentSettings{
		Harness: "work",
		Instances: []protocol.HarnessInstance{
			{ID: "work", Name: "Work account", Command: []string{os.Args[0]}},
			{ID: "spare", Command: []string{"not-on-path"}},
		},
	}}
	for _, id := range []string{"claude", "codex", "omp", "work", "spare"} {
		if !Known(id, settings) {
			t.Fatalf("%s is not known", id)
		}
	}
	for _, id := range []string{"", "nowhere", "Claude"} {
		if Known(id, settings) {
			t.Fatalf("%q is known", id)
		}
	}
	if got := DefaultID(settings); got != "work" {
		t.Fatalf("default %q, want work", got)
	}
	settings.Agent.Harness = "spare"
	if got := DefaultID(settings); got != "work" {
		t.Fatalf("an uninstalled choice did not fall back: %q", got)
	}
	settings.Agent.Instances = nil
	if got := DefaultID(settings); got != "" {
		t.Fatalf("nothing installed still has default %q", got)
	}
	settings.Agent.Instances = []protocol.HarnessInstance{{ID: "spare", Command: []string{"x"}}, {ID: "work", Name: " Work account ", Command: []string{"x"}}}
	for id, want := range map[string]string{"claude": "Claude Code", "work": "Work account", "spare": "spare", "nowhere": "nowhere"} {
		if got := Name(id, settings); got != want {
			t.Fatalf("Name(%q) = %q, want %q", id, got, want)
		}
	}
}

// writeTempBinary is a file standing in for a harness binary.
func writeTempBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

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
	registry.recordCapabilities("work", binary, userHome, work)
	registry.recordCapabilities("home", binary, userHome, home)
	registry.recordCapabilities("work", binary, userHome, protocol.HarnessCapabilities{ModelList: work.ModelList})
	if announced != 3 {
		t.Fatalf("%d records were announced, want 3", announced)
	}

	reloaded := New()
	gotWork, gotHome := reloaded.capabilitiesFor("work", binary, userHome), reloaded.capabilitiesFor("home", binary, userHome)
	if gotWork == nil || gotHome == nil {
		t.Fatalf("the cache lost an instance: work %v, home %v", gotWork, gotHome)
	}
	if gotWork.ModelList[0].ID != "opus" || gotWork.DefaultModel != "opus" || gotWork.Learned.IsZero() {
		t.Fatalf("work reads %#v", gotWork)
	}
	if gotHome.ModelList[0].ID != "haiku" || gotHome.DefaultModel != "haiku" {
		t.Fatalf("home reads %#v", gotHome)
	}
	if reloaded.capabilitiesFor("work", binary+"-other", userHome) != nil {
		t.Fatal("capabilities followed the id to another binary")
	}
	stamp := time.Now().Add(5 * time.Second)
	if err := os.Chtimes(binary, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if reloaded.capabilitiesFor("work", binary, userHome) != nil {
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
	kept := len(registry.capabilities[userHome])
	registry.mu.Unlock()
	if kept != 0 {
		t.Fatalf("%d path-keyed entries were kept", kept)
	}
}

// What a harness reports in a team's home stays that team's, and what it
// reports in the user's home stays the user's: a team run never replaces the
// list the user's own pickers show, nor the user's run a team's, and a team
// with no run of its own has no list, not the user's.
func TestCapabilitiesAreKeptPerHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	binary := writeTempBinary(t)
	registry := New()
	user := protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "a"}, {ID: "b"}, {ID: "c"}}, DefaultModel: "a"}
	team := protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "local"}}, DefaultModel: "local"}
	registry.recordCapabilities("omp", binary, userHome, user)
	registry.recordCapabilities("omp", binary, "team-dream", team)
	reloaded := New()
	if got := reloaded.capabilitiesFor("omp", binary, userHome); got == nil || len(got.ModelList) != 3 || got.DefaultModel != "a" {
		t.Fatalf("a team's run replaced the user's list: %#v", got)
	}
	if got := reloaded.capabilitiesFor("omp", binary, "team-dream"); got == nil || len(got.ModelList) != 1 || got.DefaultModel != "local" {
		t.Fatalf("the team's list reads %#v", got)
	}
	reloaded.recordCapabilities("omp", binary, userHome, protocol.HarnessCapabilities{ModelList: []protocol.Choice{{ID: "z"}}})
	if got := New().capabilitiesFor("omp", binary, "team-dream"); got == nil || got.ModelList[0].ID != "local" {
		t.Fatalf("the user's run replaced the team's list: %#v", got)
	}
	if got := New().capabilitiesFor("omp", binary, "team-other"); got != nil {
		t.Fatalf("a team that never ran the harness reads %#v", got)
	}
}

// A cache from before team homes kept their own lists holds only the user's.
func TestAnOldCacheIsTheUsersOwn(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	binary := writeTempBinary(t)
	st, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	old := map[string]any{"entries": map[string]any{"omp": map[string]any{"path": binary, "mtime": st.ModTime().UnixNano(), "capabilities": map[string]any{"defaultModel": "a"}}}}
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
	if got := registry.capabilitiesFor("omp", binary, userHome); got == nil || got.DefaultModel != "a" {
		t.Fatalf("the user's entry reads %#v", got)
	}
	if got := registry.capabilitiesFor("omp", binary, "team-a"); got != nil {
		t.Fatalf("a team reads the old entry %#v", got)
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

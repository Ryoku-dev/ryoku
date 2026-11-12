package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/harness"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// Only the app may start a harness to check it, and the harness then runs in
// a team agent's sandbox.
func TestHarnessProbeIsTheAppsAndSandboxed(t *testing.T) {
	isolateDaemon(t)
	harnesses := &fakeHarnesses{}
	d := New(Deps{Harnesses: harnesses, Version: "test"})
	raw := json.RawMessage(`{"id":"claude","model":"haiku"}`)
	if _, err := d.dispatch(context.Background(), &connection{}, protocol.MHarnessesProbe, raw); !forbidden(err) {
		t.Fatalf("another client probing: %v, want forbidden", err)
	}
	if len(harnesses.probed) != 0 {
		t.Fatal("a refused probe reached the harnesses")
	}
	out, err := d.dispatch(context.Background(), &connection{app: true}, protocol.MHarnessesProbe, raw)
	if err != nil {
		t.Fatal(err)
	}
	if result, ok := out.(protocol.HarnessProbeResult); !ok || !result.OK {
		t.Fatalf("the app's probe answered %#v", out)
	}
	if len(harnesses.probed) != 1 || harnesses.probed[0].ID != "claude" || harnesses.probed[0].Model != "haiku" {
		t.Fatalf("probed %+v", harnesses.probed)
	}
	if !slices.Contains(harnesses.base.Hidden, paths.Resolved(paths.StateDir())) || !slices.Equal(harnesses.base.Unset, agentUnset) || harnesses.base.Folder != "" {
		t.Fatalf("the probe's sandbox = %+v", harnesses.base)
	}
}

// Harnesses that report their capabilities close together reach the app as
// one harnesses.changed.
func TestLearnedCapabilitiesBroadcastOnceAfterTheDelay(t *testing.T) {
	isolateDaemon(t)
	events := NewBroadcaster()
	var mu sync.Mutex
	var heard []protocol.HarnessesResult
	events.add(func(event string, data any) {
		if event == protocol.EvtHarnessesChanged {
			mu.Lock()
			heard = append(heard, data.(protocol.HarnessesResult))
			mu.Unlock()
		}
	})
	harnesses := &fakeHarnesses{list: []protocol.HarnessInfo{{ID: "claude", Capabilities: &protocol.HarnessCapabilities{DefaultModel: "sonnet"}}}}
	New(Deps{Harnesses: harnesses, Events: events, Version: "test"})
	if harnesses.learned == nil {
		t.Fatal("the daemon did not listen for learned capabilities")
	}
	harnesses.learned()
	harnesses.learned()
	harnesses.learned()
	time.Sleep(learnedDelay / 2)
	mu.Lock()
	early := len(heard)
	mu.Unlock()
	if early != 0 {
		t.Fatal("harnesses.changed went out before the delay")
	}
	deadline := time.Now().Add(3 * learnedDelay)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(heard)
		mu.Unlock()
		if count > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(learnedDelay / 4)
	mu.Lock()
	defer mu.Unlock()
	if len(heard) != 1 || heard[0].Harnesses[0].Capabilities.DefaultModel != "sonnet" {
		t.Fatalf("heard %+v", heard)
	}
}

// settingsTeams records the settings Teams was told about.
type settingsTeams struct {
	boundTeams
	told []protocol.Settings
}

func (t *settingsTeams) SettingsChanged(settings protocol.Settings) {
	t.told = append(t.told, settings)
}

// A settings patch that reaches the teams' settings or the harness instances
// is the app's alone; the rest stay open to every client.
func TestSettingsThatSteerTeamsNeedTheApp(t *testing.T) {
	isolateDaemon(t)
	initial := config.Defaults()
	initial.Enabled = true
	if err := config.Save(initial); err != nil {
		t.Fatal(err)
	}
	crew := &settingsTeams{}
	d := New(Deps{Teams: crew, Version: "test"})
	d.mu.Lock()
	d.settings = initial
	d.mu.Unlock()
	set := func(c *connection, patch string) error {
		raw, err := json.Marshal(protocol.SettingsSetParams{Patch: json.RawMessage(patch)})
		if err != nil {
			t.Fatal(err)
		}
		_, err = d.dispatch(context.Background(), c, protocol.MSettingsSet, raw)
		return err
	}
	for _, patch := range []string{
		`{"teams":{"paused":true}}`,
		`{"teams":null}`,
		`{"agent":{"instances":[{"id":"work","name":"Work","command":["claude-agent-acp"]}]}}`,
		`{"agent":null}`,
		`null`,
	} {
		if err := set(&connection{}, patch); !forbidden(err) {
			t.Fatalf("another client setting %s: %v, want forbidden", patch, err)
		}
	}
	if d.Settings().Teams.Paused || len(crew.told) != 0 {
		t.Fatal("a refused patch changed the settings")
	}
	if err := set(&connection{}, `{"agent":{"approvals":"auto"}}`); err != nil {
		t.Fatalf("another client setting the agent approvals: %v", err)
	}
	if err := set(&connection{app: true}, `{"teams":{"paused":true}}`); err != nil {
		t.Fatalf("the app pausing every team: %v", err)
	}
	if !d.Settings().Teams.Paused {
		t.Fatal("the app's patch was not saved")
	}
	if len(crew.told) != 2 || !crew.told[1].Teams.Paused {
		t.Fatalf("Teams was told %d times", len(crew.told))
	}
}

// The teams' default harness must name a harness on this computer when it is
// chosen; one that later goes missing does not block other changes.
func TestTheTeamsDefaultHarnessMustBeKnown(t *testing.T) {
	isolateDaemon(t)
	initial := config.Defaults()
	initial.Enabled = true
	initial.Agent.Instances = []protocol.HarnessInstance{{ID: "work", Name: "Work", Command: []string{"claude-agent-acp"}}}
	if err := config.Save(initial); err != nil {
		t.Fatal(err)
	}
	d := New(Deps{Version: "test"})
	d.mu.Lock()
	d.settings = initial
	d.mu.Unlock()
	app := &connection{app: true}
	set := func(patch string) error {
		raw, err := json.Marshal(protocol.SettingsSetParams{Patch: json.RawMessage(patch)})
		if err != nil {
			t.Fatal(err)
		}
		_, err = d.dispatch(context.Background(), app, protocol.MSettingsSet, raw)
		return err
	}
	if err := set(`{"teams":{"defaults":{"harness":"nowhere"}}}`); err == nil {
		t.Fatal("an unknown default harness was saved")
	}
	for _, id := range []string{"codex", "work", ""} {
		if err := set(`{"teams":{"defaults":{"harness":"` + id + `"}}}`); err != nil {
			t.Fatalf("default harness %q: %v", id, err)
		}
	}
	if err := set(`{"teams":{"defaults":{"harness":"work"}}}`); err != nil {
		t.Fatal(err)
	}
	if err := set(`{"agent":{"instances":[]}}`); err != nil {
		t.Fatalf("removing the instance the default names: %v", err)
	}
	if err := set(`{"teams":{"paused":true}}`); err != nil {
		t.Fatalf("an unrelated change after the default went missing: %v", err)
	}
}

// Only the app opens a sign-in in a harness's team home; the terminal it runs
// ends it with the token it was handed, from any client, and the harness is
// then signed in for teams. A token works once.
func TestASignInForTeamsIsOpenedByTheAppAndEndedByItsToken(t *testing.T) {
	isolateDaemon(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "rashin"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	d := New(Deps{Harnesses: &fakeHarnesses{}, Version: "test"})
	if _, err := d.dispatch(context.Background(), &connection{app: true}, protocol.MHarnessesTeamsOpen, json.RawMessage(`{"id":"omp"}`)); err == nil {
		t.Fatal("a sign-in opened without a team")
	}
	raw := json.RawMessage(`{"id":"omp","team":"team-a"}`)
	if _, err := d.dispatch(context.Background(), &connection{}, protocol.MHarnessesTeamsOpen, raw); !forbidden(err) {
		t.Fatalf("another client opening a sign-in: %v, want forbidden", err)
	}
	out, err := d.dispatch(context.Background(), &connection{app: true}, protocol.MHarnessesTeamsOpen, raw)
	if err != nil {
		t.Fatal(err)
	}
	opened := out.(protocol.HarnessTeamsOpenResult)
	if opened.Home != paths.Resolved(harness.HomeOf("omp", "team-a")) || !slices.Equal(opened.Argv[len(opened.Argv)-2:], []string{"omp", "login"}) || !slices.Contains(opened.Argv, filepath.Join(bin, "rashin")) {
		t.Fatalf("the sign-in runs %q in %s", opened.Argv, opened.Home)
	}
	token := opened.Argv[6]
	done := func(token string) (protocol.HarnessTeamsDoneResult, error) {
		raw, _ := json.Marshal(protocol.HarnessTeamsDoneParams{Token: token})
		out, err := d.dispatch(context.Background(), &connection{}, protocol.MHarnessesTeamsDone, raw)
		if err != nil {
			return protocol.HarnessTeamsDoneResult{}, err
		}
		return out.(protocol.HarnessTeamsDoneResult), nil
	}
	if _, err := done("guessed"); err == nil {
		t.Fatal("a guessed token ended the sign-in")
	}
	if result, err := done(token); err != nil || !result.SignedIn {
		t.Fatalf("ending the sign-in: %+v %v", result, err)
	}
	if _, signed := harness.SignedIn("omp", "team-a"); !signed {
		t.Fatal("team-a is not signed in to omp")
	}
	if _, signed := harness.SignedIn("omp", "team-b"); signed {
		t.Fatal("signing team-a in signed team-b in")
	}
	if _, err := done(token); err == nil {
		t.Fatal("a token ended a sign-in twice")
	}
}

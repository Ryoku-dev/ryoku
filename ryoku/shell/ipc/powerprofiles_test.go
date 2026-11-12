package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fakePowerProfilesObject struct {
	active   string
	profiles []string
	sets     []string
}

func (f *fakePowerProfilesObject) GetProperty(name string) (dbus.Variant, error) {
	switch name {
	case ppIface + ".ActiveProfile":
		return dbus.MakeVariant(f.active), nil
	case ppIface + ".Profiles":
		rows := make([]map[string]dbus.Variant, 0, len(f.profiles))
		for _, profile := range f.profiles {
			rows = append(rows, map[string]dbus.Variant{"Profile": dbus.MakeVariant(profile)})
		}
		return dbus.MakeVariant(rows), nil
	default:
		return dbus.Variant{}, fmt.Errorf("unknown property %s", name)
	}
}

func (f *fakePowerProfilesObject) Call(_ string, _ dbus.Flags, args ...any) *dbus.Call {
	if len(args) != 3 {
		return &dbus.Call{Err: fmt.Errorf("unexpected set arguments")}
	}
	value, ok := args[2].(dbus.Variant).Value().(string)
	if !ok {
		return &dbus.Call{Err: fmt.Errorf("profile is not a string")}
	}
	f.active = value
	f.sets = append(f.sets, value)
	return &dbus.Call{}
}

func TestPowerProfilesCLIFallsBackWithoutSocket(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	gameMarker := filepath.Join(t.TempDir(), "game-mode.profile")
	if err := os.WriteFile(gameMarker, []byte("balanced\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_GAMEMODE_PROFILE_FILE", gameMarker)

	obj := &fakePowerProfilesObject{
		active:   "balanced",
		profiles: []string{"power-saver", "balanced", "performance"},
	}
	oldOpen := openDirectPowerProfiles
	openDirectPowerProfiles = func() (*powerProfilesState, func(), error) {
		return &powerProfilesState{obj: obj}, func() {}, nil
	}
	t.Cleanup(func() { openDirectPowerProfiles = oldOpen })

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = write
	err = sendCommand("powerprofiles state")
	_ = write.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	var state powerProfilesFrame
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatalf("state output %q: %v", body, err)
	}
	if state.ActiveProfile != "balanced" || len(state.Profiles) != 3 {
		t.Fatalf("state = %+v", state)
	}
	if err := sendCommand("powerprofiles set balanced"); err != nil {
		t.Fatal(err)
	}
	if len(obj.sets) != 1 || obj.sets[0] != "balanced" {
		t.Fatalf("set calls = %v", obj.sets)
	}
}

// profileNames extracts the ordered profile names from the raw Profiles
// property (aa{sv}), skipping malformed rows and preserving service order.
func TestProfileNames(t *testing.T) {
	raw := []map[string]dbus.Variant{
		{"Profile": dbus.MakeVariant("power-saver"), "Driver": dbus.MakeVariant("amd_pstate")},
		{"Profile": dbus.MakeVariant("balanced")},
		{"Driver": dbus.MakeVariant("only")}, // no Profile key, skipped
		{"Profile": dbus.MakeVariant("performance")},
	}
	got := profileNames(raw)
	want := []string{"power-saver", "balanced", "performance"}
	if len(got) != len(want) {
		t.Fatalf("profileNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("profileNames[%d] = %q, want %q (order must be preserved)", i, got[i], want[i])
		}
	}

	if profileNames("not the right type") != nil {
		t.Error("profileNames on a wrong-typed value should be nil")
	}
}

// TestLivePowerProfilesFrame exercises the real provider through the client that
// backs both the topic and socket calls. It also sets the current profile, a
// no-op that proves the write path without changing the user's active profile.
// Gated so the default `go test` stays deterministic and bus-free.
func TestLivePowerProfilesFrame(t *testing.T) {
	if os.Getenv("RYOKU_LIVE_DBUS") == "" {
		t.Skip("set RYOKU_LIVE_DBUS=1 to run the live power-profiles integration")
	}
	useLiveSession(t)
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		t.Skipf("no system bus: %v", err)
	}
	defer conn.Close()
	p := &powerProfilesState{
		conn:  conn,
		obj:   conn.Object(ppBusName, dbus.ObjectPath(ppPath)),
		topic: newStateTopic(),
	}
	sub := p.topic.subscribe()
	defer p.topic.unsubscribe(sub)
	p.publish()
	select {
	case frame := <-sub.frames:
		t.Logf("powerprofiles frame: %s", frame)
		var m powerProfilesFrame
		if err := json.Unmarshal(frame, &m); err != nil {
			t.Fatalf("frame is not valid JSON: %v", err)
		}
		if len(m.Profiles) == 0 {
			t.Error("no profiles from the running provider")
		}
		if m.ActiveProfile == "" {
			t.Fatal("running provider returned no active profile")
		}
		gameMarker := filepath.Join(t.TempDir(), "game-mode.profile")
		if err := os.WriteFile(gameMarker, []byte(m.ActiveProfile+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("RYOKU_GAMEMODE_PROFILE_FILE", gameMarker)
		d := &daemon{pp: p}
		var cliState powerProfilesFrame
		reply := d.powerProfilesCLI([]string{"state"})
		if err := json.Unmarshal([]byte(reply), &cliState); err != nil {
			t.Fatalf("powerprofiles state returned %q", reply)
		}
		if len(cliState.Profiles) != len(m.Profiles) || cliState.ActiveProfile != m.ActiveProfile {
			t.Fatalf("CLI state = %+v, topic state = %+v", cliState, m)
		}
		if reply := d.powerProfilesCLI([]string{"set", m.ActiveProfile}); reply != "ok" {
			t.Fatalf("setting current profile %q: %s", m.ActiveProfile, reply)
		}
		if got := p.activeProfile(); got != m.ActiveProfile {
			t.Fatalf("active profile changed after no-op set: got %q, want %q", got, m.ActiveProfile)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no powerprofiles frame published")
	}
}

func TestShouldApplyProfile(t *testing.T) {
	cases := []struct {
		name, cfg, active, want string
	}{
		{"no profiles block", `{"chargeLimit":80}`, "balanced", ""},
		{"profile not defined", `{"profiles":{"performance":{"epp":"performance"}}}`, "balanced", ""},
		{"empty active", `{"profiles":{"balanced":{}}}`, "", ""},
		{"malformed", `not json`, "balanced", ""},
		{"defined", `{"profiles":{"balanced":{"governor":"powersave"}}}`, "balanced", "balanced"},
	}
	for _, c := range cases {
		if got := shouldApplyProfile([]byte(c.cfg), c.active); got != c.want {
			t.Errorf("%s: shouldApplyProfile = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestApplyActiveProfileNoConfig proves the load-bearing safety rule: with a
// power.json that has no "profiles" block, the applier is never invoked, so an
// unconfigured user triggers no ryoku-power exec (and thus no pkexec prompt).
func TestApplyActiveProfileNoConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "ryoku")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "power.json")
	if err := os.WriteFile(path, []byte(`{"chargeLimit":80}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls []string
	orig := applyProfileCmd
	applyProfileCmd = func(profile string) error { calls = append(calls, profile); return nil }
	defer func() { applyProfileCmd = orig }()

	p := &powerProfilesState{}
	p.applyActiveProfile("balanced")
	if len(calls) != 0 {
		t.Fatalf("applier invoked %v with no profiles block; must stay hands-off", calls)
	}

	// Positive control: once the profile is defined, the applier runs exactly once.
	if err := os.WriteFile(path, []byte(`{"profiles":{"balanced":{"governor":"powersave"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p.applyActiveProfile("balanced")
	if len(calls) != 1 || calls[0] != "balanced" {
		t.Fatalf("applier calls = %v, want one apply of balanced", calls)
	}
}

func TestApplyDelay(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfgDir := filepath.Join(dir, "ryoku")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "power.json")

	if got := applyDelay(); got != 400*time.Millisecond {
		t.Errorf("applyDelay with no file = %v, want 400ms", got)
	}
	os.WriteFile(path, []byte(`{"applyDelayMs":1000}`), 0o644)
	if got := applyDelay(); got != time.Second {
		t.Errorf("applyDelay = %v, want 1s", got)
	}
	os.WriteFile(path, []byte(`{"applyDelayMs":0}`), 0o644)
	if got := applyDelay(); got != 400*time.Millisecond {
		t.Errorf("applyDelay with 0 = %v, want 400ms default", got)
	}
}

func TestDecideProfileChange(t *testing.T) {
	const long = time.Hour
	const near = 2 * time.Second // inside autoSwitchWindow and restoreReassertWindow
	cases := []struct {
		name                                       string
		active, saved                              string
		restoreDone, onBat, saverFeature, gameMode bool
		sinceFlip, sinceRestore                    time.Duration
		want                                       profileAction
	}{
		// boot noise and Ryoku-caused changes are never a pick.
		{"empty profile", "", "balanced", true, false, false, false, long, long, profileNone},
		{"before restore is boot noise", "performance", "balanced", false, false, false, false, long, long, profileNone},
		{"game mode owns the profile", "performance", "balanced", true, false, false, true, long, long, profileNone},
		{"game mode wins even inside the restore window", "performance", "balanced", true, false, false, true, long, near, profileNone},
		{"battery saver is not a choice", "power-saver", "balanced", true, true, true, false, long, long, profileNone},
		{"switch right after an AC flip is automatic", "performance", "balanced", true, false, false, false, near, long, profileNone},
		// a late ppd boot default is put back, not banked.
		{"ppd default drift inside the restore window is re-asserted", "performance", "balanced", true, false, false, false, long, near, profileReassert},
		{"no saved pick means nothing to re-assert", "performance", "", true, false, false, false, long, near, profilePersist},
		{"drift equal to the saved pick is not re-asserted", "balanced", "balanced", true, false, false, false, long, near, profilePersist},
		// genuine picks on either write path persist.
		{"saver on AC is a real pick", "power-saver", "balanced", true, false, true, false, long, long, profilePersist},
		{"performance after the restore window is a pick", "performance", "balanced", true, false, false, false, long, long, profilePersist},
		{"settled balanced is a pick", "balanced", "balanced", true, false, false, false, long, long, profilePersist},
	}
	for _, c := range cases {
		got := decideProfileChange(c.active, c.saved, c.restoreDone, c.onBat, c.saverFeature, c.gameMode, c.sinceFlip, c.sinceRestore)
		if got != c.want {
			t.Errorf("%s: decideProfileChange = %v, want %v", c.name, got, c.want)
		}
	}
}

// A box whose firmware boots power-profiles-daemon on performance and has never
// been given a profile gets balanced; nothing else is touched. This is the #157
// case: the CPU sat near 90 C on light use because nobody had ever chosen.
func TestShouldSeedBalanced(t *testing.T) {
	for _, c := range []struct {
		name          string
		saved, active string
		gaming, want  bool
	}{
		{"firmware boots on performance, never chosen", "", "performance", false, true},
		{"a saved pick is the user's", "performance", "performance", false, false},
		{"a saved balanced is left alone", "balanced", "performance", false, false},
		{"a quiet machine stays quiet", "", "power-saver", false, false},
		{"already balanced needs nothing", "", "balanced", false, false},
		{"a running game owns the profile", "", "performance", true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldSeedBalanced(c.saved, c.active, c.gaming); got != c.want {
				t.Errorf("shouldSeedBalanced(%q, %q, %v) = %v, want %v",
					c.saved, c.active, c.gaming, got, c.want)
			}
		})
	}
}

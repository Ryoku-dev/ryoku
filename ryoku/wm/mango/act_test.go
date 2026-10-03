package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// Every action is pinned to the exact dispatch line it emits, because the
// shape is the contract: a mistyped dispatcher answers {"error":"unknown
// function"} and would surface as a dead keybind, so these catch it at build
// time instead.

func TestActEmitsMangoDispatchers(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"close", []string{"window.close", "375"}, "dispatch killclient,client,375"},
		{"focus", []string{"window.focus", "375"}, "dispatch focusid,client,375"},
		{"fullscreen", []string{"window.fullscreen", "7"}, "dispatch togglefullscreen,client,7"},
		{"float", []string{"window.float", "7"}, "dispatch togglefloating,client,7"},
		{"moveToWorkspace", []string{"window.moveToWorkspace", "7", "3"}, "dispatch tag,3,client,7"},
		{"workspace focus", []string{"workspace.focus", "4"}, "dispatch view,4"},
		{"cycle left", []string{"workspace.cycle", "-1"}, "dispatch viewtoleft"},
		{"cycle right", []string{"workspace.cycle", "1"}, "dispatch viewtoright"},
		{"special", []string{"workspace.toggleSpecial"}, "dispatch toggle_special_tag"},
		{"exit", []string{"session.exit"}, "dispatch quit"},
		{"keyboard", []string{"keyboard.cycleLayout"}, "dispatch switch_keyboard_layout"},
		{"submap enter", []string{"submap.enter", "ryoku_capture"}, "dispatch setkeymode,ryoku_capture"},
		{"submap reset", []string{"submap.reset"}, "dispatch setkeymode,default"},
		{"overview", []string{"overview.toggle"}, "dispatch toggleoverview"},
		{"reload", []string{"config.reload"}, "dispatch reload_config"},
		{"enable on", []string{"output.enable", "eDP-1", "on"}, "dispatch enable_monitor,eDP-1"},
		{"enable off", []string{"output.enable", "eDP-1", "off"}, "dispatch disable_monitor,eDP-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
				got = cmd
				return json.RawMessage(`{"success":true}`), nil
			})
			defer restore()
			if err := runAct(tc.args); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("emitted %q, want %q", got, tc.want)
			}
		})
	}
}

// A reload with no live session is a successful no-op: apply has already
// written the tree the next login reads, and the seam's contract (niri's
// watcher owns the same step) is that the file swap lands without a
// compositor. Erroring here would report a saved change as failed, so the
// behaviour is pinned, not incidental.
func TestActReloadWithoutSessionIsNoOp(t *testing.T) {
	prevLive, prevRequest := aliveCheck, request
	aliveCheck = func(string) bool { return false }
	called := false
	request = func(string) (json.RawMessage, error) {
		called = true
		return nil, nil
	}
	t.Cleanup(func() { aliveCheck, request = prevLive, prevRequest })
	t.Setenv("MANGO_INSTANCE_SIGNATURE", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if err := runAct([]string{"config.reload"}); err != nil {
		t.Fatalf("reload with no session: %v", err)
	}
	if called {
		t.Fatal("reload dispatched without a live session")
	}
	// The no-op is scoped to the file-driven reload; every other action
	// still demands a live session.
	if err := runAct([]string{"window.close", "7"}); err == nil ||
		!strings.Contains(err.Error(), "no live mango session") {
		t.Fatalf("window.close without a session: %v", err)
	}
}

// Output power addresses one named connector or every one when unnamed: mango
// sleeps a single output per call, unlike niri's global switch.
func TestActOutputPowerFansOut(t *testing.T) {
	var got []string
	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		if cmd == "get all-monitors" {
			return json.RawMessage(`{"monitors":[{"name":"eDP-1"},{"name":"HDMI-A-1"}]}`), nil
		}
		got = append(got, cmd)
		return json.RawMessage(`{"success":true}`), nil
	})
	defer restore()

	if err := runAct([]string{string(wm.ActionOutputPower), "off"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "dispatch sleep_monitor,eDP-1 dispatch sleep_monitor,HDMI-A-1" {
		t.Fatalf("fan-out wrong: %v", got)
	}

	got = nil
	if err := runAct([]string{string(wm.ActionOutputPower), "on", "HDMI-A-1"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "dispatch wakeup_monitor,HDMI-A-1" {
		t.Fatalf("named target wrong: %v", got)
	}
}

// A missing argument must name the value, so a bad keybind is diagnosable, and
// must never reach the compositor: a mango window action without the client
// suffix silently applies to whatever has focus.
func TestActRejectsMissingArgs(t *testing.T) {
	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		t.Fatalf("compositor call with a bad argument: %s", cmd)
		return nil, nil
	})
	defer restore()

	for _, args := range [][]string{
		{"window.close"},
		{"window.focus"},
		{"workspace.focus"},
		{"window.summon"},
		{"window.moveToWorkspace", "1"},
		{"submap.enter"},
	} {
		if err := runAct(args); err == nil {
			t.Errorf("act %v: want an error", args)
		}
	}
	if err := runAct([]string{"window.close", "not-a-number"}); err == nil {
		t.Error("a non-numeric window id must be refused")
	}
}

// An action this compositor cannot perform names the capability, so a caller
// that skipped the gate is told which one to check instead of getting a silent
// no-op or an unknown-action error.
func TestActNamesTheMissingCapability(t *testing.T) {
	restore := stubRequest(t, func(string) (json.RawMessage, error) {
		return json.RawMessage(`{"success":true}`), nil
	})
	defer restore()

	err := runAct([]string{"workspace.moveToOutput", "1", "HDMI-A-1"})
	if err == nil || !strings.Contains(err.Error(), string(wm.CapWorkspaceMoveToOutput)) {
		t.Fatalf("want the missing capability named, got %v", err)
	}
	err = runAct([]string{"decoration.screenShader", "shader"})
	if err == nil || !strings.Contains(err.Error(), "screen shader") {
		t.Fatalf("want the screen shader refused, got %v", err)
	}
}

func TestActRejectsUnknownAction(t *testing.T) {
	restore := stubRequest(t, func(string) (json.RawMessage, error) { return nil, nil })
	defer restore()
	if err := runAct([]string{"not.a.thing"}); err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("want an unknown-action error, got %v", err)
	}
}

// summon raises an already-open window to the current tag and focuses it,
// matched by exact title: the newest match (highest id) is pulled onto the
// focused tag with tag, then focusid. No match is an error, which lets the
// keybind fall through to launching the app.
func TestActSummonMovesNewestTitleMatch(t *testing.T) {
	var got []string
	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		switch cmd {
		case "get all-clients":
			return json.RawMessage(`{"clients":[
				{"id":2,"title":"Ryoku Hub","monitor":"eDP-1","tags":[3]},
				{"id":9,"title":"Ryoku Hub","monitor":"eDP-1","tags":[5]}]}`), nil
		case "get all-monitors":
			return json.RawMessage(`{"monitors":[{"name":"eDP-1","active":true,"tags":[{"index":1,"is_active":true}]}]}`), nil
		}
		got = append(got, cmd)
		return json.RawMessage(`{"success":true}`), nil
	})
	defer restore()

	if err := runAct([]string{"window.summon", "Ryoku Hub"}); err != nil {
		t.Fatal(err)
	}
	want := "dispatch tag,1,client,9 dispatch focusid,client,9"
	if strings.Join(got, " ") != want {
		t.Fatalf("summon emitted %q, want %q", strings.Join(got, " "), want)
	}

	if err := runAct([]string{"window.summon", "nowhere"}); err == nil {
		t.Fatal("no match must be an error so the keybind falls through")
	}
}

// The border act pushes the live palette through setoption (probed: repaint
// immediately) and records it for the next apply. A pinned border makes it a
// no-op: nothing is pushed, so a wallpaper change leaves the chosen colour
// alone.
func TestActBorderPalettePushesAndRecords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("XDG_STATE_HOME", home)
	writeStore(t, filepath.Join(home, "ryoku", "desktop.json"), `{"desktop":{"appearance":{"borderFollowsPalette":true}},"wm":{}}`)

	var got []string
	restore := stubRequest(t, func(cmd string) (json.RawMessage, error) {
		got = append(got, cmd)
		return json.RawMessage(`{"success":true}`), nil
	})
	defer restore()

	if err := runAct([]string{"decoration.borderColors", "#ff8800", "#222222"}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "dispatch setoption,focuscolor,0xff8800ff" || got[1] != "dispatch setoption,bordercolor,0x222222ff" {
		t.Fatalf("push wrong: %v", got)
	}
	b, err := os.ReadFile(filepath.Join(home, "ryoku", "mango-border-palette.json"))
	if err != nil {
		t.Fatalf("palette not recorded: %v", err)
	}
	if !strings.Contains(string(b), "ff8800") {
		t.Fatalf("palette file wrong: %s", b)
	}
}

func TestActBorderPaletteNoOpWhenFixed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("XDG_STATE_HOME", home)
	writeStore(t, filepath.Join(home, "ryoku", "desktop.json"), `{"desktop":{"appearance":{"borderFollowsPalette":false}},"wm":{}}`)

	var calls int
	restore := stubRequest(t, func(string) (json.RawMessage, error) {
		calls++
		return json.RawMessage(`{"success":true}`), nil
	})
	defer restore()

	if err := runAct([]string{"decoration.borderColors", "#ff8800", "#222222"}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("a pinned border must not push, got %d calls", calls)
	}
}

// The touchpad lock rides a live setoption AND records the intent in a state
// file that writeInput re-emits, so the lock survives a reload and the next
// login. status reads the file, never the compositor, and toggle flips it.
func TestActTouchpadTracksStateFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("XDG_STATE_HOME", home)
	writeStore(t, filepath.Join(home, "ryoku", "desktop.json"), `{"desktop":{},"wm":{}}`)
	touchpadNotify = func(string, string) {}

	restore := stubRequest(t, func(string) (json.RawMessage, error) {
		return json.RawMessage(`{"success":true}`), nil
	})
	defer restore()

	if got := touchpadStatus(t); got != "on" {
		t.Fatalf("initial status = %q, want on", got)
	}
	if err := runAct([]string{"input.touchpad", "off"}); err != nil {
		t.Fatal(err)
	}
	if got := touchpadStatus(t); got != "off" {
		t.Fatalf("after off = %q", got)
	}
	// The emitted lock must reach the generated config too: writeInput re-reads
	// the state file on every apply.
	s := loadStore(filepath.Join(home, "ryoku", "desktop.json"))
	body := string(genSettings(s))
	if !strings.Contains(body, "disable_trackpad=1") {
		t.Fatalf("locked touchpad must be emitted into settings.conf:\n%s", body)
	}
	if err := runAct([]string{"input.touchpad", "toggle"}); err != nil {
		t.Fatal(err)
	}
	if got := touchpadStatus(t); got != "on" {
		t.Fatalf("after toggle = %q", got)
	}
}

// The colour temperature is clamped to the range the gamma client accepts and
// a missing or unparseable argument falls back to the default, so a stray
// keybind argument can never ask wlsunset for a value it would reject or 0 K.
func TestNightlightTempClamps(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{nil, 4000},
		{[]string{"3500"}, 3500},
		{[]string{"nope"}, 4000},
		{[]string{"0"}, 4000},
		{[]string{"99999"}, 4000},
	}
	for _, tc := range cases {
		if got := nightlightTemp(tc.args); got != tc.want {
			t.Errorf("nightlightTemp(%v) = %d, want %d", tc.args, got, tc.want)
		}
	}
}

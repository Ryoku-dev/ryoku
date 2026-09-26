package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every action is pinned to the exact hyprctl argv it emits, because the dialect
// is the contract with the compositor and getting it wrong fails silently. The
// classic `dispatch workspace 9` form shipped once and Hyprland's Lua config
// provider rejected all of it: dispatch evaluates its argument as
// hl.dispatch(<expr>), and hyprctl keyword refuses outright under that parser.
func TestActEmitsLuaDialect(t *testing.T) {
	const addr = "0xdeadbeef"
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"window focus", []string{"window.focus", addr},
			[]string{"dispatch", `hl.dsp.focus({ window = "address:0xdeadbeef" })`}},
		{"window close", []string{"window.close", addr},
			[]string{"dispatch", `hl.dsp.window.close({ window = "address:0xdeadbeef" })`}},
		{"window float", []string{"window.float", addr},
			[]string{"dispatch", `hl.dsp.window.float({ action = "toggle", window = "address:0xdeadbeef" })`}},
		{"window to workspace", []string{"window.moveToWorkspace", addr, "3"},
			[]string{"dispatch", `hl.dsp.window.move({ workspace = "3", window = "address:0xdeadbeef" })`}},
		{"app focus", []string{"app.focus", "org.quickshell"},
			[]string{"dispatch", `hl.dsp.focus({ window = "class:org.quickshell" })`}},
		{"workspace focus", []string{"workspace.focus", "9"},
			[]string{"dispatch", `hl.dsp.focus({ workspace = "9" })`}},
		{"workspace cycle forward", []string{"workspace.cycle", "1"},
			[]string{"dispatch", `hl.dsp.focus({ workspace = "r+1" })`}},
		{"workspace cycle back", []string{"workspace.cycle", "-2"},
			[]string{"dispatch", `hl.dsp.focus({ workspace = "r-2" })`}},
		{"workspace to output", []string{"workspace.moveToOutput", "9", "eDP-2"},
			[]string{"dispatch", `hl.dsp.workspace.move({ workspace = "9", monitor = "eDP-2" })`}},
		{"special workspace", []string{"workspace.toggleSpecial", "sharebar"},
			[]string{"dispatch", `hl.dsp.workspace.toggle_special("sharebar")`}},
		{"session exit", []string{"session.exit"},
			[]string{"dispatch", `hl.dsp.exit()`}},
		{"output power on", []string{"output.power", "on"},
			[]string{"dispatch", `hl.dsp.dpms({ action = "on" })`}},
		{"output power off", []string{"output.power", "off", "eDP-2"},
			[]string{"dispatch", `hl.dsp.dpms({ action = "off", monitor = "eDP-2" })`}},
		{"output enable on", []string{"output.enable", "DP-1", "on"},
			[]string{"eval", `hl.monitor({ output = "DP-1", disabled = false })`}},
		{"output enable off", []string{"output.enable", "DP-1", "off"},
			[]string{"eval", `hl.monitor({ output = "DP-1", disabled = true })`}},
		{"submap enter", []string{"submap.enter", "resize"},
			[]string{"dispatch", `hl.dsp.submap("resize")`}},
		{"submap reset", []string{"submap.reset"},
			[]string{"dispatch", `hl.dsp.submap("reset")`}},
		// keyword is rejected by the Lua parser, so live config changes are eval.
		{"autoreload off", []string{"config.autoreload", "off"},
			[]string{"eval", `hl.config({ misc = { disable_autoreload = true } })`}},
		{"workspace layout", []string{"workspace.layout", "9", "master"},
			[]string{"eval", `hl.workspace_rule({ workspace = "9", layout = "master" })`}},
		{"border colours", []string{"decoration.borderColors", "#f3701e", "#0e0d0b"},
			[]string{"eval", `hl.config({general={["col.active_border"]="rgb(f3701e)",["col.inactive_border"]="rgb(0e0d0b)"}})`}},
		{"clear shader", []string{"decoration.screenShader"},
			[]string{"eval", `hl.config({ decoration = { screen_shader = "" } })`}},
		// Top-level hyprctl commands are not keywords and work unchanged.
		{"cursor", []string{"cursor.set", "Bibata-Modern-Ice", "24"},
			[]string{"setcursor", "Bibata-Modern-Ice", "24"}},
		{"keyboard layout", []string{"keyboard.cycleLayout"},
			[]string{"switchxkblayout", "all", "next"}},
		{"reload config only", []string{"config.reload", "config-only"},
			[]string{"reload", "config-only"}},
		// game mode strips decorations through eval and restores them with a
		// reload, exactly what the ported ryoku-cmd-game-mode ran.
		{"game mode on", []string{"decoration.gameMode", "on"},
			[]string{"eval", gameModeLua}},
		{"game mode off", []string{"decoration.gameMode", "off"},
			[]string{"reload"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			restore := stubCtl(t, func(args ...string) ([]byte, error) {
				got = args
				return nil, nil
			})
			defer restore()
			if err := runAct(tc.args); err != nil {
				t.Fatalf("runAct(%v): %v", tc.args, err)
			}
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Errorf("argv mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// Re-enabling a connector must not rewrite its layout. hl.monitor copies the
// output's existing rule and reapplies only the fields it is handed, so an
// enable that also named a mode, position or scale would snap the output back
// to "preferred at auto" and throw away the arrangement the user set. The
// enable-on payload therefore carries disabled alone -- the regression this
// pins is a re-enable that clobbered the layout.
func TestActOutputEnablePreservesLayout(t *testing.T) {
	var evals []string
	restore := stubCtl(t, func(args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "eval" {
			evals = append(evals, args[1])
		}
		return nil, nil
	})
	defer restore()

	if err := runAct([]string{"output.enable", "DP-1", "on"}); err != nil {
		t.Fatalf("output.enable on: %v", err)
	}
	if len(evals) != 1 {
		t.Fatalf("output.enable on evaluated %d calls, want 1: %q", len(evals), evals)
	}
	got := evals[0]
	if !strings.Contains(got, `output = "DP-1"`) || !strings.Contains(got, "disabled = false") {
		t.Errorf("enable-on payload = %q, want output DP-1 with disabled = false", got)
	}
	for _, field := range []string{"mode", "position", "scale"} {
		if strings.Contains(got, field) {
			t.Errorf("enable-on payload %q sets %q, which would reset the output's layout", got, field)
		}
	}
}

// A missing argument must name the value, so a bad keybind is diagnosable, and
// must never reach the compositor with an empty selector (which Hyprland applies
// to the focused window).
func TestActRejectsMissingArgs(t *testing.T) {
	for _, args := range [][]string{
		{"window.focus"},
		{"window.moveToWorkspace", "0xabc"},
		{"workspace.focus"},
		{"workspace.moveToOutput", "9"},
		{"cursor.set", "Bibata"},
		{"output.power"},
		{"window.summon"},
	} {
		called := false
		restore := stubCtl(t, func(...string) ([]byte, error) {
			called = true
			return nil, nil
		})
		err := runAct(args)
		restore()
		if err == nil {
			t.Errorf("runAct(%v): expected an error", args)
		}
		if called {
			t.Errorf("runAct(%v): reached the compositor despite a missing argument", args)
		}
	}
}

// window.summon raises an already-open window to the current workspace and
// focuses it, matched by exact title. The live title wins over the initial
// title, and a title with no window is an error so the keybind launches the app
// instead of raising nothing.
func TestActSummonRaisesByTitle(t *testing.T) {
	const clients = `[
		{"address":"0xaaa","title":"Other","initialTitle":"Other"},
		{"address":"0xbbb","title":"Ryoku Hub","initialTitle":"org.quickshell"},
		{"address":"0xccc","title":"stale","initialTitle":"Ryoku Hub"}
	]`
	const active = `{"id":5}`

	var dispatched [][]string
	restore := stubCtl(t, func(args ...string) ([]byte, error) {
		switch {
		case len(args) == 2 && args[0] == "clients" && args[1] == "-j":
			return []byte(clients), nil
		case len(args) == 2 && args[0] == "activeworkspace" && args[1] == "-j":
			return []byte(active), nil
		case len(args) > 0 && args[0] == "dispatch":
			dispatched = append(dispatched, args)
		}
		return nil, nil
	})
	defer restore()

	if err := runAct([]string{"window.summon", "Ryoku Hub"}); err != nil {
		t.Fatalf("summon: %v", err)
	}
	want := [][]string{
		{"dispatch", `hl.dsp.window.move({ workspace = 5, window = "address:0xbbb" })`},
		{"dispatch", `hl.dsp.focus({ window = "address:0xbbb" })`},
	}
	if len(dispatched) != len(want) {
		t.Fatalf("dispatch count = %d, want %d: %q", len(dispatched), len(want), dispatched)
	}
	for i := range want {
		if strings.Join(dispatched[i], "\x00") != strings.Join(want[i], "\x00") {
			t.Errorf("dispatch %d\n got: %q\nwant: %q", i, dispatched[i], want[i])
		}
	}

	// A title with no window is an error and never dispatches, so the keybind
	// falls through to launching the app.
	dispatched = nil
	if err := runAct([]string{"window.summon", "Nonexistent"}); err == nil {
		t.Error("summon of an absent title: expected an error")
	}
	if len(dispatched) != 0 {
		t.Errorf("summon of an absent title dispatched %q", dispatched)
	}
}

func TestActRejectsUnknownAction(t *testing.T) {
	restore := stubCtl(t, func(...string) ([]byte, error) { return nil, nil })
	defer restore()
	if err := runAct([]string{"window.teleport"}); err == nil {
		t.Fatal("expected an error for an unknown action")
	}
}

// The touchpad lock flips every pad through hl.device eval and keeps the intent
// in a flag file, because Hyprland re-enables every pad on a reload and exposes
// no per-device readback. Ported from ryoku-cmd-touchpad; the eval string and
// the flag file are the contract, so both are pinned.
func TestActTouchpadFlipsPadsAndTracksState(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	flag := filepath.Join(state, "ryoku", "touchpad.disabled")

	prevNotify := touchpadNotify
	touchpadNotify = func(string, string) {}
	defer func() { touchpadNotify = prevNotify }()

	const devices = `{"mice":[{"name":"synps/2-touchpad"},{"name":"logitech-mouse"}]}`
	var evals []string
	restore := stubCtl(t, func(args ...string) ([]byte, error) {
		if len(args) == 2 && args[0] == "devices" && args[1] == "-j" {
			return []byte(devices), nil
		}
		if len(args) == 2 && args[0] == "eval" {
			evals = append(evals, args[1])
		}
		return nil, nil
	})
	defer restore()

	// off locks only the touchpad, not the mouse, and records the intent.
	evals = nil
	if err := runAct([]string{"input.touchpad", "off"}); err != nil {
		t.Fatalf("off: %v", err)
	}
	wantOff := []string{`hl.device({ name = "synps/2-touchpad", enabled = false })`}
	if strings.Join(evals, "\x00") != strings.Join(wantOff, "\x00") {
		t.Errorf("off evals\n got: %q\nwant: %q", evals, wantOff)
	}
	if _, err := os.Stat(flag); err != nil {
		t.Errorf("off did not write the flag file: %v", err)
	}

	// status reads the flag file, not the compositor.
	var buf bytes.Buffer
	prevOut := stdout
	stdout = bufio.NewWriter(&buf)
	err := runAct([]string{"input.touchpad", "status"})
	stdout.Flush()
	stdout = prevOut
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "off" {
		t.Errorf("status = %q, want off", buf.String())
	}

	// toggle from a stored off re-enables and clears the intent.
	evals = nil
	if err := runAct([]string{"input.touchpad", "toggle"}); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	wantOn := []string{`hl.device({ name = "synps/2-touchpad", enabled = true })`}
	if strings.Join(evals, "\x00") != strings.Join(wantOn, "\x00") {
		t.Errorf("toggle evals\n got: %q\nwant: %q", evals, wantOn)
	}
	if _, err := os.Stat(flag); !os.IsNotExist(err) {
		t.Errorf("toggle did not remove the flag file: %v", err)
	}

	// restore with no stored off is silent and touches nothing.
	evals = nil
	if err := runAct([]string{"input.touchpad", "restore"}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(evals) != 0 {
		t.Errorf("restore with no stored off dispatched %q", evals)
	}
}

// output.cycle steps the arrangement through ryoku-monitor, so the pin is that
// it execs `ryoku-monitor toggle`.
func TestActOutputCycleRunsMonitorToggle(t *testing.T) {
	dir := t.TempDir()
	argfile := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + argfile + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ryoku-monitor"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := stubCtl(t, func(...string) ([]byte, error) { return nil, nil })
	defer restore()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := runAct([]string{"output.cycle"}); err != nil {
		t.Fatalf("output.cycle: %v", err)
	}
	got, err := os.ReadFile(argfile)
	if err != nil {
		t.Fatalf("ryoku-monitor was not run: %v", err)
	}
	if strings.TrimSpace(string(got)) != "toggle" {
		t.Errorf("ryoku-monitor args = %q, want toggle", got)
	}
}

// stubCtl swaps the compositor call and satisfies live(), which every action
// checks before dispatching.
func stubCtl(t *testing.T, fn func(...string) ([]byte, error)) func() {
	t.Helper()
	prevCtl := ctl
	prevSig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	prevAlive := aliveCheck
	ctl = fn
	aliveCheck = func(string) bool { return true }
	os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	return func() {
		ctl = prevCtl
		aliveCheck = prevAlive
		os.Setenv("HYPRLAND_INSTANCE_SIGNATURE", prevSig)
	}
}

// The colour temperature is clamped to the range the gamma client accepts and a
// missing or unparseable argument falls back to the default, so a stray keybind
// argument can never ask hyprsunset for a value it would reject or for 0 K.
func TestNightlightTempClamps(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want int
	}{
		{nil, 4000},
		{[]string{""}, 4000},
		{[]string{"not-a-temp"}, 4000},
		{[]string{"4500"}, 4500},
		{[]string{"500"}, 1000},
		{[]string{"99999"}, 25000},
	} {
		if got := nightlightTemp(tc.args); got != tc.want {
			t.Errorf("nightlightTemp(%q) = %d, want %d", tc.args, got, tc.want)
		}
	}
}

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// Every action is pinned to the exact request it emits, because the shape is the
// contract with the compositor. niri rejects an unknown action name or a
// mistyped field with "error parsing request" rather than ignoring it, so a
// wrong shape fails loudly at runtime and these cases catch it at build time.
func TestActEmitsNiriRequests(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"window focus", []string{"window.focus", "7"},
			`{"Action":{"FocusWindow":{"id":7}}}`},
		{"window close", []string{"window.close", "7"},
			`{"Action":{"CloseWindow":{"id":7}}}`},
		{"window fullscreen", []string{"window.fullscreen", "7"},
			`{"Action":{"FullscreenWindow":{"id":7}}}`},
		{"window float", []string{"window.float", "7"},
			`{"Action":{"ToggleWindowFloating":{"id":7}}}`},
		{"window to workspace", []string{"window.moveToWorkspace", "7", "3"},
			`{"Action":{"MoveWindowToWorkspace":{"focus":true,"reference":{"Id":3},"window_id":7}}}`},
		// A numeric handle is the stable id the state frames hand out, never the
		// index, which renumbers as workspaces come and go.
		{"workspace focus by id", []string{"workspace.focus", "3"},
			`{"Action":{"FocusWorkspace":{"reference":{"Id":3}}}}`},
		{"workspace focus by name", []string{"workspace.focus", "chat"},
			`{"Action":{"FocusWorkspace":{"reference":{"Name":"chat"}}}}`},
		{"workspace to output", []string{"workspace.moveToOutput", "3", "eDP-2"},
			`{"Action":{"MoveWorkspaceToMonitor":{"output":"eDP-2","reference":{"Id":3}}}}`},
		{"session exit", []string{"session.exit"},
			`{"Action":{"Quit":{"skip_confirmation":true}}}`},
		{"output power off", []string{"output.power", "off"},
			`{"Action":{"PowerOffMonitors":{}}}`},
		{"output power on", []string{"output.power", "on"},
			`{"Action":{"PowerOnMonitors":{}}}`},
		// output.enable is niri's top-level Output request, not an Action, and
		// the action value is a unit variant so it serialises as a bare string.
		{"output enable on", []string{"output.enable", "DP-1", "on"},
			`{"Output":{"action":"On","output":"DP-1"}}`},
		{"output enable off", []string{"output.enable", "DP-1", "off"},
			`{"Output":{"action":"Off","output":"DP-1"}}`},
		{"keyboard layout", []string{"keyboard.cycleLayout"},
			`{"Action":{"SwitchLayout":{"layout":"Next"}}}`},
		{"overview toggle", []string{"overview.toggle"},
			`{"Action":{"ToggleOverview":{}}}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			restore := stubRequest(t, func(req any) (json.RawMessage, error) {
				body, err := json.Marshal(req)
				if err != nil {
					return nil, err
				}
				got = append(got, string(body))
				return nil, nil
			})
			defer restore()
			if err := runAct(tc.args); err != nil {
				t.Fatalf("runAct(%v): %v", tc.args, err)
			}
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("request mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestOutputRefreshChoosesCurrentResolution(t *testing.T) {
	outputs := []wm.Output{{
		Name: "eDP-1", Mode: "2560x1600@165.002",
		Modes: []string{"1920x1200@60", "2560x1600@60.001", "2560x1600@165.002"},
	}}
	for _, tc := range []struct {
		name string
		hz   float64
		want string
	}{
		{"exact", 165.002, "2560x1600@165.002"},
		{"nearest", 60, "2560x1600@60.001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := closestOutputMode(outputs, "eDP-1", tc.hz)
			if err != nil || got != tc.want {
				t.Fatalf("closestOutputMode = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := closestOutputMode([]wm.Output{{
		Name: "eDP-1", Mode: "3840x2160@60", Modes: []string{"2560x1600@60"},
	}}, "eDP-1", 60); err == nil || !strings.Contains(err.Error(), "current resolution") {
		t.Errorf("no-resolution-match error = %v", err)
	}
	if _, err := outputRefreshHz("sixty"); err == nil ||
		!strings.Contains(err.Error(), "positive decimal") {
		t.Errorf("malformed-hz error = %v", err)
	}
	if _, err := closestOutputMode(outputs, "DP-9", 60); err == nil ||
		!strings.Contains(err.Error(), "does not exist") {
		t.Errorf("missing-output error = %v", err)
	}
}

func TestActOutputRefreshEmitsModeRequest(t *testing.T) {
	var got string
	restore := stubRequest(t, func(req any) (json.RawMessage, error) {
		if query, ok := req.(string); ok {
			switch query {
			case "Workspaces":
				return json.RawMessage(`{"Workspaces":[]}`), nil
			case "Outputs":
				return json.RawMessage(`{"Outputs":{"eDP-1":{
					"name":"eDP-1",
					"modes":[
						{"width":2560,"height":1600,"refresh_rate":60001},
						{"width":2560,"height":1600,"refresh_rate":165002}
					],
					"current_mode":1,
					"logical":{"x":0,"y":0,"width":1707,"height":1067,"scale":1.5,"transform":"normal"}
				}}}`), nil
			}
		}
		body, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		got = string(body)
		return nil, nil
	})
	defer restore()

	if err := runAct([]string{"output.refresh", "eDP-1", "60"}); err != nil {
		t.Fatal(err)
	}
	want := `{"Output":{"action":{"Mode":{"mode":{"Specific":{"height":1600,"refresh":60.001,"width":2560}}}},"output":"eDP-1"}}`
	if got != want {
		t.Errorf("request mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestActOutputRefreshRejectsMalformedHzBeforeIPC(t *testing.T) {
	called := false
	restore := stubRequest(t, func(any) (json.RawMessage, error) {
		called = true
		return nil, nil
	})
	defer restore()
	if err := runAct([]string{"output.refresh", "eDP-1", "sixty"}); err == nil {
		t.Fatal("expected an error for malformed hz")
	}
	if called {
		t.Fatal("malformed hz reached the compositor")
	}
}

// window.summon raises an already-open window to the current workspace and
// focuses it, matched by exact title. niri focuses by id, so the title resolves
// to the newest matching window, which is pulled onto the focused workspace in
// one MoveWindowToWorkspace request.
func TestActSummonMovesNewestTitleMatch(t *testing.T) {
	const windows = `{"Windows":[
		{"id":11,"title":"Ryoku Hub","app_id":"org.quickshell","focus_timestamp":{"secs":100,"nanos":0}},
		{"id":22,"title":"Ryoku Hub","app_id":"org.quickshell","focus_timestamp":{"secs":200,"nanos":0}},
		{"id":33,"title":"Other","app_id":"org.quickshell","focus_timestamp":{"secs":300,"nanos":0}}
	]}`
	const workspaces = `{"Workspaces":[
		{"id":3,"idx":1,"output":"eDP-1","is_active":true,"is_focused":true},
		{"id":4,"idx":2,"output":"eDP-1","is_active":false,"is_focused":false}
	]}`

	var got string
	restore := stubRequest(t, func(req any) (json.RawMessage, error) {
		if s, ok := req.(string); ok {
			switch s {
			case "Windows":
				return json.RawMessage(windows), nil
			case "Workspaces":
				return json.RawMessage(workspaces), nil
			}
			return nil, fmt.Errorf("unexpected query %q", s)
		}
		body, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		got = string(body)
		return nil, nil
	})
	defer restore()

	if err := runAct([]string{"window.summon", "Ryoku Hub"}); err != nil {
		t.Fatalf("summon: %v", err)
	}
	want := `{"Action":{"MoveWindowToWorkspace":{"focus":true,"reference":{"Id":3},"window_id":22}}}`
	if got != want {
		t.Errorf("request mismatch\n got: %q\nwant: %q", got, want)
	}

	// A title with no window is an error and emits no action, so the keybind
	// falls through to launching the app.
	got = ""
	if err := runAct([]string{"window.summon", "Nonexistent"}); err == nil {
		t.Error("summon of an absent title: expected an error")
	}
	if got != "" {
		t.Errorf("summon of an absent title emitted %q", got)
	}
}

// niri has no relative workspace reference, so a cycle is one step per unit and
// the direction has to come out as the right action.
func TestActCyclesOneStepPerUnit(t *testing.T) {
	for _, tc := range []struct {
		delta string
		want  []string
	}{
		{"1", []string{`{"Action":{"FocusWorkspaceDown":{}}}`}},
		{"-2", []string{
			`{"Action":{"FocusWorkspaceUp":{}}}`,
			`{"Action":{"FocusWorkspaceUp":{}}}`,
		}},
	} {
		var got []string
		restore := stubRequest(t, func(req any) (json.RawMessage, error) {
			body, _ := json.Marshal(req)
			got = append(got, string(body))
			return nil, nil
		})
		err := runAct([]string{"workspace.cycle", tc.delta})
		restore()
		if err != nil {
			t.Fatalf("cycle %s: %v", tc.delta, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("cycle %s: got %d requests, want %d: %q", tc.delta, len(got), len(tc.want), got)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("cycle %s step %d: got %s want %s", tc.delta, i, got[i], tc.want[i])
			}
		}
	}
}

// A missing argument must name the value, so a bad keybind is diagnosable, and
// must never reach the compositor: a niri window action with a null id applies
// to the focused window, which is not what the caller asked for.
func TestActRejectsMissingArgs(t *testing.T) {
	for _, args := range [][]string{
		{"window.focus"},
		{"window.place", "7", "0", "0", "700"},
		{"window.moveToWorkspace", "7"},
		{"workspace.focus"},
		{"workspace.moveToOutput", "3"},
		{"output.power"},
		{"output.refresh"},
		{"output.refresh", "eDP-1"},
		{"app.focus"},
		{"window.summon"},
	} {
		called := false
		restore := stubRequest(t, func(any) (json.RawMessage, error) {
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

// A window id that is not a number would marshal as a string and niri would
// reject the request, so it is refused here where the message can say why.
func TestActRejectsNonNumericWindowID(t *testing.T) {
	called := false
	restore := stubRequest(t, func(any) (json.RawMessage, error) {
		called = true
		return nil, nil
	})
	defer restore()
	if err := runAct([]string{"window.close", "0xdeadbeef"}); err == nil {
		t.Fatal("expected an error for a non-numeric window id")
	}
	if called {
		t.Fatal("reached the compositor with a non-numeric window id")
	}
}

// niri powers every output together, so a request to blank one output alone is
// refused rather than blanking the others too.
func TestActRefusesPerOutputPower(t *testing.T) {
	called := false
	restore := stubRequest(t, func(any) (json.RawMessage, error) {
		called = true
		return nil, nil
	})
	defer restore()
	if err := runAct([]string{"output.power", "off", "eDP-2"}); err == nil {
		t.Fatal("expected an error for a per-output power request")
	}
	if called {
		t.Fatal("powered outputs despite a target niri cannot honour")
	}
}

// An action this compositor cannot perform names the capability, so a caller
// that skipped the gate is told which one to check instead of getting a silent
// no-op or an unknown-action error.
func TestActNamesTheMissingCapability(t *testing.T) {
	restore := stubRequest(t, func(any) (json.RawMessage, error) { return nil, nil })
	defer restore()
	for _, args := range [][]string{
		{"submap.enter", "resize"},
		{"workspace.toggleSpecial", "sharebar"},
		{"cursor.set", "Bibata", "24"},
		{"decoration.screenShader", "halftone"},
		{"config.reload"},
		{"decoration.gameMode", "on"},
	} {
		err := runAct(args)
		if err == nil {
			t.Fatalf("runAct(%v): expected an unsupported error", args)
		}
		capability := wm.Action(args[0]).Capability()
		if capability == "" {
			t.Fatalf("runAct(%v): action has no capability to name", args)
		}
		if !strings.Contains(err.Error(), string(capability)) {
			t.Errorf("runAct(%v): error %q does not name %q", args, err, capability)
		}
	}
}

func TestActRejectsUnknownAction(t *testing.T) {
	restore := stubRequest(t, func(any) (json.RawMessage, error) { return nil, nil })
	defer restore()
	if err := runAct([]string{"window.teleport"}); err == nil {
		t.Fatal("expected an error for an unknown action")
	}
}

// output.cycle steps the arrangement over IPC: from the reset position it turns
// the panel on and the external off (internal only), turning the target set on
// before the other off so no step flashes every screen dark. The request shapes
// and the persisted position are the contract with niri and the next step.
func TestActCyclesOutputsOverIPC(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var got []string
	restore := stubRequest(t, func(req any) (json.RawMessage, error) {
		body, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		got = append(got, string(body))
		if string(body) == `"Outputs"` {
			return json.RawMessage(`{"Outputs":{"eDP-1":{"name":"eDP-1"},"DP-1":{"name":"DP-1"}}}`), nil
		}
		return nil, nil
	})
	defer restore()

	if err := runAct([]string{"output.cycle"}); err != nil {
		t.Fatalf("output.cycle: %v", err)
	}
	want := []string{
		`"Outputs"`,
		`{"Output":{"action":"On","output":"eDP-1"}}`,
		`{"Output":{"action":"Off","output":"DP-1"}}`,
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("cycle requests\n got: %q\nwant: %q", got, want)
	}
	if pos, _ := os.ReadFile(outputCyclePath()); strings.TrimSpace(string(pos)) != "1" {
		t.Errorf("stored cycle position = %q, want 1", pos)
	}
}

// The touchpad lock has no runtime input IPC on niri, so it flips a state file
// and re-runs the apply path (writeInput emits `off` while the file exists).
// status reads the file, never the compositor, and toggle flips it.
func TestActTouchpadTracksStateFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	prevNotify := touchpadNotify
	touchpadNotify = func(string, string) {}
	defer func() { touchpadNotify = prevNotify }()

	restore := stubRequest(t, func(any) (json.RawMessage, error) { return nil, nil })
	defer restore()

	if err := runAct([]string{"input.touchpad", "off"}); err != nil {
		t.Fatalf("off: %v", err)
	}
	if !touchpadDisabled() {
		t.Error("off did not record the intent")
	}
	if _, err := os.Stat(filepath.Join(niriConfigDir(), "settings.kdl")); err != nil {
		t.Errorf("off did not re-render settings.kdl: %v", err)
	}

	if status := touchpadStatus(t); status != "off" {
		t.Errorf("status = %q, want off", status)
	}

	if err := runAct([]string{"input.touchpad", "toggle"}); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if touchpadDisabled() {
		t.Error("toggle did not clear the intent")
	}
	if status := touchpadStatus(t); status != "on" {
		t.Errorf("status after toggle = %q, want on", status)
	}
}

// touchpadStatus runs input.touchpad status and returns what it printed.
func touchpadStatus(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut := stdout
	stdout = bufio.NewWriter(&buf)
	defer func() { stdout = prevOut }()
	if err := runAct([]string{"input.touchpad", "status"}); err != nil {
		t.Fatalf("status: %v", err)
	}
	stdout.Flush()
	return strings.TrimSpace(buf.String())
}

// stubRequest swaps the compositor call and satisfies live(), which every action
// checks before dispatching.
func stubRequest(t *testing.T, fn func(any) (json.RawMessage, error)) func() {
	t.Helper()
	prevRequest := request
	prevAlive := aliveCheck
	prevSocket := os.Getenv("NIRI_SOCKET")
	request = fn
	aliveCheck = func(string) bool { return true }
	os.Setenv("NIRI_SOCKET", "/nonexistent/test.sock")
	return func() {
		request = prevRequest
		aliveCheck = prevAlive
		os.Setenv("NIRI_SOCKET", prevSocket)
	}
}

// The colour temperature is clamped to the range the gamma client accepts and a
// missing or unparseable argument falls back to the default, so a stray keybind
// argument can never ask wlsunset for a value it would reject or for 0 K.
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

// The border act records the live palette and regenerates settings.kdl so niri
// re-reads it, the niri twin of Hyprland's eval push. The colours are normalised
// and land in the config's border block.
func TestActBorderPaletteRewritesConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	restore := stubRequest(t, func(any) (json.RawMessage, error) { return nil, nil })
	defer restore()

	if err := runAct([]string{"decoration.borderColors", "#112233", "#445566"}); err != nil {
		t.Fatal(err)
	}
	pb, err := os.ReadFile(borderPalettePath())
	if err != nil {
		t.Fatalf("palette file not written: %v", err)
	}
	if !strings.Contains(string(pb), `"active":"#112233"`) || !strings.Contains(string(pb), `"inactive":"#445566"`) {
		t.Errorf("palette file missing the colours: %s", pb)
	}
	kb, err := os.ReadFile(filepath.Join(niriConfigDir(), "settings.kdl"))
	if err != nil {
		t.Fatalf("settings.kdl not written: %v", err)
	}
	if !strings.Contains(string(kb), `active-color "#112233"`) || !strings.Contains(string(kb), `inactive-color "#445566"`) {
		t.Errorf("settings.kdl border not recoloured:\n%s", kb)
	}
}

// A pinned border makes the act a no-op: the palette file is never written, so a
// wallpaper change leaves the chosen colour alone.
func TestActBorderPaletteNoOpWhenFixed(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ryoku := filepath.Join(cfg, "ryoku")
	if err := os.MkdirAll(ryoku, 0o755); err != nil {
		t.Fatal(err)
	}
	store := `{"desktop":{"appearance":{"borderFollowsPalette":false}}}`
	if err := os.WriteFile(filepath.Join(ryoku, "desktop.json"), []byte(store), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := stubRequest(t, func(any) (json.RawMessage, error) { return nil, nil })
	defer restore()
	if err := runAct([]string{"decoration.borderColors", "#112233", "#445566"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(borderPalettePath()); !os.IsNotExist(err) {
		t.Error("a fixed border must not write the palette file")
	}
}

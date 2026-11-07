package main

import "testing"

// route = the single source of truth for which panel a keybind toggles; a wrong
// entry silently opens the wrong surface, so pin every command.
func TestRoute(t *testing.T) {
	cases := []struct {
		cmd, id string
	}{
		{"launcher", "launcher"},
		{"tools", "tools"},
		{"overview", "overview"},
		{"visualizer", "visualizer"},
		{"visualizer-overlay", "visualizer-overlay"},
		{"menu app-launcher", "launcher"},
		{"menu quick-settings", "sidebar-left"},
		{"menu theme", "theme"},
		{"menu wallpaper", "wallpaper"},
		{"menu screenshot", "screenshot"},
		{"install", "ask#tools/install"},
		{"compress", "ask#tools/compress"},
		{"ask", "ask"},
		{"ask chat", "ask#chat"},
		{"ask tools", "ask#tools"},
		{"bar-toggle", "barToggle"},
		{"visualizer-place", "visualizer-place"},
		{"quicksettings", "sidebar-left"},
		{"wallpaper-menu", "wallpaper"},
		{"clipboard", "clipboard"},
		{"settings", "settings"},
		{"cheatsheet", "cheatsheet"},
		{"screenshot", "screenshot"},
	}
	for _, c := range cases {
		id, ok := route(c.cmd)
		if !ok {
			t.Fatalf("route(%q) not ok", c.cmd)
		}
		if id != c.id {
			t.Fatalf("route(%q) = %q, want %q", c.cmd, id, c.id)
		}
	}
	for _, cmd := range []string{
		"link", "inbox", "mixer", "calendar", "battery",
		"toolkit", "utilities", "system", "workspaces", "sysinfo", "peek", "hide",
		"voice", "lock", "wallpaper", "wallpaper-switcher", "reload", "status",
		"ping", "quit", "bogus", "", "power", "menu system", "menu recording",
		"menu clipboard", "menu stash", "stash",
		"menu", "menu bogus", "menu clock extra", "bar", "bar left", "bar left toggle",
	} {
		if _, ok := route(cmd); ok {
			t.Fatalf("route(%q) should not be a single IPC call", cmd)
		}
	}
}

// stubShellIpc replaces the shellIpc seam with one that records each emitted call
// (fn plus its non-empty args, matching the old pill socket line) on calls and
// acks "ok", so a dispatch test asserts routing without a live Quickshell. A nil
// channel drains silently for tests that only need the call to succeed.
func stubShellIpc(t *testing.T, calls chan<- string) {
	t.Helper()
	prev := shellIpc
	shellIpc = func(fn string, args ...string) string {
		if calls != nil {
			line := fn
			for _, a := range args {
				if a != "" {
					line += " " + a
				}
			}
			calls <- line
		}
		return "ok"
	}
	t.Cleanup(func() { shellIpc = prev })
}

// dispatch routes every frame surface and the menu close-all onto the single
// shell's IpcHandler; a wrong entry opens the wrong surface.
func TestDispatchFrameSurface(t *testing.T) {
	calls := make(chan string, len(frameBarMenuIDs)+2)
	stubShellIpc(t, calls)

	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1"}
	for id := range frameBarMenuIDs {
		if got := d.dispatch("menu " + id); got != "ok" {
			t.Errorf("dispatch(menu %s) = %q, want ok", id, got)
			continue
		}
		if got := <-calls; got != "openSurface DP-1 "+id {
			t.Errorf("shell IPC = %q, want %q", got, "openSurface DP-1 "+id)
		}
	}
	// menu close maps to the reference close-all on its own shell function.
	if got := d.dispatch("menu close"); got != "ok" {
		t.Errorf("dispatch(menu close) = %q, want ok", got)
	} else if got := <-calls; got != "closeAllMenus DP-1" {
		t.Errorf("menu close shell IPC = %q, want %q", got, "closeAllMenus DP-1")
	}
	for _, command := range []string{"bar", "bar left", "bar sideways toggle", "bar left sideways", "menu", "menu bogus", "menu clock extra"} {
		if got := d.dispatch(command); got == "ok" {
			t.Errorf("dispatch(%q) = ok, want rejection", command)
		}
	}
}

// Malformed bar/menu input is rejected before any shell IPC is emitted, and the
// surface/voice verbs each emit their exact openSurface call on the shell target.
func TestDispatchSurfaceRouting(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no voxtype on PATH: voice falls to the "off" note
	calls := make(chan string, 4)
	stubShellIpc(t, calls)

	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1"}
	for _, command := range []string{"bar", "bar left", "bar sideways toggle", "menu", "menu bogus", "menu stash", "ask web", "ask chat extra"} {
		if got := d.dispatch(command); got == "ok" {
			t.Errorf("dispatch(%q) = ok, want rejection", command)
		}
	}
	select {
	case got := <-calls:
		t.Fatalf("rejected input emitted shell IPC %q", got)
	default:
	}

	if got := d.dispatch("menu screenshot"); got != "ok" {
		t.Fatalf("dispatch(menu screenshot) = %q, want ok", got)
	}
	if got := <-calls; got != "openSurface DP-1 screenshot" {
		t.Fatalf("screenshot shell IPC = %q", got)
	}
	if got := d.dispatch("menu quick-settings"); got != "ok" {
		t.Fatalf("dispatch(menu quick-settings) = %q, want ok", got)
	}
	if got := <-calls; got != "openSurface DP-1 sidebar-left" {
		t.Fatalf("quick-settings alias shell IPC = %q", got)
	}
	if got := d.dispatch("voice"); got != "ok" {
		t.Fatalf("dispatch(voice) = %q, want ok", got)
	}
	if got := <-calls; got != "openSurface DP-1 voice-off" {
		t.Fatalf("voice shell IPC = %q", got)
	}
}

// Ask and file-picker verbs land on the requested Ask mode or tool.
func TestDispatchAskRoutes(t *testing.T) {
	calls := make(chan string, 4)
	stubShellIpc(t, calls)
	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1"}
	for verb, want := range map[string]string{
		"ask chat":  "openSurface DP-1 ask#chat",
		"ask tools": "openSurface DP-1 ask#tools",
		"install":   "openSurface DP-1 ask#tools/install",
		"compress":  "openSurface DP-1 ask#tools/compress",
	} {
		if got := d.dispatch(verb); got != "ok" {
			t.Fatalf("dispatch(%q) = %q, want ok", verb, got)
		}
		if got := <-calls; got != want {
			t.Fatalf("%s shell IPC = %q, want %q", verb, got, want)
		}
	}
}

// Every shell surface is reachable as a bare kebab verb, each emitting the
// exact openSurface id the shell routes to that surface. A niri keybind spawns
// `ryoku-shell <verb>`, so a missing or misspelled verb here is a dead keybind.
func TestDispatchSurfaceVerbs(t *testing.T) {
	want := map[string]string{
		"bar-toggle":         "openSurface DP-1 barToggle",
		"launcher":           "openSurface DP-1 launcher",
		"overview":           "openSurface DP-1 overview",
		"visualizer":         "openSurface DP-1 visualizer",
		"visualizer-overlay": "openSurface DP-1 visualizer-overlay",
		"visualizer-place":   "openSurface DP-1 visualizer-place",
		"quicksettings":      "openSurface DP-1 sidebar-left",
		"wallpaper-menu":     "openSurface DP-1 wallpaper",
		"clipboard":          "openSurface DP-1 clipboard",
		"screenshot":         "openSurface DP-1 screenshot",
		"ask":                "openSurface DP-1 ask",
		"compress":           "openSurface DP-1 ask#tools/compress",
		"install":            "openSurface DP-1 ask#tools/install",
	}
	calls := make(chan string, 1)
	stubShellIpc(t, calls)
	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1"}
	for verb, expect := range want {
		if got := d.dispatch(verb); got != "ok" {
			t.Fatalf("dispatch(%q) = %q, want ok", verb, got)
		}
		if got := <-calls; got != expect {
			t.Fatalf("dispatch(%q) shell IPC = %q, want %q", verb, got, expect)
		}
	}
}

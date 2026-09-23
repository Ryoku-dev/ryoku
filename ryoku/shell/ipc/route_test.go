package main

import "testing"

// route = the single source of truth for which panel a keybind toggles; a wrong
// entry silently opens the wrong surface, so pin every command.
func TestRoute(t *testing.T) {
	cases := []struct {
		cmd, config, target, fn string
	}{
		{"launcher", "shell", "shell", "openSurface"},
		{"overview", "shell", "shell", "openSurface"},
		{"visualizer", "shell", "shell", "openSurface"},
		{"visualizer-overlay", "shell", "shell", "openSurface"},
		{"menu app-launcher", "shell", "shell", "openSurface"},
		{"menu quick-settings", "shell", "shell", "openSurface"},
		{"menu theme", "shell", "shell", "openSurface"},
		{"menu wallpaper", "shell", "shell", "openSurface"},
		{"menu screenshot", "shell", "shell", "openSurface"},
		{"menu stash", "shell", "shell", "openSurface"},
		{"install", "shell", "shell", "openSurface"},
		{"compress", "shell", "shell", "openSurface"},
		{"bar-toggle", "shell", "shell", "openSurface"},
		{"visualizer-place", "shell", "shell", "openSurface"},
		{"quicksettings", "shell", "shell", "openSurface"},
		{"wallpaper-menu", "shell", "shell", "openSurface"},
		{"clipboard", "shell", "shell", "openSurface"},
		{"stash", "shell", "shell", "openSurface"},
		{"screenshot", "shell", "shell", "openSurface"},
	}
	for _, c := range cases {
		config, target, fn, ok := route(c.cmd)
		if !ok {
			t.Fatalf("route(%q) not ok", c.cmd)
		}
		if config != c.config || target != c.target || fn != c.fn {
			t.Fatalf("route(%q) = (%s,%s,%s), want (%s,%s,%s)", c.cmd, config, target, fn, c.config, c.target, c.fn)
		}
	}
	for _, cmd := range []string{
		"link", "inbox", "mixer", "calendar", "battery",
		"toolkit", "utilities", "system", "workspaces", "sysinfo", "peek", "hide",
		"voice", "lock", "wallpaper", "wallpaper-switcher", "reload", "status",
		"ping", "quit", "bogus", "", "power", "menu system", "menu recording",
		"menu clipboard",
	} {
		if _, _, _, ok := route(cmd); ok {
			t.Fatalf("route(%q) should not be a single IPC call", cmd)
		}
	}
	for _, cmd := range []string{"menu", "menu bogus", "menu clock extra", "bar", "bar left", "bar left toggle"} {
		if _, _, _, ok := route(cmd); ok {
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

// dispatch routes every frame surface, the menu close-all, and the bar reveal
// verbs onto the single shell's IpcHandler; a wrong entry opens the wrong surface.
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
	// menu close and bar edge toggles map to the reference close-all and the
	// per-edge bar operations, each on its own shell function.
	if got := d.dispatch("menu close"); got != "ok" {
		t.Errorf("dispatch(menu close) = %q, want ok", got)
	} else if got := <-calls; got != "closeAllMenus DP-1" {
		t.Errorf("menu close shell IPC = %q, want %q", got, "closeAllMenus DP-1")
	}
	if got := d.dispatch("bar left toggle"); got != "ok" {
		t.Errorf("dispatch(bar left toggle) = %q, want ok", got)
	} else if got := <-calls; got != "setBar DP-1 left toggle" {
		t.Errorf("bar toggle shell IPC = %q, want %q", got, "setBar DP-1 left toggle")
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
	calls := make(chan string, 3)
	stubShellIpc(t, calls)

	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1"}
	for _, command := range []string{"bar", "bar left", "bar sideways toggle", "menu", "menu bogus"} {
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
	if got := d.dispatch("menu stash"); got != "ok" {
		t.Fatalf("dispatch(menu stash) = %q, want ok", got)
	}
	if got := <-calls; got != "openSurface DP-1 stash" {
		t.Fatalf("stash shell IPC = %q", got)
	}
	if got := d.dispatch("voice"); got != "ok" {
		t.Fatalf("dispatch(voice) = %q, want ok", got)
	}
	if got := <-calls; got != "openSurface DP-1 voice-off" {
		t.Fatalf("voice shell IPC = %q", got)
	}
}

// The file-picker tool verbs open the stash sidebar straight onto their picker,
// so install-app.desktop and compress-video.desktop reach the right surface.
func TestDispatchFilePickerTools(t *testing.T) {
	calls := make(chan string, 2)
	stubShellIpc(t, calls)
	d := &daemon{sup: map[string]bool{"shell": true}, activeMon: "DP-1"}
	for verb, want := range map[string]string{
		"install":  "openSurface DP-1 stash#install",
		"compress": "openSurface DP-1 stash#compress",
	} {
		if got := d.dispatch(verb); got != "ok" {
			t.Fatalf("dispatch(%q) = %q, want ok", verb, got)
		}
		if got := <-calls; got != want {
			t.Fatalf("%s shell IPC = %q, want %q", verb, got, want)
		}
	}
}

// Every one of the 13 shell surfaces is reachable as a bare kebab verb equal to
// its CustomShortcut id, each emitting the exact openSurface id the shell routes
// to that surface. A niri keybind spawns `ryoku-shell <verb>` for all of these,
// so a missing or misspelled verb here is a dead keybind on niri.
func TestDispatchSurfaceVerbs(t *testing.T) {
	want := map[string]string{
		"bar-toggle":         "openSurface DP-1 barToggle",
		"launcher":           "openSurface DP-1 launcher",
		"overview":           "openSurface DP-1 overview",
		"visualizer":         "openSurface DP-1 visualizer",
		"visualizer-overlay": "openSurface DP-1 visualizer-overlay",
		"visualizer-place":   "openSurface DP-1 visualizer-place",
		"quicksettings":      "openSurface DP-1 quick-settings",
		"wallpaper-menu":     "openSurface DP-1 wallpaper",
		"clipboard":          "openSurface DP-1 clipboard",
		"stash":              "openSurface DP-1 stash",
		"screenshot":         "openSurface DP-1 quick-settings#capture",
		"compress":           "openSurface DP-1 stash#compress",
		"install":            "openSurface DP-1 stash#install",
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

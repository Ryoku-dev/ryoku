package main

import (
	"os"
	"strings"
	"testing"
)

// The shipped baseline is the first compose layer and the one file the
// package owns outright. Three mistakes hurt here: a section whose plugin is
// not in core.plugins (a bind that silently never fires, which is how the
// development rig shipped for weeks), a plugin missing that the store's rows
// or the act verbs dial (caps that lie), and leftovers from that rig riding
// along into users' sessions. This reads the repo's payload, the exact bytes
// the package installs, and pins all three.
func TestShippedBaselineIsHonest(t *testing.T) {
	raw, err := os.ReadFile("../../wayfire/wayfire.ini")
	if err != nil {
		t.Fatalf("read the shipped baseline: %v", err)
	}
	doc := parseIni(raw)
	if len(doc.sections) == 0 {
		t.Fatal("the shipped baseline parses to nothing")
	}

	loaded := map[string]bool{}
	plugins, _ := doc.get("core", "plugins")
	for _, p := range strings.Fields(plugins) {
		loaded[p] = true
	}
	if len(loaded) == 0 {
		t.Fatal("core.plugins lists no plugins")
	}

	for _, sec := range doc.sections {
		if sec.name == "core" || strings.HasPrefix(sec.name, "output:") {
			continue
		}
		if !loaded[sec.name] {
			t.Errorf("section [%s] has no plugin in core.plugins", sec.name)
		}
	}

	// What the rest of the desktop dials: the store's sections and gestures,
	// the caps, and the act verbs each need their plugin already loaded.
	for _, want := range []string{
		"animate", "autostart", "command", "decoration", "depthdeck",
		"fast-switcher", "ipc", "ipc-rules", "vswipe", "window-rules", "wm-actions",
	} {
		if !loaded[want] {
			t.Errorf("core.plugins is missing %q", want)
		}
	}

	// The rig's leftovers stay out: each of these sections shipped while its
	// plugin was off, so their binds did nothing, and the depth deck's knobs
	// belong to the store rather than to two copies of the same defaults.
	for _, dead := range []string{
		"alpha", "cube", "depthdeck", "expo", "fisheye",
		"invert", "oswitch", "switcher", "wayfire-shell", "wrot",
	} {
		for _, sec := range doc.sections {
			if sec.name == dead {
				t.Errorf("section [%s] belongs to a plugin this baseline does not run", dead)
			}
		}
	}

	// The catalogue owns its binds now: apply writes them beside this layer,
	// so a command row here would be a second copy waiting to drift.
	if term, _ := doc.get("command", "command_app_terminal"); term != "" {
		t.Errorf("command_app_terminal = %q; the baseline must not carry a second copy of the catalogue", term)
	}
	if cmd := defaultBinds()["app.terminal"].cmd; cmd != "ryoku-app terminal" {
		t.Errorf("app.terminal = %q, want the ryoku app verb", cmd)
	}

	// The login bootstrap: nothing else brings the session up, so a wayfire
	// login without this row is a bare compositor with no shell, no portals
	// restart and no depthdeck, and every other row here would be config for
	// a desktop that never starts.
	session, _ := doc.get("autostart", "0_session")
	for _, want := range []string{
		"dbus-update-activation-environment --systemd --all",
		"ryoku-power-cutover session-start-logged",
		"xdg-desktop-portal-gnome.service",
	} {
		if !strings.Contains(session, want) {
			t.Errorf("autostart.0_session is missing %q", want)
		}
	}
	if wf, _ := doc.get("autostart", "autostart_wf_shell"); wf != "false" {
		t.Errorf("autostart_wf_shell = %q; the session's shell is Quickshell, not wf-panel", wf)
	}
}

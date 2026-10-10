package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// The caps list is documented as "kept in step with that package's depends".
// A satellite added to the PKGBUILD but not to this list could never be
// reclaimed on a packaged box; the variant package itself, missing from the
// list, owns every satellite and blocks all their removal. Both drifts fail
// here, against the real PKGBUILD.
func TestReclaimListMatchesVariantDepends(t *testing.T) {
	raw, err := os.ReadFile("../../../release/packages/ryoku-desktop-wayfire/PKGBUILD")
	if err != nil {
		t.Skip("no PKGBUILD beside the test")
	}
	m := regexp.MustCompile(`(?ms)^depends=\((.*?)\)`).FindSubmatch(raw)
	if m == nil {
		t.Fatal("no depends array in the PKGBUILD")
	}
	want := map[string]bool{}
	body := regexp.MustCompile(`(?m)^\s*#.*$`).ReplaceAll(m[1], nil)
	for _, line := range strings.Fields(string(body)) {
		pkg := strings.Trim(line, "'\"")
		if i := strings.Index(pkg, "="); i >= 0 {
			pkg = pkg[:i] // a version bound: "wayfire=0.11" names wayfire
		}
		if pkg == "" || pkg == "ryoku-desktop" {
			continue // the umbrella is shared with the incoming compositor
		}
		want[pkg] = true
	}
	have := map[string]bool{}
	for _, p := range compositorPackages {
		have[p] = true
	}
	if !have["ryoku-desktop-wayfire"] {
		t.Error("the variant package must be in its own reclaim list")
	}
	for p := range want {
		if !have[p] {
			t.Errorf("PKGBUILD depends on %s but the reclaim list omits it", p)
		}
	}
}

// A capability the manifest claims but wm.All() omits never reaches a QML
// consumer: the caps frame carries one boolean per All() entry, so the shell
// reads the gate as false and the affordance silently disappears.
func TestManifestIsDeliverable(t *testing.T) {
	all := map[wm.Capability]bool{}
	for _, c := range wm.All() {
		all[c] = true
	}
	for _, c := range capsManifest {
		if !all[c] {
			t.Errorf("manifest claims %q but wm.All() omits it, so it never rides the caps frame", c)
		}
	}
}

// wayfire watches its own config file with inotify, so a configReload claim
// would offer a reload button with nothing behind it. The reload path is the
// write itself.
func TestDeniesConfigReload(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapConfigReload) {
		t.Error("wayfire must not claim configReload: it reloads its config itself")
	}
}

// focusFollowsMouse has no option to read or write in wayfire's input
// section, and a gameMode strip could never restore the user's own colours
// through the runtime-only set-config-options, so the capability behind both
// actions stays denied.
func TestDeniesLiveConfigEval(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapLiveConfigEval) {
		t.Error("wayfire must not claim liveConfigEval: neither action behind it is performable")
	}
	if wm.ActionFocusFollowsMouse.Capability() != wm.CapLiveConfigEval ||
		wm.ActionGameMode.Capability() != wm.CapLiveConfigEval {
		t.Fatal("the shared table changed: these tests pin the pair to liveConfigEval")
	}
}

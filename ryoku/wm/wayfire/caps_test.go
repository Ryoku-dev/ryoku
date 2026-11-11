package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	wm "ryoku-wm"
)

func isolatedWayfirePluginDir(t *testing.T) string {
	t.Helper()
	original := wayfirePluginDirs
	dir := t.TempDir()
	wayfirePluginDirs = []string{dir}
	t.Cleanup(func() { wayfirePluginDirs = original })
	return dir
}

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

func TestClaimsWorkspaceFullscreenState(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapWorkspaceFullscreenState) {
		t.Error("Wayfire workspace frames carry fullscreen views and must stay authoritative")
	}
}

func TestOutputCapabilitiesAndPortalContracts(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	for _, want := range []wm.Capability{wm.CapOutputPower, wm.CapOutputMirror, wm.CapOutputHdr} {
		if !c.Has(want) {
			t.Errorf("Wayfire output stack must deliver %s", want)
		}
	}
	have := map[string]bool{}
	for _, pkg := range compositorPackages {
		have[pkg] = true
	}
	for _, pkg := range []string{"xdg-desktop-portal-wlr", "wlopm", "wlr-randr"} {
		if !have[pkg] {
			t.Errorf("reclaim list omits %s", pkg)
		}
	}
	if have["xdg-desktop-portal-gnome"] {
		t.Error("the Mutter-backed GNOME portal cannot screencast a Wayfire session")
	}
}

func TestDeckCapabilityNeedsItsModule(t *testing.T) {
	dir := isolatedWayfirePluginDir(t)
	module := filepath.Join(dir, "libdepthdeck.so")
	if err := os.WriteFile(module, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !wayfirePluginAvailable("depthdeck") {
		t.Fatal("module in injected plugin directory was not discovered")
	}
	c := wm.Caps{Supports: availableCaps()}
	if !c.Has(wm.CapDeckLayout) {
		t.Error("present depthdeck module did not enable deckLayout")
	}
}

func TestSpecialWorkspaceCapabilityNeedsLoadedWsets(t *testing.T) {
	isolatedWayfirePluginDir(t)
	t.Run("loaded", func(t *testing.T) {
		stubAct(t, map[string]string{
			"list-methods": `{"methods":["wsets/set-output-wset","wsets/send-view-to-wset"]}`,
		})
		if !(wm.Caps{Supports: availableCaps()}).Has(wm.CapSpecialWorkspace) {
			t.Error("loaded wsets methods must expose the scratch workspace")
		}
	})
	t.Run("missing", func(t *testing.T) {
		stubAct(t, map[string]string{"list-methods": `{"methods":[]}`})
		if (wm.Caps{Supports: availableCaps()}).Has(wm.CapSpecialWorkspace) {
			t.Error("a live session without wsets must not expose a dead scratch workspace")
		}
	})
}

// Deploy replaces the config directory, so Wayfire must expose an imperative
// reload that reopens the configured path instead of trusting the old inotify
// inode.
func TestClaimsConfigReload(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapConfigReload) {
		t.Error("wayfire must claim configReload: deploy swaps the watched config directory")
	}
}

// Wayfire has no runtime settings for focus-follows-mouse, reversible game
// decoration stripping, or cursor visibility, so their shared capability stays
// denied.
func TestDeniesLiveConfigEval(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapLiveConfigEval) {
		t.Error("wayfire must not claim liveConfigEval: its actions are not performable")
	}
	if wm.ActionFocusFollowsMouse.Capability() != wm.CapLiveConfigEval ||
		wm.ActionGameMode.Capability() != wm.CapLiveConfigEval ||
		wm.ActionCursorVisibility.Capability() != wm.CapLiveConfigEval {
		t.Fatal("the shared table changed: these actions need liveConfigEval")
	}
}

// The workspace animation has no query IPC, so the live-draw overlay must
// take its fallback path here.
func TestDeniesWorkspaceSlideStyle(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapWorkspaceSlideStyle) {
		t.Error("wayfire cannot report its workspace animation; the manifest must not claim it")
	}
}

// Without a nest verb nothing may be promised a hidden session here.
func TestDeniesNestedSession(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapNestedSession) {
		t.Error("wayfire has no nest verb; the manifest must not claim nested sessions")
	}
}

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
	raw, err := os.ReadFile("../../../release/packages/ryoku-desktop-niri/PKGBUILD")
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
			pkg = pkg[:i] // a version bound: "niri=26.04" names niri
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
	if !have["ryoku-desktop-niri"] {
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

// niri recreates its outputs under a long-lived capturing layer surface and
// the Qt client dies resolving the vanished screen, so the manifest must keep
// denying this or the launcher frost comes back and takes the shell with it.
func TestDeniesPersistentScreenCapture(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapPersistentScreenCapture) {
		t.Error("niri must not claim persistent screen capture")
	}
}

// The Stage Editor's canvas holds the keyboard exclusively only where this is
// claimed, so Escape and the arrows reach it from the moment it opens. On
// demand, niri hands the keyboard over only on a click and takes it back on
// every click in the editor's toolbar.
func TestClaimsKeyboardGrabSharesPointer(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapKeyboardGrabSharesPointer) {
		t.Error("niri routes the pointer by input region under a keyboard grab; the manifest must say so")
	}
}

func TestServesNoForeignShellAPIs(t *testing.T) {
	if len(foreignAPIs) != 0 {
		t.Fatalf("foreign APIs = %v, want none", foreignAPIs)
	}
}

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
	raw, err := os.ReadFile("../../../release/packages/ryoku-desktop-hyprland/PKGBUILD")
	if err != nil {
		t.Skip("no PKGBUILD beside the test")
	}
	depends := regexp.MustCompile(`(?ms)^depends=\((.*?)\)`)
	m := depends.FindSubmatch(raw)
	if m == nil {
		t.Fatal("no depends array in the PKGBUILD")
	}
	want := map[string]bool{}
	body := regexp.MustCompile(`(?m)^\s*#.*$`).ReplaceAll(m[1], nil)
	for _, line := range strings.Fields(string(body)) {
		pkg := strings.Trim(line, "'\"")
		if i := strings.Index(pkg, "="); i >= 0 {
			pkg = pkg[:i] // a version bound: "hyprland=0.55" names hyprland
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
	if !have["ryoku-desktop-hyprland"] {
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
// reads the gate as false and the affordance silently disappears on Hyprland.
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

// The launcher's frost backdrop is gated on this capability, so Hyprland must
// claim it or every Hyprland user loses the blurred launcher.
func TestClaimsPersistentScreenCapture(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapPersistentScreenCapture) {
		t.Error("Hyprland hosts long-lived capturing layer surfaces; the manifest must say so")
	}
}

// The Stage Editor's canvas holds the keyboard exclusively wherever this is
// claimed. Hyprland then hands it every click as well, and the editor's toolbar
// and catalogue stop answering, so the manifest must keep denying it.
func TestDeniesKeyboardGrabSharesPointer(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapKeyboardGrabSharesPointer) {
		t.Error("Hyprland gives an exclusive keyboard grab the pointer too; the manifest must not claim it shares")
	}
}

func TestServesForeignShellAPIs(t *testing.T) {
	want := []wm.ForeignAPI{
		wm.ForeignAPIHyprctl,
		wm.ForeignAPIHyprlandIPC,
		wm.ForeignAPIQuickshellHyprland,
	}
	if strings.Join(foreignAPINames(foreignAPIs), ",") != strings.Join(foreignAPINames(want), ",") {
		t.Fatalf("foreign APIs = %v, want %v", foreignAPIs, want)
	}
}

func foreignAPINames(apis []wm.ForeignAPI) []string {
	names := make([]string, len(apis))
	for i, api := range apis {
		names[i] = string(api)
	}
	return names
}

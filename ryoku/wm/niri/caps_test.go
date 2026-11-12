package main

import (
	"encoding/json"
	"fmt"
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

func TestDeniesWorkspaceFullscreenState(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapWorkspaceFullscreenState) {
		t.Error("niri workspace frames have no fullscreen field; the shell must enrich them from foreign toplevels")
	}
}

// A screen capture on niri goes through wlr-screencopy and a second wl_output
// the Qt client later reads after it is gone, so the manifest must keep
// denying this or the launcher frost and the LiveDraw zoom come back and take
// the shell with them.
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

// The animation is a config block with no query IPC, so the overlay cannot
// learn the resolved slide; claiming this would leave it waiting for a read
// that never answers.
func TestDeniesWorkspaceSlideStyle(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapWorkspaceSlideStyle) {
		t.Error("niri cannot report its workspace animation; the manifest must not claim it")
	}
}

func TestServesNoForeignShellAPIs(t *testing.T) {
	if len(foreignAPIs) != 0 {
		t.Fatalf("foreign APIs = %v, want none", foreignAPIs)
	}
}

// nest runs a hidden niri inside a headless weston, and niri there takes
// virtual input and screencopy like on the desktop, so an agent can be given
// a screen of its own.
func TestClaimsNestedSession(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapNestedSession) {
		t.Error("niri implements the nest verb; the manifest must say so")
	}
}

// every window-rule action any provider or the App rules page knows
var knownRuleActions = []string{
	"float", "tile", "pin", "fullscreen", "maximize", "center", "pseudo", "keepaspectratio",
	"size", "move", "workspace", "columnwidth", "minsize", "maxsize", "tiledstate",
	"norounding", "noborder", "opacity", "opaque", "blur", "noblur", "noshadow", "nodim",
	"noanim", "xray", "babaisfloat", "nofocus", "stayfocused", "suppressevent",
	"scrollfactor", "blockout", "idleinhibit", "immediate",
}

func sampleRuleValue(action string) string {
	switch action {
	case "size", "minsize", "maxsize":
		return "800x600"
	}
	return "1"
}

// App rules offers exactly the advertised actions, so an advertised action the
// writer drops is a dead control and a rendered one left out is a lost feature;
// apply's report must say the same.
func TestAdvertisedWindowRuleActionsAreTheRenderedOnes(t *testing.T) {
	for _, a := range append(append([]string{}, knownRuleActions...), windowRuleActions...) {
		listed := false
		for _, l := range windowRuleActions {
			listed = listed || l == a
		}
		r := WindowRule{Class: "foo", Action: a, Value: sampleRuleValue(a)}
		rendered := windowRuleProps(r) != nil
		raw, _ := json.Marshal([]WindowRule{r})
		reported := len(unhonoredWindowRules(raw)) > 0
		if listed != rendered || rendered == reported {
			t.Errorf("action %q: advertised=%v rendered=%v reported unhonored=%v", a, listed, rendered, reported)
		}
	}
}

// one stored override per look field, set the way App rules writes it
var lookSamples = map[string]string{
	"opacity": `"opacity":0.8`, "rounding": `"rounding":4`, "borderSize": `"borderSize":0`,
	"blur": `"blur":"off"`, "shadow": `"shadow":"off"`, "dim": `"dim":"off"`,
	"anim": `"anim":"off"`, "opaque": `"opaque":"on"`,
}

// App rules shows a look override only where it is advertised, so each
// advertised field must reach the config, each other one must not, and apply
// must report exactly the ones that do not.
func TestAppOverrideFieldsAreTheWrittenOnes(t *testing.T) {
	for _, f := range appOverrideFields {
		if lookSamples[f] == "" {
			t.Errorf("advertised look field %q is not one App rules writes", f)
		}
	}
	for field, sample := range lookSamples {
		raw := fmt.Sprintf(`[{"class":"foo","title":"","opacity":-1,"rounding":-1,"borderSize":-1,`+
			`"blur":"inherit","shadow":"inherit","dim":"inherit","anim":"inherit","opaque":"inherit",%s}]`, sample)
		var apps []AppOverride
		if err := json.Unmarshal([]byte(raw), &apps); err != nil {
			t.Fatal(err)
		}
		listed := false
		for _, l := range appOverrideFields {
			listed = listed || l == field
		}
		written := len(appOverrideProps(apps[0])) > 0
		reported := len(unhonoredAppOverrides(json.RawMessage(raw))) > 0
		if listed != written || written == reported {
			t.Errorf("look field %q: advertised=%v written=%v reported unhonored=%v", field, listed, written, reported)
		}
	}
}

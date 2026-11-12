package main

import (
	"encoding/json"
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

func TestClaimsWorkspaceFullscreenState(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapWorkspaceFullscreenState) {
		t.Error("Hyprland workspace frames carry hasfullscreen and must stay authoritative")
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

// The live-draw overlay matches its ink to the compositor's workspace slide,
// read through the slide bridge; without the claim the ink drifts out of step
// with the windows on any config that changed the animation.
func TestClaimsWorkspaceSlideStyle(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if !c.Has(wm.CapWorkspaceSlideStyle) {
		t.Error("Hyprland answers `animations -j`; the manifest must say so")
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

// Hyprland cannot run inside the headless weston a nest starts on, so no
// program may be promised a hidden session here.
func TestDeniesNestedSession(t *testing.T) {
	c := wm.Caps{Supports: capsManifest}
	if c.Has(wm.CapNestedSession) {
		t.Error("Hyprland has no nest verb; the manifest must not claim nested sessions")
	}
}

// every window-rule action any provider or the App rules page knows, plus the
// layer-rule ones Hyprland once advertised for windows
var knownRuleActions = []string{
	"float", "tile", "pin", "fullscreen", "maximize", "center", "pseudo", "keepaspectratio",
	"size", "move", "workspace", "columnwidth", "minsize", "maxsize", "tiledstate",
	"norounding", "noborder", "opacity", "opaque", "blur", "noblur", "noshadow", "nodim",
	"noanim", "xray", "babaisfloat", "nofocus", "stayfocused", "suppressevent",
	"scrollfactor", "blockout", "idleinhibit", "immediate",
	"blurpopups", "abovelock", "ignorealpha", "dimaround",
}

// App rules offers exactly the advertised actions, so an advertised action the
// writer drops is a dead control and a rendered one left out is a lost feature.
func TestAdvertisedWindowRuleActionsAreTheRenderedOnes(t *testing.T) {
	for _, a := range append(append([]string{}, knownRuleActions...), windowRuleActions...) {
		listed := false
		for _, l := range windowRuleActions {
			listed = listed || l == a
		}
		rendered := genWindowRule(0, WindowRule{Class: "foo", Action: a, Value: "1"}) != ""
		if listed != rendered {
			t.Errorf("action %q: advertised=%v but genWindowRule renders=%v", a, listed, rendered)
		}
	}
}

// the field names are Hyprland's own spelling; a wrong one breaks the file
func TestWindowRuleFieldSpelling(t *testing.T) {
	for action, prop := range map[string]string{
		"nodim": "no_dim = true", "noanim": "no_anim = true", "noblur": "no_blur = true",
		"noshadow": "no_shadow = true", "nofocus": "no_focus = true", "stayfocused": "stay_focused = true",
		"keepaspectratio": "keep_aspect_ratio = true", "opaque": "opaque = true",
		"norounding": "rounding = 0", "noborder": "border_size = 0",
	} {
		got := genWindowRule(0, WindowRule{Class: "foo", Action: action})
		if !strings.Contains(got, ", "+prop+" })") {
			t.Errorf("genWindowRule(%q) = %q, want the property %q", action, got, prop)
		}
	}
}

// one stored override per look field, set the way App rules writes it
var lookSamples = map[string]string{
	"opacity": `{"opacity":0.8}`, "rounding": `{"rounding":4}`, "borderSize": `{"borderSize":0}`,
	"blur": `{"blur":"off"}`, "shadow": `{"shadow":"off"}`, "dim": `{"dim":"off"}`,
	"anim": `{"anim":"off"}`, "opaque": `{"opaque":"on"}`,
}

// App rules shows a look override only where it is advertised, so each
// advertised field must reach the config and each other one must not.
func TestAppOverrideFieldsAreTheWrittenOnes(t *testing.T) {
	for _, f := range appOverrideFields {
		if lookSamples[f] == "" {
			t.Errorf("advertised look field %q is not one App rules writes", f)
		}
	}
	for field, sample := range lookSamples {
		a := AppOverride{Class: "foo", Opacity: -1, Rounding: -1, BorderSize: -1,
			Blur: "inherit", Shadow: "inherit", Dim: "inherit", Anim: "inherit", Opaque: "inherit"}
		if err := json.Unmarshal([]byte(sample), &a); err != nil {
			t.Fatal(err)
		}
		listed := false
		for _, l := range appOverrideFields {
			listed = listed || l == field
		}
		written := genAppOverride(0, a) != ""
		if listed != written {
			t.Errorf("look field %q: advertised=%v but genAppOverride writes=%v", field, listed, written)
		}
	}
}

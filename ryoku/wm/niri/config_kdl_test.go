package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// niri's border block starts off and only merges on when the block carries an
// explicit on flag, so these pin that flag directly on the emitter output: a
// bare width would validate yet draw no frame. The layout block and the per-app
// override both resolve through the same BorderRule merge, so both must carry it.

// layoutBorderBlock returns the border sub-block inside the top-level layout
// block, so an assertion sees only the frame the user sized, not a per-app rule.
func layoutBorderBlock(t *testing.T, out string) string {
	t.Helper()
	li := strings.Index(out, "layout {")
	if li < 0 {
		t.Fatalf("no layout block\n%s", out)
	}
	rest := out[li:]
	bi := strings.Index(rest, "    border {\n")
	if bi < 0 {
		t.Fatalf("no border block in layout\n%s", out)
	}
	body := rest[bi:]
	end := strings.Index(body, "\n    }\n")
	if end < 0 {
		t.Fatalf("border block not closed\n%s", out)
	}
	return body[:end+len("\n    }\n")]
}

func TestLayoutBorderOnFlag(t *testing.T) {
	sized := layoutBorderBlock(t, string(genSettings(defaultStore())))
	if !strings.Contains(sized, "\n        on\n") {
		t.Errorf("a sized border must carry a bare on flag; niri leaves it off without one\n%s", sized)
	}
	if !strings.Contains(sized, "width 4") {
		t.Errorf("a sized border must keep its width\n%s", sized)
	}
	if strings.Contains(sized, "\n        off\n") {
		t.Errorf("a sized border must not be off\n%s", sized)
	}

	s := defaultStore()
	s.Appearance.BorderSize = 0
	off := layoutBorderBlock(t, string(genSettings(s)))
	if !strings.Contains(off, "\n        off\n") {
		t.Errorf("a zero border must be off\n%s", off)
	}
	if strings.Contains(off, "\n        on\n") || strings.Contains(off, "width") {
		t.Errorf("a zero border must carry neither on nor width\n%s", off)
	}
}

func TestLayoutBorderAppOverrideFlag(t *testing.T) {
	s := defaultStore()
	s.AppOverrides = []AppOverride{
		{Class: "kitty", Opacity: -1, Rounding: -1, BorderSize: 7},
		{Class: "mpv", Opacity: -1, Rounding: -1, BorderSize: 0},
		{Class: "foot", Opacity: -1, Rounding: -1, BorderSize: -1},
	}
	out := string(genSettings(s))

	if !strings.Contains(out, "    border {\n        on\n        width 7\n    }\n") {
		t.Errorf("a per-app sized border must carry on and its width\n%s", out)
	}
	if !strings.Contains(out, "    border {\n        off\n    }\n") {
		t.Errorf("a per-app zero border must be off\n%s", out)
	}
	if strings.Contains(out, `app-id="foot"`) {
		t.Errorf("an inherit-only override must emit no rule at all\n%s", out)
	}
}

// The frame choice steers which of niri's two frame blocks the sized width and
// colours land in: a border, a focus ring, or both. The other block stays off.
func TestFrameChoiceBorderFocusRingBoth(t *testing.T) {
	border := string(genSettings(defaultStore()))
	if !strings.Contains(border, "    border {\n        on\n        width 4\n") {
		t.Errorf("the default frame must draw the border\n%s", border)
	}
	if !strings.Contains(border, "    focus-ring {\n        off\n    }\n") {
		t.Errorf("the default frame must leave the focus ring off\n%s", border)
	}

	s := defaultStore()
	s.Niri.Frame = "focusRing"
	ring := string(genSettings(s))
	if !strings.Contains(ring, "    border {\n        off\n    }\n") {
		t.Errorf("a focus-ring frame must turn the border off\n%s", ring)
	}
	if !strings.Contains(ring, "    focus-ring {\n        on\n        width 4\n") {
		t.Errorf("a focus-ring frame must draw the ring\n%s", ring)
	}

	s = defaultStore()
	s.Niri.Frame = "both"
	both := string(genSettings(s))
	if strings.Count(both, "        on\n        width 4\n") < 2 {
		t.Errorf("a both frame must draw the border and the ring\n%s", both)
	}
}

// The active-frame gradient replaces the active solid colour with a gradient
// while the inactive frame stays a solid colour, so the focused window carries
// the fancy look and the rest read plain.
func TestBorderGradientActive(t *testing.T) {
	s := defaultStore()
	s.Niri.BorderGradient = true
	out := string(genSettings(s))
	if !strings.Contains(out, `active-gradient from="#e0563b" to="#9b3226" angle=180 relative-to="window"`) {
		t.Errorf("the active gradient must carry its colours, angle and anchor\n%s", out)
	}
	if !strings.Contains(out, `inactive-color "#313a4d"`) {
		t.Errorf("the inactive frame must stay a solid colour\n%s", out)
	}
	if strings.Contains(out, "\n        active-color ") {
		t.Errorf("the active frame must use the gradient, not a solid colour\n%s", out)
	}
}

// The border follows the live palette when borderFollowsPalette is on and the
// palette file holds usable colours: those drive the solid active and inactive
// colours, the niri twin of Hyprland's decoration.lua border. A fixed border or
// a missing palette file falls back to the store colours.
func TestBorderFollowsPalette(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	writePalette := func(t *testing.T, active, inactive string) {
		if err := os.MkdirAll(filepath.Dir(borderPalettePath()), 0o755); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf(`{"active":%q,"inactive":%q}`, active, inactive)
		if err := os.WriteFile(borderPalettePath(), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writePalette(t, "#112233", "#445566")
	following := layoutBorderBlock(t, string(genSettings(defaultStore())))
	if !strings.Contains(following, `active-color "#112233"`) || !strings.Contains(following, `inactive-color "#445566"`) {
		t.Errorf("a following border must take the palette colours\n%s", following)
	}
	if strings.Contains(following, "#e0563b") {
		t.Errorf("a following border must not keep the store colour\n%s", following)
	}

	s := defaultStore()
	s.Appearance.BorderFollowsPalette = false
	fixed := layoutBorderBlock(t, string(genSettings(s)))
	if !strings.Contains(fixed, `active-color "#e0563b"`) || !strings.Contains(fixed, `inactive-color "#313a4d"`) {
		t.Errorf("a fixed border must use the store colours even with a palette file present\n%s", fixed)
	}

	if err := os.Remove(borderPalettePath()); err != nil {
		t.Fatal(err)
	}
	missing := layoutBorderBlock(t, string(genSettings(defaultStore())))
	if !strings.Contains(missing, `active-color "#e0563b"`) || !strings.Contains(missing, `inactive-color "#313a4d"`) {
		t.Errorf("a missing palette file must fall back to the store colours\n%s", missing)
	}
}

// niri 26.04's forced window blur renders translucent windows opaque, so Ryoku
// no longer models or emits global blur: the shipped config carries no top-level
// blur block and no matchless background-effect rule, even once windows are made
// translucent (the case that used to reveal the opaque-blur bug).
func TestNoGlobalBlur(t *testing.T) {
	s := defaultStore()
	s.Appearance.ActiveOpacity = 0.9
	s.Appearance.InactiveOpacity = 0.8
	out := string(genSettings(s))
	if strings.Contains(out, "blur {") {
		t.Errorf("niri must emit no top-level blur block\n%s", out)
	}
	if strings.Contains(out, "background-effect") {
		t.Errorf("niri must emit no forced blur rule\n%s", out)
	}
}

// The layout niceties niri owns: a workspace background, tabbed columns, the
// spare empty workspace and a preset window-height cycle.
func TestLayoutExtrasEmitted(t *testing.T) {
	s := defaultStore()
	s.Niri.BackgroundColor = "#0b0e14"
	s.Niri.DefaultColumnDisplay = "tabbed"
	s.Niri.EmptyWorkspaceAbove = true
	s.Niri.PresetWindowHeights = Proportions{0.4, 0.6}
	out := string(genSettings(s))
	for _, want := range []string{
		`background-color "#0b0e14"`,
		`default-column-display "tabbed"`,
		"empty-workspace-above-first",
		"preset-window-heights {\n        proportion 0.4\n        proportion 0.6\n    }",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("layout extra missing %q\n%s", want, out)
		}
	}
}

func TestPreferNoCSD(t *testing.T) {
	if out := string(genSettings(defaultStore())); !strings.Contains(out, "\nprefer-no-csd\n") {
		t.Errorf("the default must ask apps to hide client-side title bars\n%s", out)
	}

	s := defaultStore()
	s.Niri.PreferNoCSD = false
	if out := string(genSettings(s)); strings.Contains(out, "prefer-no-csd") {
		t.Errorf("prefer-no-csd must be absent when app title bars are enabled\n%s", out)
	}
}

// The overview backdrop colour and the workspace shadow. niri draws the shadow by
// default, so an untouched shadow stays silent, a disabled one turns off, and a
// retuned one carries its fields.
func TestOverviewExtras(t *testing.T) {
	if strings.Contains(string(genSettings(defaultStore())), "workspace-shadow") {
		t.Error("an untouched workspace shadow must stay silent, matching niri's own default")
	}

	s := defaultStore()
	s.Niri.BackdropColor = "#101010"
	s.Niri.WorkspaceShadow = false
	out := string(genSettings(s))
	if !strings.Contains(out, `backdrop-color "#101010"`) {
		t.Errorf("overview backdrop colour missing\n%s", out)
	}
	if !strings.Contains(out, "workspace-shadow {\n        off\n    }") {
		t.Errorf("a disabled workspace shadow must turn off\n%s", out)
	}

	s = defaultStore()
	s.Niri.WorkspaceShadowSoft = 50
	on := string(genSettings(s))
	if !strings.Contains(on, "workspace-shadow {\n        on\n        softness 50\n") {
		t.Errorf("a retuned workspace shadow must carry its fields\n%s", on)
	}
}

// The niri-exclusive input children: pointer warp, the focus-follows-mouse scroll
// cap, the workspace toggle, the power-key handoff and the mod keys. Super is
// niri's own mod default, so it emits no line.
func TestInputExclusivesEmitted(t *testing.T) {
	s := defaultStore()
	s.Input.FollowMouse = 1
	s.Niri.FocusFollowScroll = 10
	s.Niri.WarpMouseToFocus = "center-xy"
	s.Niri.WorkspaceBackForth = true
	s.Niri.DisablePowerKey = true
	s.Niri.ModKey = "Alt"
	s.Niri.ModKeyNested = "Ctrl"
	out := string(genSettings(s))
	for _, want := range []string{
		`focus-follows-mouse max-scroll-amount="10%"`,
		`warp-mouse-to-focus mode="center-xy"`,
		"workspace-auto-back-and-forth",
		"disable-power-key-handling",
		`mod-key "Alt"`,
		`mod-key-nested "Ctrl"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("input exclusive missing %q\n%s", want, out)
		}
	}
	if strings.Contains(string(genSettings(defaultStore())), "mod-key") {
		t.Error("the Super mod default must emit no mod-key line")
	}
}

// The gestures block keeps the bare hot-corner switch by default and only tunes
// the drag-edge gestures when the user moved them off niri's own numbers.
func TestGesturesDndTuning(t *testing.T) {
	def := string(genSettings(defaultStore()))
	if strings.Contains(def, "dnd-edge") {
		t.Errorf("an untouched drag gesture must not reach the config\n%s", def)
	}
	if !strings.Contains(def, "gestures {\n    hot-corners {\n        off\n    }\n}") {
		t.Errorf("the default gestures block must keep the hot-corner switch\n%s", def)
	}

	s := defaultStore()
	s.Niri.DndViewTriggerWidth = 40
	s.Niri.DndWsMaxSpeed = 1600
	out := string(genSettings(s))
	if !strings.Contains(out, "dnd-edge-view-scroll {\n        trigger-width 40\n        delay-ms 100\n        max-speed 1500\n    }") {
		t.Errorf("a tuned view-scroll gesture must carry all three fields\n%s", out)
	}
	if !strings.Contains(out, "dnd-edge-workspace-switch {\n        trigger-height 50\n        delay-ms 100\n        max-speed 1600\n    }") {
		t.Errorf("a tuned workspace-switch gesture must carry all three fields\n%s", out)
	}
}

// The hotkey-overlay hide switch and the recent-windows toggle. Recent windows is
// on by niri default, so only turning it off reaches the config.
func TestHotkeyHideAndRecentWindows(t *testing.T) {
	s := defaultStore()
	s.Niri.HotkeyOverlayHide = true
	s.Niri.RecentWindows = false
	out := string(genSettings(s))
	if !strings.Contains(out, "hotkey-overlay {\n    skip-at-startup\n    hide-not-bound\n}") {
		t.Errorf("hide-not-bound must join the hotkey overlay block\n%s", out)
	}
	if !strings.Contains(out, "recent-windows {\n    off\n}") {
		t.Errorf("a disabled recent-windows switcher must turn off\n%s", out)
	}
	if strings.Contains(string(genSettings(defaultStore())), "recent-windows") {
		t.Error("the on-by-default recent-windows switcher must stay silent")
	}
}

// Each animation kind renders its mode: spring, ease or off. A kind left on
// default emits nothing, and an all-default tree emits no animations block at
// all; the slowdown factor and the master off keep their own behaviour.
func TestAnimKindsRendered(t *testing.T) {
	s := defaultStore()
	s.Niri.Anim.WorkspaceSwitch = AnimSpec{Mode: "spring", DampingRatio: 1, Stiffness: 900, Epsilon: 0.0001}
	s.Niri.Anim.WindowOpen = AnimSpec{Mode: "ease", DurationMs: 200, Curve: "ease-out-expo"}
	s.Niri.Anim.WindowClose = AnimSpec{Mode: "off"}
	out := string(genSettings(s))
	for _, want := range []string{
		"workspace-switch {\n        spring damping-ratio=1.0 stiffness=900 epsilon=0.0001\n    }",
		"window-open {\n        duration-ms 200\n        curve \"ease-out-expo\"\n    }",
		"window-close {\n        off\n    }",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("animation missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "overview-open-close") {
		t.Errorf("a default kind must emit nothing\n%s", out)
	}
	if strings.Contains(string(genSettings(defaultStore())), "animations {") {
		t.Error("an all-default anim tree must emit no animations block")
	}

	sd := defaultStore()
	sd.Niri.AnimationSlowdown = 1.5
	if !strings.Contains(string(genSettings(sd)), "animations {\n    slowdown 1.5\n}") {
		t.Error("the slowdown factor must still reach the animations block")
	}
	so := defaultStore()
	so.Appearance.Animations = false
	if !strings.Contains(string(genSettings(so)), "animations {\n    off\n}") {
		t.Error("the master animation switch must still turn everything off")
	}
}

func TestWelcomeBuiltinWindowRule(t *testing.T) {
	out := string(genSettings(defaultStore()))
	want := "window-rule {\n" +
		"    match title=\"^Welcome to Ryoku$\"\n" +
		"    open-floating true\n" +
		"    default-column-width { fixed 1180; }\n" +
		"    default-window-height { fixed 760; }\n" +
		"}\n"
	if !strings.Contains(out, want) {
		t.Errorf("welcome must open as a centred 1180x760 floating window\n%s", out)
	}
	if strings.Count(out, `match title="^Welcome to Ryoku$"`) != 1 {
		t.Errorf("welcome builtin rule must be emitted once\n%s", out)
	}
}

// The neutral window-rule actions niri gained: each one renders its niri property
// lines so a rule the WindowRulesPage offers on niri actually reaches the config.
func TestNiriWindowRuleActions(t *testing.T) {
	s := defaultStore()
	s.WindowRules = []WindowRule{
		{Class: "foo", Action: "columnwidth", Value: "0.65"},
		{Class: "bar", Action: "minsize", Value: "800x600"},
		{Class: "baz", Action: "maxsize", Value: "1200x900"},
		{Class: "qux", Action: "scrollfactor", Value: "1.5"},
		{Class: "a", Action: "tiledstate"},
		{Class: "b", Action: "babaisfloat"},
		{Class: "c", Action: "noshadow"},
		{Class: "d", Action: "noblur"},
		{Class: "e", Action: "blockout"},
	}
	out := string(genSettings(s))
	for _, want := range []string{
		"default-column-width { proportion 0.65; }",
		"min-width 800\n    min-height 600",
		"max-width 1200\n    max-height 900",
		"scroll-factor 1.5",
		"tiled-state true",
		"baba-is-float true",
		"    shadow {\n        off\n    }",
		"    background-effect {\n        blur false\n    }",
		`block-out-from "screencast"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("window-rule action missing %q\n%s", want, out)
		}
	}
}

// Per-app forced blur renders a background-effect the same way the global rule
// does; on forces blur, off suppresses it.
func TestPerAppBlurRendered(t *testing.T) {
	s := defaultStore()
	s.AppOverrides = []AppOverride{
		{Class: "kitty", Opacity: -1, Rounding: -1, BorderSize: -1, Blur: "on"},
		{Class: "mpv", Opacity: -1, Rounding: -1, BorderSize: -1, Blur: "off"},
	}
	out := string(genSettings(s))
	if !strings.Contains(out, "match app-id=\"kitty\"\n    background-effect {\n        blur true\n    }") {
		t.Errorf("per-app forced blur must render blur true\n%s", out)
	}
	if !strings.Contains(out, "match app-id=\"mpv\"\n    background-effect {\n        blur false\n    }") {
		t.Errorf("per-app suppressed blur must render blur false\n%s", out)
	}
}

// A user layer rule renders the fields niri can set on a matching namespace; a
// rule with no namespace is skipped rather than emitted as a blanket rule that
// would touch every layer surface.
func TestUserLayerRulesRendered(t *testing.T) {
	s := defaultStore()
	s.Niri.LayerRules = []LayerRule{
		{Namespace: "waybar", Opacity: 0.9, CornerRadius: 12, Blur: "on", Shadow: "on", BlockOut: true, BabaIsFloat: true},
		{Namespace: "", Opacity: -1, CornerRadius: -1, Blur: "inherit", Shadow: "inherit"},
	}
	out := string(genSettings(s))
	for _, want := range []string{
		`match namespace="waybar"`,
		"opacity 0.9",
		"geometry-corner-radius 12",
		"    background-effect {\n        blur true\n    }",
		"    shadow {\n        on\n    }",
		`block-out-from "screencast"`,
		"baba-is-float true",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("layer rule missing %q\n%s", want, out)
		}
	}
	// the backdrop rule plus the one named rule, never the namespaceless one.
	if n := strings.Count(out, "layer-rule {"); n != 2 {
		t.Errorf("a namespaceless layer rule must be skipped, got %d layer rules\n%s", n, out)
	}
}

package main

import (
	"encoding/json"
	"strings"

	wm "ryoku-wm"
)

// Every entry here must actually be honoured in act or apply. Claiming one this
// provider cannot perform is worse than omitting it: the desktop would offer a
// control that does nothing.
//
// The absences are niri's design, not gaps to fill later:
//
// CapWindowGeometry: tiled windows still have no on-screen position, so the
// shell cannot draw a general overview. Floating windows do report position and
// size for persistence, while CapNativeOverview supplies the general overview.
//
// CapSubmap, CapGlobalShortcuts, CapFocusGrab, CapScreenShader, CapPlugins:
// niri implements none of these protocols or subsystems.
//
// CapLiveConfigEval and CapConfigReload: the config is file-only and niri
// watches it, so there is nothing to evaluate and nothing to trigger.
//
// CapCursorSet: the cursor is a config block, so apply sets it and niri picks
// it up. There is no imperative call to re-assert it after a reload.
//
// CapTiledLayout: the layout is scrollable tiling, with no per-workspace choice
// to make.
//
// CapSpecialWorkspace: niri has no scratchpad workspace.
//
// CapPersistentScreenCapture: niri tears its outputs down and recreates them on
// a config reload and on a mode change, so a layer-shell surface that captures
// the whole screen and lives for the session races an output leaving under it.
// The Qt client segfaults resolving a screen that is already gone. niri still
// answers one-shot captures (the overview, a window preview, grim), so this is
// about the long-lived surface, not screencopy itself.
var foreignAPIs = []wm.ForeignAPI{}

var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
	wm.CapWorkspaceMoveToOutput,
	wm.CapWindowWorkspaceMap,
	wm.CapFocusHistory,
	wm.CapWindowRules,
	wm.CapLayerRules,
	wm.CapAnimations,
	wm.CapNativeOverview,
	wm.CapOverviewBackdrop,
	// Overview open/closed and tile geometry back the pause rules gated on these.
	wm.CapOverviewState,
	wm.CapColumnFill,
	wm.CapOutputPower,
	wm.CapKeyboardLayoutSwitch,
	wm.CapMonitorConfig,
	wm.CapWindowFloat,
	wm.CapSessionExit,
	wm.CapNightLight,
	wm.CapTouchpadToggle,
	wm.CapPaletteBorder,
	// niri routes the pointer by input region whatever holds the keyboard, so
	// an exclusive grab leaves every other surface clickable.
	wm.CapKeyboardGrabSharesPointer,
}

// windowRuleActions are the neutral window-rule action ids niri's config writer
// honours, in the order the Hub offers them. niri models a different set from
// Hyprland (no pin, its own tabbed/scroll mechanics instead), so the window-rules
// editor lists only what this compositor can actually apply.
var windowRuleActions = []string{
	"float", "tile", "fullscreen", "maximize", "norounding", "opacity",
	"workspace", "noborder", "noshadow", "blur", "noblur", "xray",
	"columnwidth", "minsize", "maxsize", "scrollfactor", "tiledstate",
	"babaisfloat", "blockout",
}

// The packages ryoku-desktop-niri is made of: the variant package itself, niri,
// the xwayland-satellite X11 bridge, and the GNOME portal backend its caps
// report. Kept in step with that package's depends
// (release/packages/ryoku-desktop-niri/PKGBUILD); this is the list a switch
// away from niri reclaims, minus ryoku-desktop, which is shared with the
// compositor that replaces it. The variant package belongs in the list: on a
// packaged box it owns every satellite below, so a reclaim that left it out
// could free none of them.
var compositorPackages = []string{
	"ryoku-desktop-niri",
	"niri",
	"xwayland-satellite",
	"xdg-desktop-portal-gnome",
	// wlsunset holds the warm gamma while the night light is on, over
	// wlr-gamma-control. niri's night-light backend, so its variant ships and
	// reclaims it.
	"wlsunset",
}

// The manifest is fixed, not probed: niri does not gain features while running,
// and caps is read during startup.
func runCaps() error {
	caps := wm.Caps{
		Name:     wm.ProviderNiri,
		Version:  probeVersion(),
		Instance: instanceHandle(),
		Supports: capsManifest,
		// Workspaces are created and removed per output as windows come and
		// go, so presenting them as numbered slots would be a lie.
		WorkspaceModel: wm.WorkspaceModelDynamic,
		ForeignAPIs:    foreignAPIs,
		// wm.niri.* keys stay in the store untouched while another compositor
		// is active, so they are still there on the way back.
		SettingDomains:    []string{"desktop", "wm." + wm.ProviderNiri},
		ConfigFiles:       wm.ConfigFiles(wm.ProviderNiri),
		GeneratedFiles:    wm.GeneratedConfig(wm.ProviderNiri),
		PortalBackend:     "gnome",
		NightLightProcess: "wlsunset",
		Packages:          compositorPackages,
		WindowRuleActions: windowRuleActions,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(caps)
}

// Probed best-effort: the installer and doctor need a manifest before any
// compositor is running.
func probeVersion() string {
	if !live() {
		return ""
	}
	raw, err := request("Version")
	if err != nil {
		return ""
	}
	var v struct {
		Version string `json:"Version"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.Version)
}

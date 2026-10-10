package main

import (
	"encoding/json"
	"os"
	"strings"

	wm "ryoku-wm"
)

// Every entry here must actually be honoured in act, apply or plugins. Claiming
// one this provider cannot perform is worse than omitting it: the desktop would
// offer a control that does nothing.
//
// CapNativeOverview is absent because Hyprland ships no overview, so the shell
// draws its own.
//
// CapKeyboardGrabSharesPointer is absent because a layer surface holding
// exclusive keyboard focus also takes every pointer event, wherever the pointer
// is, so no other surface can be clicked until the grab ends.
var foreignAPIs = []wm.ForeignAPI{
	wm.ForeignAPIHyprctl,
	wm.ForeignAPIHyprlandIPC,
	wm.ForeignAPIQuickshellHyprland,
}

var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
	wm.CapSpecialWorkspace,
	wm.CapWorkspaceMoveToOutput,
	wm.CapWindowWorkspaceMap,
	wm.CapWindowGeometry,
	wm.CapFocusHistory,
	wm.CapWindowRules,
	wm.CapLayerRules,
	wm.CapSubmap,
	wm.CapGlobalShortcuts,
	wm.CapFocusGrab,
	wm.CapScreenShader,
	wm.CapPlugins,
	wm.CapLiveConfigEval,
	wm.CapConfigReload,
	wm.CapAnimations,
	wm.CapCursorSet,
	wm.CapOutputPower,
	wm.CapKeyboardLayoutSwitch,
	wm.CapMonitorConfig,
	wm.CapOutputMirror,
	wm.CapOutputHdr,
	wm.CapWindowFloat,
	wm.CapTiledLayout,
	wm.CapSessionExit,
	wm.CapNightLight,
	wm.CapTouchpadToggle,
	wm.CapPaletteBorder,
	wm.CapPersistentScreenCapture,
}

// windowRuleActions are the neutral window-rule action ids genWindowRule and
// genLayerRule accept, in the order the Hub offers them. It is the source the
// window-rules editor reads, so a control is never shown for a property this
// provider's config writer would drop.
var windowRuleActions = []string{
	"float", "tile", "pin", "fullscreen", "maximize", "center", "immediate",
	"pseudo", "norounding", "noborder", "opacity", "size", "move", "workspace",
	"idleinhibit", "suppressevent", "blur", "noanim", "blurpopups", "xray",
	"abovelock", "noshadow", "ignorealpha", "dimaround",
}

// The packages ryoku-desktop-hyprland is made of: the variant package itself,
// Hyprland, its plugins, its portal and its satellites. Kept in step with that
// package's depends (release/packages/ryoku-desktop-hyprland/PKGBUILD); this is
// the list a switch away from Hyprland reclaims, minus ryoku-desktop, which is
// shared with the compositor that replaces it. The variant package belongs in
// the list: on a packaged box it owns every satellite below, so a reclaim that
// left it out could free none of them.
var compositorPackages = []string{
	"ryoku-desktop-hyprland",
	"hyprland",
	"hypr-dynamic-cursors",
	"ryoku-hypr-plugins",
	"hyprglass",
	"hyprgrass",
	"imgborders",
	"ryoku-keysounds",
	"hyprpolkitagent",
	"xdg-desktop-portal-hyprland",
	"hyprland-preview-share-picker",
	"hypridle",
	"hyprpicker",
	// hyprsunset holds the warm gamma while the night light is on. A Hyprland-only
	// CTM client, so it is the Hyprland variant's to ship and reclaim.
	"hyprsunset",
}

// The manifest is fixed, not probed: Hyprland does not gain features while
// running, and caps is read during startup.
func runCaps() error {
	caps := wm.Caps{
		Name:           wm.ProviderHyprland,
		Version:        probeVersion(),
		Instance:       instanceHandle(),
		Supports:       capsManifest,
		WorkspaceModel: wm.WorkspaceModelFixed,
		ForeignAPIs:    foreignAPIs,
		// wm.hyprland.* keys stay in the store untouched while another
		// compositor is active, so they are still there on the way back.
		SettingDomains:    []string{"desktop", "wm." + wm.ProviderHyprland},
		ConfigFiles:       wm.ConfigFiles(wm.ProviderHyprland),
		GeneratedFiles:    wm.GeneratedConfig(wm.ProviderHyprland),
		PortalBackend:     "hyprland",
		NightLightProcess: "hyprsunset",
		Packages:          compositorPackages,
		WindowRuleActions: windowRuleActions,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(caps)
}

// instanceHandle is opaque to consumers, which only string-compare it.
func instanceHandle() string {
	if !live() {
		return ""
	}
	return os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
}

// Probed best-effort: the installer and doctor need a manifest before any
// compositor is running.
func probeVersion() string {
	if !live() {
		return ""
	}
	out, err := ctl("version", "-j")
	if err != nil {
		return ""
	}
	var v struct {
		Tag string `json:"tag"`
	}
	if json.Unmarshal(out, &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.Tag)
}

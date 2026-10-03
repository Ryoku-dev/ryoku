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
// The manifest was set by probing a live 0.17.4 instance (nested, headless),
// not by reading docs; the probe answers are pinned in the local plan.
//
// The absences are mango's design, not gaps to fill later:
//
// CapGlobalShortcuts: mango implements no global-shortcuts portal, so keybinds
// reach shell surfaces by spawning `ryoku-shell <verb>`, the proven niri path.
//
// CapCursorSet: cursor_theme and cursor_size are config keys and a live reload
// picks them up, but there is no imperative call: apply writes them and
// cursor.reassert answers by regenerating and reloading, which is the honest
// shape of the verb on a file-driven compositor.
//
// CapScreenShader, CapPlugins, CapFocusGrab: mango has no plugin system, no
// per-window shader stack, and no focus-grab protocol.
//
// CapPersistentScreenCapture: unproven. reload_config re-notifies watchers and
// grim survives a reload in the nested probe, but whether a long-lived grabbing
// layer surface survives a real mode change on hardware was not established, so
// the launcher frost stays a solid drawer here exactly as on niri. One-shot
// captures (grim, the overview) work — screencopy is served.
//
// CapOutputHdr: mango's HDR lives in the separate wl-only/Vulkan branch; the
// scenefx build mango links against cannot take it, so the claim would be a
// lie for every box Ryoku installs.
//
// CapOverviewState / CapOverviewBackdrop / CapColumnFill: the watch grammar has
// no overview event and no column readback, so the wallpaper pause rules and
// the backdrop surface stand aside as they do on Hyprland.
//
// CapWorkspaceMoveToOutput: tags are monitor-bound in the dwl model; tagmon
// moves a window's tag view, not the tag itself, so the control is gated off
// rather than faked.
var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
	wm.CapSpecialWorkspace,
	wm.CapWindowWorkspaceMap,
	wm.CapWindowGeometry,
	wm.CapFocusHistory,
	wm.CapWindowRules,
	wm.CapLayerRules,
	wm.CapSubmap,
	wm.CapAnimations,
	wm.CapNativeOverview,
	wm.CapOutputPower,
	wm.CapKeyboardLayoutSwitch,
	wm.CapMonitorConfig,
	wm.CapWindowFloat,
	wm.CapTiledLayout,
	wm.CapSessionExit,
	wm.CapNightLight,
	wm.CapTouchpadToggle,
	wm.CapPaletteBorder,
	wm.CapLiveConfigEval,
	wm.CapConfigReload,
}

// windowRuleActions are the neutral window-rule action ids mango's config
// writer honours, in the order the Hub offers them. mango's windowrule grammar
// covers more of the neutral set than niri's (pin is isglobal, size and centre
// are first-class), so the rules editor lists what this compositor can actually
// apply and reports the rest per rule.
var windowRuleActions = []string{
	"float", "tile", "pin", "fullscreen", "maximize", "center", "size",
	"move", "workspace", "opacity", "noblur", "noborder", "noshadow",
	"norounding", "noanim", "nofocus",
}

// The packages ryoku-desktop-mango is made of: the variant package itself,
// mango (built by release/packages/mangowm from a pinned tag), the output
// management client that fills the display editor, the scenefx effects library
// it links, the wlr portal backend its caps report, and the gamma client that
// holds the warm screen. Kept in step with that package's depends; this is the
// list a switch away from mango reclaims, minus ryoku-desktop, which is shared.
// The variant package belongs in the list: on a packaged box it owns every
// satellite below.
var compositorPackages = []string{
	"ryoku-desktop-mango",
	"mangowm",
	"wlr-randr",
	"scenefx",
	"xdg-desktop-portal-wlr",
	"wlsunset",
}

// The manifest is fixed, not probed: mango does not gain features while running,
// and caps is read during startup.
func runCaps() error {
	caps := wm.Caps{
		Name:     wm.ProviderMango,
		Version:  probeVersion(),
		Instance: instanceHandle(),
		Supports: capsManifest,
		// Tags are a fixed numbered set (tag_num=9 in the shipped config) that a
		// user thinks of by number, exactly the model the bar's numbered pills
		// present.
		WorkspaceModel: wm.WorkspaceModelFixed,
		// wm.mango.* keys stay in the store untouched while another compositor
		// is active, so they are still there on the way back.
		SettingDomains:    []string{"desktop", "wm." + wm.ProviderMango},
		ConfigFiles:       wm.ConfigFiles(wm.ProviderMango),
		GeneratedFiles:    wm.GeneratedConfig(wm.ProviderMango),
		PortalBackend:     "wlr",
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
	var v struct {
		Version string `json:"version"`
	}
	if err := get("version", &v); err != nil {
		return ""
	}
	return strings.TrimSpace(v.Version)
}

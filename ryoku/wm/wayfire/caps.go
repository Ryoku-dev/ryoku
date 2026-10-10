package main

import (
	"encoding/json"
	"os/exec"
	"strings"

	wm "ryoku-wm"
)

// Every entry here must actually be honoured in act or apply. Claiming one
// this provider cannot perform is worse than omitting it: the desktop would
// offer a control that does nothing.
//
// The absences are wayfire's design, not gaps to fill later:
//
// CapLayerRules: wayfire's window-rules plugin matches views only, and no
// rule mechanism covers layer-shell surfaces.
//
// CapConfigReload: wayfire watches its config file with inotify and reloads
// itself, so there is nothing to trigger.
//
// CapOutputPower, CapOutputMirror, CapOutputHdr: the ipc socket exposes no
// path to DPMS, cloning or an HDR pipeline.
//
// CapSubmap, CapGlobalShortcuts, CapFocusGrab, CapScreenShader: wayfire
// implements none of these subsystems.
//
// CapCursorSet: the cursor is a config block, so apply sets it and wayfire
// picks it up. There is no imperative call to re-assert it after a reload.
//
// CapSpecialWorkspace: wayfire has no scratchpad workspace.
//
// CapTiledLayout: simple-tile tiles, but the store has no per-workspace
// layout choice for it to express.
//
// CapNativeOverview, CapOverviewBackdrop, CapOverviewState: expo exists as a
// plugin with no IPC to open it or report it, so the shell keeps the
// overview and wayfire reports nothing about it.
//
// CapPlugins: wayfire loads plugins from its config, but no manager verb
// backs the Hub's plugin page yet.
//
// CapColumnFill: wayfire's tiling has no full-width column concept.
//
// CapLiveConfigEval: the input section carries no focus_follows_mouse for
// the launcher to suppress and set-config-options is runtime-only with no
// ipc path back to the file, so gameMode could strip the decorations and
// never put the user's own back.
//
// CapPersistentScreenCapture: untested on a long-lived capturing surface,
// so it stays denied until a provider test proves the surface survives an
// output leaving under it.
var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
	wm.CapWorkspaceMoveToOutput,
	wm.CapWindowWorkspaceMap,
	wm.CapWindowGeometry,
	wm.CapFocusHistory,
	wm.CapWindowRules,
	wm.CapAnimations,
	wm.CapMonitorConfig,
	wm.CapWindowFloat,
	wm.CapSessionExit,
	wm.CapNightLight,
	wm.CapTouchpadToggle,
	wm.CapPaletteBorder,
	wm.CapKeyboardLayoutSwitch,
}

// windowRuleActions are the neutral window-rule action ids the wayfire rule
// writer honours, in the order the Hub offers them. wayfire's rule language is
// its own (a signal, a condition, an action), so only the ids with a wayfire
// spelling appear; apply reports a stored row outside this set as unhonored.
var windowRuleActions = []string{
	"pin", "maximize", "opacity", "workspace",
}

// The packages ryoku-desktop-wayfire is made of: the variant package itself,
// wayfire, the X11 bridge wayfire runs natively, the GNOME portal backend its
// caps report, and wlsunset, which holds the warm gamma while the night light
// is on over wlr-gamma-control. Kept in step with that package's depends
// (release/packages/ryoku-desktop-wayfire/PKGBUILD); this is the list a
// switch away from wayfire reclaims, minus ryoku-desktop, which is shared
// with the compositor that replaces it.
var compositorPackages = []string{
	"ryoku-desktop-wayfire",
	"wayfire",
	"xorg-xwayland",
	"xdg-desktop-portal-gnome",
	"wlsunset",
}

// The manifest is fixed, not probed: wayfire does not gain features while
// running, and caps is read during startup.
func runCaps() error {
	caps := wm.Caps{
		Name:     wm.ProviderWayfire,
		Version:  probeVersion(),
		Instance: instanceHandle(),
		Supports: capsManifest,
		// Workspaces live in a per-output grid created on demand, so
		// presenting them as stable numbered slots would be a lie.
		WorkspaceModel: wm.WorkspaceModelDynamic,
		// wm.wayfire.* keys stay in the store untouched while another
		// compositor is active, so they are still there on the way back.
		SettingDomains:    []string{"desktop", "wm." + wm.ProviderWayfire},
		ConfigFiles:       wm.ConfigFiles(wm.ProviderWayfire),
		GeneratedFiles:    wm.GeneratedConfig(wm.ProviderWayfire),
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
// compositor is running, and wayfire prints its version without connecting
// to a display.
func probeVersion() string {
	out, err := exec.Command("wayfire", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

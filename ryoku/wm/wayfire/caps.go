package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	wm "ryoku-wm"
)

// Every entry here must actually be honoured in act or apply. Claiming one
// this provider cannot perform is worse than omitting it: the desktop would
// offer a control that does nothing.
//
// Capability boundaries and conditional claims:
//
// CapLayerRules: wayfire's window-rules plugin matches views only, and no
// rule mechanism covers layer-shell surfaces.
//
// CapConfigReload: deploy replaces the config directory atomically, leaving
// wayfire's inotify watch on the old inode. wayfire/reload-config-metadata
// reopens the configured path and restores reloads after that swap.
//
// CapOutputHdr: wayfire 0.11 has an HDR toggle but no wide-gamut mode or
// separate SDR-brightness control. The supported HDR boolean is still claimed.
//
// CapSubmap, CapGlobalShortcuts, CapFocusGrab, CapScreenShader: wayfire
// implements none of these subsystems.
//
// CapWorkspaceSlideStyle: the workspace switch animation is a config block
// with no IPC to report the resolved style, curve and duration; the overlay
// keeps its built-in curve.
//
// CapCursorSet: the cursor is a config block, so apply sets it and wayfire
// picks it up. There is no imperative call to re-assert it after a reload.
//
// CapTiledLayout: simple-tile tiles, but the store has no per-workspace
// layout choice for it to express.
//
// CapDeckLayout is advertised only when the separately shipped depthdeck
// module is present. A checkout-built provider on a box without the variant
// package must not expose controls the running compositor cannot load.
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
//
// CapSystemLidEdgeSource: switch devices reach wayfire's internal switch
// signal, but neither command activators nor ipc-rules expose that signal. The
// shared clamshell owner therefore supplies the ordered physical edges.
//
// CapNestedSession: this provider has no nest verb, so nothing can ask it
// for a hidden session.
var capsManifest = []wm.Capability{
	wm.CapWorkspaces,
	wm.CapWorkspaceMoveToOutput,
	wm.CapWindowWorkspaceMap,
	wm.CapWorkspaceFullscreenState,
	wm.CapWindowGeometry,
	wm.CapFocusHistory,
	wm.CapWindowRules,
	wm.CapAnimations,
	wm.CapOutputPower,
	wm.CapOutputMirror,
	wm.CapOutputHdr,
	wm.CapMonitorConfig,
	wm.CapWindowFloat,
	wm.CapSessionExit,
	wm.CapNightLight,
	wm.CapTouchpadToggle,
	wm.CapPaletteBorder,
	wm.CapKeyboardLayoutSwitch,
	wm.CapConfigReload,
	wm.CapSystemLidEdgeSource,
}

// appOverrideFields are the per-app look fields genWindowRules writes: opacity
// alone, as an alpha rule; apply reports every other one unhonored.
var appOverrideFields = []string{"opacity"}

// The packages ryoku-desktop-wayfire is made of: the variant package itself,
// wayfire, the X11 bridge wayfire runs natively, the wlroots portal backend
// that serves screencast and screenshot outside Mutter, wlsunset for gamma,
// and wlopm for Wayfire's output-power protocol. wlr-randr supplies the mode
// inventory Wayfire's JSON IPC omits. Kept in step with that package's depends
// (release/packages/ryoku-desktop-wayfire/PKGBUILD); this is the list a
// switch away from wayfire reclaims, minus ryoku-desktop, which is shared
// with the compositor that replaces it.
var compositorPackages = []string{
	"ryoku-desktop-wayfire",
	"wayfire",
	"xorg-xwayland",
	"xdg-desktop-portal-wlr",
	"wlsunset",
	"wlopm",
	"wlr-randr",
}

// The scratch workspace is live only with both wsets methods loaded. Deck
// layout is live only when its separately shipped module exists.
func runCaps() error {
	caps := wm.Caps{
		Name:     wm.ProviderWayfire,
		Version:  probeVersion(),
		Instance: instanceHandle(),
		Supports: availableCaps(),
		// Workspaces live in a per-output grid created on demand, so
		// presenting them as stable numbered slots would be a lie.
		WorkspaceModel: wm.WorkspaceModelDynamic,
		// wm.wayfire.* keys stay in the store untouched while another
		// compositor is active, so they are still there on the way back.
		SettingDomains:    []string{"desktop", "wm." + wm.ProviderWayfire},
		ConfigFiles:       wm.ConfigFiles(wm.ProviderWayfire),
		GeneratedFiles:    wm.GeneratedConfig(wm.ProviderWayfire),
		PortalBackend:     "wlr",
		NightLightProcess: "wlsunset",
		Packages:          compositorPackages,
		WindowRuleActions: windowRuleActions,
		AppOverrideFields: appOverrideFields,
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(caps)
}

// Kept as data so capability probes can use an isolated module tree instead
// of inheriting plugins installed on the host running them.
var wayfirePluginDirs = func() []string {
	var dirs []string
	if configured := os.Getenv("WAYFIRE_PLUGIN_PATH"); configured != "" {
		dirs = append(dirs, filepath.SplitList(configured)...)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "lib", "wayfire"))
	}
	return append(dirs, "/usr/local/lib/wayfire", "/usr/lib/wayfire")
}()

func availableCaps() []wm.Capability {
	out := append([]wm.Capability(nil), capsManifest...)
	if (!live() && wayfirePluginAvailable("wsets")) ||
		specialWorkspaceMethodsAvailable() {
		out = append(out, wm.CapSpecialWorkspace)
	}
	if wayfirePluginAvailable("depthdeck") {
		out = append(out, wm.CapDeckLayout)
	}
	return out
}

func specialWorkspaceMethodsAvailable() bool {
	return wayfireMethodsAvailable("wsets/set-output-wset", "wsets/send-view-to-wset")
}

func wayfireMethodsAvailable(wants ...string) bool {
	if !live() {
		return false
	}
	raw, err := request("list-methods", nil)
	if err != nil {
		return false
	}
	var reply struct {
		Methods []string `json:"methods"`
	}
	if json.Unmarshal(raw, &reply) != nil {
		return false
	}
	have := make(map[string]bool, len(reply.Methods))
	for _, method := range reply.Methods {
		have[method] = true
	}
	for _, want := range wants {
		if !have[want] {
			return false
		}
	}
	return true
}

func wayfirePluginAvailable(name string) bool {
	for _, dir := range wayfirePluginDirs {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, "lib"+name+".so")); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
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

// Package wm is the seam between the Ryoku desktop and the window manager under
// it. A provider binary implements the contract verbs, and nothing else needs
// to know which compositor is running.
//
// Consumers ask what the compositor can do, never which one it is. A compositor
// name in a conditional means a capability is missing; bin/ryoku-dev-verify-wm-isolation
// enforces that.
package wm

type Capability string

const (
	CapWorkspaces            Capability = "workspaces"
	CapSpecialWorkspace      Capability = "specialWorkspace"
	CapWorkspaceMoveToOutput Capability = "workspaceMoveToOutput"
	CapWindowWorkspaceMap    Capability = "windowWorkspaceMap"
	CapWindowGeometry        Capability = "windowGeometry"
	CapFocusHistory          Capability = "focusHistory"
	CapWindowRules           Capability = "windowRules"
	CapLayerRules            Capability = "layerRules"
	CapSubmap                Capability = "submap"
	CapGlobalShortcuts       Capability = "globalShortcuts"
	CapFocusGrab             Capability = "focusGrab"
	CapScreenShader          Capability = "screenShader"
	CapPlugins               Capability = "plugins"
	CapLiveConfigEval        Capability = "liveConfigEval"
	CapConfigReload          Capability = "configReload"
	CapAnimations            Capability = "animations"
	CapCursorSet             Capability = "cursorSet"
	CapNativeOverview        Capability = "nativeOverview"
	// CapOverviewBackdrop is set when the compositor can lift a layer-shell
	// surface into the backdrop it shows behind the workspaces in its overview,
	// so the shell's blurred wallpaper fills that region instead of a flat
	// colour. Named for what the user gets, not for a compositor: an overview
	// that cannot place a surface in its backdrop leaves this off and the shell
	// maps nothing there.
	CapOverviewBackdrop Capability = "overviewBackdrop"
	// CapOverviewState is set when the provider reports the native overview
	// opening and closing, so a wallpaper can play only while it is up.
	CapOverviewState        Capability = "overviewState"
	CapOutputPower          Capability = "outputPower"
	CapKeyboardLayoutSwitch Capability = "keyboardLayoutSwitch"
	CapMonitorConfig        Capability = "monitorConfig"
	// CapOutputMirror and CapOutputHdr describe per-output behaviours some
	// compositors offer and others cannot: cloning one output onto another, and
	// an HDR/wide-gamut colour pipeline. Named for the behaviour so the Hub gates
	// the control on the capability, never on which compositor is running.
	CapOutputMirror Capability = "outputMirror"
	CapOutputHdr    Capability = "outputHdr"
	CapWindowFloat  Capability = "windowFloat"
	CapTiledLayout  Capability = "tiledLayout"
	// CapColumnFill is set when the provider can tell the active window spans
	// its output's width, the full-width column a scrolling tiler makes.
	CapColumnFill  Capability = "columnFill"
	CapSessionExit Capability = "sessionExit"
	// CapNightLight is set when the provider can warm the screen to a colour
	// temperature and restore it, through the nightlight.on and nightlight.off
	// actions. The warm gamma is held by a detached backend the provider owns
	// (hyprsunset over Hyprland's CTM, wlsunset over wlr-gamma-control on
	// niri); NightLightProcess below names it so a consumer can tell the light
	// is on without knowing the compositor.
	CapNightLight Capability = "nightLight"
	// CapTouchpadToggle is set when the provider can lock the touchpad the FN
	// touchpad key asks for, through the input.touchpad action. One compositor
	// flips the device live, another records the intent in its config file; the
	// capability says it can be done, never how.
	CapTouchpadToggle Capability = "touchpadToggle"
	// CapPaletteBorder is set when the provider can recolour the window border
	// from the live palette on request, through the decoration.borderColors
	// action. One compositor pushes the colours into a running config, another
	// records them and regenerates the file it watches; the capability says the
	// border can follow the wallpaper, never how.
	CapPaletteBorder Capability = "paletteBorder"
	// CapPersistentScreenCapture is set when the compositor tolerates a
	// layer-shell surface that captures the whole screen and stays created for
	// the session's life, re-armed per use instead of rebuilt. A compositor
	// without it still answers one-shot captures (the overview, a window
	// preview, grim); it is the long-lived capturing surface that races output
	// enter and leave, and where the client is Qt the race takes the whole
	// shell down while resolving a screen that no longer exists. Named for the
	// surface shape the consumer wants, never for a compositor: a shell that
	// wants a live blurred backdrop behind a transient panel asks for this and
	// falls back to a solid panel when it is absent.
	CapPersistentScreenCapture Capability = "persistentScreenCapture"
	// CapKeyboardGrabSharesPointer is set when a layer-shell surface can hold
	// exclusive keyboard focus while the pointer still reaches every other
	// surface through its own input region. A compositor without it gives the
	// surface holding the grab every pointer event too, so a surface that wants
	// keys beside other clickable surfaces (the Stage Editor's canvas under its
	// toolbar and catalogue) asks for on-demand focus there instead, and takes
	// the keyboard as the pointer crosses it.
	CapKeyboardGrabSharesPointer Capability = "keyboardGrabSharesPointer"
)

// All is every capability, so a caps payload can carry an explicit boolean for
// each one and a consumer never has to tell absent from false.
func All() []Capability {
	return []Capability{
		CapWorkspaces, CapSpecialWorkspace, CapWorkspaceMoveToOutput,
		CapWindowWorkspaceMap, CapWindowGeometry, CapFocusHistory,
		CapWindowRules, CapLayerRules, CapSubmap, CapGlobalShortcuts,
		CapFocusGrab,
		CapScreenShader, CapPlugins, CapLiveConfigEval, CapConfigReload,
		CapAnimations, CapCursorSet, CapNativeOverview, CapOverviewBackdrop,
		CapOverviewState, CapOutputPower,
		CapKeyboardLayoutSwitch, CapMonitorConfig, CapOutputMirror,
		CapOutputHdr, CapWindowFloat,
		CapTiledLayout, CapColumnFill, CapSessionExit, CapNightLight, CapTouchpadToggle,
		CapPaletteBorder, CapPersistentScreenCapture, CapKeyboardGrabSharesPointer,
	}
}

// WorkspaceModel is how a workspace list should be presented. Fixed means stable
// numbered slots a user thinks of by number; dynamic means the set grows and
// shrinks per output, so numbering them would be a lie.
type WorkspaceModel string

const (
	WorkspaceModelFixed   WorkspaceModel = "fixed"
	WorkspaceModelDynamic WorkspaceModel = "dynamic"
)

type Caps struct {
	// Name and Version are diagnostics only. Branching on Name is the
	// anti-pattern this package prevents.
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	// Instance is an opaque per-session handle, string-compared and never
	// parsed. The shell daemon restarts mid-session and has to tell a stale
	// incumbent from a same-session double-start; only the compositor knows its
	// own instance. Empty when nothing is running.
	Instance       string         `json:"instance,omitempty"`
	Supports       []Capability   `json:"supports"`
	WorkspaceModel WorkspaceModel `json:"workspaceModel"`
	// ForeignAPIs are shell and IPC interfaces plugins may consume directly.
	// Compatibility scans compare their requirements with this list before a
	// third-party plugin is enabled or loaded.
	ForeignAPIs []ForeignAPI `json:"foreignApis"`
	// SettingDomains are the setting-key prefixes this provider honours in
	// apply. The Hub gates rows on them so no row is shown with no writer.
	SettingDomains []string `json:"settingDomains"`
	// ConfigFiles are the provider's user-editable config paths, relative to
	// ~/.config, for the Hub to offer as an escape hatch. Provider-owned so the
	// Hub never spells a compositor's file names.
	ConfigFiles []string `json:"configFiles,omitempty"`
	// GeneratedFiles are the config files apply authors, relative to ~/.config.
	// They are a pure function of the store, so a caller that restores a state
	// with no store removes them instead of emitting defaults the user never had.
	GeneratedFiles []string `json:"generatedFiles,omitempty"`
	// PortalBackend is the xdg-desktop-portal backend this compositor needs as
	// the preferred default. Doctor repairs portals.conf against it, so the
	// backend name lives with the compositor rather than in a reconciler.
	PortalBackend string `json:"portalBackend,omitempty"`
	// NightLightProcess is the comm name (<=15 chars, so it survives /proc/<pid>/comm
	// truncation) of the detached backend that holds the warm gamma while the
	// night light is on. Provider-owned like PortalBackend, because only a
	// provider knows which client it starts; the daemon scans /proc for this
	// name and the neutral script pgreps it. Empty when CapNightLight is absent.
	NightLightProcess string `json:"nightLightProcess,omitempty"`
	// Packages are the pacman packages this compositor is made of: the
	// compositor package and the satellites ryoku-desktop-<name> installs for
	// it, most significant first. Provider-owned because only a provider knows
	// what it is made of, and it is what lets a switch reclaim a compositor's
	// space without the CLI or the Hub ever spelling a package name. The
	// private dependencies these orphan with are pacman's to cascade, so they
	// are not listed here.
	Packages []string `json:"packages,omitempty"`
	// WindowRuleActions are the neutral window-rule action ids this provider's
	// config writer honours, in the order the Hub should offer them. Each
	// compositor accepts a different set, so the window-rules editor lists only
	// what the active one can actually apply, never a control that writes nothing.
	WindowRuleActions []string `json:"windowRuleActions,omitempty"`
}

// Has reports whether the provider can honour want. A zero Caps supports
// nothing, so a failed probe degrades rather than assuming.
func (c Caps) Has(want Capability) bool {
	for _, got := range c.Supports {
		if got == want {
			return true
		}
	}
	return false
}

// OwnsDomain matches on the dotted prefix, so "wm.hyprland" owns
// "wm.hyprland.plugins.enabled" without listing every leaf.
func (c Caps) OwnsDomain(key string) bool {
	for _, d := range c.SettingDomains {
		if key == d {
			return true
		}
		if len(key) > len(d) && key[:len(d)] == d && key[len(d)] == '.' {
			return true
		}
	}
	return false
}

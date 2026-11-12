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
	// CapWorkspaceFullscreenState means workspace frames authoritatively carry
	// their fullscreen bit. Without it the shell may enrich the active workspace
	// from a matching foreign-toplevel handle.
	CapWorkspaceFullscreenState Capability = "workspaceFullscreenState"
	CapWindowGeometry           Capability = "windowGeometry"
	CapFocusHistory             Capability = "focusHistory"
	CapWindowRules              Capability = "windowRules"
	CapLayerRules               Capability = "layerRules"
	CapSubmap                   Capability = "submap"
	CapGlobalShortcuts          Capability = "globalShortcuts"
	CapFocusGrab                Capability = "focusGrab"
	// CapWorkspaceSlideStyle is set when the compositor reports how it slides
	// between workspaces (axis, distance, curve, duration, fade), so a
	// full-screen overlay can match the real motion instead of assuming one.
	// Without it the overlay keeps its built-in curve.
	CapWorkspaceSlideStyle Capability = "workspaceSlideStyle"
	CapScreenShader        Capability = "screenShader"
	CapPlugins             Capability = "plugins"
	CapLiveConfigEval      Capability = "liveConfigEval"
	CapConfigReload        Capability = "configReload"
	CapAnimations          Capability = "animations"
	CapCursorSet           Capability = "cursorSet"
	CapNativeOverview      Capability = "nativeOverview"
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
	CapColumnFill Capability = "columnFill"
	// CapDeckLayout is set when the compositor can arrange a workspace as an
	// overlapping deck with the focused card raised. It gates the deck controls
	// without exposing which compositor plugin supplies that behaviour.
	CapDeckLayout  Capability = "deckLayout"
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
	// CapPersistentScreenCapture is set when the shell can capture a whole
	// output inside its own process without being taken down by it. Where the
	// compositor offers only wlr-screencopy, Quickshell takes a screen capture
	// by binding a second wl_output, Qt files that object as one of its own
	// screens and reads it after the capture is gone, so even a one-shot
	// capture can crash the whole shell. Captures that run in another process
	// (grim, a separate capture tool) are not affected. Named for what the
	// consumer wants, never for a compositor: a shell that wants a blurred
	// backdrop or a still of the screen asks for this and falls back to a
	// solid panel or no still when it is absent.
	CapPersistentScreenCapture Capability = "persistentScreenCapture"
	// CapKeyboardGrabSharesPointer is set when a layer-shell surface can hold
	// exclusive keyboard focus while the pointer still reaches every other
	// surface through its own input region. A compositor without it gives the
	// surface holding the grab every pointer event too, so a surface that wants
	// keys beside other clickable surfaces (the Stage Editor's canvas under its
	// toolbar and catalogue) asks for on-demand focus there instead, and takes
	// the keyboard as the pointer crosses it.
	CapKeyboardGrabSharesPointer Capability = "keyboardGrabSharesPointer"
	// CapSystemLidEdgeSource is set when the compositor receives switch devices
	// but exposes no ordered lid binding or IPC event. The active session's
	// clamshell owner then watches physical state and routes both edges through
	// the neutral output action.
	CapSystemLidEdgeSource Capability = "systemLidEdgeSource"
	// CapNestedSession is set when the provider can run a session of its
	// compositor inside a hidden parent through the nest verb: a screen of
	// its own for a program such as an agent that drives apps, which the
	// user's desktop never shows and the program's input never leaves. The
	// session must take virtual pointer and keyboard input and answer
	// wlr-screencopy, so a caller can drive it and see it.
	CapNestedSession Capability = "nestedSession"
)

// All is every capability, so a caps payload can carry an explicit boolean for
// each one and a consumer never has to tell absent from false.
func All() []Capability {
	return []Capability{
		CapWorkspaces, CapSpecialWorkspace, CapWorkspaceMoveToOutput,
		CapWindowWorkspaceMap, CapWorkspaceFullscreenState, CapWindowGeometry, CapFocusHistory,
		CapWindowRules, CapLayerRules, CapSubmap, CapGlobalShortcuts,
		CapFocusGrab, CapWorkspaceSlideStyle,
		CapScreenShader, CapPlugins, CapLiveConfigEval, CapConfigReload,
		CapAnimations, CapCursorSet, CapNativeOverview, CapOverviewBackdrop,
		CapOverviewState, CapOutputPower,
		CapKeyboardLayoutSwitch, CapMonitorConfig, CapOutputMirror,
		CapOutputHdr, CapWindowFloat,
		CapTiledLayout, CapColumnFill, CapDeckLayout, CapSessionExit, CapNightLight,
		CapTouchpadToggle, CapPaletteBorder, CapPersistentScreenCapture,
		CapKeyboardGrabSharesPointer, CapSystemLidEdgeSource, CapNestedSession,
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
	// Packages are the native catalogue packages this compositor is made of:
	// the compositor package and the satellites ryoku-desktop-<name> installs,
	// most significant first. Provider-owned because only a provider knows what
	// it is made of, and it is what lets a switch reclaim a compositor's space
	// without the CLI or the Hub ever spelling a package name. Private
	// dependencies are left to the native package manager's removal plan, so
	// they are not listed here.
	Packages []string `json:"packages,omitempty"`
	// WindowRuleActions are the neutral window-rule action ids this provider's
	// config writer honours, in the order the Hub should offer them. Each
	// compositor accepts a different set, so the window-rules editor lists only
	// what the active one can actually apply, never a control that writes nothing.
	WindowRuleActions []string `json:"windowRuleActions,omitempty"`
	// AppOverrideFields are the desktop.appOverrides fields (opacity, rounding,
	// borderSize, blur, shadow, dim, anim, opaque) this provider's config writer
	// applies, so App rules shows a look override only where it takes effect.
	AppOverrideFields []string `json:"appOverrideFields,omitempty"`
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

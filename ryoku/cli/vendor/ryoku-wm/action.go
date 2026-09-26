package wm

// Actions are semantic ("close this window"), not a dispatcher passthrough. A
// passthrough would re-export one compositor's command language through a
// neutral door and callers would be writing Hyprland dispatchers again.
//
// Renaming an action breaks user keybinds and scripts; treat these as public.

type Action string

const (
	// Window ids are whatever the state frames used; a caller round-trips one
	// and never parses it.
	ActionWindowFocus           Action = "window.focus"
	ActionWindowClose           Action = "window.close"
	ActionWindowFullscreen      Action = "window.fullscreen"
	ActionWindowFloat           Action = "window.float"
	ActionWindowMoveToWorkspace Action = "window.moveToWorkspace"

	// ActionWindowSummon raises an already-open window to the current
	// workspace and focuses it, matched by exact title. The desktop's summon
	// keybind brings a single-instance window forward from wherever it first
	// opened; a title is the only handle when every window of an app shares one
	// app id, so the app-focus key cannot tell them apart.
	ActionWindowSummon Action = "window.summon"

	// ActionAppFocus is for callers that only know what they launched.
	ActionAppFocus Action = "app.focus"

	ActionWorkspaceFocus         Action = "workspace.focus"
	ActionWorkspaceCycle         Action = "workspace.cycle"
	ActionWorkspaceMoveToOutput  Action = "workspace.moveToOutput"
	ActionWorkspaceToggleSpecial Action = "workspace.toggleSpecial"

	ActionSessionExit Action = "session.exit"
	ActionOutputPower Action = "output.power"

	ActionKeyboardCycleLayout Action = "keyboard.cycleLayout"

	// Submaps back the Hub's keybind recorder: it needs to capture a chord
	// without the session acting on it.
	ActionSubmapEnter Action = "submap.enter"
	ActionSubmapReset Action = "submap.reset"

	// Dispatched only with CapNativeOverview; otherwise the shell draws its own.
	ActionOverviewToggle Action = "overview.toggle"

	// ActionConfigAutoreload brackets a config swap so a half-written tree is
	// never picked up mid-rename.
	ActionConfigReload     Action = "config.reload"
	ActionConfigAutoreload Action = "config.autoreload"

	ActionCursorSet Action = "cursor.set"

	// ActionCursorReassert re-applies the store's resolved cursor theme to the
	// live compositor with no settings save: recolored cursor images (the
	// Material theme tracking a new palette) stay cached in the compositor's
	// memory until the theme is set again, which is why a palette change looked
	// like it never reached the pointer. No capability gate: a provider that
	// cannot re-assert answers with an error its caller ignores.
	ActionCursorReassert Action = "cursor.reassert"

	// Transient live overrides, not persisted settings: the launcher suppresses
	// focus-follows-mouse while it is open and restores it on close.
	// ActionFocusFollowsMouse prints the PREVIOUS value so the caller can hand
	// it back, which keeps the restore exact without the provider holding state
	// across two separate invocations.
	ActionScreenShader      Action = "decoration.screenShader"
	ActionFocusFollowsMouse Action = "input.focusFollowsMouse"

	// ActionWorkspaceLayout sets the tiling layout for one workspace.
	ActionWorkspaceLayout Action = "workspace.layout"

	// ActionBorderColors is split from apply because it runs on every wallpaper
	// change, and rewriting plus reloading the config for two colours is slow
	// and visible.
	ActionBorderColors Action = "decoration.borderColors"

	// ActionGameMode strips the compositor's decorations for a latency-first
	// gaming pass (on) and reloads the config to put them back (off). It rides
	// the live config eval a file-only compositor has no equivalent for, so it
	// needs CapLiveConfigEval and a compositor without it leaves the look alone.
	ActionGameMode Action = "decoration.gameMode"

	// ActionNightLightOn takes one arg, the colour temperature in Kelvin, and
	// replaces any running backend with one warmed to it. ActionNightLightOff
	// stops the backend; the compositor restores the gamma once the client is
	// gone. Both need CapNightLight.
	ActionNightLightOn  Action = "nightlight.on"
	ActionNightLightOff Action = "nightlight.off"

	// ActionInputTouchpad locks the touchpad the FN touchpad key drives: on,
	// off, toggle, status (prints on|off) or restore (re-assert a stored off
	// after a reload). A compositor with a live input override flips the device;
	// one whose input is config-only records the intent and re-emits it, so the
	// key behaves the same either way. Needs CapTouchpadToggle.
	ActionInputTouchpad Action = "input.touchpad"

	// ActionOutputCycle steps the output arrangement one position, the display
	// toggle key's job. ActionOutputEnable turns one named connector on or off.
	// Both need CapMonitorConfig.
	ActionOutputCycle  Action = "output.cycle"
	ActionOutputEnable Action = "output.enable"
)

// Capability returns what an action needs, so callers gate on one lookup
// instead of keeping their own table. Focusing and closing a window ride
// foreign-toplevel and need nothing.
func (a Action) Capability() Capability {
	switch a {
	case ActionWindowFloat:
		return CapWindowFloat
	case ActionWindowMoveToWorkspace, ActionWindowSummon, ActionWorkspaceFocus, ActionWorkspaceCycle:
		return CapWorkspaces
	case ActionWorkspaceMoveToOutput:
		return CapWorkspaceMoveToOutput
	case ActionWorkspaceToggleSpecial:
		return CapSpecialWorkspace
	case ActionSessionExit:
		return CapSessionExit
	case ActionOutputPower:
		return CapOutputPower
	case ActionKeyboardCycleLayout:
		return CapKeyboardLayoutSwitch
	case ActionSubmapEnter, ActionSubmapReset:
		return CapSubmap
	case ActionOverviewToggle:
		return CapNativeOverview
	case ActionConfigReload, ActionConfigAutoreload:
		return CapConfigReload
	case ActionCursorSet:
		return CapCursorSet
	case ActionBorderColors:
		return CapPaletteBorder
	case ActionFocusFollowsMouse, ActionGameMode:
		return CapLiveConfigEval
	case ActionScreenShader:
		return CapScreenShader
	case ActionWorkspaceLayout:
		return CapTiledLayout
	case ActionNightLightOn, ActionNightLightOff:
		return CapNightLight
	case ActionInputTouchpad:
		return CapTouchpadToggle
	case ActionOutputCycle, ActionOutputEnable:
		return CapMonitorConfig
	}
	return ""
}

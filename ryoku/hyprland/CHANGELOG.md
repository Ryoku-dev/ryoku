# Changelog: ryoku/hyprland/

## Unreleased

### Changed
- **`binds.lua` carries the shared catalogue's new shortcuts.** Page Up/Down
  workspace navigation and sending, screen focus and send with Super+Alt and
  its Shift/Ctrl variants, Alt+Tab for the last window, Super+T group toggle,
  Super+D maximise, Super+C centre, Super+[ ] move-window-or-group, and the
  number pad for workspaces with both NumLock keysyms. Every one goes through
  `K()` so it stays rebindable. The provider's `binds` verb now parses this
  file itself and reports the legend to the Hub (`modules/binds.lua`).
- **`scripts/` keeps only Hyprland's own helpers.** The compositor-neutral leaf
  scripts (`ryoku-app`, the `ryoku-cmd-*` tools, the recorder helpers, folder
  tinting, sysinfo, clamshell) moved to the shell payload and the base hardware
  helpers so every box ships them; `ryoku-monitor`, `ryoku-workspace` and the
  new `ryoku-cursor-track` stay here because they speak Hyprland's IPC. The
  touchpad lock and the game-mode decoration strip are provider actions now
  (`modules/binds.lua`, `modules/touchpad.lua`).
- **`hypridle.conf` is generated, not shipped, and covers idle timers alone.**
  `ryoku-idle` renders it from the Hub's idle policy into `~/.config/ryoku`, with
  screen power routed through the window-manager seam and hypridle's own sleep
  inhibitor off. Suspend timers call the shell's fail-closed transaction, which
  owns qylock, login1 protection, output wake and lighting recovery.
- **The lid switch runs the shared secure policy before a docked panel handoff.**
  Both binds go through `ryoku-clamshell lid close`/`lid open`. A non-docked
  close calls `ryoku-shell suspend` and performs no late panel change after
  resume. Verified live docked clamshell deliberately remains unlocked and
  awake because closing the panel is only an output handoff; opening clears only
  the panel's disabled flag, so its configured mode, scale and position survive
  the round trip. The binds stay live while the session is locked
  (`modules/lid.lua`).

### Fixed
- **Output power goes through the seam as an explicit on/off, and re-enabling a
  panel keeps its layout.** The idle policy's screen-off stage used to shell out
  to `hyprctl dispatch dpms`; it now runs the `output.power` action with an
  explicit `on`/`off`, so the same policy serves every compositor and the shell's
  wake guard can re-assert "on" idempotently. Hyprland's native key-press and
  pointer-motion DPMS wake options are enabled too, so a dead or late idle client
  cannot strand a black panel (`modules/misc.lua`). `output.enable` used to
  re-enable a connector by authoring a fresh `preferred, auto, 1` monitor rule,
  which reset the mode, position and scale the user had saved for it. It now
  toggles only the rule's `disabled` state and leaves every layout field intact
  (`wm/hyprland/act.go`).
- **Maximize keybinds work again.**
  Ryoku no longer resets every Hyprland mode-1 fullscreen state to normal.
  The old handler worked around Hyprland #13322, which is fixed upstream in
  Hyprland 0.56.0. Removing the workaround restores native maximize behavior
  while leaving fullscreen handling to Hyprland (`hyprland.lua`; removed
  `modules/fullscreen.lua`).

- **Hiding the scratchpad no longer makes the next bar panel pop it open.**
  Super+Alt+H toggled the special workspace through Hyprland directly, which
  leaves keyboard focus on the window it just hid. Any surface that then takes
  and releases a keyboard grab (a qsbar panel, the bar settings menu, a menu
  dismissed by clicking outside) hands focus back to that window, and focusing a
  window on a special workspace shows the workspace: closing a panel revealed the
  scratchpad. The bind now goes through `ryoku-workspace scratch`, which hands
  focus to a window that is actually on screen when it hides the scratchpad, the
  same care the `hide` command already took. It focuses a window rather than the
  workspace on purpose: focusing an empty workspace leaves Hyprland with nothing
  to take the focus, so it keeps the hidden window and re-reveals the scratchpad
  on the spot (`modules/binds.lua`, `scripts/ryoku-workspace`).

- **A chosen icon theme survives login and wallpaper changes.**
  `ryoku-cmd-folders` (run at login and on every palette change) set the
  icon theme back to `ryoku-folders` whenever it differed, so a theme picked
  in the Hub or with gsettings reset on the next login. It now takes the
  setting over only from the shipped defaults (Papirus, Adwaita, hicolor, a
  stale generation name) and leaves any other choice alone
  (`scripts/ryoku-cmd-folders`).


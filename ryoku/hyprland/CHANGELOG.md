# Changelog: ryoku/hyprland/

## Unreleased

### Added
- **Super+Alt+Space opens the Rashin app.** The companion window summons the
  single-instance way: a second press focuses the open window instead of
  spawning a twin, and a window rule floats and centres it like the other
  app windows (`modules/binds.lua`, `modules/window_rules.lua`).

### Changed
- **Login raises the reload cover.** The session autostart chain calls
  `ryoku-reload-cover begin boot` after the environment push and before
  `session-start` restarts the shell services, so the desktop is covered from
  the greeter handoff until the loaded shell releases it
  (`modules/autostart.lua`).
- **Super+Q asks first when iRiS is set to.** The close bind reads
  `shell.json` in place and only goes through the shell when the iRiS bar
  style is active with its close confirmation on; every other close stays a
  native dispatch (`modules/binds.lua`).
- **Super+Shift+S and Print open the ryoshot region front on every bar style.**
  ryoshot now carries the iNiR-style front (Shot, Edit, OCR, Search, Record,
  colour pick) itself, so the screenshot binds always launch it rather than
  routing to the iRiS region toolbar when iRiS is the bar style
  (`modules/binds.lua`, `modules/ryoshot.lua`).

### Fixed
- **Hyprland reloads no longer duplicate `~/.local/bin` in PATH.** Reapplying
  the environment keeps the directory first without growing PATH on each
  config reload (`modules/env.lua`).
- **Arrow keys still move focus when a window is maximized.** Super+D
  maximizes, and with the compositor's default the direction focus binds then
  refuse to leave a maximized or fullscreen window: on the scrolling layout
  the maximized window covers the tape, so the neighbour beside it becomes
  unreachable until the state clears. Direction focus now moves to the
  neighbour and leaves the maximized window maximized (`modules/binds.lua`).
- **Ryoku Settings opens inside the work area.** Its full-page float was sized
  from the whole monitor, so it slid under whatever the shell reserves (the
  iRiS frame's edges, a bar). It now fits the work area with the same gaps and
  border a tiled window gets (`modules/window_rules.lua`).
- **The update and rollback log floats above Ryoku Settings.** Both are
  launched from the Settings window itself, and a tiled terminal always
  renders under a float, so the run's output hid behind the 99% page until
  it finished (#288). The launches name the `dev.ryoku.update` window class and
  a rule floats, sizes and centres it like every other tool terminal
  (`modules/window_rules.lua`).

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


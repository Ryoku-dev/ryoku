# Changelog: ryoku/hyprland/

## Unreleased

### Fixed
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


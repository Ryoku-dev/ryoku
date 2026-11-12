# The Ryoku Desktop

Ryoku is a set of surfaces that share one shell and one design language. You do
not need to know which process draws each one to use the desktop, but the map
helps when you want to change something.

## The parts you see

- **Shell:** Quickshell draws the desktop surfaces on every monitor. Go daemons
  provide state and own validated settings.
- **Bar:** the edge instrument for workspaces, time, media, network, audio,
  power, and installed bar plugins. QS Bar is the default style.
- **Dock:** a separate app surface that works with every bar style. It can pin
  apps, show running windows, magnify on hover, and hide to a peek strip.
- **Launcher:** press Super+Space to search apps, open windows, files, actions,
  calculations, packages, media, and web search from one field.
- **Ryoku Settings:** press Super+comma for the page-wide settings app. It owns system,
  shell, and active window-manager settings, with unsupported controls omitted.
  `ryoku-shell hub open <page-id>` opens one page, for example `bar`, `power`
  or `shortcuts`.
- **Desktop stage:** wallpaper, clock, weather, notes, system stats, music, and
  installed desktop plugins live here. Widgets can be moved and resized in
  place.
- **Controls:** press Super+Escape for activity, connections, levels, extensions,
  and hold-to-activate session actions.
- **Rashin:** press Alt+Space for optional read-only Ask. The Ryoku vault, wiki,
  skill, and source map belong to the desktop and remain when Rashin is off.

## Keybinds worth learning

| Keys | Opens or does |
|---|---|
| Super+Space | App launcher and command palette |
| Super+Tab | Workspace overview |
| Super+Escape | Controls |
| Super+comma | Ryoku Settings |
| Super+Shift+D | Stage Editor on the focused screen |
| Super+W | Wallpaper and theme picker |
| Super+V | Clipboard history |
| Super+Shift+S | Screenshot and recording action bar |
| Super+L | Lockscreen |
| Alt+Space | Rashin bar |
| Super+K | Kit: keys, commands, phone, developer tools, and typing test |

Ryoku Settings > Windows > Keyboard shortcuts is the source to use when a chord should change.
Kit is the quickest way to discover the rest: Keys, Commands, Phone, Dev tools,
and Typing test share the same search field.

## Settings have clear homes

Use Ryoku Settings for the window manager (Windows), displays and input
(Devices), power and graphics (System), the lock screen (Appearance), the
launcher (Desktop), screen capture (Apps), and the rest of the system settings.
Use QS Bar Settings for the bar layout, widgets,
and dock. Use the Super+W picker for wallpaper and theme. Those surfaces call the
same owners that command-line tools use, so they do not compete over config.

To learn which compositor backs the desktop and what it can do:

```bash
ryoku wm status
ryoku wm caps
```

Continue with [Hyprland in Lua](hyprland-lua.md) or [niri in
KDL](niri-kdl.md) only when Ryoku Settings cannot express a raw config change. See
[Rashin](rashin.md) for the two assistant lanes and approval choices.

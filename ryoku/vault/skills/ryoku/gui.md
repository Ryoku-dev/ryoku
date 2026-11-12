# The Ryoku GUI map

Ryoku has a GUI for almost every setting: Ryoku Settings for system and shell
config, the wallpaper picker for looks, the Stage Editor for the desktop and the
dock, and the active subshell or bar style's own settings window for its bar.
When a user asks how to change something, name the surface first, then give
the command the page wraps (SKILL.md, "Answer policy: GUI first"). This file is
the lookup from an intent to its surface, its keybind, and the command behind it.

## Opening Ryoku Settings

Ryoku Settings is the settings app (window title "Ryoku Settings"; internally
the `ryoku-hub` backend and `qs -c hub`). Super+comma opens it. A plain open
shows Home; to jump straight to a page, deep-link it by its page id:

```bash
ryoku-shell hub open              # Ryoku Settings, on Home
ryoku-shell hub open shortcuts    # straight to Windows > Keyboard shortcuts
ryoku-shell hub open power        # System > Power & battery
```

The ids are the Settings page ids in the table below. `hub open <page-id>/<row-id>`
lands on one row, e.g. `ryoku-shell hub open window-style/deck-frost`. Old
section ids from before the rebuild still resolve through the alias table in
`registry.js`, but always use the new ids. `widgets`, `stage`, `visualizer`
and `desktop-scene` open the Stage Editor instead. Escape steps back (out of
search, then out of the page) and then closes Ryoku Settings.

## The Settings pages

The rail lists nine categories; each holds the pages below, in this order.
`hub open <id>` is shorthand for `ryoku-shell hub open <id>`. The page list, ids,
search keywords and the alias table for old ids live in
`ryoku/hub/quickshell/registry/registry.js`; a page's rows are in
`schema/<page-id>.js`, and a page with its own layout is
`pages/<Name>.qml`, all under `ryoku/hub/quickshell/`.

| Category | Page | `hub open` id | What it owns |
|---|---|---|---|
| Appearance | Wallpaper & colours | `wallpaper-colours` | hands off to the wallpaper picker's Colours tab: palette, light and dark, accent |
| Appearance | Window style | `window-style` | corners, borders, opacity and dimming, blur, shadows, glow, title bars, and blur and dim behind the focused window |
| Appearance | Fonts & icons | `fonts-icons` | the system font and text size, monospace font, icon theme, cursor theme and size |
| Appearance | Animations | `animations` | animation curves and speed, wobbly windows, focus flash, reduce motion |
| Appearance | Lock screen | `lock-screen` | the lock screen's look (its qylock skin) |
| Desktop | Bar | `bar` | the gallery of bar styles and subshells, the displays that show the bar, the active style's options and its own settings window, and the parts installed items bring (dock designs, desktop widgets, launchers) |
| Desktop | Desktop & dock | `desktop-dock` | the desktop stage, its widgets and visualizer, the dock, the reload cover |
| Desktop | Quick settings | `quick-settings` | the Super+Escape panel: its design, sections and toggles |
| Desktop | Notifications & pop-ups | `notifications` | the notification design, the volume and brightness pop-ups (OSD), do not disturb, a test for each |
| Desktop | Sounds | `sounds` | desktop sounds and alerts, key sounds |
| Desktop | App launcher | `launcher` | the Super+Space launcher: which one, how it looks and behaves, favourites, hidden apps |
| Desktop | Clipboard | `clipboard` | clipboard history and clearing it |
| Windows | Window behaviour | `window-behaviour` | tiling layout, gaps, focus (focus follows mouse), new windows, workspaces, overview, switcher, desktop zoom |
| Windows | Keyboard shortcuts | `shortcuts` | every shortcut |
| Windows | App rules | `app-rules` | window rules, layer rules and per-app overrides (gated: window-rules support) |
| Windows | Window manager | `window-manager` | which window manager runs, switching it, and compositor plugins: status, Rebuild, add from a repository (gated: a backing provider) |
| Devices | Displays | `displays` | monitors, resolution, refresh rate, scale, arrangement, adaptive sync, HDR, night light, desktop size (gated: monitor config) |
| Devices | Keyboard | `keyboard` | layout, key repeat, caps lock, compose, num lock, remapping |
| Devices | Mouse & touchpad | `mouse-touchpad` | pointer speed and acceleration, scrolling, tap, gestures, extra mouse buttons |
| Connections | Wi-Fi & internet | `wifi` | Wi-Fi, ethernet, hotspot, the Wi-Fi backend, blocking all connections |
| Connections | Bluetooth | `bluetooth` | pairing and Bluetooth devices |
| Connections | Phone | `phone` | the paired Android phone (KDE Connect, mirroring, webcam) |
| Apps | Default apps | `default-apps` | the browser, terminal, editor, file manager and other default roles |
| Apps | Startup apps | `startup-apps` | autostart, session and login behaviour, environment variables |
| Apps | Terminal | `terminal` | the login shell, the prompt, the fastfetch splash |
| Apps | Screen capture | `screen-capture` | screenshots and screen recording |
| Apps | Voice typing | `voice-typing` | dictation |
| Apps | Rashin | `rashin` | opt-in Ask, Agent and Index controls |
| Apps | Add-ons | `add-ons` | Ryostore add-ons, shell plugins and bundles |
| Accounts | Users | `users` | the people on this computer, their pictures and permissions, and profile accounts |
| Accounts | Sign-in & security | `sign-in` | the keyring, fingerprint, security keys |
| System | Power & battery | `power` | power mode, battery and charge limit, power saver, the idle timers (dim, lock, screen off, sleep), CPU tuning |
| System | Graphics | `graphics` | GPU mode, MUX, GPU tuning, passthrough VMs |
| System | Performance | `performance` | low-power behaviour and unloading the live wallpaper |
| System | Time & language | `time-language` | language, regional formats, the 24-hour clock, time zone, location, units (weather units too) |
| System | Backup & import | `backup-import` | importing another config, backup, export and restore |
| System | About | `about` | version, edition, hardware, the system card, credits |
| Updates | Updates | `updates` | system updates, what's new, rollback and snapshots |

When someone asks about the look of a notification card or the volume and
brightness pop-ups (the OSD), send them to Ryoku Settings > Desktop >
Notifications & pop-ups (`ryoku-shell hub open notifications`); for the look of
the Super+Escape panel, to Ryoku Settings > Desktop > Quick settings
(`ryoku-shell hub open quick-settings`). Notifications & pop-ups keeps
installation separate from selection and has a test for each surface; Show a
test pop-up flashes the volume display through `ryoku-shell ipc call osd
preview`.

When someone says the desktop blurs or darkens behind an open app, on any
compositor, send them to Ryoku Settings > Appearance > Window style > *Blur and
dim behind windows* (`ryoku-shell hub open window-style/deck-frost`). Off
removes both the blur and the darkening; it is not the window shadow, which is
the Shadows section on the same page.

## Installable profiles

Profile discovery starts in **Ryostore > Profiles**. **Install** places the
official profile definition. **Create account** then opens **Ryoku Settings >
Accounts > Users > Create a profile account**, where the user chooses the separate
standard account, tools and extra packages, reviews the complete plan, and
approves it.

An applied profile account has a Profile tools section on its Users card:

- **Edit tools** changes core, optional and extra packages through a new plan.
- **Reset settings** previews and restores profile-managed files. It never
  touches user-data folders or anything the profile did not lay.
- **Checkpoints** saves, lists, reverts or deletes btrfs home checkpoints.
  Revert is unavailable while the account is logged in.
- **Remove profile** previews packages and system pieces before offering
  Archive home, Keep account or Erase home.

The profile account's own Accounts > Users view also has a This profile account
section for reset preview, reset and checkpoint save. A profile account stays standard
and cannot manage users.

## Surfaces outside Ryoku Settings

Wallpaper, theme, the dock, the active item's bar layout, the launcher,
Quick settings, ryoshot, and Ask are outside Ryoku Settings. Reach them here:

| Intent | GUI surface | How to open | The command behind it |
|---|---|---|---|
| Change wallpaper or theme | Wallpaper picker | Super+W | `ryogami wallpaper ui` (picker), `ryogami wallpaper set\|next\|random` |
| Pick the subshell or bar style | Ryoku Settings > Desktop > Bar | `ryoku-shell hub open bar` | `ryostore install <subshells\|barstyles> <id>`, then `ryoku-shell barstyle <id>` |
| The active item's own settings (bar layout, modules, look) | Its settings window: Ryoku Settings > Desktop > Bar > Open its settings, or the item's own click (Orpheus: left-click its 力 logo) | only while that item is active | the call desktop.md lists for it, e.g. `ryoku-shell ipc call orpheus settings ""`, `ryoku-shell ipc call kairos settings`, `ryoku-shell ipc call qsbar settings ""` |
| QS Bar layout and widgets (while QS Bar is active) | QS Bar Settings | the bar's launcher mark, or `ryoku-shell ipc call qsbar settings <route>` | `ryoku-shell ipc call qsbar ...` (see bar.md) |
| Dock | Stage Editor (Super+Shift+D), Dock section; Ryoku Settings > Desktop > Bar > Parts it brings > Dock design lands there too | `ryoku-shell ipc call desktop editSection dock ""` | `ryoku-shell dock ...` (see bar.md) |
| Launch an app | App launcher | Super+Space | the shell launcher (Ryoku Settings > Desktop > App launcher tunes it and picks launchers installed items provide) |
| System activity, connections, levels, and session actions | Quick settings (its look is picked in Ryoku Settings > Desktop > Quick settings) | Super+Escape | `ryoku-shell quicksettings` |
| Screenshots and recordings | ryoshot capture bar | Super+Shift+S | `ryoku-shell screenshot` |
| Read-only system answers and general free-model use | Rashin Ask bar | Alt+Space | `rashin ask <question>` |
| Persistent external-harness sessions | Rashin app, Agent tab | Super+Alt+Space | `rashin-app` |
| Install or create a separate account profile | Ryostore > Profiles, then Ryoku Settings > Accounts > Users | open Ryostore, choose Profiles | `ryostore settings profiles <id>` hands off to Ryoku Settings' `users fromProfile` IPC |

An item that is installed but not active is not loaded, so its settings window
and IPC target do not exist; the Bar page offers Use for it instead. QS Bar
Settings routes are `bars`, `identity`, `layout`, `widgets`, `dock` and
`community`; its Dock page hands off to the Stage Editor. bar.md covers
subshells, bar styles and the dock in full.

## Gated pages

Ryoku Settings hides a page that cannot work on this system instead of showing
dead settings. App rules needs window-rules support (`windowRules`), Window
manager needs a backing provider, and Displays needs monitor config
(`monitorConfig`); each appears only when the running compositor backs it.
Inside a page, a section the window manager cannot back says why in place of
its controls (Window style's Blur, Shadows, Glow and Title bars, for example).
There is no global advanced switch: the less common rows of a section sit
behind its own More options. A setting you change for a compositor applies
only after switching to that compositor.

## A worked answer

The user asks: "how do I make the screen lock after ten minutes?" The GUI-first
answer names the page first:

> Open Ryoku Settings with Super+comma and go to System > Power & battery, then
> set the lock timer under its idle timers to ten minutes, or jump straight
> there with `ryoku-shell hub open power`.

When the user asks YOU to set it, drive the idle daemon directly
(`ryoku-power idle set ac.lockSec 600`, then `ryoku-idle apply`), then tell them
it now locks after ten minutes and that Ryoku Settings > System > Power &
battery is where they see or change it. Same shape for wallpaper: point them at
Super+W first; when you set it yourself, use `ryogami wallpaper set <path>` and
tell them the picker is Super+W.

# ryoku/hub/

Ryoku Settings: the desktop's central GUI control center. A native Qt6/QML app
(Quickshell, not a webview) with a grouped, Kirigami-style sidebar and a global
fuzzy search, opened with `Super + ,`. It floats and centres on top of the
current windows. The product name is **Ryoku Settings**; the internal binary,
config directory, and `qs -c hub` invocation keep the original `hub` name.

It is where you edit anything the Hyprland (Lua) config drives, plus the Ryoku
shell, in one place: monitors, appearance, input, keybinds, window rules,
autostart, environment, the shell's look, the lock screen, and the update channel.

Every page is built to one design contract: the monochrome instrument sheet
(palette, type, motion, the control taxonomy, the poster/ornament layer, the
art pipeline) is specified in `.beta18/DESIGN.md`, and how the brand language
applies across the whole desktop is in `docs/ui-ux.md`. Read those before
changing a page's look.

## Layout

- `backend/` One Go program, `ryoku-hub`, the data plane. The QML front end shells
  out to it the same way the rest of the desktop talks to `ryoku-shell`:
  - `ryoku-hub keybinds` parses the live Hyprland binds
    (`~/.config/hypr/modules/binds.lua`) into categorised, display-ready JSON.
  - `ryoku-hub desktop get|defaults|save|preview|restore` reads and writes the
    system-settings override document and generates the Lua the live config loads
    (see "The override model" below). `cursors` and `layouts` enumerate installed
    cursor themes and X11 keyboard layouts for the pickers.
  - `ryoku-hub config get|set <key> [value]` persists hub UI state as TOML at
    `~/.config/ryoku/hub.toml` (last open section, update-check cadence).
  - `ryoku-hub reload-cover import <path>` copies a supported local image,
    animation, or video into managed user data and prints its descriptor;
    `ryoku-hub reload-cover prune [<managed-path>]` removes every managed
    reload-cover asset, or all but the validated managed path when given.
  - `ryoku-hub clipboard stats|prune` reports what the clipboard history
    occupies (items, text bytes, image bytes) as read from the shell daemon, and
    prunes it: `prune` drops every unstarred entry with its files and prints the
    refreshed report. The daemon owns the entries, so the Hub never measures or
    deletes around it.
  - `ryoku-hub lock list|set|apply-greeter <slug>` manages installed qylock
    themes: `list` is the local inventory, `set` writes the in-session
    preference and applies the SDDM greeter, and `apply-greeter` is the
    privileged half under `/usr/share/sddm/themes/ryoku`. RyoStore owns remote
    discovery and installation; Settings owns activation only.
- `quickshell/` The UI, hand-written Quickshell (QML), deployed to
  `~/.config/quickshell/hub` and launched with `qs -c hub`:
  - `shell.qml` the `FloatingWindow`; `Hub.qml` the app (rail + content + the data
    fetch and section persistence).
  - `NavRail` the sidebar: brand header, the global `SearchField`, the grouped
    section list with one sliding selection indicator, and a footer mark.
  - `HyprStore` the shared engine behind every Lua-editing page: it loads the full
    override document from the backend, holds an editable draft, previews scalar
    edits live (flash-free, via `hyprctl eval`), and persists on Save.
  - Page components, one per file: `DisplaysPage` (+ `MonitorTile`),
    `AppearancePage`, `LockscreenPage` (+ `LockscreenTile`), `InputPage`, `KeybindsPage` (+ `KeybindLegend`,
    `KeybindsEditor`), `WindowRulesPage`, `AutostartPage`, `EnvironmentPage`,
    `ShellSettingsPage`, `UpdatesPage`, and the reusable controls
    (`SettingSection`, `NumberField`, `SliderRow`, `Slider`, `ColorField`,
    `ToggleRow`, `ChoiceRow`, `Segmented`, `Dropdown`, `HubButton`, `Icon`).

## Sections

- **Displays** detect every connected monitor and arrange them on a drag canvas
  (edges snap), with per-monitor resolution, refresh, scale, Ryoku interface
  scale, bar and desktop-widget visibility, rotation, adaptive sync, mirroring,
  and enable/disable. Apply to the live session, or save a named
  profile keyed to the connected displays' hardware identity so it returns
  automatically when you plug them in again. Backed by `ryoku-monitor`.
- **Appearance** window gaps, rounding and corner softness, border thickness,
  active/inactive opacity, inactive dimming, blur (size, passes, X-ray,
  vibrancy, noise), shadows (range, sharpness), the window glow, tiling layout
  (dwindle, master, scrolling), edge resize and floating snap, animations
  (including wobbly windows for a spring in the drag, and the window open/close
  style), border colours (follow the wallpaper palette or fix them) with an
  optional rotating gradient border, and the cursor: theme, size, and
  hide-on-idle/typing. Its **Theme** tab picks the colour scheme -- Follow
  Wallpaper, the shipped Default, or one of the daemon's 57 named palettes as
  swatch cards writing `theme.theme` through the settings daemon (the same key
  the shell's sidebar theme picker reads and writes), plus the Material You
  (Matugen) engine and per-app theming; a **Comfort** tab controls backlight and
  the night light, and a **Rices** tab captures, imports, applies, exports, and
  removes installed whole-desktop looks. Its RyoStore action opens the separate
  catalogue for discovery and installation.
- **Lockscreen** lists installed qylock themes from RyoStore. Each tile previews
  the real lockscreen and can activate it for both the session lock and SDDM
  greeter; the system-path greeter step asks for a password through pkexec. The
  login/auth flow is untouched. RyoStore owns browsing and installation.
- **Add-ons** manages installed shell plugins and bundle components. Plugin
  enablement, placement, and settings remain live here; RyoStore owns plugin
  updates/removal assets and all browsing, while `ryostore-install` owns
  bundle component state and removal.
- **Animations** the live Hyprland animation tree (read via `hyprctl animations`)
  with per-leaf enable, speed, bezier, and style (pop-in, slide, fade variants),
  plus a visual bezier-curve editor that
  previews as you drag. Curves and overrides persist to `settings.lua` on Save.
- **Input** keyboard layout/variant/options, numlock, pointer feel (sensitivity,
  acceleration, left-handed, scroll speed, natural scroll, middle-click paste),
  touchpad behaviour (tap, tap-and-drag, finger-count clicks, scroll speed, and
  the workspace-swipe gesture with direction/distance tuning), and key repeat.
- **Keybinds** the full shortcut legend, read live from `binds.lua` so it never
  drifts, plus a Custom editor for your own shortcuts layered on top.
- **Window Rules** float, size, pin, place, or restyle windows by class or
  title, with the full effect set: opacity, blur/border/shadow/rounding/dim
  suppression, focus control, aspect ratio, idle inhibit, and app-request
  suppression.
- **Layer Rules** blur, dim, X-ray, or disable animations on layer-shell
  surfaces (bars, launchers, notification daemons) matched by namespace.
- **Autostart** commands run at login, after the base Ryoku autostart.
- **Environment** environment variables for the Hyprland session.
- **Shell** the live editor for the screen frame, the top island (its style: the
  classic fused island, a floating pill, or none, each with an optional
  reveal-on-hover), and the desktop visualiser (writes `~/.config/ryoku/shell.json`
  and `visualizer.json`).
- **Updates** the commits the checkout is behind on its channel, with an
  auto-check cadence.

## The override model

The Lua-editing sections never touch the shipped Hyprland modules (those are
re-laid by `ryoku materialize` on update). Instead:

- The editable source of truth is one JSON document at
  `~/.config/ryoku/hypr.json`, owned by `ryoku-hub`.
- From it, `ryoku-hub` generates `~/.config/hypr/settings.lua`, written only with
  the values that diverge from the shipped defaults, so an untouched setting falls
  through to the base module. `hyprland.lua` `require`s it after the base modules
  and before `user.lua`, so the GUI's tweaks override the defaults while a
  hand-written `user.lua` still wins.
- Editing a scalar (appearance/input/cursor) previews at once through
  `hyprctl eval` (no reload, no flash). Save persists the JSON, regenerates
  `settings.lua`, and reloads to lock in list changes (rules, binds, env,
  autostart) that an eval cannot undo. Revert and leaving a page restore the saved
  state.

Monitors are the exception: they are owned by `ryoku-monitor`, which writes
`monitors.lua` and stores profiles under `~/.config/ryoku/monitors/`. Displays
edits stage in the canvas and only touch the live screens on Apply.

## Search

A global fuzzy finder lives in the sidebar (focus it with `Ctrl + K`). Typing
searches content across every section: it ranks matching keybinds (each tagged
with its category) and lists matching section names you can jump to. The matcher
is a small subsequence scorer in `quickshell/fuzzy.js`.

## Deploy

`ryoku/shell/deploy.sh` builds `ryoku-hub` onto `PATH` and copies the quickshell
config to `~/.config/quickshell/hub`. The installer installs the prebuilt binary
and the config; `installation/iso/build.sh` prebuilds the binary into the image
payload. The `Super + ,` keybind and the float/centre window rule live in
`ryoku/hyprland/modules/binds.lua` and `window_rules.lua`.

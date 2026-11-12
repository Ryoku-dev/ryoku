# ryoku/hub/

Ryoku Settings: the desktop's central GUI control center. A native Qt6/QML app
(Quickshell, not a webview) with a rail of categories, each category's pages,
and a global search, opened with `Super + ,`. It floats and centres on top of
the current windows. The product name is **Ryoku Settings**; the internal binary,
config directory, and `qs -c hub` invocation keep the original `hub` name.

It is where you edit anything the window manager config drives, plus the Ryoku
shell, in one place: displays, window style, keyboard and mouse, shortcuts, app
rules, startup apps, the bar, the lock screen, and updates.
`ryoku-shell hub open <page-id>[/<row-id>]` opens it on one page; plain
`ryoku-shell hub open` shows Home.

How it is built and how to add a page, a row or a store is in `docs/hub.md`.
The design language every page is drawn in is Lithic (`docs/lithic/`), and how
it applies across the whole desktop is in `docs/ui-ux.md`. Read those before
changing a page's look.

## Layout

- `backend/` One Go program, `ryoku-hub`, the data plane. The QML front end shells
  out to it the same way the rest of the desktop talks to `ryoku-shell`:
  - `ryoku-hub keybinds` prints the keybind legend the active window-manager
    provider reports, as categorised, display-ready JSON.
  - `ryoku-hub desktop get|defaults|save|preview|restore` reads and writes the
    neutral desktop-settings document. `cursors`, `icons` and `layouts`
    enumerate installed choices for their pickers; `icon-theme set <theme>`
    validates and applies one icon theme to every GTK, Qt and shell consumer.
  - `ryoku-hub config get|set <key> [value]` persists hub UI state as TOML at
    `~/.config/ryoku/hub.toml` (last open page, update-check cadence).
  - `ryoku-hub reload-cover import <path>` copies a supported local image,
    animation, or video into managed user data and prints its descriptor;
    `ryoku-hub reload-cover prune [<managed-path>]` removes every managed
    reload-cover asset, or all but the validated managed path when given.
  - `ryoku-hub clipboard stats|prune` reports what the clipboard history
    occupies (items, text bytes, image bytes) as read from the shell daemon, and
    prunes it: `prune` drops every unstarred entry with its files and prints the
    refreshed report. The daemon owns the entries, so the Hub never measures or
    deletes around it.
  - `ryoku-hub lock list|set|apply-greeter` manages installed qylock themes:
    `list` is the local inventory, `set` writes the in-session preference and
    applies the SDDM greeter, and `apply-greeter` carries the global 12/24-hour
    preference through the privileged install under
    `/usr/share/sddm/themes/ryoku`. RyoStore owns remote discovery and
    installation; Settings owns activation only.
- `quickshell/` The UI, hand-written Quickshell (QML), deployed to
  `~/.config/quickshell/hub` and launched with `qs -c hub`:
  - `shell.qml` loads `ShellBody.qml` (the `FloatingWindow` and the IPC
    targets) by file URL so Qt's disk cache covers it; `Hub.qml` the app: the rail, the list
    pane with search, the work pane, navigation, and the placards.
  - `registry/registry.js` the categories and pages (ids, titles, search
    keywords) and the `aliases` table that sends an old page id to its new
    page; `registry/provider-map.js` places the window manager's own rows on
    the pages they belong to.
  - `schema/<page-id>.js` one page's sections and rows, drawn by
    `components/SettingsPage.qml`; `pages/<Name>.qml` a page with bespoke
    surfaces of its own, with its parts under `components/<page-id>/`.
  - `Singletons/Changes.qml` the one write path: shell settings over the bus,
    the window manager store through `ryoku-hub`, and backend commands, with
    undo, changed marks against defaults, and per-row errors.

## Categories and pages

- **Appearance**: **Wallpaper & colours** hands off to the wallpaper picker's
  Colours tab (the colour scheme, Matugen, per-app theming); **Window style**
  holds corners, borders (follow the wallpaper palette or fix them, with an
  optional rotating gradient), opacity and dimming, blur, shadows, the glow,
  and title bars; **Fonts & icons** holds fonts and text size, the installed
  icon-theme picker, and the pointer's theme and size; **Animations** is the
  live Hyprland animation tree with per-leaf enable, speed, bezier and style, a
  bezier-curve editor that previews as you drag, wobbly windows, focus flash,
  and reduce motion; **Lock screen** lists installed qylock themes from
  RyoStore, each tile previewing the real lockscreen and activating it for both
  the session lock and the SDDM greeter (the system-path greeter step asks for
  a password through pkexec; RyoStore owns browsing and installation).
- **Desktop**: **Bar** (the gallery of subshells and bar styles, and the
  settings of the one in use), **Desktop & dock**, **Quick settings** (the
  Super+Escape panel), **Notifications & pop-ups**, **Sounds** (including key
  sounds), **App launcher**, and **Clipboard**.
- **Windows**: **Window behaviour** holds tiling (dwindle, master, scrolling),
  gaps, new windows, focus, moving and resizing, workspaces, the overview, and
  desktop zoom; **Keyboard shortcuts** is the full shortcut legend, read live
  from the binds so it never drifts, plus your own shortcuts layered on top;
  **App rules** floats, sizes, pins, places or restyles windows by class or
  title and blurs, dims or unanimates layer-shell surfaces by namespace;
  **Window manager** switches window manager and manages Hyprland plugins.
- **Devices**: **Displays** detects every connected monitor and arranges them
  on a drag canvas (edges snap), with per-monitor resolution, refresh, scale,
  rotation, adaptive sync, mirroring, and enable/disable, the desktop size,
  saved layouts keyed to the connected displays' hardware identity so they
  return when you plug them in again, and the night light; backed by
  `ryoku-monitor`. **Keyboard** holds layouts, typing and key repeat, and special
  keys; **Mouse & touchpad** holds pointer feel, touchpad behaviour, gestures,
  and extra mouse buttons.
- **Connections**: **Wi-Fi & internet**, **Bluetooth**, and **Phone**.
- **Apps**: **Default apps**, **Startup apps** (commands run at login, after
  the base Ryoku autostart, and session environment variables), **Terminal**
  (login shell, prompt, and the fastfetch greeting), **Screen capture**,
  **Voice typing**, **Rashin**, and **Add-ons**, which manages installed shell
  plugins and bundle components: plugin enablement, placement, and settings
  stay live here; RyoStore owns plugin updates and removal and all browsing,
  while `ryostore-install` owns bundle component state and removal.
- **Accounts**: **Users** reads the host account capability, keeps standard
  accounts read-only, and sends administrator changes through `pkexec
  ryoku-host users`; passwords travel on stdin, never in process arguments.
  **Sign-in & security** holds the keyring, fingerprint, and security keys.
- **System**: **Power & battery**, **Graphics**, **Performance**, **Time &
  language** (language, regional formats, the clock, time zone, location and
  units), **Backup & import**, and **About**.
- **Updates**: the commits the checkout is behind on its channel, with an
  auto-check cadence.

## The window manager store

The window-manager pages never touch the shipped compositor config (that is
re-laid by `ryoku materialize` on update). Instead:

- The editable source of truth is one neutral JSON document,
  `~/.config/ryoku/desktop.json`, owned by `ryoku-hub`.
- An edit goes live at once through `ryoku-hub desktop preview`, which hands
  the draft to the active provider without saving it. Once edits pause,
  `Singletons/Changes.qml` saves it with `ryoku-hub desktop save`, and the
  provider writes its own generated config from it (`settings.lua` on
  Hyprland, `settings.kdl` on niri; see `docs/compositors.md`). A failed save
  puts the old value back live with `ryoku-hub desktop restore`.
- Undo, on the notice or on Home's recent changes, writes the previous value
  back the same way.

Monitors are the exception: they are owned by `ryoku-monitor`, which writes
`monitors.lua` and stores profiles under `~/.config/ryoku/monitors/`.

## Search

A global search sits above the list pane (focus it with `/`, `Ctrl + K`, or the
more familiar `Ctrl + F`). Typing searches every page title and keyword in
`registry/registry.js`, every row in each page's schema, and the hand-offs to
the wallpaper picker, and a result lands on its page with the row in view. The
ranking is `HubSearchIndex` from `Ryoku.Text`, with a synonym table in
`Hub.qml`.

## Deploy

`ryoku/shell/deploy.sh` builds `ryoku-hub` onto `PATH` and copies the quickshell
config to `~/.config/quickshell/hub`. The installer installs the prebuilt binary
and the config; `installation/iso/build.sh` prebuilds the binary into the image
payload. The `Super + ,` keybind and the float/centre window rule live in
`ryoku/hyprland/modules/binds.lua` and `window_rules.lua`.

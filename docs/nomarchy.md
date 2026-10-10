# Nomarchy

Nomarchy is Ryoku's Omarchy-compatible bar style. It keeps the public plugin
layout and command names used by Omarchy shell plugins while routing desktop
operations through Ryoku.

Select Nomarchy in Bar Studio. Ryoku then creates
`$XDG_DATA_HOME/ryoku/nomarchy`, seeds `~/.config/omarchy/shell.json`, and links
the compatibility commands into `~/.local/bin`. Switching to another bar style
removes only those Ryoku-owned command links. Plugins cannot call the ABI while
another style is selected.

`OMARCHY_PATH` defaults to
`${XDG_DATA_HOME:-$HOME/.local/share}/ryoku/nomarchy`. Its `bin`, `default`,
`config`, and `themes` entries point at the shipped payload. Its `shell` entries
point at the materialized Nomarchy host, services, plugins, Commons, and Ui.

## Plugins

Plugins remain git repositories under `~/.config/omarchy/plugins/<id>` with the
upstream schema version 1 `manifest.json` format.

```sh
omarchy plugin add https://github.com/husamemadH/omarchy-quattro-prayer-times.git
omarchy plugin add https://github.com/husamemadH/omarchy-quattro-prayer-times.git --enable --yes
omarchy plugin update [id]
omarchy plugin enable <id> [--section left|center|right]
omarchy plugin disable <id>
omarchy plugin remove <id>
omarchy plugin validate ./plugin
omarchy plugin rescan
omarchy plugin list [--json]
```

Plugins run unsandboxed as the user. Add warns before cloning, leaves a plugin
disabled unless enable is requested, and requires `--yes` outside a terminal.
Update fetches and shows the diff before a fast-forward. Add, update, and enable
ask the active window-manager provider to scan the plugin. A plugin that calls
an API the provider cannot serve is refused with the provider's reason.

## Bar commands

`omarchy bar` supports the upstream `use`, `reset`, `defaults`, `position`,
`transparent`, `put`, `move`, and `set` commands. Placement accepts `--section`,
`--index`, `--before`, `--after`, `--from-section`, and `--from-index` where the
upstream command accepts them. Mutations are sent to the live host; position and
fresh defaults are persisted in `~/.config/omarchy/shell.json` and reloaded.

`omarchy-shell [-q] <target> <method> [args...]` first uses the host's fast Unix
socket, then falls back to Quickshell IPC. The upstream `shell` target is mapped
to Ryoku's `nomarchy` target. Third-party plugin target names are unchanged.

## Helper inventory

Every shipped name is linked only while Nomarchy is active. An unavailable shim
prints `not available on Ryoku` to standard error and exits nonzero rather than
pretending the action worked.

| Helper | Ryoku mapping |
|---|---|
| `omarchy`, `omarchy-shell` | Nomarchy command router and host IPC bridge |
| `omarchy-plugin-add`, `-update`, `-remove`, `-enable`, `-disable`, `-validate`, `-rescan`, `-list`, `-catalog` | Native Nomarchy plugin lifecycle and catalogue |
| `omarchy-bar` | Nomarchy host bar configuration IPC |
| `omarchy-menu`, `omarchy-menu-plugin`, `omarchy-menu-emoji`, `omarchy-menu-emoji-insert` | Nomarchy menu or plugin IPC |
| `omarchy-menu-keybindings` | Ryoku keybinding cheatsheet |
| `omarchy-osd` | Nomarchy style-owned OSD IPC |
| `omarchy-notification-send` | Freedesktop `notify-send` |
| `omarchy-launch-browser`, `omarchy-launch-webapp` | `ryoku-app browser` or `xdg-open` |
| `omarchy-launch-terminal`, `omarchy-launch-tui`, `omarchy-launch-floating-terminal-with-presentation` | `ryoku-app terminal` |
| `omarchy-launch-editor`, `omarchy-launch-config-editor` | Ryoku editor or the XDG opener |
| `omarchy-launch-or-focus`, `omarchy-launch-or-focus-tui`, `omarchy-launch-or-focus-webapp` | `ryoku wm act app.focus`, then launch |
| `omarchy-restart-shell`, `omarchy-refresh-shell` | `ryoku-shell reload` |
| `omarchy-system-lock` | `ryoku-qylock-lock` |
| `omarchy-system-logout`, `omarchy-system-wake` | `ryoku wm act session.exit` or `output.power on` |
| `omarchy-system-reboot`, `omarchy-system-shutdown` | systemd reboot or poweroff |
| `omarchy-system-sleep-lock` | qylock, then `ryoku-shell suspend` |
| `omarchy-toggle-nightlight` | Ryoku shell night-light intent |
| `omarchy-toggle-touchpad` | `ryoku wm act input.touchpad toggle` |
| `omarchy-toggle-bar` | Nomarchy bar IPC |
| `omarchy-update`, `omarchy-update-available` | `ryoku update` and `ryoku status --json` |
| `omarchy-brightness-display` | `ryoku-cmd-brightness` or `ryoku wm act output.power` |
| `omarchy-monitor-query`, `omarchy-monitor-scale` | Neutral `ryoku wm state` readback and `ryoku wm outputs` scale apply |
| `omarchy-audio-output-volume`, `omarchy-audio-input-mute` | PipeWire `wpctl` controls |
| `omarchy-audio-output-sink`, `omarchy-audio-sink-availability`, `omarchy-audio-output-set-default`, `omarchy-audio-input-set-default`, `omarchy-audio-output-switch` | PipeWire/Pulse compatibility controls |
| `omarchy-audio-source-switch` | Nomarchy media IPC |
| `omarchy-bluetooth-power`, `omarchy-bluetooth-device` | `rfkill` and `bluetoothctl` |
| `omarchy-network-status` | NetworkManager active connection state |
| `omarchy-powerprofiles-list`, `omarchy-powerprofiles-set` | `powerprofilesctl` |
| `omarchy-file-select` | Ryoku's shipped Zenity file chooser |
| `omarchy-glyph` | Pass-through glyph output |
| `omarchy-default-browser`, `omarchy-default-terminal`, `omarchy-default-editor` | XDG or Ryoku default application state |
| `omarchy-display-text-size` | Nomarchy shell override plus live config reload |
| `omarchy-font-list`, `omarchy-font-current`, `omarchy-font-set` | fontconfig monospace selection |
| `omarchy-cmd-present`, `omarchy-cmd-missing`, `omarchy-pkg-present`, `omarchy-pkg-missing` | PATH and pacman probes |
| `omarchy-capture-screenshot`, `omarchy-capture-screenrecording` | Existing Ryoku capture actions |
| `omarchy-weather-status`, `omarchy-weather-location` | wttr.in status and persisted weather location |
| `omarchy-voxtype-config`, `omarchy-voxtype-status` | Ryoku voice control and status |
| `omarchy-reminder` | User-systemd reminder timers and Nomarchy reminder panel |
| `omarchy-theme-color`, `omarchy-theme-set`, `omarchy-theme-set-templates`, `omarchy-theme-list`, `omarchy-theme-current`, `omarchy-theme-switcher` | Nomarchy theme bridge and shipped themes |
| `omarchy-bar-text-color` | Ryoku wallpaper-tone contrast map |
| `omarchy-battery-status`, `omarchy-system-stats` | UPower and procfs data for the power panel |
| `omarchy-clipboard-open`, `omarchy-clipboard-paste-file`, `omarchy-clipboard-paste-text` | Wayland clipboard history actions |
| `omarchy-disk-speedtest` | Local direct-I/O read and write measurement |
| `omarchy-tailscale-send` | Taildrop file transfer |
| `omarchy-menu-timezone` | gum timezone picker and timedatectl |
| `omarchy-network-band`, `omarchy-network-password`, `omarchy-network-qr`, `omarchy-network-speedtest` | NetworkManager band control, secret sharing, QR generation, and live speed measurement |
| `omarchy-dns` | Active NetworkManager profile DNS |
| `omarchy-battery-low`, `omarchy-hw-display` | Ryoku battery warning and backlight selection |
| `omarchy-remove-launcher-entry` | Safe removal of user-owned desktop entries |
| `omarchy-menu-select`, `omarchy-menu-file`, `omarchy-menu-images` | Explicitly unavailable |
| `omarchy-weather-icon`, `omarchy-brightness-keyboard` | Explicitly unavailable |
| `omarchy-exec-argv`, `omarchy-image-selector`, `omarchy-monitor-state` | Explicitly unavailable |
| `omarchy-hw-laptop-closed`, `omarchy-sudo-passwordless` | Explicitly unavailable |
| `omarchy-toggle-notification-silencing`, `omarchy-toggle-idle`, `omarchy-toggle-screensaver` | Explicitly unavailable |
| `omarchy-default-agent` | Explicitly unavailable |
| `omarchy-agent-usage-update`, `omarchy-agent-usage-claude`, `omarchy-agent-usage-codex` | Display records adapted from Ryoku's cached AI usage collectors |
| `omarchy-agent-account-*` | Account controls hidden; AI credentials remain managed in Rāshin |
| `omarchy-agent`, `omarchy-agent-prompt` | Explicitly unavailable; Ryoku keeps its own agent surface |

## Themes

Nomarchy starts with **Wallpaper (Ryoku)**. Each wallpaper palette run stages an
Omarchy-compatible `colors.toml`, renders `shell.toml` from the upstream shell
template, then publishes both under
`${XDG_STATE_HOME:-$HOME/.local/state}/omarchy/current/theme/`. The host reloads
the pair immediately. This changes only Nomarchy surfaces and plugins; it does
not add a second palette owner or alter Ryoku's selected colour scheme.

The theme picker in Bar Studio shows the live wallpaper swatch beside the
shipped Omarchy themes. The same choices are available from the command line:

```sh
omarchy-theme-list
omarchy-theme-current
omarchy-theme-switcher
omarchy-theme-set "Tokyo Night"
omarchy-theme-set "Wallpaper (Ryoku)"
```

A named theme copies its `colors.toml` into the stable current-theme directory
and generates `shell.toml` when the theme does not provide one. Theme-local
`shell.<section>.toml` files override only their matching generated section. The
choice is recorded in `~/.config/omarchy/current-theme` and
`${XDG_STATE_HOME:-$HOME/.local/state}/omarchy/current/theme.name`, so it
survives a login. Ryoku wallpaper-palette runs continue to stage their latest
result while a named theme is active but do not replace that named theme;
selecting Wallpaper (Ryoku) publishes the staged palette.

User themes live in `~/.config/omarchy/themes/<name>`. A user theme with the same
name as a shipped theme overlays it, matching Omarchy's precedence. At minimum a
theme needs `colors.toml`; an optional `preview.png`, `shell.toml`, or
`shell.<section>.toml` customizes its picker art and shell tokens.

## Recovery

`ryoku doctor` checks that command links exist exactly when `barStyle` is
`nomarchy`. It can recreate the compatibility root and seed a missing config, or
remove stale Ryoku-owned links after another style is selected. It never replaces
or removes a command file it does not own.

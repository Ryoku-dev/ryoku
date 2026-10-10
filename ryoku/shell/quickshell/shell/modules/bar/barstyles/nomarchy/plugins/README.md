# Nomarchy plugins

Nomarchy preserves Omarchy quattro's manifest-based plugin host. Bundled
plugins are discovered here. User plugins are discovered at
`~/.config/omarchy/plugins/<plugin-id>/`.

## Runtime ABI

A plugin directory contains `manifest.json` with `schemaVersion: 1`, an `id`,
`name`, `version`, non-empty `kinds`, and relative `entryPoints`. Supported
kinds are `bar`, `bar-widget`, `menu`, `panel`, `overlay`, and `service`.
`barWidget.defaultSection` may be `left`, `center`, or `right`.

Nomarchy passes the upstream host properties declared by a plugin's manifest:
the shell API, plugin registry, app library, bar state, first-party service
proxies, and plugin settings. Entry points may not be absolute or contain `..`.

Before a third-party plugin is enabled or loaded, the host runs:

```sh
ryoku wm compat <plugin-dir>
```

The JSON result must contain a boolean `ok`. A missing, malformed, failed, or
negative result blocks the plugin. `requires` and the failure reason are exposed
by `omarchy-shell nomarchy listPlugins`. Successful results are cached under
`$XDG_CACHE_HOME/ryoku/nomarchy-compat` and invalidated by directory or nested
file changes. First-party plugins are trusted because this tree is shipped and
adapted with the host.

The stable control target is `nomarchy`:

```sh
omarchy-shell nomarchy listPlugins
omarchy-shell nomarchy enablePlugin <id> '{}'
omarchy-shell nomarchy disablePlugin <id>
omarchy-shell nomarchy summon <id> '<json>'
omarchy-shell nomarchy rescanPlugins
```

## Bundled surface

| Plugin | id | kinds |
|---|---|---|
| Bar | `omarchy.bar` | `bar` |
| Emojis | `omarchy.emojis` | `overlay` |
| Image picker | `omarchy.image-picker` | `overlay` |
| Menu | `omarchy.menu` | `menu`, `bar-widget` |
| Notifications | `omarchy.notifications` | `service` |
| OSD | `omarchy.osd` | `panel` |
| Reminders | `omarchy.reminders` | `overlay` |
| Audio | `omarchy.audio` | `bar-widget` |
| Bluetooth | `omarchy.bluetooth` | `bar-widget` |
| Clock | `omarchy.clock` | `bar-widget` |
| Dropbox | `omarchy.dropbox` | `bar-widget` |
| Elsewhen | `omarchy.elsewhen` | `bar-widget` |
| Monitor | `omarchy.monitor` | `bar-widget` |
| Network | `omarchy.network` | `bar-widget` |
| Power | `omarchy.power` | `bar-widget` |
| Tailscale | `omarchy.tailscale` | `bar-widget` |
| Weather | `omarchy.weather` | `bar-widget` |
| Battery | `omarchy.battery` | `service` |
| Media | `omarchy.media` | `service`, `bar-widget` |

The bar's small widgets carry sibling `*.manifest.json` files. The complete
widget catalogue and settings schema are in `bar/README.md`.

## Ryoku ownership

Ryoku keeps one authoritative implementation for desktop-wide facilities.
Nomarchy delegates wallpaper selection to Ryogami, capture and recording to
ryoshot and Recorder, notifications to the resident notification server, and
updates to Ryoku Hub. Ryoku's lock screen, clipboard, policy agent, idle
inhibitor, and night-light service replace their upstream plugin counterparts.

The image picker remains for themes. Open it with:

```sh
omarchy-shell nomarchy summon omarchy.image-picker '{"source":"themes"}'
```

The menu definitions and compatibility command shims are installed below
`$XDG_DATA_HOME/ryoku/nomarchy`. The source remains under the MIT license in
`../LICENSE`; `../NOTICE` records the pinned upstream revision and adaptations.

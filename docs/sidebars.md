# Sidebars

Ryoku has two global push-aside sidebars. They are shell surfaces, not part of a
bar style, so they work with every bar and on every supported compositor.
`Super+Escape` toggles the left sidebar and `Super+S` toggles the right sidebar
on the focused monitor. Repeating the bind, pressing Escape, or clicking the
exposed desktop closes it. Opening one side closes the other side on that
monitor.

The shell keeps the old shortcut command names at its boundary so compositor
binds do not need special cases. In
`ryoku/shell/quickshell/shell/shell.qml`, `quicksettings` routes to
`sidebar-left` and `stash` routes to `sidebar-right`. Capture, compress, and
install commands route to a sidebar tab or tool through the same surface bus.

## Push-aside model

Each side is a full-height `PanelWindow` owned by
`ryoku/shell/quickshell/shell/modules/sidebar/Sidebar.qml`.

- The panel sits on `WlrLayer.Background`, underneath the desktop surface.
- Its animated `exclusiveZone` is the configured width multiplied by reveal
  progress. This moves tiled windows when `sidebars.push` is on.
- The wallpaper and desktop widgets live in an inner item in
  `ryoku/shell/quickshell/shell/modules/desktop/Desktop.qml`. Its `x` follows
  `SidebarState.slideOffset()`, so the desktop rides over the panel while the
  panel is revealed.
- The sidebar surface extends by `sidebars.wallpaperSlide`, while the exclusive
  zone stops at the panel width. The default factor, `1.15`, gives the desktop
  edge a small parallax lead.
- `Ryoku.SidebarFx.DepthEdge` paints the moving cast shadow, dark veil, and
  one-pixel highlight inside the sidebar. It is provided by
  `ryoku/shell/sidebarfx/` and isolated behind
  `ryoku/shell/quickshell/shell/modules/sidebar/SidebarDepthEdge.qml`, so
  `Sidebar.qml` can fall back to a QML gradient if the native module is
  unavailable.
- A transparent top-layer dismiss surface covers the screen outside the open
  panel and closes the sidebar on a pointer press.

The layer-shell namespaces are part of the compositor contract and must not be
renamed casually:

| Surface | Namespace |
|---|---|
| Left panel | `ryoku-sidebar-left` |
| Right panel | `ryoku-sidebar-right` |
| Outside-click surface | `ryoku-sidebar-dismiss` |

`ryoku/shell/quickshell/shell/services/SidebarState.qml` owns per-monitor open
state, reveal progress, active tabs, cross-side exclusion, and surface-bus
requests. Enter motion is 420 ms on cubic bezier `(0.16, 1, 0.3, 1)` and exit is
260 ms ease-out before user motion scaling. Reduce-motion collapses both to zero.

## Settings

The settings live under the top-level `sidebars` object in `shell.json`:

```json
"sidebars": {
  "width": 380,
  "motion": "standard",
  "depth": true,
  "push": true,
  "wallpaperSlide": 1.15,
  "left": {
    "enabled": true,
    "cards": ["system", "notifications", "weather", "media", "capture", "stage"]
  },
  "right": {
    "enabled": true,
    "cards": ["usage", "tools", "chat"]
  }
}
```

| Key | Default | Runtime meaning |
|---|---:|---|
| `sidebars.width` | `380` | Panel width in logical pixels, clamped to 280 through 560. |
| `sidebars.motion` | `"standard"` | Motion multiplier: `quick` is 0.6, `standard` is 1, and `calm` is 1.5. |
| `sidebars.depth` | `true` | Enables `DepthEdge` or its QML fallback. |
| `sidebars.push` | `true` | Enables the animated exclusive zone. Off keeps the desktop reveal but does not reserve space for tiled windows. |
| `sidebars.wallpaperSlide` | `1.15` | Desktop parallax factor, clamped to 1.0 through 1.4. |
| `sidebars.left.enabled` | `true` | Shows and accepts requests for the left sidebar. |
| `sidebars.left.cards` | `["system","notifications","weather","media","capture","stage"]` | Ordered left-side card ids. |
| `sidebars.right.enabled` | `true` | Shows and accepts requests for the right sidebar. |
| `sidebars.right.cards` | `["usage","tools","chat"]` | Ordered right-side card ids. |

`ryoku/shell/quickshell/shell/modules/sidebar/SidebarFrameBars.js` is the runtime
normalizer and the source of these defaults. A persisted card array is
authoritative. It keeps allowed built-in ids for that side and plugin-shaped ids,
removes duplicates, and does not append omitted built-ins.
`ryoku/shell/quickshell/shell/services/Config.qml` exposes the normalized result
as `Config.sidebars`.

Users edit the values in Ryoku Hub under **Desktop > Sidebars**. The rows are in
`ryoku/hub/quickshell/schema/DesktopPage.js`, and the Hub defaults are in
`ryoku/hub/quickshell/Hub.qml`. Saving patches the `sidebars.*` paths through the
shell daemon. `ryoku/shell/ipc/settings.go` treats `sidebars` as passthrough data:
it remains the sole writer, merges the patch, and echoes the tree without adding
a second Go schema.

## Built-in card contract

A built-in card is an `Item` with `pragma ComponentBehavior: Bound`. The host in
`ryoku/shell/quickshell/shell/modules/sidebar/SidebarCardHost.qml` sets these
members:

| Member | Direction | Meaning |
|---|---|---|
| `s: real` | host to card | Per-monitor UI scale. |
| `open: bool` | host to card | Whether this side is requested open. |
| `reveal: real` | host to card | Current reveal progress from 0 to 1. |
| `tabActive: bool` | host to card | Whether the card's tab is selected. |
| `width` | host to card | The inherited `Item.width`; the card reports `implicitHeight`. |
| `index: int` | host to card, when declared | Position used for entrance staggering. |
| `page: string` | host to card, when declared | Optional deep-link page such as a Tools action. |
| `requestClose()` | card to host | Asks the owning sidebar to close. |

The host binds the properties after loading the catalog source and forwards
`requestClose()`. Cards should stop polling or other live work when `open` or
`tabActive` is false.

Every built-in card puts its visible body in
`ryoku/shell/quickshell/shell/modules/sidebar/SidebarCardShell.qml`:

```qml
SidebarCardShell {
    width: root.width
    index: root.index
    open: root.open
    reveal: root.reveal
    tabActive: root.tabActive
    title: I18n.tr("Weather")
    glyph: "cloud"
    eyebrow: I18n.tr("Current conditions")

    // Card content is the default property.
}
```

`title`, `glyph`, and `eyebrow` describe the header. `eyebrow` is optional.
`index`, `open`, `reveal`, and `tabActive` drive the 40 ms stagger, fade, and
24-pixel rise. The shell owns the `Tokens.paperLift` plate, hairline, grain,
header, and entrance motion. A card must not paint another outer plate.

Built-ins may import the Qt Quick modules and Quickshell modules they actually
need, `Ryoku.Ui`, `Ryoku.Ui.Singletons`, and `shell.services`. Relative imports
stay inside the sidebar module or point to the shared shell components and the
feature singleton that owns the data. Do not import retired quick-settings,
frame-menu, stash, `Qs*`, or Pill presentation code. Use services for data and
`Ryoku.Ui` primitives for controls.

Plugin cards have a public, narrower contract. Do not copy the built-in relative
imports into a plugin. Follow section 4, **Sidebar card**, in
[`docs/plugins.md`](plugins.md#4-sidebar-card---lives-in-a-global-sidebar) for
entry points, public imports, placement, and `pluginApi`.

## Built-in catalog

`ryoku/shell/quickshell/shell/modules/sidebar/SidebarCatalog.js` is the single
registry for built-in ids, sides, tab ids, labels, glyphs, and component sources.
The persisted card arrays determine which catalog rows appear and in what order.

| Id | Side | Tab id | Tab label | Glyph | Source |
|---|---|---|---|---|---|
| `system` | left | `controls` | Controls | `settings` | `cards/SystemCard.qml` |
| `notifications` | left | `notices` | Notices | `notifications` | `cards/NotificationsCard.qml` |
| `weather` | left | `weather` | Weather | `cloud` | `cards/WeatherCard.qml` |
| `media` | left | `media` | Media | `play_circle` | `cards/MediaCard.qml` |
| `capture` | left | `capture` | Capture | `photo_camera` | `cards/CaptureCard.qml` |
| `stage` | left | `stage` | Stage | `graphic_eq` | `cards/StageCard.qml` |
| `usage` | right | `overview` | Overview | `monitor_heart` | `cards/UsageCard.qml` |
| `tools` | right | `tools` | Tools | `download` | `cards/ToolsCard.qml` |
| `chat` | right | `chat` | Chat | `chat` | `cards/ChatCard.qml` |

The source paths in the table are relative to the sidebar module directory.

## Adding a built-in card

1. Add one component under
   `ryoku/shell/quickshell/shell/modules/sidebar/cards/`.
2. Give its root the host properties and `requestClose()` signal above. Report
   content height through `implicitHeight`.
3. Wrap its visible body in `SidebarCardShell` and pass the motion properties.
4. Add one row to
   `ryoku/shell/quickshell/shell/modules/sidebar/SidebarCatalog.js`.
5. Add the id to the matching defaults in
   `ryoku/shell/quickshell/shell/modules/sidebar/SidebarFrameBars.js` only when
   the card should ship enabled. Keep the matching Hub card options and defaults
   in sync.

## Adding a `sidebarCard` plugin

A plugin uses `service/Main.qml` for persistent logic and `content/Widget.qml`
for its card view. Its placement selects a side, tab, order, label, and glyph.
The shell discovers enabled `sidebarCard` placements in
`ryoku/shell/quickshell/shell/modules/sidebar/SidebarPlugins.qml`, groups cards
with the same plugin tab, and orders them by `order` and then id.

Use the complete plugin contract in
[`docs/plugins.md`, section 4](plugins.md#4-sidebar-card---lives-in-a-global-sidebar).
That is the source of truth for manifests, public imports, `pluginApi`, settings,
and installation. Built-in sidebar internals are not a plugin API.

## File map

| Path | Responsibility |
|---|---|
| `ryoku/shell/quickshell/shell/modules/sidebar/Sidebar.qml` | One side's layer-shell panel, exclusive zone, depth edge, Escape handling, and outside-click surface. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarDepthEdge.qml` | Optional import boundary around the native `DepthEdge`; a loader error leaves the QML fallback active. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarChrome.qml` | Header, pill tab rail, built-in and plugin tab resolution, and scrolling card stack. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarCardHost.qml` | Catalog loading, card property binding, close forwarding, and plugin content hosting. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarCardShell.qml` | Shared card plate, header, grain, and staggered entrance. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarCatalog.js` | Built-in card registry and default tab grouping. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarFrameBars.js` | Defaults and normalization for the `sidebars` settings tree. |
| `ryoku/shell/quickshell/shell/modules/sidebar/SidebarPlugins.qml` | Discovery and ordering for enabled `sidebarCard` plugins. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/SystemCard.qml` | Connectivity, audio, brightness, battery, power-profile, and session controls. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/NotificationsCard.qml` | Notification history and do-not-disturb controls. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/WeatherCard.qml` | Current, hourly, and daily weather. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/MediaCard.qml` | Active-player metadata and transport controls. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/CaptureCard.qml` | Screenshot and recording actions plus recent captures. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/StageCard.qml` | Desktop stage and visualizer controls. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/UsageCard.qml` | Current and historical screen-time summaries. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/ToolsCard.qml` | Link downloads, recent jobs, compression, and package installation. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/ChatCard.qml` | Persistent Rashin conversation UI backed by the shared chat service. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/FilePickerOverlay.qml` | File browser used by the Tools compression and installation flows. |
| `ryoku/shell/quickshell/shell/modules/sidebar/cards/CobaltSetupOverlay.qml` | Setup flow for the Tools download service. |
| `ryoku/shell/quickshell/shell/services/SidebarState.qml` | Per-monitor open state, progress, tabs, motion, desktop offset, and surface-bus handling. |
| `ryoku/shell/quickshell/shell/services/Config.qml` | Watched `shell.json` input and normalized `Config.sidebars`. |
| `ryoku/shell/quickshell/shell/modules/desktop/Desktop.qml` | Wallpaper and widget slide wrapper. |
| `ryoku/shell/quickshell/shell/shell.qml` | Per-monitor left and right mounts plus shortcut and IPC routing. |
| `ryoku/shell/sidebarfx/README.md` | Native `Ryoku.SidebarFx` module summary. |
| `ryoku/shell/sidebarfx/depthedge.hpp` and `ryoku/shell/sidebarfx/depthedge.cpp` | Scene-graph `DepthEdge` implementation. |
| `ryoku/hub/quickshell/schema/DesktopPage.js` | Desktop > Sidebars controls. |
| `ryoku/hub/quickshell/Hub.qml` | Hub factory defaults for every `sidebars.*` path. |
| `ryoku/shell/ipc/settings.go` | Daemon persistence and passthrough patching. |

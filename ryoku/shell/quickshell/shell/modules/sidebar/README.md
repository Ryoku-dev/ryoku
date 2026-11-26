# Sidebar module

Quick settings is the control centre shared by every bar style and compositor. It
opens at the bar's trailing end inside the work area and wears the design chosen in
Style (Lithic, Material, Adwaita or Fluent).

- `Sidebar.qml` owns placement in the work area, the content and click-away windows,
  and each design's open and close motion.
- `QsLook.qml` holds the per-design tables every part reads (sizes, cuts, radii, type,
  motion) on top of the Alt+Space bar's `Look`. A part asks it, never `theme.design`.
- `SidebarFrame.qml` and `SidebarChrome.qml` provide the surface and its head.
- `ControlsBoard.qml` renders the persisted block order and instantiates only the
  blocks the user has chosen to show; `SectionHead.qml` heads each block and
  `PageSlot.qml` moves between the overview and a page.
- `Tile.qml` is one quick tile in each design's form; `ControlsConnections.qml`
  lays out the Wi-Fi, Bluetooth, wired network and VPN tiles and the switches.
- `ControlsLevels.qml`, `LevelSlider.qml`, `MixerDrawer.qml`, `MixerRow.qml`, and
  `BrightnessDrawer.qml` provide fine audio and per-display brightness controls.
- `ControlsMedia.qml` is the optional now-playing block, `PowerProfileControl.qml`
  the power mode.
- `ControlsHero.qml`, `Vital*.qml`, `Meter.qml` and `Plate.qml` present native system
  activity.
- `ControlsBar.qml`, `HoldButton.qml`, and `ControlsSettingsPopup.qml` own the
  hold-to-activate session actions and the panel settings popover.
- `cards/SystemWifiPage.qml` and `cards/SystemBluetoothPage.qml` are the built-in
  pages, sharing `cards/RadioRow.qml`, `PageRow.qml`, `RowPlane.qml`,
  `PageSearching.qml` and `SearchButton.qml`.
- `ExtensionsBoard.qml` uses `SidebarCardHost.qml` for installed plugins;
  `SidebarPlugins.qml` discovers and orders them while the panel is open.
- `shell/services/SidebarState.qml` owns per-display Quick settings state and routing,
  normalises its saved layout, position, and edge gap, and keeps its motion
  preference.

Capture is a separate surface under `shell/modules/capture/`; Ask and its desktop
bubble are under `shell/modules/ask/`. Do not route either back through Quick settings.

The root shell loads surfaces asynchronously and unloads them after closing.
Gate live work on the owning surface's active state. `shell.json` `controls`
stores position, edge gap, section order, section visibility, and hidden
sub-elements. Keep its shipped defaults aligned with `SidebarState.qml`,
`services/Config.qml`, the daemon settings contract (`ryoku/shell/ipc/settings.go`),
the Quick settings page's schema in Ryoku Settings
(`ryoku/hub/quickshell/schema/quick-settings.js`), and the doctor reconciler that moved
boxes off the old default (`ryoku/cli/internal/doctor/reconcile_quick_settings_layout.go`).
A hidden block must not be loaded or keep a probe, timer, file watch, or animation alive.

Store designs import this module as `Kit`; keep the names in `qmldir` and the
properties they use stable.

See [`docs/sidebars.md`](../../../../../../docs/sidebars.md) for behavior and the
[`sidebarCard` contract](../../../../../../docs/plugins.md#4-sidebar-card---lives-in-a-global-sidebar)
for contributor plugins. All sidebar cards appear under Quick settings > Extensions.

# Sidebar module

Controls is the compact top-left panel shared by every bar style and compositor.

- `Sidebar.qml` owns placement, the content window, and click-away input.
- `SidebarFrame.qml` and `SidebarChrome.qml` provide the shared surface and header.
- `ControlsBoard.qml` renders the persisted section order and instantiates only
  the blocks the user has chosen to show.
- `ControlsHero.qml` and `Vital*.qml` present native system activity.
- `ControlsConnections.qml` and `ConnectionTile.qml` present Wi-Fi, Bluetooth,
  Ethernet, and VPN state.
- `ControlsLevels.qml`, `LevelSlider.qml`, `MixerDrawer.qml`, `MixerRow.qml`, and
  `BrightnessDrawer.qml` provide fine audio and per-display brightness controls.
- `ControlsMedia.qml` is the optional now-playing block.
- `ControlsBar.qml`, `HoldButton.qml`, and `ControlsSettingsPopup.qml` own the
  hold-to-activate session actions, quick toggles, and gear popup.
- `cards/SystemWifiPage.qml` and `cards/SystemBluetoothPage.qml` are the remaining
  built-in detail pages.
- `ExtensionsBoard.qml` uses `SidebarCardHost.qml` for installed plugins;
  `SidebarPlugins.qml` discovers and orders them while the panel is open.
- `shell/services/SidebarState.qml` owns per-display Controls state and routing.

Capture is a separate surface under `shell/modules/capture/`; Ask and its desktop
bubble are under `shell/modules/ask/`. Do not route either back through Controls.

The root shell loads surfaces asynchronously and unloads them after closing.
Gate live work on the owning surface's active state. `shell.json` `controls`
stores section order, section visibility, and hidden sub-elements. Keep its
shipped defaults aligned with `SidebarState.qml`, the daemon settings contract,
and the Hub Controls schema. A hidden block must not be loaded or keep a probe,
timer, file watch, or animation alive.

See [`docs/sidebars.md`](../../../../../../docs/sidebars.md) for behavior and the
[`sidebarCard` contract](../../../../../../docs/plugins.md#4-sidebar-card---lives-in-a-global-sidebar)
for contributor plugins. All sidebar cards appear under Controls > Extensions.

# Sidebar module

- `Sidebar.qml` owns one edge panel, its depth treatment, focus, and click-away surface.
- `SidebarChrome.qml` draws the header, tab rail, and scrolling card stacks.
- `SidebarCatalog.js` is the built-in card registry and default tab order.
- `SidebarCardHost.qml` resolves a card and supplies the runtime card contract.
- `SidebarCardShell.qml` is the shared plate, heading, and entrance treatment used by built-in cards.
- `SidebarFrameBars.js` normalizes the persisted `sidebars` settings tree.
- `SidebarPlugins.qml` discovers installed `sidebarCard` plugins.
- `cards/` contains the built-in card implementations and their supporting overlays.

To add a built-in card, create one component in `cards/`, wrap its contents in `SidebarCardShell`, declare the host properties and `requestClose()` signal, then add one registry row to `SidebarCatalog.js`. Add its id to the matching default list in `SidebarFrameBars.js` only when it should ship enabled.

Contributor plugins use the `sidebarCard` host documented in [`docs/plugins.md`](../../../../../../docs/plugins.md).

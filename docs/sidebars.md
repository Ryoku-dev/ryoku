# Quick settings, ryoshot, and Ask

Three global tools are available under every subshell and bar style and on both Hyprland and
niri:

- `Super+Escape` opens **Quick settings** at the top-left of the focused display.
- `Super+Shift+S` opens the floating **ryoshot** capture bar.
- `Alt+Space` opens **Ask** near the top third of the focused display.

Quick settings and Ask close with Escape, their close button, or a click outside.
ryoshot has its own close control. The shortcuts are editable in
**Ryoku Settings > Windows > Keyboard shortcuts**.

## Quick settings

Quick settings is a compact, configurable 688-logical-pixel panel.

### Vitals

The hero is a compact system monitor. It shows the user and host, uptime and
load average, total CPU load with a per-core grid, CPU frequency and
temperature, memory and swap, GPU load, temperature and VRAM, network traffic,
disk traffic with root usage and drive temperature, and battery state. Missing
sensors are omitted rather than shown as zero.

`SystemMonitor` samples four times a second on a worker thread while the panel
is active; loads and transfer rates are measured over a sliding one-second
window, and slow sensors (temperatures, battery, disk usage) refresh once a
second. `SystemGraph` draws bounded native histories for CPU, memory, GPU,
network, disk, and temperatures over a one-minute window, keeps a second more
than it shows so the oldest sample is always beyond the left edge (the trace
runs off the graph rather than ending on a point), and eases each new sample
in over one sample period so the trace never steps. Sampling and graph
animation stop when the panel is inactive. Hiding Vitals prevents its monitor
from being created; hiding the live graph prevents its graph renderer from
being created.

### Connections

Wi-Fi and Bluetooth tiles show useful connection state at a glance and open
their device lists in the same panel. Ethernet and VPN appear as presence chips
when available. The Extensions view holds every installed `sidebarCard` plugin.

### Levels

Volume and brightness stay available as fine sliders with direct value entry and
small step controls. Expanding Volume opens an inline drawer for output devices,
playback apps, recording apps, microphones, mute controls, and default-device
selection. Expanding Brightness opens a drawer with one control per display.
Changing a level does not silently change its mute state.

### Bottom bar

Lock, Sleep, Log out, Restart, and Power off are hold-to-activate actions. Hold a
button until its fill completes; releasing early cancels it. Keyboard users can
hold Space or Enter. The completed hold is the confirmation, so there is no
second dialog. Lock runs `ryoku-shell lock`, the same command as Super+L on
every compositor; Sleep runs `ryoku-shell suspend`, the fail-closed lock then
suspend the lid uses. Log out, Restart, and Power off go through the daemon's
`session.*` calls.

The same bar carries Night light when supported, Keep awake, Do not disturb,
Mic mute, and Gaming mode when supported. Keep awake is an idle inhibitor: the
machine will not dim, lock, or sleep on its own while it is on, and the request
persists across logins until it is switched off. It never refuses a deliberate
sleep: the lid, the power key, and Sleep here still suspend. The gear opens a
small popup for the remaining Quick settings options. Unsupported controls are not
shown as dead options.

### Customising Quick settings

**Ryoku Settings > Desktop > Quick settings** has a miniature of the panel rather than a list of
switches. Drag Vitals, Connections, Power profile, Now playing, Levels, and
Bottom controls into the order you want. The eye on each block shows or hides
the whole block. Selecting a block exposes chips for its smaller parts, including
individual readings, the live graph, connection tiles, level sliders, media
details and transport, session actions, quick controls, and plugin cards.

The shipped arrangement matches the original Quick settings panel. Now playing is
available but hidden until selected. **Reset** restores that arrangement. The
choice is stored in `shell.json` under `controls`, applies immediately to an
open panel on every display, and is used the next time it opens. Hidden blocks
are not instantiated, so their panel-owned probes, processes, watches, and
animations do not continue in the background.

The gear in Quick settings also saves where the panel opens and how far it sits from
the screen edge. **Top left** keeps the shipped placement, **Top center** and
**Top right** keep it against the upper edge, and **Center** centres the panel
in both directions. **Edge gap** accepts 0 to 96 pixels and starts at 12,
matching the earlier fixed spacing. The settings popup stays inside the panel
window at every placement and scale, scrolling when the available room is
short. These choices live beside the panel arrangement in `shell.json` under
`controls`.

Quick settings remains a global shell surface. The same arrangement is used by every
subshell, bar style and supported compositor.

### Store designs

Sidebar designs from RyoStore change the panel content without taking over the
surface. The shell keeps the window, focus, click-away area, Escape handling,
open and close motion, and per-display state. A design receives the screen,
scale, active state, and available room; it may choose a supported placement
and implicit size, and may ask the host to close it. A design that declares a
placement takes precedence over the saved Quick settings position.

Pick the design in **Ryoku Settings > Desktop > Quick settings**. Design authors can use
the stock blocks with `import shell.modules.sidebar as Kit`, including the
complete Quick settings board, individual Quick settings sections, frame, and buttons.



## ryoshot

ryoshot is the capture UI. Its bar offers Shot, Beautify, OCR, Search, and
Record (keys 1 to 5), a target (Region, Window, Screen; Space cycles them, and
Window needs a compositor that reports window geometry), colour picking (C),
the screenshots folder, and close. Enter takes the screen under the pointer.
Beautify opens the region straight in the Beautify studio.

The bar wears the design chosen in Style and stands where that platform puts
it: Lithic a ruled strip flush on the bottom edge, Material a floating toolbar
24 px up with a FAB that takes the screen, Adwaita GNOME's capture panel with
its shutter, Fluent the Snipping Tool bar 16 px under the top bar. It shows on
the output under the pointer and steps aside while a region is dragged, a
colour picked or a delay runs.

The selection follows the same design: its scrim, frame, grips, a live size
readout, a loupe at the moving corner (with the hex while picking a colour), a
tag naming the hovered window, and the design's alert colour when the drag will
record. Lithic adds rulers and dimension lines. Once a region is taken, the
editing dock opens below it (above it, or inside, when there is no room) in two
rows: the tools in groups, then colour, width, fill, sketch, undo, redo,
Beautify (Ctrl+B), Copy, Save and More (pin, upload, send to phone, shortcuts,
settings).

Beautify lifts the selection in place into a card over the dimmed screen: its
background blooms out, the corners round and the shadow drops, and a panel of
edits floats beside it in the design's own idiom. Dragging the background's
edges sets the padding; 1 to 7 pick the looks, [ and ] change the padding,
Ctrl+C and Ctrl+S export, and Esc folds the card back into the selection with
the markup intact. "Use every time" saves the style as the default, and the
dock's Copy then carries a dot and styles every shot it copies or saves.

The delay (0, 1, 3, 5, or 10 seconds) captures the live region after the wait.
With Record selected the bar adds desktop audio, microphone, key-press overlay,
and webcam toggles. The delay and microphone choices persist in `ryoshot.json`.
The settings popover keeps the save behaviour and links to **Ryoku Settings >
Apps > Screen capture**.

## Ask

Ask is a compact client for the optional Rashin daemon. Its QML is created only
when Alt+Space opens the bar, connects to Rashin's local socket, and is released
after the bar closes. If Rashin is off or absent, the bar does not start a
daemon or leave a watcher running.

- **Ask** sends one-shot, read-only questions to Rashin's free-model router.
  Answers stream into the bar with copy, file, folder, link and proposed-command
  actions. Recent asks come from daemon-owned sessions, so history survives the
  bar and app closing.
- **Agent** shows the most recent external-harness session, or starts one with
  the harness you choose. Permissions, prompts and model choices belong to that
  harness. **Open in Rashin** moves the same session into the app.
- **Tools** and **Web** remain shell features. They work whether Rashin is on or
  not.

**Move to Agent** keeps the original question and opens an Agent session for
work that needs changes. The Rashin app is another client of the same daemon:
its Ask, Agent and Index pages see the same sessions and index state rather than
holding a separate transcript.

## Commands

```text
ryoku-shell quicksettings
ryoku-shell screenshot
ryoku-shell ask
ryoku-shell ask chat
ryoku-shell ask tools
ryoku-shell compress
ryoku-shell install
```

`quicksettings` opens Quick settings, `screenshot` launches ryoshot, and the Ask
commands open the matching mode or tool. The compositor providers expose the
same actions without putting compositor-specific QML in the shell.

## Rendering and lifecycle

`Sidebar.qml` owns the Quick settings content window and its click-away window.
`AskBar.qml` is one full-screen overlay surface that holds the bar and its
click-away area, so the keyboard always stays with the bar on both compositors.
They use overlay-layer keyboard focus while open and reserve no window space.
Per-display UI scale is applied to every dimension, and reduced-motion settings
remove decorative movement without removing required hold gestures.

The root shell loads its heavy surfaces on demand and unloads them after
closing. ryoshot is a separate on-demand Quickshell process, so capture work is
not resident in the shell.
Detail pages, plugin discovery, file watches, native sampling, audio enumeration,
and display probing run only while their owning surface is active.

## Contributor map

Quick settings components live under
`ryoku/shell/quickshell/shell/modules/sidebar/`:

| File | Responsibility |
|---|---|
| `Sidebar.qml` | Saved placement, focus, input region, and click-away dismissal |
| `SidebarFrame.qml`, `SidebarChrome.qml` | Shared surface, header, and board routing |
| `ControlsBoard.qml` | Persisted section order, visibility, and lazy block loading |
| `ControlsHero.qml`, `Vital*.qml` | Native activity graphs and individual system readings |
| `ControlsConnections.qml`, `ConnectionTile.qml` | Wi-Fi, Bluetooth, Ethernet, and VPN summaries |
| `ControlsLevels.qml`, `LevelSlider.qml` | Volume and brightness controls |
| `MixerDrawer.qml`, `MixerRow.qml` | Per-device and per-application audio controls |
| `BrightnessDrawer.qml` | Per-display brightness controls |
| `ControlsMedia.qml` | Optional now-playing details and transport |
| `ControlsBar.qml`, `HoldButton.qml`, `ControlsSettingsPopup.qml` | Session actions, toggles, and gear popup |
| `ExtensionsBoard.qml`, `SidebarCardHost.qml`, `SidebarPlugins.qml` | Plugin discovery, ordering, and hosting |
| `cards/SystemWifiPage.qml`, `cards/SystemBluetoothPage.qml` | Connection detail pages |

ryoshot lives in `ryoku/shell/quickshell/ryoshot/`. The shell keeps only the
shared recording, key-press, and webcam overlay services under
`shell/modules/capture/`. Ask components live under
`ryoku/shell/quickshell/shell/modules/ask/`, including `AskBar.qml`, its mode
views, and `AskBubble.qml`.

Shared state is owned by `shell/services/SidebarState.qml`, `ShellState`, and the
Ask and chat services. Native sampling and rendering live in
`ryoku/shell/plugin/systemmonitor.{hpp,cpp}` and
`ryoku/shell/plugin/systemgraph.{hpp,cpp}`; the Ask orb and the field beam are
`ryoku/shell/plugin/thinkingorb.{hpp,cpp}` and
`ryoku/shell/plugin/borderbeam.{hpp,cpp}`. Contributor plugins use the public
[`sidebarCard` contract](plugins.md#4-sidebar-card---lives-in-a-global-sidebar).

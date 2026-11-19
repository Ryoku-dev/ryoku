# Quick settings, ryoshot, the Rashin bar, and Kit

Four global tools are available under every subshell and bar style and on both Hyprland and
niri:

- `Super+Escape` opens **Quick settings** at the top-left of the focused display.
- `Super+Shift+S` opens the floating **ryoshot** capture bar.
- `Alt+Space` opens the **Rashin bar** near the top third of the focused display.
- `Super+K` opens **Kit** in the middle of the focused display.

Quick settings and the Rashin bar close with Escape, their close button, or a click outside;
Kit closes with Escape, a click outside, or `Super+K` again. ryoshot has its own close control.
The shortcuts are editable in **Ryoku Settings > Windows > Keyboard shortcuts**.

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

## Rashin bar

The Rashin bar is a keyboard-first command bar in the style of Raycast. It draws
in the desktop's design (`theme.design`: Lithic, Material, Adwaita or Fluent) and
palette, and follows a change live. Its QML is created when Alt+Space opens it
and released 15 s after it closes, so a closed bar costs nothing.

One field leads every view. The root view ranks what you type: a calculator
answer for an expression, `Download <host>` first for a link or magnet,
`Download N links` when the field holds several, active and recent downloads,
recent asks when Rashin is on, and two fallbacks that always close the list,
`Ask Rashin` and `Search the web`.

- **Quick Ask** (Tab, or `\`) sends a one-shot, read-only question to Rashin.
  The answer streams into the bar with copy, file, folder, link and
  proposed-command actions; **Open in Rashin** continues it in the app and
  **Pin** keeps it as a bubble on the desktop. With Rashin off or absent the view
  says so and opens its settings; the bar never starts Rashin.
- **Web** (`?`) searches in place. While you type it shows suggestions, a
  Wikipedia row and, for a word or a "what does X mean" question, a definition
  card. The full search runs on Enter, on picking a suggestion, or once you stop
  typing for 1.2 s. Its Web section merges Keenable and DuckDuckGo results, with
  Bing as the last resort; Wikipedia, GitHub, Stack Overflow, MDN, npm, crates.io,
  the Arch Wiki, Flathub, YouTube and Hacker News answer too, plus Arch packages
  and the AUR on Arch-based editions. All keeps the best few that match, a scope
  or a source shows its whole list. An engine that is blocking this computer
  shows as paused until a time instead of failing, and when every web engine is
  out the view offers Ask Rashin and Search in the browser. Enter opens a result
  in the browser; its actions copy the link, ask Rashin about the page, or
  download it.
- **Downloads** (`/`) hands over to Kit's Downloads, where the download manager
  lives (see Kit below). The bar keeps the quick paths: a link and Enter starts
  it with your defaults, `Download N links` and Enter adds every pasted link,
  and the home list shows what is moving. Ctrl+Enter on a link, or on several,
  opens Kit's intake card with them, and **Show in Downloads** opens Kit on that
  download.

Enter runs the highlighted row, Ctrl+Enter its second action, and Ctrl+K lists
every action for it. Up and Down move, Ctrl+Up and Ctrl+Down jump between
sections. Escape closes the action list, then clears the field, then goes back,
then closes the bar; Backspace on an empty field goes back. Ctrl+1 and Ctrl+2
jump to Quick Ask and Web; Ctrl+3 opens Kit's Downloads.

### Downloads engine

Downloads run in `ryoget` (`ryoku/shell/ryoget/`), a Go daemon shipped with the
shell. It starts on demand through `ryoku-host svc --user start ryoget` (a
systemd user unit, or a runit service on Void), keeps working after the bar
closes, and exits after a minute with nothing to do. No container is involved.

- Files download over up to 16 connections into a `.part` file whose sidecar
  lets a stopped or crashed download resume where it was.
- Media links resolve natively for YouTube, X, Instagram, TikTok, Reddit video,
  Bluesky, SoundCloud, Vimeo, Twitch, Streamable, Dailymotion, Pinterest, Tumblr,
  Bilibili, Loom, OK.ru, Rutube, VK and Facebook; any other site goes through an
  installed yt-dlp while that fallback is on. ffmpeg joins video and audio
  without re-encoding.
- Torrents and magnet links download with file selection and seeding limits.
  The torrent client exists only while a torrent runs or seeds.
- A finished or failed download posts a notification with Open and Show in
  folder.

Settings live in `~/.config/ryoku/downloads.json`, written only by the daemon,
and the queue in `~/.local/state/ryoku/ryoget/`. `ryoget add`, `list`, `pause`,
`resume`, `cancel`, `remove` and `retry` drive it from a terminal; `ryoget add -`
reads any text on standard input and adds every link in it, cleaned the same way
as a paste. `ryoget.desktop` offers it for magnet links and `.torrent` files
without claiming them as the default. The wire protocol is in the module's
README.

### Search engine

Web search runs in `ryoseek`, a small daemon built from the same Go module and
shipped with the shell. It starts when the bar searches, serves every bar and
terminal through one socket, and exits after five idle minutes, so it remembers
which engine is blocking this computer and for how long between bar openings.

- Each engine has a request budget. An engine that answers with a bot check, a
  captcha or a refusal pauses for its own Retry-After, or 15 minutes doubling up
  to 6 hours, and nothing is sent to it meanwhile.
- Results are cached for 30 minutes and suggestions for 10. A request already
  sent is never thrown away; it fills the cache.
- It identifies itself honestly as `ryoseek` and never disguises itself as a
  browser.
- What leaves the computer: typed text goes to DuckDuckGo (or Bing) suggestions
  and Wikipedia; a full search goes to Keenable, which logs queries with the
  address they came from, and to DuckDuckGo, and to Bing only when both fail;
  definitions come from FreeDictionaryAPI.com, or from Wiktionary itself when it
  has nothing (both Wiktionary, CC BY-SA 4.0).

Its health and budgets live in `~/.local/state/ryoku/ryoseek/health.json` and
its cache in `~/.cache/ryoku/ryoseek/`. `ryoseek query <text>` searches from a
terminal.

## Kit

Kit (`Super+K`) is a tabbed manual of six sections in the desktop's design. Its
tabs run down the left: Downloads, Keys, Commands, Phone, Dev tools and Typing
test, each with one live figure (the download speed or what finished, the
shortcut count, your saved commands, the phone's charge, the tool count, your
best speed). A tab whose section changed while you were elsewhere stays lit until
you open it ("1 finished", "2 new"). One field on top filters the open section;
Ctrl+F searches all of Kit, with the answers grouped by section and a count on
each tab. The foot names what Enter and Ctrl+Enter do beside Actions (Ctrl+K).
Kit opens on the section you used last (Keys the first time) and remembers each
section's page.

Kit is a small program of its own. Super+K starts it, closing hides it, and a
reopen within the next minute is instant; a minute after you close it, it quits,
so a closed Kit costs nothing. The first open after that starts it again, which
takes a moment longer.

Ctrl+1 to Ctrl+6 pick a section, Ctrl+Tab and Ctrl+Shift+Tab step through them,
and Ctrl+PgDn and Ctrl+PgUp step through a section's pages. Up and Down move.
Escape steps back one layer: the action list, all of Kit, the section's own
layer, the field, then Kit itself.

- **Downloads** is the download manager. Paste a link and press Enter to
  download it with your defaults, or Ctrl+Enter to choose the format, the items
  of a post or playlist, the torrent's files and the folder first. Paste several
  links, in any text, and they are cleaned (tracking tags and redirect wrappers
  removed, short and mobile addresses made normal, duplicates dropped) and shown
  on one card where each fills in as it is read; pick one format for all or per
  link, untick any, and Enter adds the rest. Enter on an empty field takes the
  links on the clipboard. Each row draws its transfer's live piece map, and the
  selected download shows beside the list its pieces, its speed over the last
  minute, its files and its connections or peers. The Settings page holds the
  folder, downloads at once, connections, speed limits, video quality, audio
  format, seeding and notifications.
- **Keys** lists every shortcut of the running compositor in one scroll, under
  headings that stay in view, with the compositor's own section and the
  shortcuts it cannot honour marked. Search by name in your language or by chord
  ("super+k"). Tab starts find-by-pressing-keys: the next shortcut you press
  jumps to its row. Rebind in Settings opens Keyboard shortcuts.
- **Commands** is a library of Ryoku and Linux commands plus your own: a dense
  list grouped by topic, with the selected command's detail beside it. Enter
  copies the command, after asking for any blank in it; Ctrl+Enter runs it in a
  terminal that stays open until you press Enter. Commands marked Careful, or
  written as usage, are copy-only. Ctrl+K stars, adds, imports, edits and
  deletes your own, which live in `~/.local/share/ryoku/kit/commands.json`.
- **Phone** controls a paired phone through KDE Connect and adb: quick actions
  and recent notifications on its overview, then Notifications with replies,
  Screen (mirror, record, webcam, microphone, wireless debugging), Contacts and
  Apps. Without a paired phone it points to **Ryoku Settings > Phone**.
- **Dev tools** are eleven offline helpers: password and UUID generators, JSON,
  Base64, URL, JWT, Unix time, colours, regular expressions, text diff and chmod.
  Pick one from the list; Tab moves between its panes, Ctrl+Enter runs it,
  Ctrl+Shift+C copies the output and Ctrl+L clears.
- **Typing test** runs timed, word-count and zen tests, with a results screen,
  History and Stats with graphs, and Settings. While a test is up every key goes
  to it.

Kit keeps its memory in `~/.local/state/ryoku/kit.json`: the last section, each
section's page, the dev tools' options and the typing history.

## Commands

```text
ryoku-shell quicksettings
ryoku-shell screenshot
ryoku-shell ask
ryoku-shell ask web
ryoku-shell ask downloads
ryoku-shell cheatsheet
ryoku-kit [section [page]]
```

`quicksettings` opens Quick settings, `screenshot` launches ryoshot, `ask` and
`ask web` open the Rashin bar on its root or Web view, and `ask downloads` opens
Kit's Downloads. `cheatsheet` toggles Kit, and `ryoku-kit` opens it on a section
(`downloads`, `keys`, `commands`, `phone`, `devtools`, `typing`) and, if given, one
of its pages. The compositor providers expose the same actions without putting
compositor-specific QML in the shell.

## Rendering and lifecycle

`Sidebar.qml` owns the Quick settings content window and its click-away window.
`AskBar.qml` is one full-screen overlay surface that holds the bar and its
click-away area, so the keyboard always stays with the bar on both compositors.
Kit's `Kit.qml` is the same kind of overlay, with the dim, on the focused display.
They use overlay-layer keyboard focus while open and reserve no window space.
Per-display UI scale is applied to every dimension, and reduced-motion settings
remove decorative movement without removing required hold gestures.

The root shell loads its heavy surfaces on demand and unloads them after
closing. Kit and ryoshot are separate on-demand Quickshell programs, so their
work is never resident in the shell. `kit.qml`, beside the shell's `shell.qml`,
runs Kit with the shell's directory as its root; `ryoku-kit` starts it or hands
the request to the running one; it quits a minute after closing, and only the
open section's page exists while it is up.
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

Kit starts from `ryoku/shell/quickshell/shell/kit.qml` (its IPC, the window on the
asked display, and the minute it waits before quitting) and lives under
`ryoku/shell/quickshell/shell/modules/kit/`. `Kit.qml` is the overlay with its
dim; `KitFrame.qml` holds the field, the page
strip, the foot, the Ctrl+K panel and all of Kit, with `KitRail.qml`,
`KitPages.qml`, `KitFoot.qml`, `KitSearch.qml` and `KitMemory.qml` (`kit.json`)
beside it. `lib/` holds the keyboard map, routes, memory keys and search, each
with its tests, and `parts/` the setting rows the pages share. Each section is a
folder under `sections/`: `Section.qml` is its page, built only while it is
open, and `Source.qml` is what it knows while Kit is open (its tab's figure, what
is new, its rows for all of Kit).

Shared state is owned by `shell/services/SidebarState.qml`, `ShellState`, and the
Ask and chat services. Native sampling and rendering live in
`ryoku/shell/plugin/systemmonitor.{hpp,cpp}` and
`ryoku/shell/plugin/systemgraph.{hpp,cpp}`; the Ask orb and the field beam are
`ryoku/shell/plugin/thinkingorb.{hpp,cpp}` and
`ryoku/shell/plugin/borderbeam.{hpp,cpp}`. Contributor plugins use the public
[`sidebarCard` contract](plugins.md#4-sidebar-card---lives-in-a-global-sidebar).

# Quick settings, ryoshot, the Alt+Space bar, the clipboard, and Kit

Five global tools are available under every subshell and bar style and on both Hyprland and
niri:

- `Super+Escape` opens **Quick settings** at the top-right of the focused display, under the bar.
- `Super+Shift+S` opens the floating **ryoshot** capture bar.
- `Alt+Space` opens the **Alt+Space bar** for search and downloads near the top third of the focused display.
- `Super+V` opens the **clipboard** history on the Alt+Space bar's line of the focused display.
- `Super+K` opens **Kit** in the middle of the focused display.

Quick settings closes with Escape or a click outside, and Escape first steps back out of an
open page. The Alt+Space bar closes with Escape, its close button, or a click outside. The
clipboard closes once something is copied, with Escape, or with a click outside. Kit closes
with Escape, a click outside, or `Super+K` again. ryoshot has its own close control.
The shortcuts are editable in **Ryoku Settings > Windows > Keyboard shortcuts**.

## Quick settings

Quick settings is the desktop's control centre. It opens at the trailing end of the bar, where
the bar's own status (controls, battery, clock) sits and where GNOME, macOS, Android and
Windows all put theirs, and it stays inside the work area, so it never covers the bar. The bar
keeps working while it is open.

The panel reads top to bottom in the order people reach for things: who and when, then
connections and switches, then levels, then power, then how the machine is doing, then the
session. Each block can be hidden or moved (see Customising below).

### One panel, four designs

The panel wears the design chosen in **Style** (Lithic, Material, Adwaita or Fluent) in
Ryoku's or the wallpaper's colours, light or dark, and follows a change live. Every design
keeps the same blocks and the same order, drawn the way its platform draws a control centre:

- **Lithic** is a ruled specimen sheet: a `§ QUICK SETTINGS 早設` masthead with the date and
  time over a tick ruler, numbered block heads, cut corners, tick-ruler sliders with odometer
  figures, and hold buttons whose blade sweeps across. It unrolls from the bar.
- **Material** follows Android's shade: a large clock head, rounded tiles whose icon square
  switches while the rest opens the page, thick pill sliders with the icon inside, and a
  spring scale on open.
- **Adwaita** follows GNOME's quick settings: pill toggles with an arrow segment for the page,
  a boxed card for the readings, and a small scale and drop on open.
- **Fluent** follows the Windows action centre: square buttons with the name under them, a
  chevron part for the page, and the session row on a footer band. It drops in from the bar.

Reduced motion keeps every state change and drops the travel.

### Head

The head names the user and host with the uptime. Lithic sets the date and time in its
masthead, Material leads with a large clock, Adwaita sets the time at the end of the name row,
and Fluent leaves the clock to the bar as Windows does. When a page is open, the head becomes
a back button with the page title.

### Connections and switches

Wi-Fi and Bluetooth are large tiles with their state (the network name, the connected device).
Each has two targets: one switches the radio, the other opens its page in the same panel, a
network list with connect, disconnect, forget and hidden-network actions, or a device list
with pairing and scanning. Wired network and VPN appear as tiles when present and open Wi-Fi
and internet settings. The quick switches follow: Night light (when the compositor supports
it), Keep awake, Do not disturb, Microphone, and Game mode (when supported). The Microphone
tile is on while the microphone is live. Unsupported controls are not shown as dead options.

Keep awake is an idle inhibitor: the machine will not dim, lock, or sleep on its own while it
is on, and the request persists across logins until it is switched off. It never refuses a
deliberate sleep: the lid, the power key, and Sleep here still suspend.

The Extensions page holds every installed `sidebarCard` plugin.

### Volume and brightness

Volume and brightness are fine sliders that follow the wheel and the arrow keys, and outside
Material a click on the figure takes a typed value. The chevron beside Volume unfolds a
drawer with output devices, playing apps, microphones, mute controls, and default-device
selection. The chevron beside Brightness unfolds one control per display. Changing a level
does not silently change its mute state.

### Power mode

Power saver, Balanced and Performance in one segmented control, with a line under it saying
what the chosen mode does.

### System status

System status is a compact monitor: processor load with a live graph, frequency, and
temperature, one bar per core, then memory, graphics, network traffic, disk traffic, and
battery. Hovering a reading shows its detail: swap, VRAM, the network interface and full
rates, root usage and drive temperature. Missing sensors are omitted rather than shown as
zero.

`SystemMonitor` samples four times a second on a worker thread while the panel
is active; loads and transfer rates are measured over a sliding one-second
window, and slow sensors (temperatures, battery, disk usage) refresh once a
second. `SystemGraph` draws bounded native histories for CPU, memory, GPU,
network, disk, and temperatures over a one-minute window, keeps a second more
than it shows so the oldest sample is always beyond the left edge (the trace
runs off the graph rather than ending on a point), and eases each new sample
in over one sample period so the trace never steps. Sampling and graph
animation stop when the panel is inactive. Hiding System status prevents its
monitor from being created; hiding the activity graph prevents its graph
renderer from being created.

### Session buttons

Lock, Sleep, Log out, Restart, and Power off are hold-to-activate actions. Hold a
button until its fill completes; releasing early cancels it. Keyboard users can
hold Space or Enter. The completed hold is the confirmation, so there is no
second dialog. Lock runs `ryoku-shell lock`, the same command as Super+L on
every compositor; Sleep runs `ryoku-shell suspend`, the fail-closed lock then
suspend the lid uses. Log out, Restart, and Power off go through the daemon's
`session.*` calls.

The gear at the end of the row opens the panel settings: where the panel opens (pick a spot
on the small screen, or use the arrow keys), the edge gap, how fast it moves (Quick, Standard
or Calm), and shortcuts into Ryoku Settings.

### Customising Quick settings

**Ryoku Settings > Desktop > Quick settings** lists the blocks in their order. Each block can
be shown, hidden, or moved, and each part inside it (a reading, a tile, a slider, a session
button) can be hidden on its own. **Restore the panel** puts every block back in its shipped
order with all its parts. Now playing is available but hidden until you add it. The choice is
stored in `shell.json` under `controls`, applies immediately to an open panel on every
display, and is used the next time it opens. Hidden blocks are not instantiated, so their
panel-owned probes, processes, watches, and animations do not continue in the background.

**Position** saves where the panel opens: **Top right** (the default) beside the bar's status,
**Top centre** and **Top left** against the upper edge, and **Centre** in the middle of the
work area. **Edge gap** accepts 0 to 96 pixels and starts at 12. These live beside the
arrangement in `shell.json` under `controls`. `ryoku doctor` moves a box still on the old
top-left default and block order to the new ones once; a later choice of either stands.

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
`Kit.ControlsHero` keeps its own name-and-load row for designs that have no head of their own.



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

## The Alt+Space bar

The Alt+Space bar is a keyboard-first command bar in the style of Raycast. It draws
in the desktop's design (`theme.design`: Lithic, Material, Adwaita or Fluent) and
palette, and follows a change live. Its QML is created when Alt+Space opens it
and released 15 s after it closes, so a closed bar costs nothing.

One field leads every view. The root view ranks what you type: a calculator
answer for an expression, `Download <host>` first for a link or magnet,
`Download N links` when the field holds several, downloads, and a fallback that
always closes the list, `Search the web`.

The empty list is news, not history. It shows every download that is moving,
then what finished, failed, stopped or paused since you last looked: once the
list has shown one, it leaves ten minutes later, and anything that settled more
than a day ago without being seen stays off it. Delete clears the highlighted
one at once and Shift+Delete clears them all; Kit's Downloads keeps them until
its own clearing.

- **Web** (`?`) searches in place. While you type it shows suggestions, a
  Wikipedia row and, for a word or a "what does X mean" question, a definition
  card. The full search runs on Enter, on picking a suggestion, or once you stop
  typing for 1.2 s. Its Web section merges Keenable and DuckDuckGo results, with
  Bing as the last resort; Wikipedia, GitHub, Stack Overflow, MDN, npm, crates.io,
  the Arch Wiki, Flathub, YouTube and Hacker News answer too, plus Arch packages
  and the AUR on Arch-based editions. All keeps the best few that match, a scope
  or a source shows its whole list. An engine that is blocking this computer
  shows as paused until a time instead of failing, and when every web engine is
  out the view offers Search in the browser. Enter opens a result in the
  browser; its actions copy the link or download it.
- **Downloads** (`/`) hands over to Kit's Downloads, where the download manager
  lives (see Kit below). The bar keeps the quick paths: a link and Enter starts
  it with your defaults, `Download N links` and Enter adds every pasted link,
  and the home list shows what is moving and what is new (see above). Ctrl+Enter on a link, or on several,
  opens Kit's intake card with them, and **Show in Downloads** opens Kit on that
  download.

Enter runs the highlighted row, Ctrl+Enter its second action, and Ctrl+K lists
every action for it. Up and Down move, Ctrl+Up and Ctrl+Down jump between
sections. Escape closes the action list, then clears the field, then goes back,
then closes the bar; Backspace on an empty field goes back. Ctrl+2 jumps to
Web and Ctrl+3 opens Kit's Downloads.

### Downloads engine

Downloads run in `ryoget` (`ryoku/shell/ryoget/`), a Go daemon shipped with the
shell. It starts on demand through `ryoku-host svc --user start ryoget` (a
systemd user unit, or a runit service on Void), keeps working after the bar
closes, and exits after a minute with nothing to do. No container is involved.

- Files download over up to 16 connections into a `.part` file whose sidecar
  lets a stopped or crashed download resume where it was. The first request
  already carries data; four connections open first and the rest once those
  deliver, a host that refuses more is remembered for a day, and a slow last
  piece is raced by a second connection. Free space is checked before anything
  is written. A resume keeps the partial file only when the server still serves
  the same file, so a stopped download can take a fresh link (an expired signed
  one, say) without starting over.
- Media links resolve natively for YouTube, X, Instagram, TikTok, Reddit video,
  Bluesky, SoundCloud, Vimeo, Twitch, Streamable, Dailymotion, Pinterest, Tumblr,
  Bilibili, Loom, OK.ru, Rutube, VK, Facebook, Bandcamp, TED, Odysee, Rumble and
  LinkedIn; any other site goes through an installed yt-dlp while that fallback
  is on. ffmpeg joins video and audio without re-encoding, and audio keeps its
  cover picture while cover art is on.
- A page with no media offers the files it links to, and a plain download form
  is sent the way a browser would, only once the download starts. A page behind
  a bot check, or one that hands its file out through a script behind a
  captcha, fails saying it needs a browser and offers Open in the browser;
  ryoget does not script its way past either.
- A pasted range such as `file[001-100].zip` or `photo-[a-f].jpg` expands into
  its links, 500 at most.
- Torrents and magnet links download with file selection and seeding limits.
  The torrent client exists only while a torrent runs or seeds.
- Each download can have its own speed limit. A link may carry a checksum
  (`#sha256=`, `#sha1=` or `#md5=`), checked when the file lands; a file that
  does not match is deleted and the download fails. With sorting by type on,
  files land in Videos, Music, Documents, Programs, Archives or Pictures inside
  the download folder.
- A finished download posts a notification with Open and Show in folder, a
  failed one with Try again.
- Finished and failed downloads leave the list a week after they settle (the
  number of days is a setting, 0 keeps them); the files stay, a failed
  download's partial data goes.

Settings live in `~/.config/ryoku/downloads.json`, written only by the daemon,
and the queue in `~/.local/state/ryoku/ryoget/`. `ryoget add`, `list`, `pause`,
`resume`, `cancel`, `remove`, `retry` and `clear` drive it from a terminal;
`ryoget add -` reads any text on standard input and adds every link in it,
cleaned the same way as a paste. `ryoget.desktop` offers it for magnet links and
`.torrent` files without claiming them as the default. The wire protocol is in
the module's README.

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

## Clipboard

`Super+V` opens the clipboard history as a palette on the Alt+Space bar's line, so both open where
the eye already is. One field searches it, the list sits on the left, a preview of the selected
item fills the right, and the foot says what Enter does and opens every other action with
`Ctrl+K`. It is built from the Alt+Space bar's parts, so it draws in the desktop's design and
palette the same way and follows a change live.

The tabs at the end of the field switch between **Recent**, everything newest first under
Today, Yesterday and Earlier, and **Starred**, only what you starred. Tab switches them, and
the search reads whichever is open.

Search matches words, not scattered letters: every word typed has to be found in an item, in
its text, a link's host, file names, kinds such as image, link, colour or code, and, when it is
turned on, the text in a picture. A word that starts a word of the title ranks first, then one
that starts a word further in or sits inside one, then a word of five letters or more with one
slip (a letter missing, extra, wrong or swapped), then letters scattered through a short title.
Ties go to the newest. The title underlines what matched, and when the match is out of sight,
further into the text or in a picture, the row shows that line with the words underlined.

Each row is drawn for what it holds: a picture's thumbnail with its pixel size, a colour's
swatch, a link as host and path, a command or code in mono, a file list by its first name and
folder, and text by its first line and length. The time it was copied sits at the end, with a
star when it is starred. While Ctrl is held the first nine rows show the number that copies
them.

The preview reads the selected item closer: the picture whole, with the text read in it under
it, the colour as a swatch with its RGB and HSL values, the link's full address, files one per
line, and text set to read. Text longer than the daemon's 200-character preview says so. Under
it are its type, size and when it was copied, and Copy, Copy text (for a picture with text),
Open (for links), Star and Delete for the pointer.

Picking an item copies it and closes the palette; paste it as usual with Ctrl+V, or
Ctrl+Shift+V in a terminal. Delete removes the selected item after a five-second undo (Ctrl+Z,
or Undo in the foot), and Ctrl+S stars it. Starred items stay when the history is cleared and
are never pushed out by new copies. Clearing the history lives in **Ryoku Settings > Desktop >
Clipboard**, which asks first; the action list leads there.

| Key | Does |
|---|---|
| Enter | Copy and close |
| Ctrl+Enter | Open a link in the browser |
| Ctrl+1 to Ctrl+9 | Copy that row |
| Up, Down, Ctrl+N, Ctrl+P, Page Up, Page Down | Move |
| Ctrl+Up, Ctrl+Down | Jump between days |
| Home, End | First or last row, while the field is empty |
| Tab | Recent or Starred |
| Ctrl+S | Star or unstar |
| Delete | Delete, while nothing in the field is after the cursor |
| Ctrl+Z | Undo the last delete |
| Ctrl+K | Every action for the selected item |
| Escape | Close the actions, clear the field, leave Starred, then close |

The history is the daemon's (`ryoku/shell/ipc/clipboard.go`, the picture reader in
`clipboard_read.go`): 100 entries, items up to 10 MiB, an image thumbnail each, the time each was
copied and the text read in each picture, kept across restarts.

### Text in pictures

**Find pictures by the text in them** makes a screenshot of an error, a receipt or a chat
findable by what it says. It is off by default, because reading a picture costs memory for a
moment, and turns on in **Ryoku Settings > Desktop > Clipboard** or from `Ctrl+K` in the palette,
where it also turns off.

- The daemon reads only while the clipboard is in use: from opening it until two minutes after
  it closes. The palette asks for that on open, once a minute while open, and as it closes, and
  outside that window nothing runs, so a closed clipboard costs nothing.
- Pictures are read newest first, each once, by tesseract in short-lived processes at the
  lowest priority on one thread: about 100 MB for a second or two a picture (about 150 MB for
  a 4K screenshot), gone when the picture is read. A screenshot is read at full size, a small one
  enlarged first; a photo only from its thumbnail, where big lettering still reads. Only words
  tesseract is sure of are kept, so a photo's grain adds nothing, and a QR code's text is read
  too.
- It uses the clipboard's own `clipboard.ocrLanguages`, restricted to languages installed for
  tesseract, with English as the fallback. It does not depend on Hyakki or Oni.
- The text stays with the history on this computer and survives a restart. Turning the switch
  off forgets it. **Copy text** in the preview copies a picture's text.

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
  links on the clipboard, and links or `.torrent` files dropped on the section
  are added the same way. A link already in the list says so, with Show in
  folder when its file is there. Chips over the queue show All, Active,
  Waiting, Done or Failed with their counts (Alt+1 to Alt+5, remembered), and
  Clear finished and Clear failed sit beside them: finished files stay on disk,
  failed partial data goes. Ctrl+K also retries every failed download. Each row
  draws its transfer's live piece map, and the selected download shows beside
  the list its pieces, its speed over the last minute with the average and the
  peak, whether it can resume, a verified checksum, its files and its
  connections or peers. A download that has not landed takes a speed limit of
  its own (- and +, or Alt+Left and Alt+Right); a failed, stopped or paused one
  can take a new link, and a finished one can be downloaded again. The Settings
  page holds the folder, downloads at once, connections, speed limits, video
  quality, audio format and cover art, sorting new files into folders by type,
  seeding, notifications, and how long finished downloads stay in the list.
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
ryoku-shell clipboard
ryoku-shell screenshot
ryoku-shell ask
ryoku-shell ask web
ryoku-shell ask downloads
ryoku-shell cheatsheet
ryoku-kit [section [page]]
```

`quicksettings` opens Quick settings, `clipboard` toggles the clipboard, `screenshot`
launches ryoshot, `ask` and `ask web` open the Alt+Space bar on its root or Web view, and
`ask downloads` opens Kit's Downloads. `cheatsheet` toggles Kit, and `ryoku-kit` opens it
on a section (`downloads`, `keys`, `commands`, `phone`, `devtools`, `typing`) and, if given,
one of its pages. The compositor providers expose the same actions without putting
compositor-specific QML in the shell.

## Rendering and lifecycle

`Sidebar.qml` owns the Quick settings content window and its click-away window,
both laid out inside the work area so the bar stays uncovered and live.
`AskBar.qml` is one full-screen overlay surface that holds the bar and its
click-away area, so the keyboard always stays with the bar on both compositors.
Kit's `Kit.qml` is the same kind of overlay, with the dim, on the focused display, and
`ClipboardSurface.qml` is one too, on the Alt+Space bar's line, built when it first opens and
released 15 s after it closes. They use overlay-layer keyboard focus while open and reserve no
window space.
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
| `Sidebar.qml` | Saved placement in the work area, focus, input region, open and close motion, and click-away dismissal |
| `QsLook.qml` | The per-design tables every part reads: sizes, cuts, radii, type, and motion |
| `SidebarFrame.qml`, `SidebarChrome.qml` | The panel surface and its head, with the page back button |
| `ControlsBoard.qml`, `SectionHead.qml`, `PageSlot.qml` | Persisted block order and visibility, lazy block loading, block heads, and pages |
| `Tile.qml`, `ControlsConnections.qml` | Quick tiles in each design's form, the connection tiles, and the switches |
| `ControlsLevels.qml`, `LevelSlider.qml` | Volume and brightness sliders |
| `MixerDrawer.qml`, `MixerRow.qml` | Per-device and per-application audio controls |
| `BrightnessDrawer.qml` | Per-display brightness controls |
| `ControlsMedia.qml` | Optional now-playing details and transport |
| `PowerProfileControl.qml`, `SidebarSegments.qml` | Power mode and the segmented control it shares |
| `ControlsHero.qml`, `Vital*.qml`, `Meter.qml`, `Plate.qml` | System status: native graphs, readings, meters, and the card under them |
| `ControlsBar.qml`, `HoldButton.qml`, `ControlsSettingsPopup.qml` | Session buttons and the panel settings popover |
| `CornerButton.qml`, `CornerTip.qml`, `SidebarButton.qml`, `SidebarToggle.qml`, `Avatar.qml`, `FocusFrame.qml` | Shared buttons, tooltips, switches, the avatar, and the keyboard focus ring |
| `ExtensionsBoard.qml`, `SidebarCardHost.qml`, `SidebarPlugins.qml` | Plugin discovery, ordering, and hosting |
| `cards/SystemWifiPage.qml`, `cards/SystemBluetoothPage.qml` | The Wi-Fi and Bluetooth pages |
| `cards/RadioRow.qml`, `cards/PageRow.qml`, `cards/RowPlane.qml`, `cards/PageSearching.qml`, `cards/SearchButton.qml` | The rows, scanning state, and search button those pages share |

The clipboard lives under `ryoku/shell/quickshell/shell/modules/clipboard/`:
`ClipboardSurface.qml` is the window and its arrival, `ClipPalette.qml` the body (its state,
keys and actions), `ClipRow.qml` one row, `ClipDetail.qml` the preview, `ClipTabs.qml` the
Recent and Starred tabs, and `lib/clip.js` reads an entry (link, colour, command, code, files),
its age and its day, with its tests beside it. It reuses the Alt+Space bar's `look/` parts and its
`bar/` field, list, foot and action panel. The topic is `shell/services/Clipboard.qml`.

ryoshot lives in `ryoku/shell/quickshell/ryoshot/`. The shell keeps only the
shared recording, key-press, and webcam overlay services under
`shell/modules/capture/`. Ask components live under
`ryoku/shell/quickshell/shell/modules/ask/`, including `AskBar.qml` and its views.

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
The one-time move off the old top-left layout is
`ryoku/cli/internal/doctor/reconcile_quick_settings_layout.go`.

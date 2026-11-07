# Subshells and bar styles

A **subshell** is a complete shell look: its bar plus its own panels, quick
settings and settings window. A **bar style** is a bar design. Both are chosen by
the same key, `barStyle` in `~/.config/ryoku/shell.json`, and one is active at a
time. `orpheus` is the value when the key is missing and the fallback whenever
the selected item cannot be served. The default rice wears Kairos, the Noc rice
wears Orpheus, and the first-login welcome offers both.

Ryoku ships two subshells, **Orpheus** (`orpheus`) and **Kairos** (`kairos`).
Everything else lives in the catalogue repo
[Ryoku-dev/ryostore-revamped](https://github.com/Ryoku-dev/ryostore-revamped) and
installs from RyoStore: the subshells under `subshells/` (Python, Caelestia,
Gokko, Imi, Tansu) and the bar styles under `barstyles/` (Sumi, Shima, QS Bar,
Nomarchy and the rest). The category folder decides which one an item is. No
Ryoku code keeps a list of item ids.

## Where items live

| What | Where |
|---|---|
| Shipped subshell | `ryoku/shell/quickshell/shell/subshells/<id>/` in this repo, installed to `/usr/share/ryoku/config/quickshell/shell/subshells/<id>/` and materialized to `~/.config/quickshell/shell/subshells/<id>/` |
| Installed store item (packed) | `$XDG_DATA_HOME/ryoku/<category>/<id>/bundle.tar.gz` |
| Receipt | `$XDG_STATE_HOME/ryoku/store/<category>/<id>.json` |
| Unpacked while in use | `$XDG_STATE_HOME/ryoku/store/unpacked/<category>/<id>/<digest>/` |
| QML module links while in use | `$XDG_STATE_HOME/ryoku/store/qml/<module>` |
| Shells index | `$XDG_STATE_HOME/ryoku/store/shells.json` |

A shipped subshell has the same layout as a catalogue item: `item.toml`,
`README.md`, `LICENSE` and `qml/` with the entry `qml/Scene.qml`.

## From repo folder to Loader

1. **Repo folder.** A catalogue item is one folder with `item.toml` and `qml/`.
   The catalogue's `tools/build.py` writes its `manifest.json` and a row in the
   category's `registry.json`.
2. **Packed bundle.** `ryostore install <category> <id>` fetches the files the
   manifest lists, checks each hash, stores them as `bundle.tar.gz` and writes
   the receipt. Nothing is unpacked or loaded.
3. **Unpacked folder.** When a selection names the item, `ryostore internal
   shells-sync` verifies the bundle, unpacks it under `unpacked/` and links each
   `[shell] modules` root into `store/qml/`.
4. **shells.json.** The same sync writes the index: every shipped subshell and
   every installed item, with `entry`, `inUse`, `prepared`, `owns`,
   `ownsSetting`, `settings`, contributions, lifecycle and runtime scripts, and
   the active item's `activeOwns`.
5. **Loader.** The `Shells` singleton (`shell/services/Shells.qml`) reads
   `shells.json`. `modules/bar/Frame.qml` loads `Shells.activeEntry` once per
   monitor, or once on the primary output when the item says
   `frameHost = "primary"`, and hands it the screen as `modelData`.
   Contributions load the same way through `Shells.contributions(kind)`.

Shipped Orpheus and Kairos skip steps 1 to 3. Their paths in `shells.json` are
relative to the shell config root. When `shells.json` is missing or unreadable,
`Shells` falls back to a built-in record of both, so the shell always boots.

An item that is installed but still packed reads as pending: the current Scene
stays up until the sync has prepared it. A Scene that fails to load is retried
ten times, 800 ms apart, then marked broken, and Orpheus draws until `barStyle`
changes.

## Switching

- Ryoku Hub > **Bar Studio** lists subshells and bar styles from `shells.json`
  and the catalogue. Hub > **Subshells** is the same page showing subshells only.
  Picking a card stages `barStyle`. The switch is committed when you leave the
  page or press **SWITCH NOW**.
- RyoStore's **APPLY** on an installed item does the same.
- From a terminal, `ryoku-shell barstyle <id>` sets `barStyle` and commits.

`ryoku-shell shells sync` asks the daemon to run `ryostore internal
shells-sync`. The daemon also runs it at startup and whenever `barStyle` or
`dock.design` changes. `ryoku-shell shells commit` restarts the shell under the
reload cover when the active item or the set of items in use differs from what
the running shell started with, then runs `ryostore internal shells-sync
--prune`.

`ryostore internal shells-require [--install] [--fallback] [--json]` lists
selections that name an item that is neither shipped nor installed.
`--install` installs the ones the catalogue offers and `--fallback` moves the
rest to safe defaults (`barStyle` to `orpheus`). `ryoku doctor` runs it as the
"selected shells and parts are installed" check, and in fix mode runs
`--install --fallback` and then `ryoku-shell shells commit`. Doctor also moves
the old `qsbar.audioBoost` key to `audio.boost`, and a `barStyle` that is
missing or names a retired style to `orpheus`. Removing the worn item in
RyoStore falls back to `orpheus` too.

## The resource rule

Nothing of an item runs unless it is in use: it is the active `barStyle`, or one
of its contributions is selected. No Ryoku file imports item code, not even
Orpheus or Kairos; items load only through Loaders whose source comes from
`shells.json`. Installed store items stay packed until they are in use. After a
committed switch the shell restarts, so the previous item's singletons, timers
and processes are gone, and the prune removes unpacked folders and module links
that are no longer in use.

## Surfaces an item owns

`[shell] owns` lists Ryoku surfaces the item draws itself while it is active.
Ryoku turns its own version off for that time:

| Value | Ryoku keeps off |
|---|---|
| `notifications` | its notification banners |
| `notificationSound` | its notification sound |
| `osd` | its volume and brightness OSD |
| `polkit` | its polkit island |
| `dock` | its dock surfaces, dock menus and the ii dock host |
| `widgets` | its desktop widgets and deck frost |
| `cheatsheet` | its shortcuts sheet |
| `windowClose` | its close keybind handling; the item answers IPC target `closeConfirm`, function `trigger` |

`[shell.ownsSetting]` maps an owned surface to a dotted `shell.json` key. The
ownership lapses while that key is `false`, so the item's own settings can hand
a surface back to Ryoku:

```toml
[shell.ownsSetting]
notifications = "python.general.notifications"
```

`Shells.owns(surface)` is the one check the shell makes. Compositor binds read
only `activeOwns` from `shells.json`. An item that owns `widgets` may give
Ryoku one desktop canvas through `Shells.desktopCanvas`, which the Desktop hosts
below windows on that output.

Desktop-wide shortcuts stay Ryoku's under every item: Super+Space opens the
launcher chosen on the App Launcher page, Super+V clipboard history,
Super+comma Ryoku Settings, Super+Escape quick settings, Alt+Space Rashin Ask.

## Contributions

An item can provide parts Ryoku uses while it is installed, even when another
item is active. Selecting one puts the item in use.

- `[[provides.dock]]`, selected by `shell.json` `dock.design`.
- `[[provides.launcher]]`, selected by `launcher.json` `variant`. Kairos ships
  its island launcher this way.
- `[[provides.desktopWidgets]]`, a roster of widget faces placed with Ryoku's
  desktop widget keys in `widgets.json`.

Bar Studio's **PARTS FROM ITEMS** card lists them. Its CHOOSE button opens the
Stage Editor's Dock or Widgets section, or the Hub's App Launcher page.

## Lifecycle and runtime

Scripts live in the item's `bin/` and run as the user, with `RYOKU_ITEM_DIR`
(the unpacked folder) and `RYOKU_ITEM_ID` set.

- `[lifecycle]`: the daemon runs the old item's `deactivate`, then the new
  item's `activate`, when the active item changes, and the active item's
  `palette` after a wallpaper palette regeneration, with the palette JSON path as
  its argument.
- `[runtime]`: for an item that runs as its own process. While it is active the
  daemon runs `check`; on exit 0 it runs `install` once when needed, then
  supervises `start` and falls back when it keeps failing.

No daemon code names an item id. Nomarchy's command links and Shima's optional
full upstream runtime (opt-in with `RYOKU_SHIMA_UPSTREAM=1` on niri) are these
scripts inside their items; `barstyles/nomarchy/README.md` and
`barstyles/iris/README.md` in the catalogue repo cover how they work and how
they are updated from upstream.

## Settings

Item settings data stays in `Config.qml` namespaces (`orpheus`, `kairos`,
`qsbar`, `python`, `inir`); they hold data only. `[shell.settings]` tells the Hub
how to open an item's own settings:

- `ipc`, `call`, `args`: Bar Studio's **ACTIVE ITEM** card has **OPEN
  SETTINGS**, which runs `ryoku-shell ipc call <ipc> <call> <args...>`. Inactive
  items are not loaded and their IPC targets do not exist, so for a selected
  item that is not running yet the card offers **SWITCH NOW** instead.
- `hubEditor`: one of the built-in Hub editors `frame-rails` (Sumi's frame and
  rails, see `docs/bar.md`), `nacre` or `obi`. They edit `shell.json` data and
  work whether the item is active or not.

`ryoku-shell bar ...` is gone. QS Bar's settings API is its own IPC target
`qsbar`, present only while QS Bar is active; its README in the catalogue lists
the calls. `ryoku-shell dock show|hide|edge|autohide|pin|unpin|list` stays: it
edits Ryoku's universal dock.

## Orpheus

Orpheus hangs from the top edge as drops drawn by `Ryoku.Blobs`: a left drop, a
right drop, and the voice in the middle. Every drop moves with the music. Its
lower edge rises on the beat and settles on the chosen feel, traced in ink where
sound lifts it, then lies flat and still in silence. The Sound page's Gain slider
sets how much it reacts. The `islands` span keeps the three drops apart with the
side drops tucked into the screen corners, `fused` joins them into one centred
body, and `full` runs edge to edge; switching spans morphs the drops into place.
The `liquid` form melts every drop into the top edge and swells the voice like a
drop; the `square` form keeps crisp corners and hangs a row of level blocks
under the voice instead. Anything that reaches a side edge melts into it the way
every drop melts into the top. The body is the frame's paper; inside it the bar,
its pages, and its settings use Ryoku's own controls (hairlines, a hair of
rounding, a bone plate for whatever is on or chosen). All text and numbers
follow the global font.

The voice shows the time, date, and weather at rest. It turns into the current
track while music plays, into the newest notification for a few seconds when one
arrives, and into a timer while the screen is recorded. While it is busy the time
moves into a small bubble beside it, and a recording over music puts the cover in
a second bubble. Click the voice to open the page it shows (Today, the player,
notifications, or the recording), scroll over it to step through those pages,
middle-click to play or pause, and right-click to open Controls.

Pages grow out of the bar and stay joined to the drop they came from; a page
that reaches a side edge melts into it. Click anywhere outside one, over a
window or the desktop, or press Escape to close it.
Today is a month calendar with events, holidays, and the day's weather. Controls
holds Wi-Fi, Bluetooth, quick toggles, volume and brightness with their own
pages, power profiles, and hold-to-activate session actions. The other pages
cover media, weather, notifications grouped by app, tray menus, power, system,
and session. Right-click empty bar space to open Controls.

Left-click the 力 logo to open settings, and right-click it to open the
launcher. Bar Studio's OPEN SETTINGS button opens them too, and
`ryoku-shell ipc call orpheus settings <page|close>` does the same from a
keybind or script; pass `""` to open the default Bar page. The settings pages
are Bar, Layout, Modules, Sound, and About. A choice applies at once and is
written through the shell daemon; the foot of the
settings rail reads Saved, Saving, or Not saved, and hovering Not saved shows
why. On Layout, drag a module between the left, centre, and right lanes, or into
Not shown to remove it; click a hidden module to add it back, or right-click a
placed one to remove it. The logo and the voice stay put. The modules are logo;
workspaces as dots, numbers, kanji, Greek, dashes, or app icons; window title;
voice; media; weather; system monitor; keyboard layout; recording and microphone
activity; coding-agent limits; a paired phone; VPN (Tailscale, with its exit
node picker, LAN access and run-as-exit-node switches, and NetworkManager VPN
and WireGuard profiles, each with its own switch in the VPN card, which also
shows the public IP); updates; tray; notifications; controls; battery; clock; shortcuts for the
launcher, Ask, overview, clipboard, screenshot, wallpaper, and Ryoku Hub; and
power. Auto-hide can be `off`, `smart`, or `always`; smart hides the bar while
the current workspace has windows, and touching the top edge brings it back. AI
usage, Phone and VPN are available in the Layout editor but stay out of the
default layout.
When shown, AI usage marks a pace trend beside the five-hour total and lists
five-hour and weekly pace details in its tooltip when reset times are available.

Its settings use the `orpheus` namespace in `shell.json`:

| key | values | default | purpose |
|---|---|---|---|
| `span` | `islands`, `fused`, or `full` | `islands` | Keep the drops apart, join them, or run edge to edge. An old `fit` reads as `fused`. |
| `form` | `liquid` or `square` | `liquid` | Melt the drops into the edge with a smooth wave, or keep crisp corners with a block wave and square workspace marks. |
| `scale` | `0.85` to `1.3` | `1.0` | Scale the bar. |
| `opacity` | `0.6` to `1.0` | `0.94` | Set the body's opacity at rest; an open page is always solid. |
| `rim` | boolean | `true` | Draw a faint rim along the bar's edge. |
| `autoHide` | `off`, `smart`, or `always` | `off` | Set when the bar hides. |
| `hoverOpen` | boolean | `false` | Open a page after a short hover. |
| `hoverSwitch` | boolean | `true` | Switch pages on hover while one is open. |
| `layout` | `{left:[ids], center:[ids], right:[ids]}` | `{left:["logo","workspaces"], center:["voice"], right:["tray","controls","battery"]}` | Arrange modules in three lanes. A layout saved before the voice existed drops media, weather, and clock and puts the voice in the centre. |
| `gain` | `0` to `3` | `1` | Set how much the edge reacts to sound; `0` keeps it still. An old `reaction` of lively, calm or still reads as 1, 0.55 or 0. |
| `depth` | `8` to `40` | `22` | Set how far sound can pull the lower edge, in pixels. |
| `mirror` | boolean | `true` | Mirror the spectrum around the centre. |
| `feel` | `fluid` or `tight` | `fluid` | Set the wave spring and damping. |
| `workspaceStyle` | `dots`, `numbers`, `kanji`, `greek`, `dashes`, or `icons` | `dots` | Choose the workspace marks. |
| `workspaceShowEmpty` | boolean | `true` | Include empty workspaces. Where workspaces are numbered, gaps fill in and one free slot follows the last used unless `workspaceSpare` is off. |
| `workspaceSpare` | boolean | `true` | With `workspaceShowEmpty` on, add one free slot after the last used workspace. Off ends the strip at the last workspace that exists. |
| `showDate` | boolean | `true` | Show the date under the idle time. |
| `seconds` | boolean | `false` | Show seconds in the idle time. |
| `voiceWeather` | boolean | `true` | Show the weather beside the idle time. |
| `clock12h` | global `TimeFormat` preference | global `TimeFormat` setting | Use the shared 12 or 24-hour format. |
| `mediaWidth` | `120` to `320` | `220` | Set the track title's maximum width. |
| `coverSpin` | boolean | `true` | Spin the artwork while media plays. |
| `wavy` | boolean | `true` | Draw a wavy media progress line. |
| `spectrum` | boolean | `true` | Show the spectrum on the media page. |
| `windowWidth` | `120` to `420` | `240` | Set the window title width. |
| `sysmonItems` | subset of `cpu`, `mem`, `temp`, `gpu`, `net` | `["cpu","mem","temp"]` | Choose system readings. |
| `actions` | subset of `launcher`, `ask`, `overview`, `clipboard`, `capture`, `wallpaper`, `hub` | `["launcher","capture","clipboard"]` | Choose shortcut actions. |
| `trayLimit` | `3` to `12` | `6` | Set the visible tray item limit. |
| `statusItems` | subset of `wifi`, `bluetooth`, `volume`, `mic` | all four | Choose the icons in the Controls module. |
| `aiTools` | subset of `claude`, `codex`, `opencode` | all three | Choose which coding agents the AI usage module counts. Modules page, AI usage. An empty list hides the module. |
| `aiAlertAt` | `50` to `100` | `80` | Share of a window at which the AI usage icon turns to the alert colour. |

The name is Orpheus, the musician of Greek myth whose lyre moved rivers and
stones. Made by neur0map.

## Kairos

Kairos is a single island at the top centre that carries the clock: hovering
opens it into a rolling date wheel, a track that plays adds a cover bubble beside
it, and Super+Space grows the same island into its own app launcher when the
Kairos launcher is picked (`docs/launcher.md`).

Its settings open from the gear in the top-right of the expanded clock island,
from Bar Studio's OPEN SETTINGS, or with `ryoku-shell ipc call kairos settings`.
The window is styled like the island, near-black with a hairline rim and the
same accent, and has three routes over the same `shell.json` namespace:

| Route | Holds |
|---|---|
| Island | clock island size, top offset |
| Clock | 12-hour time, seconds, the date wheel |
| Music | the now-playing bubble and its hover peek |

Every control writes the `kairos` key in `shell.json` through the shell daemon
(the sole writer) and applies live through `Config.kairos`.

Left of the gear, the **tune** icon grows the expanded clock island into its own
quick settings. This is not a second window: the island's own pill morphs to the
panel size on the same surface, with the clock and date wheel fading out and the
panel fading in, so open and close read as one body. It carries the radio tiles,
a weather card in place of a media card, Display and Sound fader rows, and the
notification list. Each tile opens a page in place: **Wi-Fi** (networks,
password, disconnect), **Bluetooth** (connected, saved, nearby, pair and
disconnect), **Sound** (output and input volume, mute, device pick) and
**Display** (per-output brightness, scale, resolution and Night Light). It lives
in `subshells/kairos/qml/quicksettings/` and closes on a click outside it, when
the pointer moves away, on Escape, or on the tune icon again.

## Writing an item

The catalogue repo holds the authoring guide: its `CONTRIBUTING.md` (add or
update an item), `docs/items.md` (`item.toml` and the folder layout),
`docs/shells.md` (Scenes, platform imports, `owns`, contributions, lifecycle,
runtime, translations) and `templates/`. Items reach Ryoku's platform through
`shell.services` (import it qualified, `import shell.services as Ryoku`),
`shell.barkit`, `shell.modules.stage.frame`, `stage`, `Ryoku.Ui`,
`Ryoku.Ui.Singletons`, `Ryoku.Blobs`, `Ryoku.Bus`, `Ryoku.PluginKit` and
`Ryoku.FrameBars`, plus Qt and Quickshell modules. Window-manager state comes
only from `Wm` in `Ryoku.Ui.Singletons`, never a compositor module, so an item
runs on Hyprland, niri and Wayfire alike.

To change Orpheus or Kairos, edit `ryoku/shell/quickshell/shell/subshells/<id>/`
here. A new file or import inside a loaded Scene needs a shell restart
(`systemctl --user restart ryoku-shell`); property edits reload live.

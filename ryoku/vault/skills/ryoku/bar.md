# Bar styles, subshells and the dock

One shell item draws the bar at a time, and `barStyle` in
`~/.config/ryoku/shell.json` names it. There are two kinds:

- A **subshell** is a complete shell look: its bar plus its own panels, quick
  settings and settings window.
- A **bar style** is a bar design on Ryoku's own panels.

Ryoku ships two subshells, **Orpheus** (`orpheus`) and **Kairos** (`kairos`).
The default rice wears Kairos, the Noc rice wears Orpheus, and Orpheus is the
fallback whenever the selected item cannot load. Every other subshell and bar
style (QS Bar, Shima, Sumi, Python, Nomarchy and the rest) is a Ryostore item
the user installs when they want it.

An installed item that is not in use stays packed as
`~/.local/share/ryoku/<category>/<id>/bundle.tar.gz` and costs nothing: no QML,
timers or processes of it run. It is unpacked only while it is the active
`barStyle` or one of its parts (a dock design, a launcher, a desktop widget set)
is selected. That is also why an item's own IPC target, settings window and
widgets exist only while it is active.

## Picking one

GUI first:

- **Ryoku Settings > Desktop > Bar** (`ryoku-shell hub open bar`) lists bar
  styles and subshells (complete desktops) together under Bar style.

The gallery shows the shipped and installed items from the shells index and
the catalogue items from Ryostore. A subshell's card says Complete desktop, and
a card flags an item that uses more resources than the others. Use wears an
installed item right away (it writes `barStyle`, then runs
`ryoku-shell shells commit`); Install fetches a store item in place and Update
refreshes one; Browse RyoStore opens the store for more. The shell restarts
under the reload cover.

Below the gallery, the Bar page has Show the bar on (turn the bar off on any
display), This style's options (Open its settings for an item with a settings
window of its own, only while it is in use, and the built-in editors some bar
styles use: Sumi's frame and edge bar, Obi's widgets, Nacre's layout), and Parts
it brings (Dock design and Desktop widgets hand off to the Stage Editor, App
launcher opens Ryoku Settings > Desktop > App launcher), each naming what
installed items add.

From a terminal:

```bash
ryostore catalog --category subshells     # shipped and store subshells
ryostore catalog --category barstyles     # store bar styles
ryostore install barstyles qsbar          # fetch one; it stays packed until used
ryoku-shell barstyle qsbar                # select it and restart the shell on it
ryostore remove barstyles qsbar           # remove it; a selection of it falls back to Orpheus
```

`ryoku-shell barstyle <id>` sets `barStyle` and commits. It does not install, so
install a store item first. `ryoku-shell shells commit` applies a selection that
was already written (the Bar page's Use runs it after writing `barStyle`) and
`ryoku-shell shells sync` rewrites the shells index. Orpheus and Kairos ship with
Ryoku and cannot be removed.

When a selected item is missing (a restored config, a rice that names a store
item), `ryoku doctor` installs it when the catalogue offers it and otherwise
falls back to Orpheus.

## What the active item can take over

An item may draw some of Ryoku's surfaces itself while it is active: the
notification banners, the OSD, the polkit prompt, the dock, the desktop widgets,
and the window close. The window close bind (Super+Q) asks the active item
first only when it owns that; otherwise it closes the window directly. Switch
away and Ryoku's own surfaces come back. An item can tie one of these to its own
setting, so the user can hand a surface back to Ryoku without switching.

## Orpheus

Orpheus drops on the top edge with a voice in the middle for the time, music,
notifications and recordings. Its Sound page's gain slider sets how much the bar
reacts to music. Its Bar page picks a liquid or square form; the square one
keeps crisp corners and shows sound as blocks. Left-click its 力 logo to open its
settings, where the Layout page arranges its modules by drag and drop;
right-click the logo for the launcher. Optional modules include coding-agent
limits from the shared usage caches, a paired phone shortcut into Kit, and a VPN
mark whose card switches Tailscale and NetworkManager VPN/WireGuard profiles;
none is in the default layout. Its keys live under `orpheus` in `shell.json` and
are listed in `docs/subshells.md`.

While Orpheus is active its settings window also opens with
`ryoku-shell ipc call orpheus settings ""`.

## Kairos

Kairos keeps a compact clock island at rest and grows it into music, quick
settings and its own launcher. While it is active its settings open with
`ryoku-shell ipc call kairos settings`. Its launcher is a part: pick it on Ryoku
Settings > Desktop > App launcher (`ryoku-shell hub open launcher`).

## QS Bar

QS Bar (`qsbar`) is a store bar style (`ryostore install barstyles qsbar`). While
it is active its layout is data in `shell.json` under `qsbar.layout` (ordered id
lists for left, center and right), it carries **QS Bar Settings** (the bar's
launcher mark opens it), and it answers on its own IPC target `qsbar`. Nothing of
this exists while another item is active, and there is no `ryoku-shell bar`
command any more.

```
ryoku-shell ipc call qsbar list | catalog | layout
ryoku-shell ipc call qsbar move <id> <left|center|right> <placement>
ryoku-shell ipc call qsbar show <id> | hide <id> | set <id> <key> <value>
ryoku-shell ipc call qsbar position top|bottom | form full|fit|dock|notch|islands | defaults
ryoku-shell ipc call qsbar settings <route>
```

Pass every argument. A placement is `""` (the end), a 0-based index,
`before:<id>` or `after:<id>`. `settings ""` opens QS Bar Settings, a route
(`bars`, `identity`, `layout`, `widgets`, `dock`, `community`) opens that page,
and `settings close` closes it. Writes answer `ok` or `err qsbar: <reason>`;
`list`, `catalog` and `layout` answer JSON. `launcher`, `workspaces` and `clock`
are always shown, so `show` and `hide` refuse them.

```bash
ryoku-shell ipc call qsbar move clock right 0
ryoku-shell ipc call qsbar hide gpu
ryoku-shell ipc call qsbar set cputemp barTemperatureSource gpu
ryoku-shell ipc call qsbar position bottom
ryoku-shell ipc call qsbar form islands
```

Volume boost is platform policy at `audio.boost` in `shell.json`, shared by every
bar; the QS Bar volume widget's `audioBoost` setting writes it. The doctor moves
an old `qsbar.audioBoost` there.

QS Bar is also the bar that hosts bar plugins (host `topbarGlyph`). A bar plugin
shows on the bar and under QS Bar Settings > Community only while QS Bar is
active. The full widget table and every call are in the item's README
(`barstyles/qsbar/README.md` in the `Ryoku-dev/ryostore-revamped` catalogue).

## The dock

Ryoku's own dock is platform: it works under every subshell and bar style,
unless the active item draws its own. Its GUI is the **Stage Editor**
(Super+Shift+D, then its Dock section), also reached from Ryoku Settings >
Desktop > Bar > Parts it brings > Dock design or straight with
`ryoku-shell ipc call desktop editSection dock ""`. Installed items can add dock
designs; they show under Parts it brings and in the Stage Editor.

```
ryoku-shell dock show | dock hide
ryoku-shell dock edge auto|top|bottom|left|right
ryoku-shell dock autohide on|off
ryoku-shell dock pin <app-id> | dock unpin <app-id>
ryoku-shell dock list [--json]
```

`dock list --json` returns the `dock` object from `shell.json`. A bad verb or
value answers `err dock: <reason>`.

```bash
ryoku-shell dock pin org.mozilla.firefox
ryoku-shell dock edge bottom
ryoku-shell dock autohide on
```

The shell daemon is the only writer of `shell.json`; change these through the
GUI or the commands above, never by editing the file.

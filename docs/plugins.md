# Writing a Ryoku shell plugin

A plugin is a small widget you write that the **user** drops into the Ryoku
desktop. You write the *content and the logic*. **Ryoku owns everything about how
it looks, moves, sizes, and where it lives.** That split is the whole point: a
plugin always looks and behaves like a native part of the shell, because the
shell - not you - draws the surface around it.

If you only read one thing, read **"Who does what"** below.

This is about **shell** plugins, widgets that run in Quickshell. A **Hyprland
compositor** plugin (a `.so` the compositor loads: title bars, cursor motion,
key sounds) is a different thing, managed on Settings > Plugins; see
`docs/hyprland-plugins.md`.

---

## The rules

Eleven rules make a plugin safe to install and native to the shell.
`ryoku plugin new` writes them into the plugin's `AGENTS.md`,
`ryoku plugin validate` enforces the ones a machine can check, and Ryostore's
review runs that same validator on every plugin PR (its CI will, too).

- **R1 Place.** A plugin is one folder named after its id, authored under
  `$(xdg-user-dir DOCUMENTS)/ryoku-plugins/<id>/` (fallback
  `~/ryoku-plugins/<id>/`); `ryoku plugin new <id>` creates it there. Never write
  into `~/.local/share/ryoku/plugins/` (the store's receipt-owned install root)
  or `~/.config/quickshell/` (the shipped shell). *Why:* those trees belong to
  the installer and the shell, so a stray folder there loads without a receipt or
  clobbers shipped files.
- **R2 Shape.** `manifest.json` at the root; `service/Main.qml` (logic, no UI);
  `content/Widget.qml` (the one view); optional `content/Panel.qml` (a bar
  panel); `README.md`; `LICENSE`; `assets/preview-widget.png` (a real capture);
  scripts only under `bin/` with a shebang and the exec bit. Every extra file is
  listed in manifest `files`. *Why:* install fetches exactly the entry points,
  `commands`, `README.md` and the `files` list, so anything unlisted is missing
  on the user's disk.
- **R3 Id.** Lowercase `[a-z0-9][a-z0-9-]*`, unique, and not a built-in widget
  id. *Why:* the id is the folder name, the layout key and the settings
  namespace, so a collision shadows a built-in or another plugin.
- **R4 Imports.** Only `QtQuick*`, `Quickshell*`, `Ryoku.PluginKit`,
  `Ryoku.PluginKit.Singletons`, and files inside the plugin folder. Never
  `shell.*`, `Ryoku.Ui*` internals, or a relative import that climbs out of the
  folder. *Why:* the kit is the only stable surface, so reaching into shell
  internals breaks on every update and escapes the folder.
- **R5 Settings.** Declared in `metadata.settings` (the types the bar renders:
  `toggle`, `choice`, `multi`, `int`, `text`); read through
  `pluginApi.pluginSettings` behind a default; written only through
  `pluginApi.saveSetting(key, value)`. Never edit `shell.json` or `plugins.json`
  directly. *Why:* the shell renders and persists settings for you, and
  hand-editing those files corrupts state it owns.
- **R6 Commands.** Every external program is in the plugin's own `bin/` or listed
  in `dependencies.commands`. No `sudo`, `doas`, `su`. A privileged action is
  allowed only through `pkexec`, only on an explicit click, only when listed in
  manifest `capabilities.privileged` (exact command strings) and explained in the
  README. *Why:* a reader and the validator can then see every binary the plugin
  runs and every escalation it can ask for.
- **R7 Network.** Every host the plugin talks to is listed in
  `capabilities.network`. No `curl … | sh`, no downloading and running code.
  *Why:* the manifest declares the whole network reach up front, and
  fetch-and-run hides code from review.
- **R8 Files.** Write only under `pluginApi.stateDir`
  (`$XDG_STATE_HOME/ryoku/plugins/<id>`), `$XDG_CACHE_HOME/ryoku/plugins/<id>`,
  or a temp dir. Never touch `~/.ssh`, `/etc`, shell rc files, or another
  plugin's folder. *Why:* a widget has no business outside its own state, and
  those paths are where damage and data theft happen.
- **R9 Shell.** No `sh -c` with a string built from settings or program output;
  pass argv arrays. *Why:* a value spliced into a shell string is a command
  injection hole.
- **R10 Secrets and binaries.** No tokens, keys or credentials in the tree; no
  compiled binaries (ELF/Mach-O/`.so`) and no symlinks; scripts only. *Why:* the
  tree is public and readable, so a secret leaks and a binary cannot be reviewed.
- **R11 Honesty.** `official` is never `true` for a community plugin; `author` is
  `Name <mail>`; the README says what runs, what it reads, what it writes, and
  every privileged or network capability. *Why:* users decide to trust an
  unsandboxed widget from the manifest and README, so both must be truthful.

---

## The workflow

Start to finish, a plugin is one command to scaffold, an edit loop, one command
to check, one to install, and one to publish:

1. **Scaffold.** `ryoku plugin new <id> [--bar|--desktop|--popout|--sidebar]`
   writes R1's folder from a working
   template (default `--bar`, which also adds `entryPoints.panel` and
   `content/Panel.qml`): the manifest, `service/Main.qml`,
   `content/Widget.qml`, `README.md`, `LICENSE`, an `AGENTS.md` carrying these
   rules, and a first git commit as the author.
2. **Edit.** Fill in the service, the view, the panel, the manifest and the
   README, and capture `assets/preview-widget.png`.
3. **Validate.** `ryoku plugin validate <dir>` runs the manifest checks and the
   static audit. Clear every blocking finding (see [Security](#security)); the
   template validates clean, so keep it that way.
4. **Install.** `ryoku plugin add <dir> --bar --yes` validates again, installs
   through Ryostore's transaction, and enables it on the bar; the shell picks it
   up on the `plugins.json` change, no restart.
5. **Verify.** Confirm it on the bar and under **QS Bar Settings > Community**.
6. **Share.** `ryoku plugin share <id>` opens the Ryostore pull request. Run it
   only when the user asks to publish.

---

## Who does what

| You (the contributor) write... | Ryoku (the shell) handles for you... |
|---|---|
| The **logic**: fetch data, hold state, run commands (`service/Main.qml`). | **Where the widget lives** (frame popout, desktop widget, bar glyph, or sidebar card) and letting the user choose and place it. |
| **One view** of your widget (`content/Widget.qml`) - the labels, buttons, grid, etc. | The **card / popout surface** behind your view: background, rounded corners, shadow, hairline. |
| A small **manifest** describing your plugin and its defaults. | **Dragging, resizing, the right-click menu, hover-to-open** - all the interaction. |
| A **settings schema** in your manifest (`metadata.settings`). | **Sizing**: the popout grows to fit your content; the desktop tile scales as the user resizes it. |
| Optionally, **shipped scripts/binaries** (`bin/`). | **Theming, motion, the brand look** (the "deck dialect"), so you match the shell automatically. |

**The golden rule:** never set your own position, never draw your own window
chrome, never assume your size. You declare *how big your content naturally
wants to be*; Ryoku does the rest. Break this rule and your plugin will look
bolted-on instead of native.

---

## Files you ship

```
<your-plugin-id>/
  manifest.json        what your plugin is + its suggested defaults   (required)
  service/Main.qml     persistent logic and state, no UI               (required)
  content/Widget.qml   ONE view of your widget                          (required)
  bin/                 scripts or binaries your plugin runs            (optional)
  README.md            what it is + a preview.gif                      (recommended)
```

- **Installed** to `~/.local/share/ryoku/plugins/<id>/`.
- **For local dev**, point `RYOSTORE_PLUGINS_DIR` at a folder of plugins (colon-
  separated for several) and the dev shell discovers them live.

---

## The four hosts a user can choose

When a user enables your plugin they pick **one** of these in Ryoku Settings →
Plugins. Your *same* `content/Widget.qml` is used for all four - Ryoku just
renders it differently.

### 1. Desktop widget - a tile on the wallpaper

It sits on the wallpaper next to the clock and weather, and behaves **exactly
like them**, because Ryoku gives it the identical machinery:

- **Left-drag** to move it anywhere (snaps to a grid).
- **Right-click** it for its menu (Lock to freeze it, Hide to turn it off).
- **Drag the bottom-right corner** to resize (scales 50%–250%, live).
- It's drawn on a **card** (rounded, translucent, soft shadow) matching the
  clock/weather.

**You do nothing to get any of that.** You just write the content; Ryoku wraps it
in the draggable, resizable, right-clickable card. The only thing you owe it: a
content root that reports its natural size (see "Sizing" below), so the card can
size itself around you.

### 2. Frame popout - grows out of a screen edge, or floats at the centre

It melts out of the frame border on hover (like the volume mixer and power
menu), fused into the same blob. The user picks the **edge** (top / right /
bottom / left) and where along it the popout sits (`start`, `center`, `end`), by
dragging the popout chip around a mock of their screen in Ryoku Settings.

Dropped in the **middle** of that screen instead, the popout becomes a centred
surface: it floats in the middle of the display, all four corners rounded, with
no hover edge. A centred popout opens only when it is asked for, by
`ryoku-shell plugin <id>` or a click, which makes it the placement
for a modal view. It shares the middle of the screen with quick settings
(`Super+Escape`) and the stash (`Super+S`), but the shell only ever shows one
surface at a time, so they take turns rather than overlap.

- Ryoku handles the **hover trigger, the open/close animation, and the fuse into
  the frame**.
- **Dimensions & Sizing**:
  - By default, edge popouts use a compact `360px` width budget and grow vertically
    to fit your content's `implicitHeight`.
  - For rich or modal views (calculators, email, system monitors), you can specify
    a custom width and height:
    - **In manifest defaults**: declare `"width"` and/or `"height"` under `defaults.framePopout`.
    - **In plugin settings**: declare `"width"` and/or `"height"` controls in `metadata.settings`.
      Ryoku Settings will expose them directly to users, and the popout will resize live.
    - **Implicit fallback**: if no fixed setting is set, Ryoku respects your root
      `implicitWidth` (falling back to standard 680px for centered popouts).
- **Dismissal & Lifecycle**:
  - **Edge hover popouts** auto-retract after the pointer leaves (`autoDismiss: true`).
  - **Centered modal popouts** stay open until explicitly closed via `Escape`, clicking
    outside, or toggling the shortcut (`autoDismiss: false`).
  - To override this behavior on any popout, set `"autoDismiss": true` or `false`
    in `defaults.framePopout`.

`ryoku-shell plugin <id>` toggles your frame popout open (bind it to a key).

### 3. Bar glyph - rides the bar

It rides the top bar as its own module, in a section you choose (left, centre or
right) and at the position in that lane you drag it to. Your content is rendered
at `glyph` density on the bar's axis, so it must stay small: report a mark-sized
`implicitWidth`/`implicitHeight` (roughly the bar's inner height) and Ryoku
centres it and reserves exactly that much room; in islands form it gets its own
island.

- Ryoku handles the **island pill, spacing, and the bar's width arithmetic**.
- A bar glyph **is a layout entry**, the same kind of entry as a built-in widget:
  it lives in `shell.json` `qsbar.layout`, so a user drags it between lanes in QS
  Bar Settings' Layout route or moves it from a terminal with `ryoku-shell bar
  move <id> --section left|center|right`. It shows once it is enabled with
  `host: topbarGlyph` in `plugins.json`.

Declare `topbarGlyph` in `hosts`, and include `"glyph"` in
`capabilities.densities` - a plugin that only draws a card is a poor bar glyph.

What the bar host sets on your `content` (read them, never assign them):

| Property | Value on the bar |
| --- | --- |
| `density` | `"glyph"` |
| `widthBudget` | `220` (logical px; cap any text you draw to it) |
| `active` | `true` while mounted |
| `pluginApi` | your handle: `mainInstance`, `pluginSettings`, `pluginDir` |

The host reads your `implicitWidth`/`implicitHeight` and centres the view in a
slot about 32 px tall. Draw the mark with the shell's own icon font: `Text {
font.family: "Material Symbols Rounded"; text: "vpn_lock" }` (the font ships
with the desktop, ligature names are the symbol names). Hover text is yours to
add; `QtQuick.Controls`' `ToolTip` resolves inside a plugin and matches the
shell when styled from the kit `Theme`. On the bar your settings are rendered
by QS Bar Settings, which draws these `metadata.settings` types: `toggle`,
`choice` (a segment bar up to four plain options, chips beyond), `multi`, `int`
(a stepper with `min`/`max`) and `text`; `slider` and `image` are desktop-widget
menu types and do not render there, so a bar widget picks from the first set.

Install a bar plugin straight from git, or from a folder on this desktop, with
`ryoku plugin add <git-url|dir> --bar`: it validates the manifest, installs the
files through Ryostore's transaction (a receipt and a content-hashed copy under
`~/.local/share/ryoku/plugins/<id>`), enables it on the bar, and the shell
picks it up on the `plugins.json` change, no restart. It lists under QS Bar
Settings > Community (any plugin whose manifest is not `"official": true`)
with the community warning, its author, its switch and settings, and
EXPORT / SHARE TO RYOSTORE / REMOVE. `ryoku plugin list|remove|validate`
manage it from a terminal; see "Share it" below for export and share.

### 4. Sidebar card - lives in a global sidebar

A sidebar card lives in the left or right global push-aside sidebar. Write the
same two entry points as every other plugin: `service/Main.qml` for persistent
logic and `content/Widget.qml` for the card view. There is no
`content/Sidebar.qml`. Ryoku mounts `content/Widget.qml` inside the same card
shell as built-in sidebar cards, so it supplies the plate, title treatment,
motion, width, and placement.

The card root must be an `Item` with `pragma ComponentBehavior: Bound`. It
declares the properties and signal below; read host-set properties, never assign
them:

```qml
pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.PluginKit
import Ryoku.PluginKit.Singletons

Item {
    property real s: 1
    property bool open: false
    property real reveal: 0
    property bool tabActive: false
    property var pluginApi

    signal requestClose()

    implicitHeight: 240 * s
    // Ryoku sets the built-in width property.
}
```

| Contract member | Meaning |
| --- | --- |
| `s` | UI scale multiplier. |
| `open` | `true` once the sidebar is settled open. |
| `reveal` | Current reveal progress from `0` to `1`. |
| `tabActive` | Whether the card's tab is the visible tab. |
| `width` | Host-managed card width. Report `implicitHeight`; do not assign the width. |
| `pluginApi` | `mainInstance`, `pluginSettings`, `pluginDir`, `stateDir`, and `saveSetting(key, value)`. |
| `requestClose()` | Signal the card emits when its action should close the sidebar. |

Sidebar placement is stored under the plugin's `sidebarCard` placement object:

- `side`: `"left"` or `"right"`.
- `tab`: the tab label; `"Plugins"` is the shared trailing plugin tab.
- `order`: numeric order among cards in that tab.
- `label`: the card's displayed label.
- `glyph`: a Material Symbols Rounded ligature, default `"extension"`.

Manifest suggestions use `"defaults": { "host": "sidebarCard", "sidebar": {
"side": "left", "tab": "Plugins", "order": 10 } }`. The manifest declares
`sidebarCard` in `hosts`, keeps `entryPoints.content` at
`content/Widget.qml`, and uses `["compact"]` for
`capabilities.densities`.

The golden rule is unchanged: the plugin draws content, never its own plate,
window chrome, position, or motion. R1-R11 are identical for sidebar cards;
their public imports are still `QtQuick*`, `Quickshell*`, `Ryoku.PluginKit`,
`Ryoku.PluginKit.Singletons`, and files inside the plugin folder.


> Island and window hosts are planned but not built yet. `sidebarCard` is
> shipped: declare only `framePopout`, `desktopWidget`, `topbarGlyph`, or
> `sidebarCard` in your manifest today.

---

## A bar panel

A bar plugin (host `topbarGlyph`) can ship a second view, `content/Panel.qml`,
and declare it as `entryPoints.panel`. When it does, the bar host renders that
file in the shared plugin panel window under the plugin's glyph, in the same
surface the built-in panels use: a `PillShadow`, a connected surface, `Escape`
or an outside click closes it, and only one panel is open at a time. The mark on
the bar stays your `content/Widget.qml`; the panel is what opens beneath it.

`Panel.qml` receives the same host-set properties as `content/Widget.qml`, at
`full` density:

| Property Ryoku sets | Value on `Panel.qml` |
| --- | --- |
| `pluginApi` | your handle (the reference below) |
| `density` | `"full"` |
| `s` | scale multiplier; multiply your sizes and font sizes by it |
| `widthBudget` | the panel width from manifest `panel.width` (default `320`, clamped `240..520`) |
| `active` | `true` while the panel is open |

The host fills `Panel.qml` to the card's inside width, `panel.width` minus 24 px
of padding, and reads back its `implicitHeight` to size the card, capping that
height to the screen. `content/Widget.qml`'s click handler is your own; the
sanctioned pattern is a left click that toggles the panel, and no click ever
mutates anything:

```qml
onClicked: pluginApi.togglePanel()
```

### The `pluginApi`

Every entry point (`main`, `content`, `panel`) receives one `pluginApi` object.
Read it; never reassign it.

| Member | Kind | What it is |
| --- | --- | --- |
| `mainInstance` | `var` | your live `service/Main.qml` instance |
| `pluginSettings` | `var` | the resolved settings; read each key behind its default |
| `pluginDir` | `string` | the installed plugin folder (build `bin/` paths from it) |
| `stateDir` | `string` | `$XDG_STATE_HOME/ryoku/plugins/<id>`, created by the host on load; the only place you write (R8) |
| `saveSetting(key, value)` | function | persists one setting through `ryoku-plugins-place`; `pluginSettings` re-derives on the `plugins.json` change (R5) |
| `panelOpen` | `bool` (readonly) | whether the bar panel is open (bar host; present but inert elsewhere) |
| `openPanel()` / `closePanel()` / `togglePanel()` | function | open, close, or toggle the bar panel |

Your service and its `pluginApi` survive a settings write: `saveSetting`
persists the value and re-derives `pluginSettings` without tearing anything
down, so the service stays alive and the panel stays open. The host destroys and
recreates the service and the api only when the bar row itself is rebuilt, when a
widget in that lane is moved, added, or removed.

The VPN plugin in Ryostore is the worked bar example: a glyph that shows the
connection state, a 320-wide panel with the switch, the details, and an
AUTHORISE button, and not one click on the bar that changes the network.

---

## Writing `content/Widget.qml`

For bar glyphs, desktop widgets, and frame popouts, Ryoku sets these properties
on the root **for you to read** - **never assign them**. Sidebar cards use the
separate card contract in section 4 above.

| Property Ryoku sets | What it means | What you do with it |
|---|---|---|
| `pluginApi` | handle to your service + settings + folder | read `pluginApi.mainInstance` for your live `service/Main.qml` |
| `density` | `"glyph"`, `"compact"`, or `"full"` | lay out smaller/larger for the space you're given |
| `widthBudget` | the width you have to lay out within | size your content to this width, not to the screen |
| `active` | true while your widget is open/visible | start/stop work (e.g. don't poll when hidden) |
| `screen` | the monitor this copy is on | usually ignore it |

### Sizing - the one thing you MUST get right

**Declare your content's natural size; never hardcode geometry.** Ryoku reads
your root's `implicitWidth` / `implicitHeight` to size desktop and popout
surfaces. A sidebar card gets its width from Ryoku and reports `implicitHeight`.
If you don't report a size, your widget collapses to nothing.

```qml
import QtQuick
import Ryoku.PluginKit          // the deck kit: Theme, Motion, Card, etc.

Item {
    id: root

    // Ryoku sets these; you only read them.
    property var pluginApi
    property string density: "full"
    property real widthBudget: 0
    property bool active: false

    readonly property var service: pluginApi ? pluginApi.mainInstance : null

    // Pick ONE content width from the budget, and lay everything out from it.
    readonly property real contentW: widthBudget > 0 ? widthBudget : 360

    // Report your natural size so the host can size its surface around you.
    implicitWidth: contentW
    implicitHeight: column.implicitHeight

    Column {
        id: column
        width: root.contentW          // bind children to contentW, NOT parent.width
        spacing: 12
        // ... your eyebrow, search, list, grid ...
    }
}
```

Rules that keep it native:
- **Bind children to your own `contentW`**, never to `parent.width` through
  nested layouts (that leaves width-derived heights at zero and your widget
  collapses).
- **Reflow with `Column` / `Row` / `Grid` / `Flow`** so you adapt to the width
  Ryoku gives you. Don't position with absolute `x`/`y`.
- **Three densities**: `glyph` = an icon (+ optional badge); `compact` = a small
  summary; `full` = the rich panel. Lay out for whichever `density` you're handed.

---

## Styling - use the kit, match the shell

Your runtime content imports the **deck kit** so it looks like the rest of the
shell automatically:

```qml
import Ryoku.PluginKit            // Theme, Motion, GlyphIcon, MicroLabel,
                                  // SearchField, Card, CornerTicks, WaveMeter
```

Use `Theme` colors and `Motion` curves - mono eyebrows, hairline dividers, the
vermillion accent, the project's morph timing. **Don't hardcode colors**; read
them from `Theme`. The vermillion accent is the one accent - use it sparingly.

Your **settings are not QML** - you declare them as a `metadata.settings` schema
in the manifest (below) and Ryoku renders native controls for them, both in
Ryoku Settings and in the desktop widget's right-click menu.

---

## `service/Main.qml` - your logic

A non-visual `QtObject`/`Item` that holds state and does the work (HTTP, running
your `bin/` scripts, parsing results). Ryoku keeps one instance alive and hands
it to your content as `pluginApi.mainInstance`.
Declare user options as a `metadata.settings` schema in your manifest (below).
Ryoku renders the controls, seeds your defaults when the plugin is enabled, and
persists changes to `plugins.json`; read the live values from
`pluginApi.pluginSettings`. Read every key behind its default anyway: a plugin
enabled before a setting existed, or a settings block a user trimmed, has no
value for it, and `undefined` must not become your poll interval.

---

## `manifest.json`

```json
{
  "id": "your-plugin",
  "name": "Your Plugin",
  "version": "1.0.0",
  "author": "You <you@example.com>",
  "description": "One sentence describing it.",
  "license": "MIT",
  "official": false,
  "entryPoints": {
    "main": "service/Main.qml",
    "content": "content/Widget.qml",
    "panel": "content/Panel.qml"
  },
  "panel": { "width": 320 },
  "files": ["content/Helper.qml", "assets/example.jpg"],
  "capabilities": {
    "densities": ["glyph", "compact", "full"],
    "network": ["login.tailscale.com"],
    "privileged": ["pkexec tailscale set --operator"]
  },
  "hosts": ["topbarGlyph", "desktopWidget"],
  "defaults": {
    "host": "topbarGlyph",
    "bar": { "section": "right" },
    "icon": "vpn_lock",
    "label": "Your Plugin"
  },
  "commands": ["bin/your-tool"],
  "dependencies": { "commands": ["curl", "jq"] },
  "metadata": {
    "settings": [
      { "key": "poll", "type": "int", "label": "Refresh seconds", "group": "General", "default": 10, "min": 3, "max": 60 },
      { "key": "barLabel", "type": "choice", "label": "Beside the mark", "group": "General", "default": "name",
        "options": [ { "value": "none", "label": "Nothing" }, { "value": "name", "label": "Name" }, { "value": "ip", "label": "IP" } ] },
      { "key": "confirmOff", "type": "toggle", "label": "Ask before turning off", "group": "General", "default": true }
    ]
  }
}
```

- `hosts` - declare **only the hosts you actually support and have tested**
  (today: `framePopout`, `desktopWidget`, `topbarGlyph`, `sidebarCard`). Don't
  list hosts that don't work.
- `defaults` - *suggestions*. The user's choices in Settings always win. For
  `framePopout`, `align` is `start`, `center` or `end`, and `edge: "center"`
  asks for the centred (modal) surface, which opens only on request. A bar
  widget's defaults are just `{ "host": "topbarGlyph", "icon": "...", "label":
  "..." }`, plus an optional `"bar": { "section": "left|center|right" }` for
  the lane it first lands in. A sidebar card uses `{ "host": "sidebarCard",
  "sidebar": { "side": "left|right", "tab": "Plugins", "order": 10 } }`.
  `icon` and sidebar `glyph` values are Material Symbols Rounded ligature names
  (`vpn_lock`, `extension`, ...).
- `official` - leave `false`. Only first-party Ryoku plugins set `true`; every
  other plugin lists under QS Bar Settings > Community and carries the store's
  community warning.
- **A plugin never gets a keybind of its own.** Ryoku has no plugins-menu leader
  and reads no `key` field from your manifest; do not ship one, and do not tell
  users a chord opens your plugin. A frame popout opens on hover at its edge, or
  through `ryoku-shell plugin <id>`, which the user can bind to whatever chord
  they like in Settings → Keybinds. The shipped bind table stays Ryoku's.
- `files` - any extra files the plugin ships beyond its entry points and
  `commands` (helper QML a view imports, images, data). Install fetches the entry
  points, `commands`, `README.md`, and everything listed here; a file you forget
  to list is simply missing on install, so the plugin can fail to render.
- `entryPoints.panel` - a bar plugin's panel view, `content/Panel.qml` (see
  [A bar panel](#a-bar-panel)). Declare it only alongside host `topbarGlyph`.
- `panel.width` - the panel's width budget in logical px, default `320`, clamped
  `240..520`.
- `capabilities.network` / `capabilities.privileged` - every host the plugin
  talks to and every exact `pkexec` command it can run (R6, R7). Both are
  optional; omit them when the plugin does neither. The two lines shown above are
  the VPN plugin in Ryostore, which reaches `login.tailscale.com` and asks for
  `pkexec tailscale set --operator`; declare only your own.

---

## Security

`ryoku plugin validate <dir>` runs the manifest checks and a static audit, and
prints each finding as `rule-id  path:line  message`. **Blocking** rules make
`validate` exit non-zero and make `ryoku plugin add` refuse to install (unless
you pass `--allow-findings`); `--allow <rule>` downgrades one named blocking rule
to a warning for the run. **Warning** rules are printed but do not block.

| Rule id | Blocks? | What it catches |
| --- | --- | --- |
| `symlink` | **yes** | any symlink in the tree (R2, R10) |
| `escalation` | **yes** | `sudo`, `doas` or `su` anywhere, or `pkexec` not declared in `capabilities.privileged` (R6) |
| `pipe-shell` | **yes** | `curl … \| sh` and other download-and-run (R7) |
| `internal-import` | **yes** | importing `shell.*` / `Ryoku.Ui*` internals or climbing out of the folder (R4) |
| `config-write` | **yes** | writing to `shell.json`, `plugins.json`, `~/.ssh`, `/etc`, or shell rc files (R5, R8) |
| `secret` | **yes** | a token, key or credential in the tree (R10) |
| `binary` | **yes** | a compiled binary, ELF/Mach-O/`.so` (R10) |
| `undeclared-command` | no | a `Process` / `execDetached` `argv[0]` not in `bin/`, `dependencies.commands`, or the allowlist (R6) |
| `undeclared-host` | no | an `https?://host` literal not in `capabilities.network` (R7) |
| `dynamic-shell` | no | `sh` / `bash -c` whose argument is not a single string literal (R9) |
| `outside-write` | no | a write outside `stateDir`, cache, or a temp dir (R8) |
| `large-tree` | no | a tree larger than 20 MB |

The audit is a safety net, not the whole story: the review that lists a plugin
on Ryostore checks the listing, not the code, so a community plugin still runs
unsandboxed with your permissions. R1..R11 are what keep it honest.

---

## Install, enable, place

- **Install**: `ryoku plugin add <git-url|dir> [--bar]` (validated, then
  installed through Ryostore's transaction so the shell's `discover.sh` loads
  it), or install it from Ryostore itself. Never hand-copy into
  `~/.local/share/ryoku/plugins/`: a folder without a receipt is not loaded.
- **Enable & place**: Ryoku Settings → Plugins. The user toggles it on, picks a
  host, and sets the host placement: edge for a frame popout, or side, tab and
  order for a sidebar card. Placement saves to
  `~/.config/ryoku/plugins.json`; the shell watches that file and retunes live -
  no restart.
- **Desktop widgets** are then moved/resized/hidden directly on the wallpaper
  (drag, corner-resize, right-click) - not from Settings.

---

## Share it

A widget written on one desktop (by hand, or by asking Rashin) reaches every
other one through Ryostore. Two commands do the packaging, so the catalogue's
per-file hashes are never typed by hand:

```bash
ryoku plugin export vpn      # ~/Documents/ryoku-plugins/vpn/: the files,
                             # product-manifest.json, registry-entry.json, a git repo
ryoku plugin share vpn       # exports if needed, then opens the Ryostore pull
                             # request (gh logged in) or the submission form
```

`export` copies the installed plugin out, writes `product-manifest.json` (the
sha256/size/mode of every file, docs and preview media marked `install: false`,
executables `0755`) and `registry-entry.json` (a complete `plugins/registry.json`
row: `official: false`, `hosts` from the manifest, the `bar-widget` or
`desktop-widget` tag added), and puts the folder under git. `share` lays that
into a fork of `ryoku-dev/ryostore` as `plugins/<id>/`, upserts the registry
entry, pushes `plugin/<id>` and opens the pull request with the catalogue's
checklist; without `gh` it opens the submission form prefilled and tells you to
push the folder somewhere public first. A real `assets/preview-widget.png` is
required either way. Both actions are also buttons on the plugin's row under QS
Bar Settings > Community.

Ryostore lists community plugins under a warning: the review is for listing, not
a security audit, and a plugin runs unsandboxed in the user's shell. The store's
Plugins tab parts BAR widgets (manifest `hosts` includes `topbarGlyph`) from
DESKTOP ones; tag yours `bar-widget` or `desktop-widget` to match.

---

## Gotcha: images in grid/list delegates

An `Image` inside an inline QML `component` used as a delegate **loads but never
paints** in Quickshell (a scene-graph quirk). Put image/thumbnail delegates
inline - a plain `Rectangle { Image {} }` - or in their own `.qml` file, **not**
inside an inline `component { ... }`. See `wallhaven`'s grid for the working
pattern.


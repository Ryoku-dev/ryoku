# Bar styles

Ryoku ships six bar styles, and a single key decides which one runs. The
shipped default is **QS Bar** (`qsbar`), the full-colour top bar. **Shima**
(`iris`), **Python** (`python`), and **Nomarchy** (`nomarchy`) are the frame
family: whole-desktop systems hosted once on the primary output. Shima is an
island on any screen edge that morphs into whatever you clicked, with glass
popups, bubbles, a dock, a live Studio and its own settings overlay. Python is a
bar of pill widgets that open into one morphing stage (its own guide carries its
settings), based on Serpantinum by ilyamiro. Nomarchy preserves Omarchy
quattro's bar, panels, widgets and plugin host while routing desktop state and
actions through Ryoku. **Sumi** is the monochrome left rail, and **Kairos** is a
single island at the top centre that carries the clock: hovering opens it into a
rolling date wheel, a track that plays adds a cover bubble beside it, and
Super+Space grows the same island into its own app launcher
(`docs/launcher.md`). Sumi is not a folder: the shell paints it from the
built-in frame scene in `shell.qml`, so it has no scene file of its own. The
other five live under `ryoku/shell/quickshell/shell/modules/bar/barstyles/`,
ship their own scene, and load once per monitor. Shima is special: its scene is
a thin host that mounts the vendored frame family in
`ryoku/shell/quickshell/inir/`. It is based on iNiR by snowarch
(https://github.com/snowarch/inir) and tracks its releases (the one it follows is
in `inir/VERSION`). iNiR's NOTICE grants no rights to the names iNiR and iRiS,
their logos or the Kira mascot, so Ryoku ships it as Shima with its own mark and
no mascot, keeps that NOTICE and the credit in the About page, and leaves out
upstream's IrisGate, which closes the surfaces under any host but iNiR. Internal
names (`iris` ids, `Iris*` files, `inir.iris.*` keys) are unchanged so settings
carry over. Nothing in it is tied to niri: it
reads the desktop only through the window-manager seam and the daemon's
settings topic, so it runs on every compositor Ryoku supports.
Store-installed styles land under the same folder contract.

**QS Bar wears colour on purpose, and it is the one place the desktop does.** It
is a Quickshell "Rise" bar ported onto Ryoku's data plane, and it keeps that
project's full-colour skin: its `Theme.qml` reads the live wallpaper palette from
`~/.cache/ryoku/colors.json` into a seven-slot set (`color01..07`) over a warm,
near-black `paper`, so the bar retints with the wallpaper like a terminal theme
rather than clamping to bone-on-black. Sumi is the paper-and-ink bar for anyone
who wants the rest of the desktop's restraint on the edge too; `docs/ui-ux.md` is
where colour is and is not allowed, and the bar is the sanctioned exception.

## Nomarchy

Nomarchy is the Ryoku name for the Omarchy quattro shell port pinned to upstream
commit `d9b970dd`. Its source lives under `barstyles/nomarchy/`; the upstream
MIT license and port notice sit at the root of that folder. The port retains
Omarchy's visual bar, panels, widgets, menus, history UI, OSD, reminders, media
integration, configuration and manifest-based plugin ABI.

Ryoku remains authoritative for desktop-wide facilities. Nomarchy uses the
neutral `Wm` singleton for outputs, workspaces, windows and keyboard layouts.
Ryogami opens wallpapers, ryoshot and Recorder handle capture, the resident
notification server owns D-Bus notifications, and Ryoku Hub owns updates. The
stock Agents panel remains available for usage and marketplace extensions that
write into it. Account add, rename, reauthentication, switching and automatic
switching controls are hidden when the platform capability check reports them
unavailable. Its launch action opens Ryoku Ask, and the duplicate
starter-prompt section is not loaded. The Ryoku lock screen,
clipboard, policy agent, idle inhibitor and night-light service replace the
corresponding upstream plugins.

The theme carousel remains part of Nomarchy and can be opened from Bar Studio.
The same card opens Omarchy's plugin menu. Third-party plugins still live at
`~/.config/omarchy/plugins/<id>/`, but every enable and load is gated by
`ryoku wm compat <plugin-dir>`. Results are cached only while the plugin tree is
unchanged; failures and malformed output block the plugin.

The stable IPC target is `nomarchy`, for example:

```sh
omarchy-shell nomarchy listPlugins
omarchy-shell nomarchy summon omarchy.image-picker '{"source":"themes"}'
```

The detailed plugin manifest and host contract are documented in
`barstyles/nomarchy/plugins/README.md`.


Most bar styles own the bar and nothing else: frame border, menus, service
surfaces and tokens stay shared. Nomarchy is the deliberate exception because
its upstream contract includes companion panels, menus, notifications and OSD;
those exist only while the style is active and still hand desktop-wide work to
Ryoku's resident services.

## How selection works

The `barStyle` key in `~/.config/ryoku/shell.json` picks the active style by id.
It is a top-level string, default `"qsbar"`:

```json
{
  "barStyle": "sumi"
}
```

`Config.qml` surfaces it as `Config.barStyle`, and the file is watched, so a save
retunes the running shell without a reload. The valid ids are resolved by the
`BarProducts` singleton (`shell/services/BarProducts.qml`), not a static registry
file. Built-in folder styles ship inside the shell, one row each:

```qml
// BarProducts.qml
readonly property var builtins: ({
    "qsbar": "barstyles/qsbar/Scene.qml",
    "kairos": "barstyles/kairos/Scene.qml",
    "iris": "barstyles/iris/Scene.qml",
    "python": "barstyles/python/Scene.qml",
    "nomarchy": "barstyles/nomarchy/Scene.qml"
})
```

`BarProducts.sceneUrl(id)` is the one lookup the shell needs, and it returns:

- `""` for `"sumi"`, an empty id, or a style that has failed to load. An empty
  scene is the built-in frame scene (Sumi), which `shell.qml` paints itself.
- the built-in's relative `Scene.qml` for a built-in id (`"qsbar"`, `"kairos"`,
  `"iris"`, `"python"`, `"nomarchy"`).
- a `file://` path drawn from `~/.local/state/ryoku/store/barstyles.json` for a
  store-installed folder style. The store writes that index and a `revision.json`;
  `BarProducts` watches both and reloads live.

`Frame.qml` reads the result through one derived flag:

```qml
import shell.services

readonly property bool sumiActive: root.degraded
    || BarProducts.sceneUrl(Config.barStyle) === ""
```

`sumiActive` is the gate. While it is true, the built-in frame chrome and the four
rails draw, and each `FrameEdge` reserves its exclusive zone
(`reserve: root.sumiActive ? root.edgeReserve("top") : 0`). While it is false, the
frame scene hides, the edges release their reserves, and a per-monitor `Loader`
mounts the active style's `Scene.qml`:

```qml
Loader {
    id: barStyleLoader
    active: !root.sumiActive && (root.barEnabled || root.primaryHost)
    source: BarProducts.sceneUrl(Config.barStyle)
    onLoaded: if (item) item.modelData = root.modelData
}
```

`Frame.qml` is itself instantiated once per screen by `shell.qml`'s `Variants`, so
the contract is: your `Scene.qml` loads once per screen and takes the screen
through a `modelData` property. Everything else is yours.

A load error is handled by kind. A store style is failed at once
(`BarProducts.fail`), so `sceneUrl` returns `""` and Sumi draws until the store
index reloads. A built-in style cannot be legitimately broken, so Frame retries it
ten times, 800 ms apart, to ride out an update's plugin swap; if it still fails,
`BarProducts.markBroken` records it and every monitor degrades to the Sumi rail
instead of an empty frame, until the user picks a style again.

Ryoku Settings > Displays can suppress the active bar on any output. Sumi releases
its rail reserve there, normal folder styles are not instantiated there, and QS
Bar filters that output from its shared multi-monitor bar model. The primary
output still hosts QS Bar and any frame-family style (`BarProducts.isFrameFamily`,
today Shima, Python and Nomarchy), because their popups, docks and panels serve
the whole desktop. A missing per-display setting means enabled, so upgrades
preserve the existing layout.

**To add a built-in style, drop its folder under `barstyles/` and add one row to
`BarProducts.builtins`.** A store style needs no shell edit: it installs into
`~/.local/state/ryoku/store/barstyles.json` and resolves through the same
`sceneUrl` path.

The shipped Sumi profile is left-only: `FrameBars.js` `defaultConfig()` enables the
left rail and leaves the other three off. That profile applies only while
`barStyle` is `"sumi"`.

### One gotcha, three parts

A folder scene is loaded by URL, not compiled into the shell, and that changes
how edits land:

- **Structural edits need a restart.** Adding a file, adding an import, or
  changing the shape of a loaded `Scene` is not picked up by hot-reload the way
  an edit to a resident QML file is. Restart the shell after a structural
  change: `systemctl --user restart ryoku-shell`. Property tweaks inside an
  already-loaded scene reload live; new files and new imports do not.
- **Same-directory types are not auto-imported.** QML does not put sibling files
  in scope just because they share a folder. Keep shared pieces in a subdirectory
  and import it namespaced: `import "components" as C`, then `C.BarPill { ... }`.
  A bare `BarPill { ... }` next to `BarPill.qml` will not resolve.
- **Reach the shell's shared code through the SDK modules, not relative paths.**
  A product imports the shell's singletons with `import shell.services` (Theme,
  Media, Notifs, Battery, Network, ...) and the non-singleton primitives with
  `import shell.barkit as Pill` (the icon and brand types, `MusicBars`,
  `TrayMenu`, `NotificationCard`, the `Popout` base building blocks, and the
  audio and notification menus). Both are named modules resolved through the
  Quickshell import path, so they work from any folder depth. `shell.services`
  hands back the shell's own live singleton instances -- a product's `Notifs` is
  the one notification server, never a second one. A product still reaches its
  OWN files by relative path (`import "components" as C`). Get a module name
  wrong and the scene loads to a blank strip with import errors in the shell log.

## The shape of a style

A folder style owns its own tree; QS Bar's is the shipped example:

```
barstyles/
  qsbar/
    Scene.qml        // the per-monitor entry the Loader mounts
    Theme.qml        // its palette, retinted from the wallpaper
    components/      // shared pieces (import them namespaced)
    modules/         // the bar widgets
    panels/          // the popout bodies
    controlcenter/   // its own settings panel, QS Bar Settings (the logo opens it)
```

Only `Scene.qml` is required; the rest is the style's own business, and a
store-installed style unpacks the same shape under
`~/.local/state/ryoku/store/barstyle-views/<id>/`. Keep shared pieces in
subdirectories and import them namespaced (QML does not put siblings in scope just
because they share a folder), and reach the shell's singletons through the SDK
modules below rather than guessing a relative path.

`Scene.qml` is a `PanelWindow`, one instance per monitor. It takes the screen
through `modelData`, anchors itself to an edge, reserves its band with an
exclusive zone, and masks input to just the interactive pills so the rest of the
strip is click-through. A minimal scene:

```qml
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import shell.barkit as Pill
import "components" as C
import "widgets" as W

PanelWindow {
    id: win

    property var modelData      // the screen, set by shell.qml's Loader
    screen: modelData

    color: "transparent"
    exclusionMode: ExclusionMode.Normal
    exclusiveZone: 46
    WlrLayershell.layer: WlrLayer.Top
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    WlrLayershell.namespace: "ryoku-bar"

    anchors { top: true; left: true; right: true }
    implicitHeight: 52

    // Only the pill takes clicks; the rest of the bar passes them through.
    mask: Region { Region { item: pill } }

    C.BarPill {
        id: pill
        anchors.centerIn: parent
        W.Clock {}
    }
}
```

`BarPill` is a small helper: a rounded `Theme.surface` rectangle that hugs a
centred `Row` of whatever widgets you drop in, and hides itself when empty. A
widget, in turn, is any `Item` that reports an implicit size; the pill lays them
out left to right. The smallest useful widget is a bound `Text`:

```qml
import QtQuick
import shell.services

Text {
    text: "Desktop"
    color: Theme.onSurfaceVariant
    font.family: Theme.fontPrimary
    font.pixelSize: Theme.fontSm
    elide: Text.ElideRight
    width: Math.min(implicitWidth, 260)
}
```

Everything past this point is about what a widget binds to.

## Fetching data

This is the real work. The shell already gathers every live fact through its
singletons; a widget reads them and draws. Import the singleton's module once
and use that contract rather than probing a compositor directly.

### Workspaces and windows

Window-manager state and actions live in `Wm` from `Ryoku.Ui.Singletons`. Every
style must render and behave identically on every compositor. Never import
`Quickshell.Hyprland`, `Quickshell.Services.Hyprland`, or a compositor-specific
backend from a style.

```qml
import QtQuick
import Ryoku.Ui.Singletons

Item {
    readonly property var workspaces: Wm.workspaces
    readonly property var activeWorkspace: Wm.focusedWorkspace
    readonly property string activeTitle: Wm.focusedWindow
        ? Wm.focusedWindow.title : ""

    function occupied(workspace) {
        return workspace.occupied === true;
    }

    function activateWorkspace(workspace) {
        Wm.focusWorkspace(workspace.id);
    }

    function cycle(delta) {
        Wm.cycleWorkspace(delta);
    }
}
```

Each workspace has `id`, `name`, `active`, `urgent`, `occupied`, `windows`,
`output`, `fullscreen`, `special`, `layout`, and `canActivate`. Use the opaque
`id` for actions and the human `name` for labels. The live window list is
`Wm.windows`; `Wm.focusedWindow` and `Wm.focusedWorkspace` carry focus. Window
actions go through `Wm.focusWindow(id)` and `Wm.closeWindow(id)`, just as
workspace actions go through `Wm.focusWorkspace(id)` and
`Wm.cycleWorkspace(delta)`.

### Media

The `Media` singleton is the one now-playing pick every surface shares. It
prefers a sounding MPRIS player and ignores the live wallpaper's video.

```qml
visible: Media.present            // a real track is loaded
Text { text: Media.line }         // "title · artist", ready to bind

// Media.player is the raw MPRIS player, or null. Guard every read.
Image { source: Media.player ? (Media.player.trackArtUrl || "") : "" }
Text  { text: Media.player ? (Media.player.trackTitle || "") : "" }
Text  { text: Media.player ? Theme.joinArtists(Media.player.trackArtists, Media.player.trackArtist) : "" }
```

Transport and seek read and drive the same player:

- `Media.playing` is the convenience for `Media.player.isPlaying`.
- `Media.player.position` and `Media.player.length` are seconds; the fraction is
  `position / length`. The position does not tick on its own; pulse
  `Media.player.positionChanged()` on a `Timer` while the card is open to keep a
  seek line live.
- `Media.player.previous()`, `Media.player.next()`, and `Media.toggle()` drive
  it, each gated by `Media.player.canGoPrevious`, `canGoNext`, and
  `canTogglePlaying`.

### Sysinfo

`Sysinfo` carries CPU, memory, and a best-effort package temperature. It is
owner-refcounted: it polls (on a 1.5s tick) only while a visible owner claims
it, so an unseen widget costs nothing. **Claim it on show and release it on
destruction**, or every reading stays at zero:

```qml
Component.onCompleted: Sysinfo.setActive(root, true)
Component.onDestruction: Sysinfo.setActive(root, false)
```

Then read: `Sysinfo.cpu` and `Sysinfo.mem` are `0..1` loads; `Sysinfo.memUsedGiB`
and `Sysinfo.memTotalGiB` are GiB; `Sysinfo.tempC` is degrees, present only when
`Sysinfo.hasTemp` is true (a machine with no readable CPU sensor reports none).

```qml
value: Sysinfo.cpu                                   // ring/bar fill 0..1
text:  Sysinfo.memUsedGiB.toFixed(1) + " / " + Sysinfo.memTotalGiB.toFixed(1) + " GiB"
ResBar { visible: Sysinfo.hasTemp; value: Math.round(Sysinfo.tempC) + "°C" }
```

### Battery

`Battery` reads UPower. On a desktop with no cell it reports
`Battery.present === false`, so gate the whole widget on it. `Battery.pct` is the
integer percent and `Battery.frac` the `0..1` fraction. State comes as
`Battery.charging`, `Battery.full`, `Battery.low`, and a ready string
`Battery.stateLabel`. Time-to-full or time-to-empty is `Battery.timeStr` and is
valid only while `Battery.hasTime` is true. Health is
`Battery.health` percent, shown only when `Battery.healthSupported`.

```qml
visible: Battery.present
Text { text: Battery.pct + "%"; color: Battery.low ? Theme.error : Theme.onSurface }
Text {
    text: Battery.stateLabel + (Battery.hasTime
        ? " · " + Battery.timeStr + (Battery.charging ? " to full" : " left") : "")
}
Text { visible: Battery.healthSupported; text: "Health " + Battery.health + "%" }
```

The power-profile picker in a battery card is a second singleton,
`PowerProfiles`: `PowerProfiles.available` gates it, `PowerProfiles.profiles` is
the list, `PowerProfiles.profile` is the current one, and
`PowerProfiles.setProfile(name)` switches it.

### Weather

`Weather` is a view of the daemon's `weather` topic; QML makes no HTTP call. Gate
on `Weather.available`. The compact readout uses `Weather.temp` (a ready display
string like `18°`) and `Weather.condition`; the fuller card uses
`Weather.humidity`, `Weather.wind`, `Weather.feels`, and `Weather.location`.

`Weather.current` is the current-conditions object (`code` is the WMO code,
`isDay` the day/night flag, plus `feelsLike`, `humidity`, `windValue`,
`windUnits`). `Weather.daily` is the forecast array, each entry carrying `day`,
`code`, `high`, and `low`.

The daemon ships a base glyph name as `Weather.glyph`, but the bar maps the WMO
code to the shell's own `weather-*` symbolic icon set itself, so day and night
variants and the finer conditions read right. Keep that mapping in the widget:

```qml
function iconFor(code, day) {
    const d = day ? "day" : "night";
    if (code === 0) return "weather-clear-" + d;
    if (code === 1 || code === 2) return "weather-partly-cloudy-" + d;
    if (code === 3) return "weather-overcast";
    if (code >= 51 && code <= 57) return "weather-drizzle";
    if (code >= 95) return "weather-thunderstorm";
    return "weather-cloudy";
}
// Pill.SymbolIcon { name: root.cur ? iconFor(cur.code, cur.isDay) : "weather-unknown" }
```

### Audio

`Audio` classifies the Pipewire graph and exposes the default devices as
`Audio.sink` (output) and `Audio.source` (input); either can be null, so guard
both. Volume and mute live on the node's `audio` block and are writable:

```qml
readonly property real vol: Audio.sink && Audio.sink.audio ? Audio.sink.audio.volume : 0   // 0..1
readonly property bool micMuted: !!(Audio.source && Audio.source.audio && Audio.source.audio.muted)

// set them by assignment
onMoved: v => { if (Audio.sink && Audio.sink.audio) Audio.sink.audio.volume = v; }
onTapped: { if (Audio.sink) Audio.sink.audio.muted = !Audio.sink.audio.muted; }
```

Switch the default device with `Audio.setOutput(n)` and `Audio.setInput(n)`. The
singleton also lists `Audio.outputs`, `Audio.inputs`, and per-app
`Audio.streams` for a full mixer, and resolves Bluetooth codec and profile, but a
bar widget usually wants only the two defaults.

### Network

`Network` is a view of the daemon's `network` topic. The derived status a bar
reads: `Network.kind` is `"ethernet"`, `"wifi"`, or `""`; `Network.level` is the
`0..1` Wi-Fi strength; `Network.wifiRadio` is the radio on/off; `Network.activeSsid`
and `Network.wifiConnectivity` describe the current link. A VPN indicator is
`Network.vpnActive` with `Network.vpnName`. Intents ride back as method calls:
`Network.refresh()` (scan), `Network.setWifiEnabled(on)`,
`Network.connectWifi(ssid, password)`, `Network.disconnectWifi()`,
`Network.forgetWifi(ssid)`.

### Tray

`Tray` is a view of the daemon's `tray` topic. `Tray.items` is the live SNI row;
each item carries a `service`, a resolved `iconPath` (a file) or `iconName` (a
theme name), so pick an image source from those. Left click activates,
right click asks for the item's menu, both anchored to a global point:

```qml
visible: Tray.items.length > 0
Repeater {
    model: Tray.items
    delegate: Item {
        required property var modelData
        // source: iconPath ? ("file://" + iconPath) : Quickshell.iconPath(iconName, ...)
        MouseArea {
            acceptedButtons: Qt.LeftButton | Qt.RightButton
            onClicked: event => {
                const g = mapToGlobal(0, height);
                if (event.button === Qt.LeftButton)
                    Tray.activate(modelData.service, Math.round(g.x), Math.round(g.y));
                else
                    Tray.contextMenu(modelData.service, Math.round(g.x), Math.round(g.y));
            }
        }
    }
}
```

### The audio visualizer (cava)

Two singletons feed the frequency bars, both driven by `cava` behind the scenes.
`AudioBars` is the playback spectrum: `AudioBars.levels` is an array of
`AudioBars.bars` values (40), each `0..1`, refreshed at `AudioBars.fps` (30), and
`AudioBars.energy` is the mean across the bands. Like `Sysinfo`, it is
owner-refcounted, so cava runs only while a visible surface claims it:

```qml
Component.onCompleted: AudioBars.setActive(root, true)
Component.onDestruction: AudioBars.setActive(root, false)

Repeater {
    model: AudioBars.bars
    delegate: Rectangle {
        required property int index
        height: 4 + AudioBars.levels[index] * 40   // 0..1 per band
    }
}
```

`VoiceBars` mirrors it for the microphone: `VoiceBars.levels` over
`VoiceBars.bars` (16), gated by a plain boolean you set (`VoiceBars.active = true`,
not a refcount), with a small noise `floor` so room tone does not ripple the
resting line. Both settle flat when frames stop arriving, so an idle visualizer
falls to its rest slivers rather than freezing on the last peak.

### Tokens and icons

Never hardcode a colour, a font, a size, or a duration. `Theme` carries the
shell palette and metrics: colours (`Theme.surface`, `Theme.onSurface`,
`Theme.onSurfaceVariant`, `Theme.primary`, `Theme.onPrimary`, `Theme.outline`,
`Theme.error`), fonts (`Theme.fontPrimary`, `Theme.mono`, `Theme.fontJp`,
`Theme.display`), sizes (`Theme.fontSm`/`fontMd`/`fontLg`/`fontXl`/`fontXxl`,
`Theme.iconSm`/`iconMd`/`iconLg`, `Theme.radiusWidget`, `Theme.radiusWindow`,
`Theme.borderWidth`), and `Theme.windowOpacity` for surface translucency. It also
holds small helpers like `Theme.joinArtists(artists, single)`.

`Motion` carries the timing tokens: `Motion.fast` (140ms), `Motion.standard`
(300ms), `Motion.morph` (420ms) and the rest, plus `Motion.easeStandard`. Gate
any `Behavior` on `!Motion.reduce`, which collapses animation to an instant cut
on a weak GPU or when the user asks for less motion:

```qml
Behavior on color {
    enabled: !Motion.reduce
    ColorAnimation { duration: Motion.fast; easing.type: Motion.easeStandard }
}
```

There are two glyph primitives, from the shell.barkit module
(`import shell.barkit as Pill`):

- **`Pill.MaterialIcon`** is a Material Symbols Rounded ligature. The glyph name
  is the text, and `fill` (0 or 1) picks the outline or filled variant. Use it
  for UI verbs: `Pill.MaterialIcon { text: "music_note"; font.pixelSize: Theme.iconSm }`.
- **`Pill.SymbolIcon`** is one flattened symbolic SVG from the shell's own icon
  set (`components/icons/`, freedesktop names without the `-symbolic` suffix),
  tinted to a single colour. Use it for the status glyphs the frame renders
  (battery levels, weather, network, mic):
  `Pill.SymbolIcon { name: "weather-clear-day"; size: 18; color: Theme.onSurface }`.

## The popout pattern

A status widget grows a hover card. A style provides its own popout card,
and its contract is three properties:

- `target`: the widget `Item` the card anchors under.
- `targetHovered`: a boolean, bound to a `HoverHandler` on the widget, that says
  the pointer is over the target.
- `content`: a `Component` drawn inside the card.

The popout opens while the pointer is over the target or the card, and eases shut
a moment after both are left. It is its own Overlay `PanelWindow`, click-through
outside the card, and it centres itself under the target while clamping to the
screen. Wire it into a widget:

```qml
import "../components" as C

Item {
    id: root
    // ... the compact readout ...
    HoverHandler { id: hh }

    C.Popout {
        target: root
        targetHovered: hh.hovered
        content: popContent
    }

    Component {
        id: popContent
        Item {
            implicitWidth: col.implicitWidth + 40
            implicitHeight: col.implicitHeight + 36
            Column { id: col; anchors.centerIn: parent; /* the card body */ }
        }
    }
}
```

The card sizes itself from the `content` component's implicit size, so give the
body an `implicitWidth`/`implicitHeight`. Keep any live work (a seek `Timer`, a
poll) inside the `content`, so it runs only while the card is open.

## Per-style settings

A style keeps its own settings in a namespaced key in `shell.json`, read through
`Config`, so a user tweak survives updates and never lives in a shipped file. Give
the key the style's id: a top-level alias plus a var in the adapter.

```qml
// services/Config.qml: a top-level alias, plus a var in the JsonAdapter
property alias qsbar: adapter.qsbar
// inside JsonAdapter { ... }
property var qsbar: ({})
```

The Scene reads it and gates each widget. An absent key reads as its default, so
the bar is whole until a value explicitly changes it:

```qml
function shows(id) { return !Config.qsbar || Config.qsbar[id] !== false; }
...
W.Media { visible: win.shows("media") && Media.present }
```

QS Bar's `barScale` setting is a 1.0–2.0 multiplier for its height. QS Bar
Settings exposes it as **Size** (100–200%), so a high-DPI output can use a
larger bar without changing its display scale.

Where that key is edited is the style's call. The built-in Sumi bar is edited from
**Bar Studio** in Ryoku Settings (`hub/quickshell/pages/BarStudioPage.qml`), which
snapshots the keys and applies them live. A folder style usually ships its own
settings surface instead, the way QS Bar carries **QS Bar Settings** (see below).
A style with no settings omits all of this.

## QS Bar Settings

QS Bar carries its own settings panel, **QS Bar Settings** (`controlcenter/`), the
plate the bar's 力 logo opens. It is a quick panel, not a second Hub: six routes
over the same `Ryoku.Ui` form kit the rest of Ryoku Settings uses.

| Route | Gloss | Holds |
|---|---|---|
| Bar | 帯 | position, form, surface, gaps, scale, accent, gap animation, auto-hide, picker style |
| Identity | 印 | launcher mark and workspace count and marker style |
| Layout | 配置 | the three lanes: move, hide, add and reset the bar's widgets |
| Widgets | 部品 | every widget as a row: on/off, density, colour and its own settings |
| Dock | 台 | the app dock: enabled, edge, autohide, magnify, labels, frost, pinned apps |
| Community | 有志 | installed third-party bar widgets and adding from git or Ryostore |

Open it from the bar logo, or from a terminal or a keybind with `ryoku-shell bar
settings [route]`. Picker style lives on the Bar route; desktop widgets are
edited in place from the wallpaper's right-click menu (Widgets). Session and
mid-work toggles live in the Super+Escape quick settings.

## Kairos Settings

Kairos carries its own island settings, opened from the gear in the top-right of
the expanded clock island and only by that gear. It is a small surface styled
like the island -- near-black, hairline rim, the same accent -- with three routes
over the same `shell.json` namespace:

| Route | Holds |
|---|---|
| Island | clock island size, top offset |
| Clock | 12-hour time, seconds, the date wheel |
| Music | the now-playing bubble and its hover peek |

Every control writes the `kairos` key in `shell.json` through the shell daemon
(the sole writer) and applies live through `Config.kairos`; Ryoku Settings is
untouched. A style with no settings omits this, and only the active style's
`Scene` instantiates the surface, so it exists only while Kairos is the bar.

### Kairos quick settings

Left of the gear, the **tune** icon grows the expanded clock island into the
style's own quick settings. This is not a second window: the island's own pill
morphs to the panel size on the same surface, with the clock and date wheel
fading out and the panel fading in, so open and close read as one body (the same
`Motion.morph` the clock uses). It carries the radio tiles, a weather card in
place of a media card, Display and Sound fader rows, and the notification list;
each tile opens a page in place -- **Wi-Fi** (networks, password, disconnect),
**Bluetooth** (connected/saved/nearby, pair and disconnect), **Sound** (output
and input volume, mute, device pick) and **Display** (per-output brightness,
scale, resolution and Night Light). It is written and drawn inside the island
(`barstyles/kairos/quicksettings/`) and dismissed by clicking anywhere outside
it, by moving the pointer away, by Escape, or by tapping the tune icon again; it
never replaces Ryoku Settings.

## Python Settings

Python keeps its settings in the style itself: the **guide**, opened by the bar's
gear pill (or `qs -c shell ipc call main handleCommand toggle guide`). It is
serpantinum's own settings surface, ported 1:1, with tabs for Welcome, General,
Display, Theme, Bar, Dock, Launcher, On-Screen Display, Notifications, Wellbeing
and About. Every
control writes the `python` key in `shell.json` through the shell daemon (the
sole writer) and applies live; Ryoku Settings is untouched except that Bar
Studio carries an **OPEN PYTHON SETTINGS** button that routes to the guide.

What stays Ryoku's while Python is the bar: the app launcher (Super+Space), the
clipboard history, the wallpaper picker (Super+W, ryogami), the lock screen,
screen capture, updates and the desktop widgets. The bar's buttons for those
hand over to Ryoku's own surfaces. Two visual takeovers are the style's: while
`barStyle` is `python`, the shell's own notification banners and OSD pills stand
down (`Config.styleOwnsBanners` / `styleOwnsFeedback`) and Python draws its
ported versions; a user can flip `python.general.notifications` (or
`python.general.osd`) off to get Ryoku's back.

Python also brings serpantinum's desktop-widget faces to Ryoku's global desktop
widgets: the style folder carries the ported face library under
`barstyles/python/widgets/faces/`, and `shell/modules/desktop/` adapts each type
(clock, music, weather, visualizer, image, user, cpu, ram, temp, disk, battery,
github, each with its variant ladder) into the same `WidgetSlot` that hosts the
built-ins and the Shima faces. Placement, size, lock, backing and the colour
modes are Ryoku's (widgets.json, the desktop edit bar and the widget menu); the
face only paints. Pick **Serpantinum** or **Ryoku** as the widget's style in its
right-click menu: Serpantinum draws the upstream plate, Ryoku lets the slot draw
the shared paper-and-ink backing, so the widget can read as part of either
system while staying on the same host. The Customize panel carries each face's
own options (clock seconds, music layout, the image path, the GitHub account)
under each widget's own tab; they persist in the widget's `<prefix>Opts` slot of
widgets.json and reach the live face through `PythonFaceProvider`.

The style's palette follows Ryoku's live wallpaper palette, with one exception:
picking a preset in the guide's Theme tab stores that palette under
`python.theme.colors` and Python paints itself with it (the ported Catppuccin
slot names map onto the Material roles in `ThemeBackend`). The font picker
writes Ryoku's single global `fontFamily`, so the whole desktop retunes, and
the blue-light rows drive the daemon's night light.

## Frame menus

The wallpaper picker (Super+W) and the rail popout cards are frame surfaces,
not bar widgets, so they are the same in every style. They normally anchor to
the Sumi rail edges and read against the
frame band. The two sidebars (Super+Escape left, Super+S right) are not frame
menus: they are independent screen-edge overlays (`docs/sidebars.md`), which is
why they behave identically under every style. A folder style has no rails and hides the band, so `shell.qml` sets
`topBar` on the per-monitor `FrameMenuManager`: side and bottom anchors fold up to
the matching top edge or corner, the menus drop a small inset below the bar, and
each menu paints its own card since the frame is not there to draw it. Nothing
per-style is needed; a top-bar folder style like QS Bar gets this for free.

## Checklist to ship a style

1. **Folder.** Create `barstyles/<id>/` with a `Scene.qml` (a per-monitor
   `PanelWindow` that takes `property var modelData` for its screen), plus any
   `components/`, `modules/` and `panels/` subdirs it needs. Import the subdirs
   namespaced, and reach the shell's singletons and idioms through the SDK modules
   (`import shell.services`, `import shell.barkit as Pill`), never a relative path.
2. **Register it.** For a built-in, add one row to `BarProducts.builtins`
   (`"<id>": "barstyles/<id>/Scene.qml"`). A store-installed style needs no shell
   edit; it lands in `~/.local/state/ryoku/store/barstyles.json` and resolves by
   the same `sceneUrl` path.
3. **Select it.** Set `"barStyle": "<id>"` in `~/.config/ryoku/shell.json`.
4. **Restart.** Run `systemctl --user restart ryoku-shell`. Structural edits (new
   files, new imports, a reshaped scene) need the restart; property tweaks inside
   an already-loaded scene reload live.

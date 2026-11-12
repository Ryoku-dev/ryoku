# Lithic for QML

This directory contains the reusable Qt 6.12 and Quickshell implementation of Lithic. It has no Quickshell dependency, so the same module loads under plain `qml6`.

## Import

Put the directory that contains `Lithic/` on the QML import path:

```qml
import QtQuick
import Lithic

CutRect {
    width: 160
    height: 48
    fill: Lithic.stone2
    cut: LithicTokens.cut.mid
}
```

From `docs/lithic/examples/qml`, run the gallery with:

```sh
QT_FORCE_STDERR_LOGGING=1 QML_XHR_ALLOW_FILE_READ=1 qml6 -I ../../library/qml Gallery.qml
```

A Quickshell host can add the library parent through the process environment:

```sh
QML_IMPORT_PATH=/absolute/path/to/docs/lithic/library/qml qs -p /path/to/shell.qml
```

Ryoku ships the module compiled: `build.sh <qml-import-dir>` builds it to bytecode with Geologica and Martian Mono inside the plugin and installs `Lithic/` there, and `ryoku-desktop` runs it into `/usr/lib/qt6/qml`. A dev box runs `install.sh [<qml-import-root>]` instead, which copies the sources and the faces into `~/.local/lib/qt6/qml` by default, so an edit shows on the next load. Do not copy token values into the host.

## Palette lifecycle

`Lithic` reads palette files through synchronous `file://` XHR only during startup and an explicit `reload()`. Plain QML therefore needs `QML_XHR_ALLOW_FILE_READ=1`. Set `QT_FORCE_STDERR_LOGGING=1` on every plain QML run so Qt writes warnings to stderr instead of sending them only to the journal. The library never polls and has no palette timer.

Resolution order is:

1. `themePalette` from `$XDG_CONFIG_HOME/ryoku/shell.json`.
2. `$XDG_CACHE_HOME/ryoku/colors.json` when wallpaper following is enabled, or in a forced mode.
3. Signature roles with the `/etc/os-release` edition colour as the action accent.

`$XDG_CONFIG_HOME/ryoku/theme.json` provides `followWallpaper`, defaulting to true. `LITHIC_MODE` accepts `follow`, `dark`, or `light`. `LITHIC_PALETTE_JSON` names a palette file override for deterministic gallery and test runs. Forced dark and light modes keep signature neutrals and read only accents from the supplied palette.

A Quickshell host should watch the files it already owns and feed the resolved palette to the library. `FileView.watchChanges` follows atomic file replacement without polling:

```qml
import Quickshell.Io
import Lithic

FileView {
    id: paletteFile
    path: palettePath
    watchChanges: true
    onFileChanged: reload()
    onLoaded: {
        var parsed = {};
        try {
            parsed = JSON.parse(text() || "{}");
        } catch (error) {
            parsed = {};
        }
        Lithic.setPalette(parsed, "follow");
    }
}
```

The host remains responsible for applying named-scheme precedence when it watches all three files. Call `setPalette` again after each watched reload. Palette changes snap to the new roles. State changes may animate separately.

## Lithic singleton

### State

| Name | Type | Meaning |
|---|---|---|
| `mode` | string | `follow`, `dark`, or `light`. Invalid values reset to `follow`. |
| `contrast` | string | `standard` (default) or `high`. Invalid values reset to `standard`. See [palette.md](../../palette.md#contrast-and-tint). |
| `tint` | string | `off` (default), `low`, or `medium`. Invalid values reset to `off`. |
| `dark` | bool, read only | Resolved palette brightness. |
| `source` | string, read only | `signature`, `wallpaper`, `scheme`, or `host`. |
| `reduceMotion` | bool | Makes `duration(ms)` return zero. |
| `paletteReady` | bool, read only | True after initial palette resolution. |

### Roles

Every role is a read-only `color`: `ground`, `stone1`, `stone2`, `stone3`, `stone4`, `slab`, `field`, `vein`, `edge`, `fg`, `fg2`, `fg3`, `sel`, `onSel`, `onSel2`, `gold`, `onGold`, `goldInk`, `jade`, `lapis`, `amber`, `jasper`, and `scrim`.

Use `Lithic.inkOn(Lithic.jasper)` for danger-fill content. Do not assume a fixed light or dark danger ink.

### Fonts and type

The singleton loads Lithic's two faces, Geologica and Martian Mono, from `library/go/fonts` with `FontLoader`, so they need no install. `Lithic.font` exposes the resolved families:

- `font.sans`
- `font.mono`
- `font.icons`

Each token candidate list is checked against `Qt.fontFamilies()` in order, and the first installed family wins. The icons font, Material Symbols Sharp, comes from the system.

`Lithic.type` exposes `name`, `sentence`, `section`, `title`, `row`, `body`, `proof`, `label`, `caption`, `detail`, `group`, `mono`, and `monoSm`. Every style is an object with `family`, `pixelSize`, `weight`, and `lineHeight`.

### Palette and colour functions

| Function | Result |
|---|---|
| `setPalette(object, mode)` | Injects a host palette, optionally unwraps `themePalette`, sets a valid mode, and resolves roles. |
| `reload()` | Clears host injection and rereads the environment and palette files. |
| `stone(tone)` | State mineral for `idle`, `alive`, `becoming`, `attention`, `paused`, or `failed`. |
| `mineralField(tone)` | Mineral-field colour for a canonical tone. |
| `mix(a, b, t)` | Per-channel linear colour mix, including alpha. `t` is clamped to 0 through 1. |
| `alpha(c, a)` | Colour `c` with its alpha multiplied by clamped `a`. |
| `inkOn(c)` | Black or white, whichever has greater contrast on `c`. |
| `contrastRatio(a, b)` | WCAG contrast ratio. |
| `environment(name)` | Process environment value, or an empty string. Intended for gallery hosts. |

### Recipes

| API | Use |
|---|---|
| `hoverFill(fill, t)` | Filled-control hover mix. Omit `t` for the complete state. |
| `hoverInk(ink, t)` | Transparent-control hover ink. |
| `hoverPlane(plane, t)` | Plane hover transition toward `stone3`; `stone4` instead mixes in `fg` at the hover-plane alpha. |
| `pressRow(fill, t)` | Row and menu press tint. |
| `pressCutGrow` | Extra cut depth while a control is pressed. |
| `pressSink` | Content offset while a control is pressed. |
| `disabledOpacity` | Whole-control opacity while disabled. Never run hover while disabled. |
| `focusColor` | Keyboard focus ring colour. |
| `focusWidth` | Keyboard focus ring width. |
| `focusGap` | Space outside the control before the focus ring. |
| `focusCut(controlCut)` | Focus-ring cut for a control cut. |

A filled control should bind its normal and hovered fill through `hoverFill`, add `pressCutGrow` to every cut while pressed, and offset content by `pressSink`. A transparent control uses `hoverInk`. Rows use `hoverPlane` on hover and `pressRow` while pressed. `hoverPlane` keeps the plane-up recipe below `stone4`; on `stone4` it mixes in `fg` because no plane sits above it. Apply `disabledOpacity` to the whole control.

Interactive phase 2 controls build on `QtQuick.Templates`. They use the template's `visualFocus` to distinguish keyboard focus from pointer focus:

```qml
FocusRing {
    keyboardFocusVisible: control.visualFocus
    cut: controlCut
}
```

Do not show a focus ring for pointer focus.

### Motion

`Lithic.duration(ms)` returns `ms`, or zero when `reduceMotion` is true. `Lithic.curve` is the six-value `Easing.BezierSpline` list for the chisel curve.

`ChiselNumberBehavior` and `ChiselColorBehavior` are reusable Behaviors. Their `duration` property takes an unmodified duration from `LithicTokens.motion`; each Behavior applies `Lithic.duration` and the shared curve:

```qml
ChiselNumberBehavior on x {
    duration: LithicTokens.motion.select
}

ChiselColorBehavior on color {
    duration: hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
}
```

Behaviors request frames only while a value changes. Do not add idle timers or infinite animations to foundation components.

`Lithic.sp`, `Lithic.cut`, `Lithic.motion`, `Lithic.easing`, and `Lithic.easingCurve` are temporary names retained only for the moved phase 2 component files. New code uses `LithicTokens`, `Lithic.duration`, and `Lithic.curve`.

The temporary objects expose:

- `sp`: `LithicTokens.space`
- `cut`: `token`, `small`, `mid`, `large`, `notch`, `toggle`, `toggleTrack`, and `toggleKnob`
- `motion`: reduced-motion-aware `press`, `hover`, `hoverExit`, `select`, `field`, `notch`, and `overlay`
- `easing`: `Easing.BezierSpline`
- `easingCurve`: the same list as `curve`

## Translation

Every word the library shows or announces on its own (Cancel, Changed, Reset %1, Not set, the rail's Home and New, key faces, accessible names) goes through `Lithic.tr("...")`, with the English literal written inside the call so a catalogue extractor can find it. Words with a value use a `%1` template that is translated first: `Lithic.tr("Reset %1").arg(title)`.

A translated host sets the translator once:

```qml
Component.onCompleted: Lithic.translator = s => I18n.tr(s)
```

`tr` reads `translator`, and a translator may read reactive state such as a catalogue revision, so bindings that show these words update when the language or the translator changes. Defaults that are properties, such as `ChordField.placeholder` or `Confirm.actionText`, stay overridable. An untranslated host needs nothing: with no translator the words stay English.

## Primitives

### CutRect

A filled polygon with independent 45 degree corner cuts and an optional wholly inside stroke.

- `fill: color`
- `strokeColor: color`
- `strokeWidth: real`
- `cut: real`, the default for all corners
- `cutTopLeft`, `cutTopRight`, `cutBottomRight`, `cutBottomLeft: real`
- default child content fills the primitive

`CutRect` reparents its default children into an internal content item. Inside those children, `parent` is the content item, not the `CutRect`, so `parent.cut` and `parent.fill` are undefined. Give the `CutRect` an id and bind through it, such as `controlShape.cut` and `controlShape.fill`.

The inside path uses `cut - strokeWidth * (2 - sqrt(2))`, clamped at zero. `cutNW`, `cutNE`, `cutSE`, and `cutSW` are temporary aliases used only by the moved phase 2 files.

### Notch

A state triangle at one corner.

- `size: real`, default `LithicTokens.cut.notch`
- `fill: color`
- `corner: string`, one of `topLeft`, `topRight`, `bottomRight`, `bottomLeft`
- `toneColor` is a temporary alias for `fill`

### Blade

A flat track with a 45 degree leading edge.

- `fraction: real`, clamped to the bar width
- `explicitWidth: real`, used instead of `fraction` when nonnegative
- `fill: color`
- `track: color`
- `bladeWidth: real`, read only
- implicit height is `LithicTokens.size.bladeTrack`

`progress`, `bladeColor`, and `trackColor` are temporary aliases used by the moved phase 2 files.

### Flake

A state parallelogram. Idle is outlined. Becoming can be partly filled.

- `tone: string`, a canonical tone
- `fillFraction: real`, clamped while drawing
- `size: real`, nominal size
- `outline: color`
- `outlineWidth: real`
- `fill: color`
- implicit dimensions use `LithicTokens.size.flakeWidthRatio` and `flakeHeightRatio`

`progress` is a temporary alias for `fillFraction`.

### Hatch

Clipped 45 degree engraving lines.

- `spacing: real`
- `lineWidth: real`
- `color: color`

Geometry changes only when its dimensions or inputs change.

### FocusRing

A cut outline that draws only when keyboard focus is visible.

- `keyboardFocusVisible: bool`
- `cut: real`, the control's cut before focus growth
- `gap: real`
- `ringWidth: real`
- `outset: real`, read only sum of gap and ring width
- `color: color`

Size the ring item to the control plus `outset` on every side, then bind `keyboardFocusVisible` to a template control's `visualFocus`. The stroke sits `gap` outside the control and the primitive grows the cut by the focus recipe token.

When the ring is declared inside a `CutRect`, bind all shape geometry through the `CutRect` id:

```qml
CutRect {
    id: controlShape
    width: 160
    height: 44
    cut: LithicTokens.cut.mid
    fill: Lithic.stone2

    FocusRing {
        x: -outset
        y: -outset
        width: controlShape.width + outset * 2
        height: controlShape.height + outset * 2
        cut: controlShape.cut
        keyboardFocusVisible: control.visualFocus
    }

    Label {
        anchors.centerIn: parent
        text: "Inspect"
        ink: Lithic.inkOn(controlShape.fill)
    }
}
```

Do not use `parent.cut` or `parent.fill` in this arrangement. The immediate parent is `CutRect`'s content item.

### Glyph

A Material Symbols ligature in `Lithic.font.icons`.

- `name: string`, an alias for `text`
- `ink: color`
- `size: int`

The canonical design-system Icon maps to `Glyph` in QML so it cannot collide with template icon properties.

### Label

Role-aware text.

- `typeRole: string`, a key in `Lithic.type`
- `inkRole: string`, a Lithic role name; defaults to `fg2` for the `detail` type and `fg` otherwise
- `ink: color`, defaults from `inkRole`
- `numeric: bool`, tabular figures for counts, proofs and values that change in place
- `token: object`, read only resolved type style


## QML authoring traps

- `Keys` has no `onHomePressed` or `onEndPressed` handler. Handle `Qt.Key_Home` and `Qt.Key_End` inside `Keys.onPressed`.
- Never declare a property whose name matches `on[A-Z]`. QML reserves that spelling for signal handlers.
- A root derived from `Text` cannot take an `implicitHeight` assignment. Size a wrapping `Item` instead.
- Never derive implicit size from `childrenRect` when those children anchor to the parent. That creates a size binding loop. Measure explicit child implicit sizes instead.
- Use `glyph`, not `icon`, on an `AbstractButton` subclass. The template `icon` property is final.
- Qt Quick overlays draw in the window `Overlay` layer. A gallery cannot rely on normal child stacking to capture them, so use the component's `previewOpen` or `previewEmbedded` flag.

## Component reference

This reference covers the 53 component types after the `# components` marker in `Lithic/qmldir`. Each API list names the properties, signals, and functions added by Lithic, plus the inherited template members used in normal calls. Standard Qt Quick properties remain available. A `previewState` accepts `""`, `hover`, `pressed`, or `focus`, plus the extra states a component names below. A component only responds to states it can draw.

Fields (`TextField`, `Search`, the `Picker` anchor and the `Stepper` value box) share one surface: `field` fill, a 1 dp `edge` hairline at rest, an `fg2` hairline with focus and `jasper` on error. Selected rows, chips and segments are a `sel` fill and nothing more.

### Controls

#### Plaque

A cut action button. `kind` is one of `gold`, `line`, `tonal`, `quiet`, `on`, `danger`, or `dangerLine`. `tonal` is a `field` fill with `fg` ink and no edge; `quiet` is `fg2` ink with no edge and no fill at rest.

- Properties: inherited `text` and `enabled`, plus `kind`, `small`, `glyph`, `busy`, `previewState`, and `accessibleName`. Read-only properties are `controlCut`, `sideInset`, `glyphSize`, `contentGap`, and `ink`.
- Signal: inherited `clicked()`
- Preview: `previewState`

```qml
Plaque {
    kind: "gold"
    glyph: "save"
    text: "Save"
    busy: saving
    onClicked: save()
}
```

#### SplitPlaque

A two-segment action with a main button and a separate menu button. It takes the same `kind` values as `Plaque`.

- Properties: `kind`, `small`, `glyph`, `text`, `busy`, `menuOpen`, `previewState`, `morePreviewState`, `accessibleName`, and `moreAccessibleName`. Read-only properties are `controlHeight`, `controlCut`, and `ink`.
- Signals: `mainClicked()` and `moreClicked()`
- Preview: `previewState` for the main segment and `morePreviewState` for the menu segment

#### IconPlaque

A square, glyph-only `Plaque` with an optional state notch.

- Properties: `notchTone`, plus inherited `Plaque` properties including `kind`, `small`, `glyph`, `busy`, `previewState`, and `accessibleName`
- Signal: inherited `clicked()`
- Preview: inherited `previewState`

#### TextField

A labelled text field with help or error copy and an optional secret-value reveal action.

- Properties: inherited `text`, `placeholderText`, `readOnly`, and `enabled`, plus `label`, `helpText`, `errorText`, `secret`, `revealed`, `mono`, and `previewState`
- Signals: inherited `accepted()` and `textEdited()`
- Preview: `previewState`

```qml
TextField {
    width: 360
    label: "Server name"
    placeholderText: "prowl"
    helpText: "Shown in the fleet list"
    text: serverName
    onTextEdited: serverName = text
}
```

#### Search

A search field with a leading glyph, shortcut hint, Escape-to-clear behavior, and a clear action.

- Properties: inherited `text`, `placeholderText`, `readOnly`, and `enabled`, plus `glyph`, `shortcutHint`, `previewState`, and `accessibleName`
- Signals: inherited `accepted()` and `textEdited()`
- Preview: `previewState`

#### Toggle

A cut-track switch with an optional text label.

- Properties: inherited `text`, `checked`, and `enabled`, plus `previewState`
- Signals: inherited `clicked()` and `toggled()`
- Preview: `previewState`

#### Slider

A blade-track slider with a cut handle and keyboard stepping.

- Properties: inherited `from`, `to`, `value`, `stepSize`, and `enabled`, plus `previewState` and `accessibleName`
- Signal: inherited `moved()`
- Preview: `previewState`

#### Stepper

A spin box with quiet decrement and increment faces either side of a field box holding the value and an optional unit. With `editable`, the field takes a typed value: Return, Enter or leaving it commits within the bounds and on the step, Escape drops the edit, and the arrows still step. Parsing goes through `valueFromText`, so a host that formats with `textFromValue` parses its own format.

- Properties: inherited `from`, `to`, `value`, `stepSize`, `editable`, and `enabled`, plus `unit`, `previewState`, and `accessibleName`
- Signal: inherited `valueModified()`
- Preview: `previewState`

#### Filters

A bounded row of filter chips with counts. Chips rest on tonal `field` plates; the `sel` fill glides to the selected chip.

- Properties: `model`, `currentIndex`, `previewState`, and `accessibleName`
- Signal: `activated(int index)`
- Function: `move(delta)`
- Model entries: a string, or an object with `label` and optional `count`
- Preview: `previewState`

#### Segmented

A track of 2 to 5 equal-width options. The selected segment is an inset `sel` fill that glides over `motion.select`.

- Properties: `model`, `currentIndex`, `previewState`, and `accessibleName`
- Signal: `activated(int index)`
- Function: `select(index)`
- Model entries: a string or an object with `label`
- Keys: arrows move the selection, Home and End jump to the ends
- Natural width: the option count times the widest label plus `space.s4` on each side, plus `size.segmentInset` on each side. Set `width` to stretch it.
- Preview: `previewState`. `hover` and `pressed` apply to the first unselected segment.

```qml
Segmented {
    model: ["Follow", "Dark", "Light"]
    currentIndex: modeIndex
    onActivated: index => modeIndex = index
}
```

#### Tabs

A tab row with a gliding blade and wrapping left and right keyboard navigation.

- Properties: `model`, `currentIndex`, `previewState`, and `accessibleName`
- Signal: `activated(int index)`
- Function: `move(delta)`
- Model entries: a string or an object with `label`. At most the token-defined maximum is shown.
- Preview: `previewState`

#### Disclosure

A quiet plaque that unfolds arbitrary child content.

- Properties: `open`, `text`, `glyph`, `previewState`, `accessibleName`, and the default `disclosureContent` child list
- Signal: `toggled(bool open)`
- Preview: `previewState`

#### SettingRow

One setting in a settings `Slab`: a title in `row` over an optional detail in `detail` `fg2`, `size.settingsDetailGap` apart, both wrapping and never elided, with the control at the right, vertically centred and kept `space.s4` from the text. A row is at least `size.factsRowHeight` high. A row that is not `first` carries the `size.settingsRowGap` above itself with a 1 dp `vein` line centred in it, so rows stack in a `Column` with no spacing.

- Properties: inherited `enabled`, plus `title`, `detail`, `changed`, `unavailable`, `highlighted`, `activatable`, `first`, `plane`, `previewState`, and the default `control` child list. Read-only `hovered` is inherited.
- Signals: `resetRequested()` and `activated()`
- `changed`: a "Changed" mark in `detail` `fg3` on the title's baseline, `space.s3` after it, and a small quiet `restart_alt` `IconPlaque` named "Reset <title>" between the text and the control. Its slot stays while `changed`, so nothing shifts; the plaque fades in while the row is hovered or holds focus.
- `unavailable`: a non-empty reason disables the control slot and the reset plaque and replaces the detail in `fg3`. Lithic controls draw themselves at `disabledOpacity`.
- `activatable`: the whole row is a pointer and keyboard target. Enter, Return and Space emit `activated()`; hosts flip the row's toggle or open a hand-off. Hover mixes `fg` into `plane` at `alpha.hoverPlane`, press adds the row-press tint, and the keyboard ring follows the plane. The plane reaches `space.s2` past the row's sides and `space.s1` past its top and bottom, cut `cut.small`.
- `highlighted`: the search spotlight. Setting it shows the plane at the hover colour and fades it to clear once over `motion.highlight` on the chisel curve; the host clears the flag afterwards. Under reduced motion the plane holds until the flag clears.
- The control slot is a `Row` with `space.s2` between children, at most `size.settingsControlMax` wide by contract; give a wide control that width.
- Preview: `previewState`, plus `highlight` for the spotlight at full strength

```qml
Slab {
    Column {
        width: parent.width
        SettingRow {
            width: parent.width
            first: true
            activatable: true
            title: "Dim inactive windows"
            detail: "Windows without focus sit a step darker."
            changed: dimInactive !== defaults.dimInactive
            onActivated: dimInactive = !dimInactive
            onResetRequested: dimInactive = defaults.dimInactive
            Toggle { checked: dimInactive; onToggled: dimInactive = checked }
        }
    }
}
```

#### ChordField

A keyboard shortcut field. At rest the chord shows as keycaps: each a `cut.token` plate with the `field` fill and an `edge` hairline, `label` type with `space.s1` above and below and `space.s2` either side, `space.s1` apart, no "+" glyph. With no chord the field shows the placeholder in `fg3`. The field itself is a `TextField` box.

- Properties: `chord`, `placeholder` (default "Not set"), `conflict`, `accessibleName`, and `previewState`. Read-only `recording` and `keys`, the faces on the caps.
- Signals: `recorded(string chord)` and `cleared()`
- Functions: `startRecording()`, `cancelRecording()`, `canonical(text)`, `displayKeys(text)`, and `keyName(event)`
- Recording: a click, Enter, Return or Space starts it; the field reads "Press a shortcut" in `fg2` with the `fg2` focus hairline. The next press holding Super, Ctrl, Alt or Shift with another key, or F1 to F24 or a media key alone, sets `chord` and emits `recorded`. Escape cancels. Backspace or Delete with no modifier clears `chord` and emits `cleared()`; with a modifier they record (`Super+BackSpace`). Auto-repeat is ignored, bare Tab still moves focus, and losing focus cancels. Hold a compositor shortcut inhibitor while `recording` is true; the field never talks to the compositor.
- Chord text: modifiers in the order Super, Ctrl, Alt, Shift, then the key, joined by "+" with no spaces (`Super+Shift+Return`). Key names follow the Hub's `Combos.qtKeyName` (`Q`, `1`, `F5`, `Return`, `Space`, `Prior`, `minus`, `grave`, `KP_3`, `KP_Enter`) plus `XF86AudioRaiseVolume`, `XF86AudioLowerVolume`, `XF86AudioMute`, `XF86AudioMicMute`, `XF86AudioPlay`, `XF86AudioNext`, `XF86AudioPrev`, `XF86MonBrightnessUp` and `XF86MonBrightnessDown`. The number pad always records `KP_<digit>`, whichever way NumLock sits, and Qt's shifted symbols fold back to their key, so Shift with 1 records `1`, not `!`. `chord` accepts any spelling, such as the store's `SUPER + Q`, and shows it canonically.
- Caps show `wm.DisplayKeys` faces: `Return` reads Enter, `Prior` Page Up, `KP_3` Num 3, `XF86AudioRaiseVolume` Vol +.
- `conflict`: a `jasper` hairline and a wrapping `caption` line in `jasper`, `size.textFieldHelpGap` below the field ("Already used by Close window").
- Preview: `previewState`, plus `recording`

```qml
ChordField {
    accessibleName: "Open terminal"
    chord: binds.terminal
    conflict: binds.clash(chord)
    onRecordingChanged: inhibitor.enabled = recording
    onRecorded: chord => binds.terminal = chord
    onCleared: binds.terminal = ""
}
```

#### PathField

A file or folder value: a mono `TextField` holding the path, with a small quiet `folder_open` `IconPlaque` in the field's trailing slot, 6 in. The library never opens a dialog: the plaque emits `chooseRequested()`, the host opens the portal and sets `path`. Typing and committing (Enter or leaving the field) emits `edited(path)`; the field never writes `path` itself, and the host validates and sets `errorText`.

- Properties: `path`, `label`, `kind` (`folder` or `file`; it only words the default placeholder), `placeholder` (default "Choose a folder" or "Choose a file"), `errorText`, `accessibleName`, and `previewState`
- Signals: `chooseRequested()` and `edited(string path)`
- The choose plaque is named "Choose <label>", or the placeholder when there is no label.
- Preview: `previewState`, plus `chooseFocus` for the ring on the choose plaque

```qml
PathField {
    label: "Screenshots"
    path: settings.screenshotDir
    errorText: folderExists(path) ? "" : "This folder does not exist"
    onChooseRequested: portal.chooseFolder(dir => settings.screenshotDir = dir)
    onEdited: dir => settings.screenshotDir = dir
}
```

#### ColorField

A colour value: a 44 square swatch plate (`cut.token`, the colour over a `Hatch` so alpha shows, `edge` hairline), `space.s2` from a mono hex `TextField`, whose trailing slot holds a small quiet `expand_more` `IconPlaque` that opens a `Popover`. The popover is a `Menu` plane `size.settingsControlMax` wide: the host's swatches as menu rows with a 20 plate, the current one checked like a menu item, then the hex field and, with `allowAlpha`, an "Opacity" slider with its percent.

- Properties: `color`, `label`, `allowAlpha`, `swatches`, `overlayParent`, `accessibleName`, `previewState`, and `previewOpen`. Read-only `opened`, `hex` and `invalidText`.
- Signal: `picked(color c)`
- Functions: `open()`, `close()`, `toggle()`, `choose(value)`, `formatHex(value)`, and `parseHex(text)`
- Swatch entries: objects with `label` and `color`
- Hex: `#rgb` and `#rrggbb`, plus `#rgba` and `#rrggbbaa` in CSS order when `allowAlpha`. It shows lowercase, with alpha only below full opacity. Enter or leaving the field commits a valid value; an invalid one shows the field's error, "Use a colour like #ce5e32", and emits nothing. Choosing a swatch closes the popover; moving the opacity slider picks as it moves.
- Preview: `previewState` (`hover`; `focus` rings the disclosure; `error` shows a half-typed value) and `previewOpen`

```qml
ColorField {
    label: "Accent colour"
    color: settings.accent
    swatches: [
        { label: "Gold", color: Lithic.gold },
        { label: "Jade", color: Lithic.jade }
    ]
    overlayParent: appOverlay
    onPicked: c => settings.accent = c
}
```

### Lists, containers and state

#### Tablet

A selectable fleet-style row with identity copy, state proof, an optional mark, and a contextual action. A selected row is a `sel` fill.

- Properties: inherited `enabled`, plus `title`, `facts`, `claim`, `proof`, `tone`, `progress`, `selected`, `compact`, `pane`, `mark`, `actionText`, `actionKind`, `actionGlyph`, `actionBusy`, and `previewState`. Read-only properties are `stateHovered`, `statePressed`, `stateFocused`, `actionVisible`, `titleInk`, `quietInk`, `claimInk`, and `baseFill`.
- Signals: inherited `clicked()`, plus `actionTriggered()` and `primaryAction()`
- Preview: `previewState`

```qml
Tablet {
    width: 520
    title: "prowl"
    facts: "192.0.2.24"
    claim: "Alive"
    proof: "12 ms"
    tone: "alive"
    selected: true
    actionText: "Connect"
    actionGlyph: "login"
    onActionTriggered: connectToHost()
    onPrimaryAction: openHost()
}
```

#### RowList

A keyboard and wheel-navigable `ListView` with a gliding `sel` selection slab and a cut scroll thumb.

- Properties: `model`, `delegate`, `currentIndex`, `contentY`, and `previewState`. `listView` is read only.
- Functions: `select(index)` and `scrollByNotch(delta)`
- Preview: `previewState`

```qml
RowList {
    width: 520
    height: 360
    model: hosts
    delegate: Tablet {
        required property var modelData
        width: ListView.view.width
        title: modelData.name
        facts: modelData.address
        tone: modelData.tone
        selected: ListView.isCurrentItem
        onClicked: ListView.view.currentIndex = index
    }
}
```

#### GroupHeader

A list group heading with a trailing count that hides when the count is zero.

- Properties: `title` and `count`
- Preview: none

#### Skeleton

A compact or regular tablet placeholder with an active hatched sweep.

- Properties: `active` and `compact`
- Preview: none

#### Empty

A centered empty state with a framed glyph, title, body, and action row.

- Properties: `glyph`, `title`, `body`, and `actions`
- Signal: `actionTriggered(int index)`
- Action entries: objects with `text`, optional `kind`, `glyph`, and `enabled`
- Preview: none

#### Pane

A top-cut list or work plane that hosts arbitrary content.

- Properties: `kind`, `inset`, `fill`, and the default `content` child list. Read-only properties are `bodyItem`, `contentImplicitWidth`, and `contentImplicitHeight`.
- Preview: none

#### MineralField

A state-tinted content field whose tone changes with the shared field recipe.

- Properties: `tone`, `inset`, `fieldColor`, and the default `content` child list. Read-only properties are `bodyItem`, `contentImplicitWidth`, and `contentImplicitHeight`.
- Preview: none

#### Header

A detail header with a mark, name, proof, note, traits, actions, and an optional loaded body.

- Properties: `name`, `proof`, `note`, `traits`, `actions`, `mark`, and `below`
- Signal: `actionTriggered(int index)`
- Action entries: objects with `text`, optional `kind`, `glyph`, `busy`, and `enabled`
- Preview: none

#### Slab

A lifted cut container: `slab` fill and a 1 dp `vein` hairline inside the cut, with configurable inset, fill, and cut depth.

- Properties: `inset`, `fill`, `cut`, and the default `content` child list. Read-only properties are `bodyItem`, `contentImplicitWidth`, and `contentImplicitHeight`.
- Preview: none

#### Section

A section title followed by an arbitrary measured body.

- Properties: `title` and the default `content` child list. Read-only properties are `bodyItem`, `contentImplicitWidth`, and `contentImplicitHeight`.
- Preview: none

#### Facts

A label and value list with optional monospace literal values. Labels keep the 140 column and values start `s4` after it; either side wraps, and a row grows past 44 to fit its taller side.

- Property: `facts`
- Fact entries: objects with `label`, `value`, and optional `literal`
- Preview: none

#### Aside

A `Slab` for preview copy and fact rows beside or below a draft form.

- Properties: `title`, `facts`, and `preview`, plus inherited `Slab` properties
- Preview: none

#### DraftColumns

A responsive pair of loaded form and aside components.

- Properties: `form` and `aside`. Read-only properties are `wide`, `asideWidth`, and `formWidth`.
- Preview: none

#### Claim

A mineral flake and state word for a canonical tone.

- Properties: `tone`, `text`, and `progress`
- Preview: none

#### Marks

A width-clipped row of ordered traits in the `detail` type, set apart by `space.s3`, with key lifecycle traits placed first.

- Properties: `traits` and `maximumWidth`. `orderedTraits` is read only.
- Preview: none

#### Sentence

A wrapping flow of styled text spans.

- Properties: `spans` and `typeRole`. Read-only properties are `naturalWidth` and `naturalHeight`.
- Span entries: objects with `text` and optional `mono`
- Preview: none

#### ScrollRail

The cut scroll thumb for any `Flickable` pane, the thumb `RowList` draws: `size.scrollThumb` wide, at least `size.scrollThumbMin` long, both ends cut `cut.token` at 45 degrees. It shows only while the content overflows, in `edge` at rest and `fg3` under the pointer or while dragged. Dragging the thumb scrolls; a press on the track pages by one view over `motion.wheel`. The wheel stays with the pane, and the keyboard scrolls the pane, not the rail. Its pointer target reaches `space.s2` into the pane.

- Properties: `flickable` and `previewState`. `overflowing` is read only.
- Functions: `scrollTo(contentY)` and `page(direction)`
- Preview: `previewState` (`hover`)

```qml
Flickable {
    id: page
    anchors.fill: parent
    contentHeight: column.implicitHeight
}
ScrollRail {
    anchors.top: page.top
    anchors.bottom: page.bottom
    anchors.right: page.right
    flickable: page
}
```

#### ListEditor

An editable list inside a settings `Slab`: startup apps, session variables, window rules, shell layers. Rows stack like `SettingRow` (`size.settingsRowGap` apart with a `vein` centred in the gap, none above the first, at least `size.factsRowHeight`): an optional `drag_indicator` handle in `fg3`, `space.s3` before the title in `row` and the optional detail in `detail` `fg2`, both wrapping; then, `space.s4` from the text, the trailing control and the small quiet Edit and Remove plaques, `space.s2` apart. The last row is a quiet `add` Plaque with `addText`; with no items, `emptyText` in `body` `fg2` sits `space.s2` above it.

Edit and Add open the host's editor under that row (Add: in place of the add row) on a `field` plane, cut `cut.small`, that reaches `space.s2` past the row's sides with `space.s3` padding, `space.s2` below the row. Cancel (quiet) and Done (line) sit right-aligned `space.s3` below the editor; Done says `doneText` for a new item and is enabled only while the editor is valid. Enter in a single-line field means Done, Escape means Cancel, and opening another editor closes the first without saving. Remove is immediate; the host offers Undo.

The component never changes its model. It reports what the reader did, the host applies it and passes the new model; focus follows the edited, moved or removed row once the model arrives. Replacing the model closes an open editor.

- Properties: `model` (an array), `titleOf(item)` and `detailOf(item)` (default `item.title` and `item.detail`), `addText` (default "Add"), `doneText` (default "Add"), `emptyText`, `editor`, `trailing`, `reorderable`, `plane` (default `Lithic.slab`), `accessibleName`, and `previewState`. Read-only `count`, `editingIndex` (-1, an index, or `count` while adding) and `editorValid`.
- Signals: `added(var item)`, `edited(int index, var item)`, `removed(int index)`, and `moved(int from, int to)`
- Functions: `openEditor(index)` (`count` opens the add editor), `done()`, `cancel()`, `remove(index)`, and `move(from, to)`
- `editor`: a `Component` whose root declares `property var item` (a shallow copy of the row's item, `{}` for a new one), `readonly property bool valid` and `function value()`, returning the edited item. It takes the editor's width, its `implicitHeight` sets the height, and its first field takes focus.
- `trailing`: a `Component` whose root declares plain `property var modelData` and `property int index`, such as a `Toggle` that turns the item on or off.
- `reorderable`: the handle drags the row, which lifts onto the hover plane (`fg` into `plane` at `alpha.hoverPlane`, reaching `space.s2` and `space.s1` past it) and follows the pointer while the others stay; the move is reported on release. Alt+Up and Alt+Down move the focused row.
- Accessibility: a list of list items named by their titles; plaques named "Edit <title>", "Remove <title>" and "Move <title>"; the editor region named "Edit <title>" or the add text.
- Preview: `previewState` `editing` (the first row), `adding`, `empty`, or `dragging`

```qml
ListEditor {
    width: parent.width
    model: startup.apps
    addText: "Add a startup app"
    emptyText: "No apps open at login."
    editor: Component {
        Column {
            property var item: ({})
            readonly property bool valid: command.text.trim().length > 0
            function value() { return Object.assign({}, item, { command: command.text }) }
            TextField { id: command; width: parent.width; label: "Command"; mono: true; text: parent.item.command || "" }
        }
    }
    onAdded: item => startup.add(item)
    onEdited: (index, item) => startup.replace(index, item)
    onRemoved: index => startup.removeAt(index)
}
```

#### DesignTile

One look in a gallery of looks: bar styles, lock screens, launchers, notifications. A `cut.small` tile on `slab` with a 1 dp `vein` hairline. The preview sits `space.s1` inside the top and sides at `size.designPreviewRatio` (16:10), cropped to fill and clipped to concentric top cuts, or a `Hatch` on `field` when there is no image. Below, with `space.s2` above and below and `space.s3` at the sides: the title in `row` with the resource mark in `detail` at the right, then the status in `caption` `fg2` beside an optional small action `Plaque`. The second line always keeps `size.plaqueHeightSmall`, so tiles in a grid line up.

- Properties: inherited `enabled`, plus `title`, `status`, `resource`, `source`, `current`, `selected`, `unavailable`, `busy`, `actionText`, `actionKind` (default `line`; `gold` only for the one consequential action; over bone, while `current` or `selected`, any other kind draws as `on`), `actionGlyph`, `focusVisible`, and `previewState`
- `resource`: `""`, `light`, `standard` or `heavy`, shown as the translated Light, Standard or Heavy; heavy is `amber`, and on bone every mark takes `onSel2`.
- Signals: inherited `clicked()` and `actionTriggered()`
- `current`: the look in use, a `sel` plane under the text with `onSel` ink; the host passes the status, such as "In use". `selected`: the whole tile on `sel` and its hairline gone, so the bone frame around the preview tells it from `current`. `unavailable`: the reason in the status line and the preview at `disabledOpacity`; the tile stays selectable. `busy`: `Working` before the status.
- `focusVisible` lets a container that owns keyboard focus, such as `DesignGallery`, ring its selected tile.
- Preview: `previewState`

#### DesignGallery

A `GridView` of looks whose delegate defaults to `DesignTile`. Columns fit `size.catalogueTileWidthMin` tiles `space.s2` apart, so a ringed tile never meets its neighbour, and tiles stretch up to `size.catalogueTileWidthMax`. Arrow keys move the selection, Home and End jump to the ends, and Enter, Return or Space activates; a click selects and activates. Its implicit height holds every row, so it scrolls with its page; a fixed shorter height scrolls the grid with a `ScrollRail`.

- Properties: `model`, `currentIndex`, `delegate`, `accessibleName`, and `previewState`. Read-only `gridView`, `gap`, `columns`, `tileWidth`, and `tileHeight`.
- Signals: `activated(int index)` and `actionTriggered(int index)`
- Functions: `select(index)` and `activate(index)`
- Model entries: objects with `title`, `status`, `resource`, `source`, `current`, `unavailable`, `busy`, `actionText`, `actionKind`, and `actionGlyph`
- Preview: `previewState` (`focus` rings the selected tile)

```qml
DesignGallery {
    width: parent.width
    accessibleName: "Bar styles"
    model: barStyles
    currentIndex: barStyles.findIndex(style => style.current)
    onActivated: index => previewStyle(barStyles[index])
    onActionTriggered: index => install(barStyles[index])
}
```

### Progress

#### BladeMeter

A determinate `Blade` meter that changes to amber at an alert threshold.

- Properties: `value`, `bladeInk`, and `alertAt`, plus inherited `Blade` properties
- Preview: none

#### BladeProgress

A determinate or sweeping progress blade with an optional gold live marker.

- Properties: `value`, `active`, and `indeterminate`
- Preview: none

#### Working

A three-flake working indicator that animates only while active and visible.

- Properties: `active` and `size`
- Preview: none

#### BecomingName

A name reveal, stage label, progress blade, and log-line stack for a resource being created.

- Properties: `name`, `stage`, `progress`, `lines`, and `active`
- Preview: none

### Overlays, navigation, data and frame

#### Popover

An anchored non-modal popup that flips or clamps to its available parent area.

- Properties: `anchorItem`, `opener`, `previewState`, `previewEmbedded`, `placementSide`, `placementPosition`, `openAbove`, and `slideOffset`. `lithicPopoverMarker` is read only.
- Functions: inherited `open()` and `close()`, plus `calculatePlacement()`, `placeAtAnchor()`, and `closeOtherPopovers()`
- Signals: inherited `opened()` and `closed()`
- Preview: `previewState` and `previewEmbedded`

```qml
Item {
    id: surface

    Plaque {
        id: opener
        text: "Actions"
        onClicked: actions.open()
    }

    Popover {
        id: actions
        parent: surface
        anchorItem: opener
        opener: opener

        Menu {
            items: [{ label: "Inspect", glyph: "search" }]
        }
    }
}
```

#### Menu

A cut menu plane with keyboard focus and icon, shortcut, checked, danger, disabled, and divider rows. A checked row shows a trailing `check` glyph in place of its shortcut.

- Properties: `items`, `menuWidth`, `previewState`, `previewIndex`, and `focusedIndex`
- Signal: `triggered(int index)`
- Functions: `itemEnabled(index)`, `moveFocus(step)`, and `activateFocused()`
- Item entries: objects with `label`, and optional `glyph`, `shortcut`, `checked`, `danger`, `disabled`, or `divider`
- Preview: `previewState` applies to `previewIndex`. The containing overlay controls whether the menu is visible.

#### Picker

A labelled combo-style field that opens its options in a `Popover`, with the current option checked.

- Properties: `label`, `options`, `currentIndex`, `overlayParent`, `previewState`, and `previewOpen`. `opened` and `value` are read only.
- Signal: `activated(int index)`
- Functions: `optionLabel(index)`, `menuItems()`, `open()`, `close()`, and `toggle()`
- Option entries: a string or an object accepted by `Menu`
- Preview: `previewState` and `previewOpen`

#### Dialog

A modal cut dialog with arbitrary body children and a generated action row.

- Properties: inherited `title`, the default `bodyData` child list, `dialogWidth`, `actions`, `opener`, `previewState`, `previewEmbedded`, and `slideOffset`
- Signal: `actionTriggered(int index)`
- Functions: inherited `open()` and `close()`
- Action entries: objects with `text`, optional `glyph`, `kind`, `small`, `busy`, `enabled`, and `closeOnTrigger`
- Preview: `previewState` and `previewEmbedded`

```qml
Dialog {
    id: renameDialog
    parent: appOverlay
    title: "Rename server"
    actions: [
        { text: "Cancel", kind: "quiet" },
        { text: "Save", kind: "gold" }
    ]
    onActionTriggered: index => {
        if (index === 1)
            saveName()
    }

    TextField {
        width: parent.width
        label: "Name"
        text: serverName
    }
}
```

#### Confirm

A confirmation dialog with an optional exact-name gate.

- Properties: `title`, `text`, `actionText` (default "Confirm"), `cancelText` (default "Cancel", the quiet first action, such as Revert), `requiredName`, `destructive`, `busy`, `overlayParent`, `opener`, `previewState`, `previewEmbedded`, `previewOpen`, and `inputText`. `ready` and `opened` are read only.
- Signals: `confirmed()` and `cancelled()`
- Functions: `open()`, `close()`, and `reset()`
- Preview: `previewState`, plus either `previewEmbedded` or `previewOpen`

#### Tooltip

A delayed anchored tooltip that flips above its anchor when needed.

- Properties: inherited `text`, plus `anchorItem`, `previewOpen`, `previewState`, `openAbove`, `placementPosition`, and `slideOffset`
- Functions: `calculatePlacement()` and `placeAtAnchor()`
- Preview: `previewState` and `previewOpen`

#### Placard

A stacked transient notice with an optional undo-style action and draining blade.

- Properties: `message`, `actionText`, `stackIndex`, `autoDismiss`, `previewOpen`, `previewState`, `remaining`, and `slideOffset`
- Signal: `actionTriggered()`
- Functions: inherited `open()` and `close()`
- Preview: `previewState` and `previewOpen`

#### Rail

A responsive vertical rail or compact bottom rail with Home, destinations, and the New menu. The selected destination is a `sel` fill.

- Properties: `sizeClass`, `currentIndex`, `destinations`, `newMenuItems`, `overlayParent`, `previewState`, `previewNewOpen`, `showNew`, `footIndex`, `notchIndex`, and `brandMark`. Read-only properties are `compact`, `expanded`, `railWidth`, `tileSize`, `tileWidth`, `startY`, `newY`, and `compactColumns`.
- Signals: `homeTriggered()`, `destinationTriggered(int index, string label)`, and `newItemTriggered(int index, string label)`
- Functions: `destinationY(index)`, `tileHeight(index)`, `compactColumn(index)`, and `toggleNewMenu()`
- Destination and New-menu entries: objects with `label` and `glyph`
- `showNew` (default true): false hides the New plaque, and its slot, in every size class.
- `footIndex` (default the last destination): destinations before it stack from `startY`, New taking the slot after the first; the destinations from it on keep their order as one group standing on the foot, `size.railItemBottomInset` above the bottom. With three destinations the defaults draw today's rail.
- `notchIndex` (default -1): an amber `Notch` at that tile's top-left corner, the one badge the rail may show.
- `currentIndex` -1: no destination is current (Home is), so no `sel` tile is drawn; it reappears in place on the next destination and glides between destinations as before.
- Expanded labels: every tile takes the widest label's width plus `space.s1` a side, from `tileSize` (64) to the rail less `space.s1` a side (80), centred, and the `sel` tile follows it. A label still too wide wraps to two centred lines, at word boundaries unless one word alone is too long, and its tile grows by one caption line, moving the tiles below it. Ryoport's short labels keep the 64 tiles. Medium and compact are unchanged.
- `brandMark` (default null): a host mark passed to both BrandMarks; see BrandMark's `mark`.
- Compact: Home, the first destination, New when shown, then the rest, in equal columns.
- Preview: `previewState`. `previewNewOpen` embeds the New popover.

#### BrandMark

The interactive Home mark used by the rail.

- Properties: `markSize`, `mark`, and `previewState`. `forcedHover` and `forcedPress` are read only.
- `mark` (default null, which draws Ryoport's octagon and pointer): a `Component` for the host's own mark, sized to the `sel` square inset by its cut. Draw it in `Lithic.onSel` with at most one `Lithic.jasper` accent; the square, size, cut, Home role, hover, press and focus stay the library's.
- Signal: inherited `clicked()`
- Preview: `previewState`

#### NewPlaque

The rail's consequential New action in regular or compact form.

- Properties: `compact` and `menuOpen`, plus inherited `Plaque` properties including `previewState`
- Signal: inherited `clicked()`
- Preview: inherited `previewState`

#### SectionIndex

A settings section index. Expanded, it is a side list whose selected row is a `sel` fill; at any narrower size class it is a `Picker` of the sections.

- Properties: `sections`, `currentIndex`, `sizeClass`, `overlayParent`, and `previewState`. `expanded` is read only.
- Signal: `activated(int index)`
- Functions: `sectionLabel(index)` and `choose(index)`
- Section entries: a string or an object with `label`
- Preview: `previewState`

#### EngravedChart

A line chart with a hatched area, baseline, optional ticks, and an empty state.

- Properties: `values`, `ticks`, `minimum`, `maximum`, `emptyText`, `lineInk`, `baselineInk`, and `previewState`. `hasSeries` is read only.
- Functions: `rangeMinimum()`, `rangeMaximum()`, and `requestChartPaint()`
- Preview: `previewState`

#### AppFrame

The responsive application frame that composes a rail, optional list pane, work pane, window controls, and overlay host.

- Properties: the default `workData` child list, plus `listData`, `overlayData`, `showList`, `compactView`, `customDecorations`, `currentDestination`, `destinations`, `showNew`, `footIndex`, `notchIndex`, `brandMark`, `compactBar`, `windowControlsComponent`, and `previewState`. Read-only properties are `sizeClass`, `compact`, `railWidth`, `listWidth`, `contentX`, and `paneBottom`.
- `showNew`, `footIndex`, `notchIndex` and `brandMark` pass through to `Rail`. `compactBar` (default true): false shows no bottom rail in compact, the panes run to the bottom, and the host provides its own navigation.
- Signals: `homeTriggered()`, `destinationTriggered(int index, string label)`, `newItemTriggered(int index, string label)`, `minimizeRequested()`, `maximizeRequested()`, and `closeRequested()`
- Preview: `previewState`

```qml
AppFrame {
    anchors.fill: parent
    destinations: [
        { label: "Fleet", glyph: "dns" },
        { label: "Settings", glyph: "tune" }
    ]
    currentDestination: pageIndex
    onDestinationTriggered: (index, label) => pageIndex = index

    Column {
        width: parent.width
        Label { text: "Fleet"; typeRole: "name" }
    }

    listData: [
        RowList {
            anchors.fill: parent
            model: hosts
        }
    ]
}
```

## Gallery page contract

Phase 2 adds `Controls.qml`, `Lists.qml`, and `Overlays.qml` beside `Foundation.qml` in `examples/qml/pages`.

Each page must:

1. Have an `Item` root.
2. Provide an accurate `implicitHeight` for its complete content.
3. Fill the 1440 dp gallery width supplied by the Loader.
4. Title each section with the exact component name.
5. Label every demonstrated state in plain sentence case.
6. Use fleet demonstration data from the shared contract where data is needed.
7. Remain static at rest and avoid timers, binding loops, and warnings.

`Gallery.qml` chooses the page through `LITHIC_PAGE=foundation|controls|lists|overlays`, defaults to foundation, grows its window to the page's full height, and captures the complete content when `LITHIC_CAPTURE_PATH` is set.

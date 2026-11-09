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

A packaged shell can instead place the `Lithic` directory below one of its existing module roots. Do not copy token values into the host.

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
| `dark` | bool, read only | Resolved palette brightness. |
| `source` | string, read only | `signature`, `wallpaper`, `scheme`, or `host`. |
| `reduceMotion` | bool | Makes `duration(ms)` return zero. |
| `paletteReady` | bool, read only | True after initial palette resolution. |

### Roles

Every role is a read-only `color`: `ground`, `stone1`, `stone2`, `stone3`, `stone4`, `vein`, `fg`, `fg2`, `fg3`, `sel`, `onSel`, `onSel2`, `gold`, `onGold`, `goldInk`, `jade`, `lapis`, `amber`, `jasper`, and `scrim`.

Use `Lithic.inkOn(Lithic.jasper)` for danger-fill content. Do not assume a fixed light or dark danger ink.

### Fonts and type

`Lithic.font` exposes resolved installed families:

- `font.sans`
- `font.mono`
- `font.icons`

Each token candidate list is checked against `Qt.fontFamilies()` in order. The first installed family wins. This lets `SpaceMono Nerd Font` satisfy the mono role when `Space Mono` is absent.

`Lithic.type` exposes `name`, `sentence`, `section`, `title`, `row`, `body`, `proof`, `label`, `caption`, `group`, `mono`, and `monoSm`. Every style is an object with `family`, `pixelSize`, `weight`, and `lineHeight`.

### Palette and colour functions

| Function | Result |
|---|---|
| `setPalette(object, mode)` | Injects a host palette, optionally unwraps `themePalette`, sets a valid mode, and resolves roles. |
| `reload()` | Clears host injection and rereads the environment and palette files. |
| `stone(tone)` | State mineral for `idle`, `alive`, `becoming`, `attention`, `paused`, or `failed`. |
| `field(tone)` | Mineral-field colour for a canonical tone. |
| `mix(a, b, t)` | Per-channel linear colour mix, including alpha. `t` is clamped to 0 through 1. |
| `alpha(c, a)` | Colour `c` with its alpha multiplied by clamped `a`. |
| `inkOn(c)` | Black or white, whichever has greater contrast on `c`. |
| `contrastRatio(a, b)` | WCAG contrast ratio. |
| `contrastTone(ink, bg, lighter)` | Retones HSL lightness until the token minimum contrast is met. |
| `environment(name)` | Process environment value, or an empty string. Intended for gallery hosts. |

### Recipes

| API | Use |
|---|---|
| `hoverFill(fill, t)` | Filled-control hover mix. Omit `t` for the complete state. |
| `hoverInk(ink, t)` | Transparent-control hover ink. |
| `hoverPlane(plane, t)` | Plane hover transition toward `stone3`. |
| `pressRow(fill, t)` | Row and menu press tint. |
| `pressCutGrow` | Extra cut depth while a control is pressed. |
| `pressSink` | Content offset while a control is pressed. |
| `disabledOpacity` | Whole-control opacity while disabled. Never run hover while disabled. |
| `focusColor` | Keyboard focus ring colour. |
| `focusWidth` | Keyboard focus ring width. |
| `focusGap` | Space outside the control before the focus ring. |
| `focusCut(controlCut)` | Focus-ring cut for a control cut. |

A filled control should bind its normal and hovered fill through `hoverFill`, add `pressCutGrow` to every cut while pressed, and offset content by `pressSink`. A transparent control uses `hoverInk`. Rows use `hoverPlane` on hover and `pressRow` while pressed. Apply `disabledOpacity` to the whole control.

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
- `inkRole: string`, a Lithic role name
- `ink: color`, defaults from `inkRole`
- `token: object`, read only resolved type style


## QML authoring traps

- `Keys` has no `onHomePressed` or `onEndPressed` handler. Handle `Qt.Key_Home` and `Qt.Key_End` inside `Keys.onPressed`.
- Never declare a property whose name matches `on[A-Z]`. QML reserves that spelling for signal handlers.
- A root derived from `Text` cannot take an `implicitHeight` assignment. Size a wrapping `Item` instead.
- Never derive implicit size from `childrenRect` when those children anchor to the parent. That creates a size binding loop. Measure explicit child implicit sizes instead.
- Use `glyph`, not `icon`, on an `AbstractButton` subclass. The template `icon` property is final.
- Qt Quick overlays draw in the window `Overlay` layer. A gallery cannot rely on normal child stacking to capture them, so use the component's `previewOpen` or `previewEmbedded` flag.

## Component reference

This reference covers the 44 component types after the `# components` marker in `Lithic/qmldir`. Each API list names the properties, signals, and functions added by Lithic, plus the inherited template members used in normal calls. Standard Qt Quick properties remain available. A `previewState` accepts `""`, `hover`, `pressed`, or `focus`. A component only responds to states it can draw.

### Controls

#### Plaque

A cut action button with gold, line, quiet, selected, and danger treatments.

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

A two-segment action with a main button and a separate menu button.

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

A spin box with cut decrement and increment faces and an optional unit.

- Properties: inherited `from`, `to`, `value`, `stepSize`, `editable`, and `enabled`, plus `unit`, `previewState`, and `accessibleName`
- Signal: inherited `valueModified()`
- Preview: `previewState`

#### Filters

A bounded row of filter chips with counts and a sliding selection slab.

- Properties: `model`, `currentIndex`, `previewState`, and `accessibleName`
- Signal: `activated(int index)`
- Function: `move(delta)`
- Model entries: a string, or an object with `label` and optional `count`
- Preview: `previewState`

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

### Lists, containers and state

#### Tablet

A selectable fleet-style row with identity copy, state proof, an optional mark, and a contextual action.

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

A keyboard and wheel-navigable `ListView` with a gliding selection slab and cut scroll thumb.

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

A cut container with configurable inset, fill, and cut depth.

- Properties: `inset`, `fill`, `cut`, and the default `content` child list. Read-only properties are `bodyItem`, `contentImplicitWidth`, and `contentImplicitHeight`.
- Preview: none

#### Section

A section title followed by an arbitrary measured body.

- Properties: `title` and the default `content` child list. Read-only properties are `bodyItem`, `contentImplicitWidth`, and `contentImplicitHeight`.
- Preview: none

#### Facts

A divided label and value list with optional monospace literal values.

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

A width-clipped row of ordered traits, with key lifecycle traits placed first.

- Properties: `traits` and `maximumWidth`. `orderedTraits` is read only.
- Preview: none

#### Sentence

A wrapping flow of styled text spans.

- Properties: `spans` and `typeRole`. Read-only properties are `naturalWidth` and `naturalHeight`.
- Span entries: objects with `text`, optional `mono`, and optional `attention`
- Preview: none

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

A cut menu plane with keyboard focus and icon, shortcut, danger, disabled, and divider rows.

- Properties: `items`, `menuWidth`, `previewState`, `previewIndex`, and `focusedIndex`
- Signal: `triggered(int index)`
- Functions: `itemEnabled(index)`, `moveFocus(step)`, and `activateFocused()`
- Item entries: objects with `label`, and optional `glyph`, `shortcut`, `danger`, `disabled`, or `divider`
- Preview: `previewState` applies to `previewIndex`. The containing overlay controls whether the menu is visible.

#### Picker

A labelled combo-style field that opens its options in a `Popover`.

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

- Properties: `title`, `text`, `actionText`, `requiredName`, `destructive`, `busy`, `overlayParent`, `opener`, `previewState`, `previewEmbedded`, `previewOpen`, and `inputText`. `ready` and `opened` are read only.
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

A responsive vertical rail or compact bottom rail with Home, destinations, and the New menu.

- Properties: `sizeClass`, `currentIndex`, `destinations`, `newMenuItems`, `overlayParent`, `previewState`, and `previewNewOpen`. Read-only properties are `compact`, `expanded`, `railWidth`, `tileSize`, `startY`, and `newY`.
- Signals: `homeTriggered()`, `destinationTriggered(int index, string label)`, and `newItemTriggered(int index, string label)`
- Functions: `destinationY(index)`, `compactColumn(index)`, and `toggleNewMenu()`
- Destination and New-menu entries: objects with `label` and `glyph`
- Preview: `previewState`. `previewNewOpen` embeds the New popover.

#### BrandMark

The interactive Home mark used by the rail.

- Properties: `markSize` and `previewState`. `forcedHover` and `forcedPress` are read only.
- Signal: inherited `clicked()`
- Preview: `previewState`

#### NewPlaque

The rail's consequential New action in regular or compact form.

- Properties: `compact` and `menuOpen`, plus inherited `Plaque` properties including `previewState`
- Signal: inherited `clicked()`
- Preview: inherited `previewState`

#### SectionIndex

A settings section index that becomes filter-like plaques at medium width and hides at compact width.

- Properties: `sections`, `currentIndex`, `sizeClass`, and `previewState`. `expanded` and `medium` are read only.
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

- Properties: the default `workData` child list, plus `listData`, `overlayData`, `showList`, `compactView`, `customDecorations`, `currentDestination`, `destinations`, `windowControlsComponent`, and `previewState`. Read-only properties are `sizeClass`, `compact`, `railWidth`, `listWidth`, `contentX`, and `paneBottom`.
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

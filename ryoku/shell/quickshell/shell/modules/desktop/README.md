# Desktop clock

The desktop widget host carries the wallpaper clock and any enabled
`desktopWidget` plugins. Weather and calendar are not part of this host.

`qs -c widgets` loads `shell.qml`. The shell daemon keeps that process alive as
the `widgets` component. It creates one bottom-layer surface per monitor, below
normal windows, and accepts pointer input only on bare wallpaper or widget
surfaces. Run `ryoku-shell reload` after changing QML.

## Structure

    shell.qml                monitor layers, clock slot, plugin slots, menus
    WidgetSlot.qml           clock placement, drag, resize, card
    DesktopGuides.qml        drag grid, centre guides and snap-line flash
    DesktopMenu.qml          shared right-click chrome (card, masthead, scroll)
    MenuRow.qml              one labelled action / value row
    MenuSection.qml          a section eyebrow / hairline divider
    MenuChip.qml             a selectable chip (choice, style, snap zone)
    WidgetMenu.qml           desktop and clock right-click actions
    PluginDesktopSlot.qml    one placed third-party widget
    PluginWidgetMenu.qml     third-party widget actions
    ContributedWidgets.js    widget sets installed items provide (Shells index)
    WindowPresence.js        window count behind the deck frost
    Singletons/
      Config.qml             ~/.config/ryoku/widgets.json
      WidgetKeys.js          suffix rules and defaults for item-provided keys
      Registry.qml           enabled plugin catalogue
      Theme.qml              widget tokens
      Scheme.qml             live wallpaper palette
      Now.qml                shared clock tick
    clock/
      Clock.qml              face and date dispatcher
      Clock*.qml             one clock face per file
      Date*.qml              one date treatment per file

The clock reads its settings directly from `Config`. `WidgetSlot` owns placement
and the optional card, while `Clock.qml` selects the face. Enabled third-party
widgets use `PluginDesktopSlot` on the same monitor layer so they do not compete
for wallpaper input.

## Widgets from installed items

A subshell or bar style can ship a widget set (`[[provides.desktopWidgets]]` in
its `item.toml`). `Desktop.qml` reads every set from
`Services.Shells.contributions("desktopWidgets")` and hosts one `WidgetSlot` per
roster record, loading the set's `widget` file only while that record's
`<prefix>Enabled` is on and the item is unpacked (`prepared`). Selecting a record
of a packed item writes widgets.json and then asks for `ryoku-shell shells sync`,
which unpacks it. Nothing here names an item.

Roster records are `{id, prefix, label, icon, gloss, kind, sizes}`. The widget
root declares `faceId`, `kind`, `prefix`, `screen`, `hostX`, `hostY`,
`scaleCfg`, `sizeCfg`, `inkColorA`, `ryokuStyle`, `radiusCfg` and
`optionOverrides` (the parsed `<prefix>Opts`). Option pages live in the set's
`options` folder as `<Prefix>Options.qml` and receive `widget` and `config`.

Item-provided keys are not adapter properties: `Config` keeps them under
`contributed` in widgets.json and serves them through `get`/`set`, with the
defaults in `Singletons/WidgetKeys.js`. Read them with `Config.get(key)`, never
`Config[key]`.

## Visual rules

- Scale the dimensions and font sizes inside a face. Do not use `Item.scale`,
  which blurs rendered text and hides the slot's real size.
- Report `implicitWidth` and `implicitHeight` from every face.
- Let typography, spacing and short purposeful motion carry the design.
- Read wallpaper-following colour from `Scheme`; use fixed brand colour only
  for a deliberate accent.
- Keep one face or date treatment per file.
- The right-click chrome is one shared component, `DesktopMenu`, in the
  quick-settings sidebar idiom: a surface card with a masthead eyebrow, section
  eyebrows, ink-washed rows and chips, and a press dip. `WidgetMenu` and
  `PluginWidgetMenu` compose it, so no menu carries its own card or a second
  copy of the row vocabulary.

## Adding a clock face

1. Add one `Item` under `clock/` with explicit implicit dimensions.
2. Register its key in `clock/Clock.qml`.
3. Add the same key to `WidgetMenu.qml`'s clock design cycle.
4. Add the option and preview treatment to the Hub's Desktop Widgets page.

The Hub preview is a small independent rendering, not an import of the live
face. Keep it visually equivalent without duplicating runtime behavior.

## Configuration

The desktop menu, dragging code and Ryoku Settings all meet at
`~/.config/ryoku/widgets.json`. `Singletons/Config.qml` watches that file, so a
write retunes the running clock without IPC or a shell reload.

Desktop actions use four small helpers:

    Config.set("dateShow", false)
    Config.toggle("clockSeconds")
    Config.setAnchor("clock", "top-left")
    Config.setFree("clock", 480, 320)

Dragging updates the in-memory values while the pointer moves, then persists one
write on release. A compass anchor survives monitor changes; `"free"` stores raw
monitor pixels. The lock setting disables drag and resize without changing the
saved position.

The Hub has a Save/Revert draft over the same file. Any new clock key must be
added to both `Singletons/Config.qml` and the Hub's Widgets schema and adapter in
the same change, otherwise a Hub save can discard a key it does not know.

The desktop menu opens that page with:

    ryoku-hub config set section widgets

and then starts the guarded Hub process. There is no direct menu-to-Hub IPC.

## Plugin widgets

`Registry` filters enabled plugins whose placement host is `desktopWidget`.
The host repeats a stable list of plugin IDs so a placement write updates the
live entry without rebuilding its service or losing its internal state.
Placement writes go through `ryoku-plugins-place`, resolved from `RYOKU_SHELL_DIR`
in a dev run (else the packaged `/usr/bin` name); plugin settings stay in the
plugin store rather than `widgets.json`.

## Deliberate constraints

- The full monitor layer accepts only right-click on bare wallpaper. Left-click
  passes through.
- Widget placement is `auto` (the wallpaper's calmest region, re-followed on
  every wallpaper change), one of nine compass zones, or free pixels.
- The drag guides -- grid, centre lines and the release snap-line flash --
  appear only while a widget moves.
- The basic render loop is intentional: this mostly static layer should idle
  instead of waking a render thread every refresh.

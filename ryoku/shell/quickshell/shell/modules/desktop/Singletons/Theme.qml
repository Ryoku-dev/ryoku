pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons as Ui

// Shared widget tokens: the design-language geometry, motion and type, with
// every colour resolved from the daemon palette through Scheme so the clock,
// its card and the right-click menu follow the active theme (a fixed named
// scheme or the live wallpaper) instead of a hardcoded tint. `brand` stays the
// fixed vermillion identity (the 力 seal and the clock's deliberate "brand"
// accent option); `accent` is the live system accent the interactive chrome
// paints with. motion = the shell's morph curve (OutExpo).
Singleton {
    // brand: fixed vermillion identity. Never themes -- that is the point. Used
    // by the 力 seal and the clock's deliberate "brand" accent option.
    readonly property color brand: "#e2342a"
    readonly property color sun:   "#e2342a"
    readonly property color gold:  "#d9a441"

    // accent: the live system accent (the palette's primary role). Ryoku keeps
    // colour on the frame, not the content, so the menu chrome no longer tints
    // its selected/on states with it -- those invert (see `bone` below). Only a
    // widget's own live colour (resize grips, active surfaces) still paints here.
    readonly property color accent: Scheme.accent

    // inversion emphasis: docs/ui-ux.md marks a selected chip or a primary row
    // with a bone plate carrying dark ink, never an accent wash. The kit's
    // inverse-surface pair keeps that contrast on a light OR dark palette.
    readonly property color bone:      Ui.Tokens.bone
    readonly property color inkOnBone: Ui.Tokens.inkOnBone

    // inks for the menu, which sits on its own card: the palette's own on-surface
    // roles, correct against `surface` below.
    readonly property color ink:     Scheme.onSurface
    readonly property color inkDim:  Scheme.onSurfaceVariant
    readonly property color inkSoft: Qt.rgba((ink.r + inkDim.r) / 2,
                                             (ink.g + inkDim.g) / 2,
                                             (ink.b + inkDim.b) / 2, 1)
    readonly property color shadow:  Qt.rgba(0, 0, 0, 0.55)

    // inks for a widget with no card under it: resolved against its own patch of
    // wallpaper rather than a surface that is not there.
    function inkOn(l)     { return Scheme.inkOn(l); }
    function inkDimOn(l)  { return Scheme.inkDimOn(l); }
    function inkSoftOn(l) { return Scheme.inkSoftOn(l); }
    function accentOn(l)  { return Scheme.accentOn(l); }

    // override-aware inks: a pinned per-widget colour ("" follows the wallpaper).
    // One colour paints the whole widget a single tone; dim/soft derive from it so
    // the widget reads as one colour rather than a clash. Gradient is layered on
    // top of these by the slot, for the widgets that carry no card.
    function inkOn2(l, c)     { return (c && c.length > 0) ? c : Scheme.inkOn(l); }
    function inkDimOn2(l, c)  { return (c && c.length > 0) ? Qt.rgba(Qt.color(c).r, Qt.color(c).g, Qt.color(c).b, 0.7) : Scheme.inkDimOn(l); }
    function inkSoftOn2(l, c) { return (c && c.length > 0) ? Qt.rgba(Qt.color(c).r, Qt.color(c).g, Qt.color(c).b, 0.85) : Scheme.inkSoftOn(l); }
    function accentOn2(l, c)  { return (c && c.length > 0) ? c : Scheme.accentOn(l); }

    // carbon-dossier surface for the desktop menu: the palette surface over a
    // recessed floor, with ink-derived hairline + faint eyebrow tints so the
    // chrome follows the theme.
    readonly property color cardTop: Scheme.surface
    readonly property color cardBot: Scheme.deep
    readonly property color hair:    Qt.rgba(ink.r, ink.g, ink.b, 0.13)
    readonly property color faint:   Qt.rgba(ink.r, ink.g, ink.b, 0.42)
    readonly property color lineStrong: Qt.rgba(ink.r, ink.g, ink.b, 0.42)

    // ── menu surface: the right-click chrome, in the sidebar design idiom ──
    // an opaque lifted plate (the palette surface); rows and chips wash with
    // ink-derived tints so hover and press read on any palette.
    readonly property color surface:   Scheme.surface
    readonly property color line:      Qt.rgba(ink.r, ink.g, ink.b, 0.16)
    readonly property color tile:      Qt.rgba(ink.r, ink.g, ink.b, 0.06)
    readonly property color tileHover: Qt.rgba(ink.r, ink.g, ink.b, 0.10)
    readonly property color tilePress: Qt.rgba(ink.r, ink.g, ink.b, 0.16)

    readonly property string display: "Fraunces"
    // sans for every clock face and desktop widget. Follows the user's widget
    // font (Hub -> Widgets), bundled faces included, falling back to the default.
    readonly property string font:   Config.widgetFont.length > 0 ? Config.widgetFont : "Space Grotesk"
    readonly property string fontJp: "Noto Sans CJK JP"
    readonly property string mono:   "JetBrainsMono Nerd Font"
    // Material Symbols Rounded ligatures, for the icon glyph a menu row or quick
    // tile leads with (the iRiS menu's structure in the paper-and-ink language).
    readonly property string iconFont: "Material Symbols Rounded"
    // brand mark + name, user-overridable via ~/.config/ryoku/brand.json (Shell ->
    // Global). defaults to the 力 seal / "Ryoku". BrandMark renders `mark`, or
    // `markSource` (an image) when set. Ryoku's own apps never read these.
    readonly property string mark: Ui.UiPreferences.brandMark(Config.markText)
    readonly property string markSource: Config.markImage
    readonly property bool markTint: Config.markTint
    readonly property string brandName: Config.brandName.length > 0 ? Config.brandName : "Ryoku"
    readonly property int radius: 0
    // rounded corners for the menu card and its tiles (the sidebar radiusWidget
    // idiom); the sharp `radius: 0` above stays the clock-face default.
    readonly property int radiusWidget: 14
    readonly property int radiusTile:   9

    // ── studio scale, projected from the kit so the right-click menu never
    // hardcodes a spacing, a radius or a font size: it speaks the same
    // 4-8-12-16-24-32-48 rhythm and 6px control radius as QS Bar Settings. ──
    readonly property int s1: Ui.Tokens.s1        // 4
    readonly property int s2: Ui.Tokens.s2        // 8
    readonly property int s3: Ui.Tokens.s3        // 12
    readonly property int s4: Ui.Tokens.s4        // 16
    readonly property int s5: Ui.Tokens.s5        // 24
    readonly property int s6: Ui.Tokens.s6        // 32
    readonly property int s7: Ui.Tokens.s7        // 48
    readonly property int ctlH: Ui.Tokens.ctlH    // 26: one control tall
    // a floating card doubles the documented control radius; its inner tiles and
    // chips take that control radius itself.
    readonly property int menuRadius:     Ui.Tokens.radius * 2   // 12
    readonly property int menuTileRadius: Ui.Tokens.radius       // 6
    readonly property int fBody:  Ui.Tokens.fBody     // 14: a row label
    readonly property int fSmall: Ui.Tokens.fSmall    // 13: a value / a chip
    readonly property int fMicro: Ui.Tokens.fMicro    // 11: a tracked eyebrow
    readonly property real trackMark:  Ui.Tokens.trackMark    // eyebrow tracking

    // motion: short + smooth. OutExpo mirrors the shell's open curve.
    readonly property int quick:  140
    readonly property int medium: 260
    readonly property int slow:   420
    readonly property int ease:   Easing.OutExpo
}

pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons

// ryoshot in the desktop's design (theme.design): the flags, the roles Lithic
// resolves for that design, its shapes, type and motion. ryoshot is its own short
// qs process, so it watches the four look files itself, as the shell does.
Singleton {
    id: look

    readonly property string design: Lithic.design
    readonly property bool lithic: look.design === "lithic"
    readonly property bool material: look.design === "material"
    readonly property bool adwaita: look.design === "adwaita"
    readonly property bool fluent: look.design === "fluent"
    readonly property bool dark: Lithic.dark
    readonly property bool reduceMotion: Lithic.reduceMotion

    // roles
    readonly property var planes: Lithic.planes(look.design)
    readonly property color ground: Lithic.ground
    readonly property color stone1: Lithic.stone1
    readonly property color stone2: Lithic.stone2
    readonly property color stone3: Lithic.stone3
    readonly property color stone4: Lithic.stone4
    readonly property color slab: Lithic.slab
    readonly property color well: Lithic.field
    readonly property color vein: Lithic.vein
    readonly property color edge: Lithic.edge
    readonly property color fg: Lithic.fg
    readonly property color fg2: Lithic.fg2
    readonly property color fg3: Lithic.fg3
    readonly property color sel: Lithic.sel
    readonly property color onSel: Lithic.onSel
    readonly property color container: Lithic.container
    readonly property color containerInk: Lithic.containerInk
    readonly property color gold: Lithic.gold
    readonly property color jade: Lithic.jade
    readonly property color jasper: Lithic.jasper
    // a drag that will record, and the close button's hover
    readonly property color alert: Lithic.jasper
    readonly property color onAlert: Lithic.inkOn(Lithic.jasper)

    // the plane a floating bar, a dock or a menu stands on, and its outline
    readonly property color plane: look.material || look.adwaita ? Lithic.stone4
        : look.fluent ? look.planes.card
        : look.planes.pane
    readonly property color planeStroke: look.material ? "transparent"
        : look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.1 : 0.16)
        : look.fluent ? look.planes.cardStroke
        : Lithic.vein
    // Fluent's acrylic: its plane faintly tinted with the accent
    readonly property color acrylic: Lithic.mix(look.planes.card, Lithic.sel, look.dark ? 0.03 : 0.025)
    readonly property color highlight: Lithic.alpha(Lithic.fg, look.dark ? 0.06 : 0.05)
    readonly property color shadow: Lithic.alpha(Lithic.scrim, look.dark ? 0.8 : 0.35)

    // the frozen frame's dim
    readonly property real scrimAlpha: look.material ? 0.40 : look.adwaita ? 0.50 : look.fluent ? 0.45 : 0.58
    readonly property color scrim: Qt.rgba(0, 0, 0, look.scrimAlpha)

    // control states
    readonly property color hover: Lithic.alpha(Lithic.fg, look.fluent ? (look.dark ? 0.06 : 0.04) : 0.08)
    readonly property color press: Lithic.alpha(Lithic.fg, look.fluent ? 0.04 : 0.12)
    // a chosen glyph's plate and ink: Lithic's sel plate, Material's primary
    // squircle, Adwaita's shade, Fluent's accent
    readonly property color checkedFill: look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.15 : 0.12) : Lithic.sel
    readonly property color checkedInk: look.adwaita ? Lithic.fg : Lithic.onSel
    readonly property color ink: Lithic.fg
    readonly property color inkDim: Lithic.fg2
    readonly property color inkFaint: Lithic.fg3
    readonly property color divider: look.adwaita ? Lithic.alpha(Lithic.fg, 0.15) : Lithic.vein

    // shapes: every size is a cut, rounded in the round designs (Lithic.cornerRadius)
    readonly property real cutSm: 3
    readonly property real cutMd: 6
    readonly property real cutLg: 8
    function corner(cut, width, height) {
        return Lithic.cornerRadius(cut, width, height, look.design);
    }
    // a corner at the output's UI scale u, from a shape measured in scaled px
    function cornerAt(cut, width, height, u) {
        return look.corner(cut, width / u, height / u) * u;
    }
    readonly property real hairline: 1
    readonly property real glyphStroke: look.material ? 2 : look.fluent ? 1.5 : 1.7

    // type
    readonly property string sans: Lithic.font.sans
    readonly property string mono: Lithic.font.mono
    readonly property string icons: Lithic.font.icons
    readonly property string jp: "Noto Sans CJK JP"
    readonly property real tracking: look.lithic ? 1.2 : 0
    // a chord as the design writes it: Lithic's mono keycap marks, the others as typed
    function keyHint(chord) {
        const text = String(chord);
        if (!look.lithic)
            return text;
        return text.replace(/Ctrl\+/g, "\u2303").replace(/Shift\+/g, "\u21e7").replace(/Alt\+/g, "\u2325").replace(/Super\+/g, "\u2756");
    }

    // motion: each design's own curve and its enter and exit times
    function dur(ms) {
        return Lithic.duration(ms);
    }
    readonly property var curve: Lithic.curve
    readonly property var decelerate: [0, 0, 0.2, 1, 1, 1]
    readonly property var accelerate: [0.3, 0, 0.8, 0.15, 1, 1]
    readonly property var springFast: [0.42, 1.67, 0.21, 0.9, 1, 1]
    readonly property var springDefault: [0.38, 1.21, 0.22, 1, 1, 1]
    readonly property var effects: [0.34, 0.8, 0.34, 1, 1, 1]
    readonly property var fluentIn: [0, 0, 0, 1, 1, 1]
    readonly property var fluentOut: [0.7, 0, 1, 0.5, 1, 1]
    readonly property int enterMs: look.material ? 350 : look.fluent ? 250 : 200
    readonly property int exitMs: look.adwaita ? 200 : 150
    readonly property var enterCurve: look.material ? look.springFast : look.fluent ? look.fluentIn
        : look.adwaita ? [0.25, 0.46, 0.45, 0.94, 1, 1] : look.decelerate
    readonly property var exitCurve: look.fluent ? look.fluentOut : look.accelerate
    // the background of a control changing state
    readonly property int stateMs: look.adwaita ? 100 : look.fluent ? 83 : 120

    // the output's UI scale, as the shell's own surfaces use it
    function scaleFor(screenName) {
        return Tokens.uiScaleFor(screenName);
    }

    // ── Theme's members, so a file migrates on its own ──────────────────────
    readonly property color accent: Lithic.sel
    readonly property color accentInk: Lithic.onSel
    readonly property color panel: look.plane
    readonly property color panelSolid: look.plane
    readonly property color hair: look.lithic ? Lithic.vein : Lithic.alpha(Lithic.fg, 0.14)
    readonly property color danger: Lithic.jasper
    readonly property color field: Lithic.alpha(Lithic.fg, 0.05)
    readonly property string ui: Lithic.font.sans
    readonly property int radius: Math.round(look.corner(look.cutLg, 200, 200))
    readonly property int snap: look.dur(90)
    readonly property int move: look.dur(170)

    // An annotation ink must stay recognisable whatever the wallpaper palette is,
    // so the fixed marker inks are literals rather than palette-derived.
    readonly property var swatches: [Lithic.sel, "#ffffff", "#111111", "#e23b3b", "#f2c14e", "#5bbf73", "#4f8fe0", "#b06fe0"]

    // A redaction must not retint with the palette, or a screenshot shared later
    // would read differently from the one the user checked before sending.
    readonly property color redactSolid: "#121216"

    // The spotlight's dim is part of the exported picture, so it stays the same
    // whatever design the capture was taken in.
    readonly property color spotlightDim: Qt.rgba(0, 0, 0, 0.62)
    // the black or white that reads on a given ink, for a numbered step badge
    function inkOn(c) {
        return Lithic.inkOn(c);
    }

    readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")
    readonly property string cacheHome: Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")
    Variants {
        model: [look.configHome + "/ryoku/shell.json", look.configHome + "/ryoku/theme.json",
            look.configHome + "/ryoku/matugen.json", look.cacheHome + "/ryoku/colors.json"]
        FileView {
            required property string modelData
            path: modelData
            watchChanges: true
            printErrors: false
            onFileChanged: Lithic.reload()
        }
    }

    Component.onCompleted: {
        Lithic.translator = s => I18n.tr(s);
        Lithic.reduceMotion = Qt.binding(() => Tokens.reduceMotion);
    }
}

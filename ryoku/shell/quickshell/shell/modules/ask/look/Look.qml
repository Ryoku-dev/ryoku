pragma Singleton

import QtQuick
import Quickshell
import Lithic

// The Hyakki bar in the desktop's design (theme.design), followed live. A design
// decides corners, how the planes are layered, the type's manner and the motion;
// every colour is a Lithic role, so the palette stays the reader's own.
//   lithic    a stone4 slip cut top-left and bottom-right, ruled by hairlines,
//             mono tracked micro labels, short OutCubic moves and a blade wipe
//   material  a tonal container with no outline, a pill field, pill rows on the
//             secondary container, springs that overshoot a little, shared axis
//   adwaita   the page plane with boxed lists on it, calm OutCubic and OutQuad
//   fluent    a faintly tinted layer with two thin outlines, small corners, the
//             decelerate curve and a drill-in on push
Singleton {
    id: look

    readonly property string design: Lithic.design
    readonly property bool lithic: look.design === "lithic"
    readonly property bool material: look.design === "material"
    readonly property bool adwaita: look.design === "adwaita"
    readonly property bool fluent: look.design === "fluent"
    readonly property bool dark: Lithic.dark
    readonly property bool still: Lithic.reduceMotion
    readonly property var planes: Lithic.planes(look.design)

    // planes. Lithic: one stone4 slip. Material: one tonal container. Adwaita: the
    // page plane, its lists boxed as cards on it. Fluent: three layers (the field's
    // band, the list, the foot), each set off by a hairline
    readonly property color frame: look.adwaita || look.fluent ? look.planes.pane : Lithic.stone4
    readonly property color frameStroke: look.lithic ? Lithic.vein
        : look.fluent ? look.planes.paneStroke
        : look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.12 : 0.16)
        : "transparent"
    readonly property color frameInner: look.fluent ? Lithic.alpha(Lithic.fg, look.dark ? 0.06 : 0.05) : "transparent"
    readonly property color rule: look.lithic ? Lithic.vein
        : look.fluent ? Lithic.alpha(Lithic.vein, 0.7)
        : "transparent"
    readonly property color fieldBand: look.fluent ? Lithic.mix(look.planes.card, Lithic.sel, look.dark ? 0.03 : 0.025) : "transparent"
    readonly property color footer: look.fluent ? Lithic.alpha(Lithic.ground, look.dark ? 0.36 : 0.5) : "transparent"
    // a group lifted off the frame: Adwaita's boxed list, an answer or detail card
    readonly property color card: look.adwaita ? look.planes.card
        : look.material ? Lithic.field
        : look.fluent ? look.planes.card
        : "transparent"
    readonly property color cardStroke: look.lithic ? Lithic.vein
        : look.fluent ? look.planes.cardStroke
        : "transparent"
    readonly property bool boxed: look.adwaita
    // rows in a boxed list are ruled apart
    readonly property color divider: look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.08 : 0.1) : "transparent"
    // the field's own plane: Material's search pill, Adwaita's entry, Fluent's text box
    readonly property color well: look.material ? Lithic.mix(Lithic.field, Lithic.fg, 0.04)
        : look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.08 : 0.06)
        : look.fluent ? Lithic.alpha(Lithic.fg, look.dark ? 0.05 : 0.04)
        : "transparent"
    readonly property color wellStroke: look.fluent ? Lithic.alpha(Lithic.fg, look.dark ? 0.09 : 0.12) : "transparent"
    // the Action Panel, over the frame
    readonly property color popover: look.material ? Lithic.field
        : look.adwaita ? look.planes.card
        : look.fluent ? look.fieldBand : look.frame
    readonly property color popoverStroke: look.material ? "transparent"
        : look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.12 : 0.14)
        : look.fluent ? look.planes.cardStroke : look.frameStroke
    readonly property color shadow: Lithic.alpha(Lithic.scrim, look.dark ? 0.82 : 0.34)

    // rows
    readonly property color plate: look.material ? Lithic.container
        : Lithic.alpha(Lithic.fg, look.fluent ? (look.dark ? 0.07 : 0.05) : (look.dark ? 0.1 : 0.08))
    readonly property color plateStroke: look.fluent ? Lithic.alpha(Lithic.fg, look.dark ? 0.08 : 0.1) : "transparent"
    readonly property color plateInk: look.material ? Lithic.containerInk : Lithic.fg
    readonly property color plateMark: look.material ? Lithic.containerInk : Lithic.sel
    readonly property color hover: Lithic.alpha(Lithic.fg, look.dark ? 0.05 : 0.04)
    readonly property color press: Lithic.alpha(Lithic.fg, look.dark ? 0.1 : 0.12)
    readonly property color cap: look.lithic ? "transparent"
        : look.material ? Lithic.alpha(Lithic.fg, 0.09)
        : look.adwaita ? Lithic.alpha(Lithic.fg, look.dark ? 0.1 : 0.08)
        : "transparent"
    readonly property color capStroke: look.lithic ? Lithic.vein
        : look.fluent ? Lithic.alpha(Lithic.fg, look.dark ? 0.16 : 0.18)
        : "transparent"

    // shapes: cuts, which each design turns into its own corner
    readonly property real cutToken: LithicTokens.cut.token
    readonly property real cutRow: LithicTokens.cut.small
    readonly property real cutCard: LithicTokens.cut.mid
    readonly property real cutMenu: LithicTokens.cut.menu
    readonly property real cutFrame: look.lithic ? LithicTokens.cut.large : LithicTokens.cut.notch
    readonly property real cutField: look.material ? LithicTokens.cut.notch : look.adwaita ? LithicTokens.cut.mid : LithicTokens.cut.small
    // Lithic's slip keeps two corners square
    readonly property real cutFrameQuiet: look.lithic ? 0 : look.cutFrame
    function corner(cut, width, height) {
        return Lithic.cornerRadius(cut, width, height, look.design);
    }
    function cornerAt(cut, width, height, u) {
        return look.corner(cut, width / u, height / u) * u;
    }

    // type
    readonly property string sans: Lithic.font.sans
    readonly property string mono: Lithic.font.mono
    readonly property string icons: look.lithic ? Lithic.font.icons
        : look.fluent ? "Material Symbols Outlined" : "Material Symbols Rounded"
    readonly property int iconWeight: look.material ? 500 : look.fluent ? 300 : 400
    readonly property bool fillChosen: look.material
    readonly property bool monoMicro: look.lithic
    readonly property real tracking: look.lithic ? 1.2 : 0
    // numerals: Lithic sets figures in its mono, the others in tabular sans
    readonly property string figures: look.lithic ? Lithic.font.mono : Lithic.font.sans
    readonly property real figureScale: look.lithic ? 0.84 : 1
    // how an answer's prose is set (services/lib/markdown.js render): a half line
    // between blocks, code on a tint of the plane it lies on. px is the body size
    // in reference px, plane the opaque plane under the text
    function prose(s, px, plane) {
        return {
            px: Math.round(px * s),
            gap: Math.round(px * 0.65 * s),
            itemGap: Math.round(3 * s),
            lineHeight: 138,
            mono: look.mono,
            monoPx: Math.round(px * 0.82 * s),
            codeBg: "" + Lithic.mix(plane, Lithic.fg, look.dark ? 0.08 : 0.06),
            codePad: Math.round(9 * s),
            link: "" + Lithic.goldInk,
            dim: "" + Lithic.fg2
        };
    }

    // motion
    function dur(ms) {
        return Lithic.duration(ms);
    }
    readonly property var outCubic: [0.215, 0.61, 0.355, 1, 1, 1]
    readonly property var outQuad: [0.25, 0.46, 0.45, 0.94, 1, 1]
    readonly property var accelerate: [0.3, 0, 0.8, 0.15, 1, 1]
    readonly property var springSoft: [0.38, 1.21, 0.22, 1, 1, 1]
    readonly property var fluentIn: [0, 0, 0, 1, 1, 1]
    readonly property var fluentOut: [0.7, 0, 1, 0.5, 1, 1]

    readonly property var arrive: look.material ? look.springSoft
        : look.fluent ? look.fluentIn
        : look.adwaita ? look.outQuad : look.outCubic
    readonly property var leave: look.fluent ? look.fluentOut : look.accelerate
    readonly property int openMs: look.material ? 420 : look.fluent ? 250 : look.adwaita ? 220 : 170
    readonly property int closeMs: look.material ? 180 : look.adwaita ? 160 : look.fluent ? 150 : 130
    readonly property int morphMs: look.material ? 380 : look.fluent ? 200 : look.adwaita ? 220 : 160
    readonly property int plateMs: look.material ? 320 : look.fluent ? 167 : look.adwaita ? 180 : 140
    readonly property int viewMs: look.material ? 360 : look.fluent ? 250 : look.adwaita ? 220 : 180
    readonly property int stateMs: look.adwaita ? 100 : look.fluent ? 83 : 120
    // how far a view travels on a push: Material's shared axis, Adwaita's short slide
    readonly property real viewTravel: look.material ? 28 : look.adwaita ? 12 : 0
    readonly property real viewScale: look.fluent ? 0.96 : 1
    readonly property bool viewWipe: look.lithic
}

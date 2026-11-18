pragma Singleton

import QtQuick
import Quickshell
import Lithic
import shell.services
import Ryoku.Ui.Singletons

// Orpheus in the desktop's design language (theme.design), switched live. The
// design changes corners, plane layering and motion personality only; colour
// stays on the shell's Theme/Tokens palette chain, so switching the design never
// touches the colours (docs/lithic/styles.md). There is no per-bar design picker:
// the bar wears whatever Settings › Style chose, and follows it without a restart.
//
// The four styles, as the Hub draws them:
//   lithic   cut corners, one paper drawn by hairlines, mono tracked labels,
//            the chisel curve, a restrained engraved shine on landing.
//   material large round corners and pills, tonal fills with no outline, soft
//            elevation, a springy overshoot.
//   adwaita  rounded cards, a soft shade border, an OutBack pop.
//   fluent   small corners, layered outlined planes with an acrylic tint, a
//            smooth decelerate and a reveal light.
Singleton {
    id: look

    readonly property string design: Lithic.design
    readonly property bool lithic: look.design === "lithic"
    readonly property bool material: look.design === "material"
    readonly property bool adwaita: look.design === "adwaita"
    readonly property bool fluent: look.design === "fluent"
    readonly property bool dark: Lithic.dark
    readonly property bool round: Lithic.shape.corner === "round"

    // ── Shapes: every size is a cut, rounded in the round designs ───────────
    // The cut scale mirrors LithicTokens.cut; corner() turns a cut into the
    // design's corner (a chamfer length for Lithic, a radius for the rest).
    readonly property real cutToken: 3
    readonly property real cutSm: 6
    readonly property real cutMd: 7
    readonly property real cutLg: 8
    readonly property real cutXl: 16
    function corner(cut, width, height) {
        return Lithic.cornerRadius(cut, width, height, look.design);
    }
    // A corner at the bar's UI scale, from a shape measured in scaled px.
    function cornerAt(cut, width, height, u) {
        return look.corner(cut, width / u, height / u) * u;
    }
    readonly property real hairline: 1

    // ── Planes: how the bar body, a card and the settings sheet layer ───────
    // Colour comes from the shell palette (Style.base/ink), not Lithic roles, so
    // the design only decides structure: where a hairline is drawn and whether a
    // plane lifts a tone. The stroke alphas below are the design's own recipe.
    readonly property bool bodyHairline: look.lithic || look.fluent
    readonly property bool cardHairline: look.lithic || look.fluent
    readonly property bool sheetHairline: look.lithic || look.fluent
    // Material and Adwaita lift their cards a tone instead of ruling them.
    readonly property bool tonalCards: look.material || look.adwaita
    // Fluent's acrylic: the plane faintly tinted with the accent.
    readonly property bool acrylic: look.fluent
    readonly property real outlineAlpha: look.fluent ? 0.7 : 1.0
    readonly property real shadeAlpha: look.adwaita ? (look.dark ? 0.5 : 0.14) : 0

    // ── Type personality ────────────────────────────────────────────────────
    // Lithic tracks its mono micro labels; the round designs set them plainly.
    readonly property real tracking: look.lithic ? 1.2 : 0
    readonly property bool monoMicro: look.lithic
    readonly property bool kanjiGloss: look.lithic

    // ── Motion personality: each design's own curve and times ───────────────
    function dur(ms) { return Lithic.duration(ms); }
    readonly property var chisel: Lithic.curve
    readonly property var decelerate: [0, 0, 0.2, 1, 1, 1]
    readonly property var accelerate: [0.3, 0, 0.8, 0.15, 1, 1]
    readonly property var springFast: [0.42, 1.67, 0.21, 0.9, 1, 1]
    readonly property var springDefault: [0.38, 1.21, 0.22, 1, 1, 1]
    readonly property var effects: [0.34, 0.8, 0.34, 1, 1, 1]
    readonly property var fluentIn: [0, 0, 0, 1, 1, 1]
    readonly property var fluentOut: [0.7, 0, 1, 0.5, 1, 1]
    readonly property var adwaitaIn: [0.25, 0.46, 0.45, 0.94, 1, 1]

    // The card weld's open and close personality.
    readonly property int enterMs: look.material ? 380 : look.fluent ? 300 : look.adwaita ? 320 : 300
    readonly property int exitMs: look.adwaita ? 220 : look.fluent ? 220 : 200
    readonly property var enterCurve: look.material ? look.springFast
        : look.fluent ? look.fluentIn
        : look.adwaita ? look.adwaitaIn : look.decelerate
    readonly property var exitCurve: look.fluent ? look.fluentOut : look.accelerate
    // A control changing state (hover wash, fill, ink).
    readonly property int stateMs: look.adwaita ? 100 : look.fluent ? 83 : 120
    readonly property int hoverInMs: 160
    readonly property int hoverOutMs: 100
    readonly property int pressMs: 90
    // The press sink: Lithic deepens the cut and sinks 1px; Material squashes the
    // pill; the others scale gently.
    readonly property real pressScale: look.material ? 0.96 : look.lithic ? 0.985 : 0.97
    readonly property real pressSink: look.lithic ? 1 : 0
    readonly property real pressCutGrow: look.lithic ? 4 : 0

    // ── Eye-candy budget ────────────────────────────────────────────────────
    // Micro-animations (hover, press, the card weld) answer the hand and run
    // whenever motion is allowed. Passive/ambient life (the idle wave drift, a
    // landing shine, the reveal light) is the eye-candy that only runs while the
    // desktop has headroom: Balanced or Performance, never Power Saver. Perf
    // already folds lowPowerMode, Game Mode and the user's reduce setting.
    readonly property bool microMotion: !Perf.reduceMotion
    readonly property bool ambientMotion: !Perf.reduceMotion && !Perf.saver
    readonly property bool fullRate: Perf.ambientMotion

    // The bar's liquid form: Lithic keeps its cut even in the blob field (the
    // chamfer reads as a crisp shoulder), the round designs swell softer.
    readonly property real liquidSmoothing: look.material ? 16 : look.adwaita ? 13 : look.fluent ? 9 : 12
    readonly property real squareSmoothing: look.fluent ? 4 : 5
}
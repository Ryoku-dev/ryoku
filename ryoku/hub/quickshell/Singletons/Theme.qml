pragma Singleton
import QtQuick
import Quickshell

// Ryoku Settings palette, the website's "Greek-noir yakuza" language in QML.
// A near-black canvas so the one vermillion accent (力 mark, active section,
// focused search, numerals) reads as deliberate, never a wash. Warm-white type,
// gold used only as kintsugi. Depth from hairlines + hard offset shadows, not
// gradients. Display face is Fraunces (editorial serif); UI is Geologica;
// labels are JetBrains Mono. Mirrors ryoku-site/app/assets/css/tokens.css.
Singleton {
    // brand: one vermillion, sparingly + boldly. (site --sun / --sun-deep)
    readonly property color brand:     "#e2342a"
    readonly property color ember:     "#e83b30"   // a hair brighter for hover/active
    readonly property color emberDeep: "#b81f19"
    readonly property color sun:       "#e2342a"   // explicit alias (red-sun disc, accents)
    readonly property color sunDeep:   "#b81f19"
    readonly property color sunInk:    "#fbeee2"   // text on vermillion
    readonly property color gold:      "#d9a441"   // kintsugi seams, warnings, sparingly

    // log status accents (update console).
    readonly property color ok:        "#7fbf6a"
    readonly property color bad:       "#e05a5a"

    // canvas + surfaces. flat; depth comes from hairlines + hard shadow.
    readonly property color bgTop:    "#16110b"   // site --paper-2
    readonly property color bgBot:    "#0f0c07"   // site --paper, a touch deeper
    readonly property color rail:     "#120e08"
    readonly property color surface:  "#1b150e"
    readonly property color surfaceLo:"#140f09"
    readonly property color keyTop:   "#221a12"
    readonly property color keyBot:   "#17110b"
    readonly property color line:     Qt.rgba(243 / 255, 237 / 255, 225 / 255, 0.14)
    readonly property color lineSoft: Qt.rgba(243 / 255, 237 / 255, 225 / 255, 0.06)
    readonly property color lineStrong: Qt.rgba(236 / 255, 226 / 255, 205 / 255, 0.40)  // card/art borders (site --rule-strong)
    readonly property color shadow:   "#000000"   // hard brutalist offset (site --shadow)

    // profile card specimen = trading-card surface in the hub's warm palette.
    readonly property color cardTop:  "#1b140d"
    readonly property color cardBot:  "#120d08"
    readonly property color frameBg:  Qt.rgba(226 / 255, 52 / 255, 42 / 255, 0.10)
    readonly property color hair:     Qt.rgba(243 / 255, 237 / 255, 225 / 255, 0.12)

    // text (warm neutrals, site --ink ramp).
    readonly property color bright:   "#f3ede1"   // --ink
    readonly property color cream:    "#e6dccb"   // --ink-2
    readonly property color subtle:   "#c7bfae"   // --ink-soft
    readonly property color dim:      "#8f8770"   // --ink-faint
    readonly property color faint:    "#5c5249"
    readonly property color onAccent: "#fbeee2"

    // type stack, mirrors the website.
    readonly property string display: "Fraunces"                 // editorial serif headlines
    readonly property string font:    "Geologica"            // UI / body
    readonly property string fontJp:  "Noto Sans CJK JP"         // kanji marks
    readonly property string mono:    "JetBrainsMono Nerd Font"  // labels / code

    // brutalist geometry, the website's poster language: SHARP corners, hairline
    // borders, hard offset shadows. Panels/cards/inputs use radius 0; only true
    // circles (badges, dots, toggle knobs) stay round. The outer Hyprland window
    // rounding is the user's; inside our surfaces we are sharp.
    readonly property int radius:      0     // was 8-14; now sharp everywhere
    readonly property int radiusChip:  0     // chips/buttons, sharp too
    readonly property real border:     1     // hairline border width
    readonly property int shadowStep:  6     // hard offset shadow distance (px)
    readonly property int shadowStepLg: 8    // larger cards

    // motion. short + smooth; OutExpo ~ the site's cubic-bezier(0.22,1,0.36,1).
    readonly property int quick:  120
    readonly property int medium: 240
    readonly property int slow:   360
    readonly property int ease:   Easing.OutExpo
}

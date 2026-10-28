pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons as Ui

// Overview palette + geometry, a focused twin of the shell Theme (docs/ui-ux.md
// tokens). Accent follows the wallpaper when Match wallpaper is on; the fixed
// fallback is the Ryoku brand vermillion. Brutalist geometry: radius 0, hairline
// borders, hard offset shadows.
Singleton {
    readonly property color brand:    Config.matchWallpaper ? Scheme.accent : "#e2342a"
    readonly property color vermLit:  Config.matchWallpaper ? Qt.lighter(Scheme.accent, 1.22) : "#e83b30"
    readonly property color vermDeep: Config.matchWallpaper ? Qt.darker(Scheme.accent, 1.3) : "#b81f19"
    readonly property color gold:     "#d9a441"

    // warm-white text ramp (website --ink).
    readonly property color bright:   "#f3ede1"
    readonly property color cream:    "#e6dccb"
    readonly property color subtle:   "#c7bfae"
    readonly property color dim:      "#8f8770"
    readonly property color faint:    "#5c5249"

    // near-black canvas (website --paper), or palette surfaces when matching.
    readonly property color cardTop:  Config.matchWallpaper ? Scheme.base     : "#16110b"
    readonly property color cardBot:  Config.matchWallpaper ? Scheme.deep     : "#0f0c07"
    readonly property color tileBg:   Config.matchWallpaper ? Scheme.elevated : "#1b150e"
    readonly property color border:   Config.matchWallpaper ? Scheme.line     : Qt.rgba(243/255, 237/255, 225/255, 0.14)
    readonly property color hair:     Qt.rgba(243/255, 237/255, 225/255, 0.12)
    readonly property color sheen:    Qt.rgba(243/255, 237/255, 225/255, 0.06)
    readonly property color shadow:   "#000000"

    // accent tints for cell fills / drop targets.
    readonly property color frameBg:     Qt.rgba(226/255, 52/255, 42/255, 0.10)
    readonly property color frameBorder: Qt.rgba(243/255, 237/255, 225/255, 0.18)
    readonly property color threadBg:    Qt.rgba(226/255, 52/255, 42/255, 0.13)

    // type stack + brutalist geometry (website language).
    readonly property string display: "Fraunces"
    readonly property string font:    Config.fontFamily.length > 0 ? Config.fontFamily : "Space Grotesk"
    readonly property string fontJp:  "Noto Sans CJK JP"
    readonly property string mono:    "JetBrainsMono Nerd Font"
    // brand mark + name, user-overridable via ~/.config/ryoku/brand.json (Shell ->
    // Global). defaults to the 力 seal / "Ryoku". BrandMark renders `mark`, or
    // `markSource` (an image) when set. Ryoku's own apps never read these.
    readonly property string mark: Ui.UiPreferences.brandMark(Config.markText)
    readonly property string markSource: Config.markImage
    readonly property bool markTint: Config.markTint
    readonly property string brandName: Config.brandName.length > 0 ? Config.brandName : "Ryoku"
    readonly property int    radius:  0
    readonly property int    shadowStep: 6
}

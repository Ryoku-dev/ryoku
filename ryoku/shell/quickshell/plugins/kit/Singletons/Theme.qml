pragma Singleton
import QtQuick
import Quickshell

// The plugin kit's design tokens. Colour resolves from the daemon palette
// through the Scheme singleton so a plugin's card, ink and accent follow the active theme (a
// fixed named scheme or the live wallpaper) instead of a hardcoded tint; the
// RASHIN flame and the rest-card sky stay fixed identity, deliberately
// independent of the accent. brand/verm are the live accent (the palette
// primary); sun/gold are the fixed brand marks.
Singleton {
    // accent family: the live system accent (palette primary), with lit/deep
    // variants tinted off it. brand + verm are the accent under their historical
    // names, so existing plugin content follows the theme with no change.
    readonly property color accent:   Scheme.accent
    readonly property color brand:    Scheme.accent
    readonly property color verm:     brand
    readonly property color vermLit:  Qt.lighter(Scheme.accent, 1.22)
    readonly property color vermDeep: Qt.darker(Scheme.accent, 1.3)
    // fixed brand marks, never themed (the deliberate identity accents).
    readonly property color sun:      "#e2342a"
    readonly property color gold:     "#d9a441"
    // text ramp, resolved from the palette so a plugin's copy reads on the themed
    // card in light and dark alike.
    readonly property color cream:    Scheme.onSurface
    readonly property color bright:   Scheme.onSurface
    readonly property color dim:      Scheme.onSurfaceVariant
    // card surface, from the palette depth ramp. panelTop/panelBot are the Card
    // gradient stops (aliases of cardTop/cardBot).
    readonly property color cardTop:  Scheme.base
    readonly property color cardBot:  Scheme.deep
    readonly property color panelTop: cardTop
    readonly property color panelBot: cardBot
    readonly property color border:   Scheme.line
    readonly property color lineStrong: Qt.rgba(bright.r, bright.g, bright.b, 0.40)
    readonly property color shadow:   "#000000"
    readonly property color tileBg:   Scheme.elevated
    readonly property color subtle:   Scheme.onSurface
    readonly property color faint:    Scheme.onSurfaceVariant
    readonly property color iconDim:  Scheme.onSurfaceVariant
    readonly property color hair:     Qt.rgba(bright.r, bright.g, bright.b, 0.12)
    readonly property color sheen:    Qt.rgba(bright.r, bright.g, bright.b, 0.06)
    // ember tones for the flame gradients: warm browns of the fixed RASHIN
    // identity, held independent of the accent.
    readonly property color vermDim:  "#b05a43"
    readonly property color vermDimDeep: "#65342b"
    readonly property color vermBurn: "#8f321d"
    readonly property color tickRest: Scheme.onSurfaceVariant
    readonly property color threadBg: Qt.rgba(accent.r, accent.g, accent.b, 0.13)
    readonly property color flameCore: "#ffd2bf"
    readonly property color flameGlow: "#ff9e64"

    // flame canvas ramp: literal hex strings (color type breaks here), fed
    // straight into Canvas addColorStop/strokeStyle. a color property serializes
    // to #aarrggbb and corrupts the gradient render. Fixed RASHIN identity.
    readonly property string flameInk:   "#e83b30"
    readonly property string flameEmber: "#7a2a1a"
    readonly property string flameBurn:  "#8f321d"
    readonly property string flameTip:   "#ffd2bf"
    readonly property color todayWarm: "#ff9e64"
    readonly property color ghost:     "#414868"
    // Rest-card sky: fixed day/night scene colours, deliberately independent of
    // the accent so the sun stays golden and the night cool on any wallpaper.
    readonly property color sunGold:  "#ffc777"
    readonly property color moonGlow: "#7aa2f7"
    readonly property color moonDisc: "#c8d3f5"
    readonly property color frameBg:     Qt.rgba(accent.r, accent.g, accent.b, 0.10)
    readonly property color frameBorder: Qt.rgba(bright.r, bright.g, bright.b, 0.18)
    readonly property color creamMenu:   Qt.rgba(bright.r, bright.g, bright.b, 0.82)
    readonly property real shadowOpacity: 0.5
    // type stack + brutalist geometry, the website language.
    readonly property string display: "Fraunces"
    readonly property string font: "Geologica"
    readonly property string fontJp: "Noto Sans CJK JP"
    readonly property string mono: "JetBrainsMono Nerd Font"
    readonly property int radius: 0
    readonly property real border2: 1
    readonly property int shadowStep: 6

    // MPRIS trackArtists comes back as a JS array from some players and a
    // plain string from others (Spotify). calling .join on the string throws
    // and kills the whole binding. handle both, fall back to trackArtist.
    function joinArtists(artists, single) {
        if (artists && typeof artists.join === "function" && artists.length > 0)
            return artists.join(", ");
        if (artists && String(artists).length > 0)
            return String(artists);
        return single ? String(single) : "";
    }
}

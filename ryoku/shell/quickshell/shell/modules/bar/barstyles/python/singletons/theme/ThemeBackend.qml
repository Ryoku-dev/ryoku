pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import "../../"

// Ryoku seam: serpantinum reads its whole skin through this one singleton, so
// the port keeps the property names (base, mantle, crust, surface0..2, text,
// subtext0/1, overlay0..2, and the Catppuccin accent hues) and retargets the
// source: the shell's live Theme, which follows the wallpaper palette. The
// scheme vocabulary (light, dark, ink) folds into Ryoku's scheme.
Item {
    id: root

    // Serpantinum ships an icon font for its glyphs; Ryoku's own mono face is
    // already a Nerd Font, so the bundled name resolves without a FontLoader.
    // The style's Config passes Ryoku's single global font through as
    // globalFontFamily (inside this file's scope `Config` names the style's
    // own singleton, which shadows the service of the same name).
    property string fontFamily: (Config.globalFontFamily && Config.globalFontFamily.length > 0)
        ? Config.globalFontFamily : "Iosevka Nerd Font"
    // Upstream's buttons read the glyph face through `iconFont`, which it loads
    // from a bundled SymbolsNerdFontMono. Ryoku's mono is already a Nerd Font, so
    // the name resolves directly and the glyphs draw without a FontLoader.
    readonly property string iconFont: root.fontFamily
    readonly property bool iconFontReady: true
    property int borderRadius: 8
    property int clampedBorderRadius: {
        const r = borderRadius;
        return Math.floor(
            r <= 24
                ? r
                : 24 + Math.pow(r - 24, 0.55)
        );
    }

    // ---- Python's own overrides (shell.json under "python".theme) --------
    // A preset picked in the style's guide is a full Catppuccin-slot palette;
    // while one is active it wins over the wallpaper. No colours (the dynamic
    // tile) means follow Ryoku's live palette like the rest of the desktop.
    property var overrides: Config.getSetting("theme", {})
    readonly property var palette: (overrides && overrides.colors) ? overrides.colors : ({})

    function slot(name, fallback) {
        const p = root.palette[name];
        return (p !== undefined && p !== null && String(p).length > 0) ? p : fallback;
    }

    // ---- Catppuccin slot names, filled from Ryoku's live palette ---------
    property color base:     slot("base", Theme.surfaceContainerLowest)
    property color mantle:   slot("mantle", Theme.surfaceContainerLow)
    property color crust:    slot("crust", Theme.surface)
    property color text:     slot("text", Theme.onSurface)
    property color subtext0: slot("subtext0", Theme.onSurfaceVariant)
    property color subtext1: slot("subtext1", mix(Theme.onSurfaceVariant, Theme.onSurface, 0.5))
    property color surface0: slot("surface0", Theme.surfaceContainer)
    property color surface1: slot("surface1", Theme.surfaceContainerHigh)
    property color surface2: slot("surface2", Theme.surfaceContainerHighest)
    property color overlay0: slot("overlay0", Theme.outline)
    property color overlay1: slot("overlay1", mix(Theme.outline, Theme.outlineVariant, 0.5))
    property color overlay2: slot("overlay2", mix(Theme.outlineVariant, Theme.onSurfaceVariant, 0.5))

    // The accent hues: serpantinum spreads a palette across named slots; the
    // wallpaper's Material roles carry the same intent, one accent + neighbours.
    property color blue:     slot("blue", Theme.primary)
    property color sapphire: slot("sapphire", mix(Theme.primary, Theme.tertiary, 0.4))
    property color peach:    slot("peach", Theme.tertiary)
    property color green:    slot("green", mix(Theme.secondary, Theme.tertiary, 0.3))
    property color red:      slot("red", Theme.error)
    property color mauve:    slot("mauve", mix(Theme.primary, Theme.error, 0.35))
    property color pink:     slot("pink", mix(Theme.tertiary, Theme.error, 0.45))
    property color yellow:   slot("yellow", mix(Theme.secondary, Theme.tertiary, 0.5))
    property color maroon:   slot("maroon", mix(Theme.error, Theme.outline, 0.35))
    property color teal:     slot("teal", mix(Theme.secondary, Theme.primary, 0.35))

    function mix(a, b, t) {
        return Qt.rgba(a.r + (b.r - a.r) * t,
                        a.g + (b.g - a.g) * t,
                        a.b + (b.b - a.b) * t,
                        a.a + (b.a - a.a) * t)
    }

    Connections {
        target: Config
        function onSettingsLoaded() {
            root.updateColors();
        }
    }

    function reloadColors() {
        root.updateColors();
    }

    function updateColors() {
        root.overrides = Config.getSetting("theme", {});
        if (root.overrides.borderRadius !== undefined) {
            root.borderRadius = root.overrides.borderRadius;
        }
    }

    Component.onCompleted: updateColors()
}

pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui.Singletons as Ui

/**
 * Shell-wide style tokens: 30 Material colour roles plus shadow/scrim, sizing,
 * and fonts. Every colour resolves through a three-layer fallback chain, highest
 * priority first:
 *
 *   named scheme  ->  live wallpaper  ->  compiled default
 *
 * - compiled default: the Solitude Dark base palette. What shows when no dynamic
 *   theme is active, and the value the shell ships with.
 * - live wallpaper: the Scheme singleton, active while Config.matchWallpaper is
 *   on; a wallpaper change retunes every role live.
 * - named scheme: one of the static presets. The full 30-role palettes are owned
 *   by the Go theme daemon (a role() lookup honours a `namedScheme` palette
 *   object the daemon assigns here); null while no preset is selected. This is
 *   the single extension point for the static-theme catalog.
 *
 * The Material "on" roles (onSurface, onPrimary, ...) are declared plain and
 * bound through Binding elements. QML parses an inline `onSurface: <expr>` as a
 * change-handler for the sibling `surface` property and silently drops the
 * binding, leaving the colour at transparent black; naming the property as a
 * string in a Binding sidesteps that grab.
 *
 * Sizing and fonts are compiled defaults; opacity takes its value from the
 * Config frameOpacity knob.
 */
Singleton {
    id: root

    // The live wallpaper palette is active only while Match wallpaper is on.
    readonly property bool matchWallpaper: Config.matchWallpaper

    // Extension point for the static theme catalog (the 57 presets). Their full
    // palettes live in the Go theme daemon, which resolves the active preset into
    // the themePalette key of shell.json; Config surfaces it here. null = no
    // preset (a dynamic variant), so role() falls back to wallpaper or the base.
    property var namedScheme: Config.themePalette

    // Resolve one colour role through the layer chain: a selected named scheme
    // wins, then the live wallpaper palette, then the compiled Solitude base.
    function role(key, base) {
        if (Scheme.previewRoles[key] !== undefined)
            return Scheme.previewRoles[key];
        if (namedScheme && namedScheme[key] !== undefined)
            return namedScheme[key];
        if (matchWallpaper)
            return Scheme[key];
        return base;
    }

    // --- 30 Material colour roles (compiled defaults: Solitude Dark) ----------
    readonly property color surface:                 role("surface", "#101315")
    readonly property color surfaceVariant:          role("surfaceVariant", "#1a1d1f")
    readonly property color surfaceContainerLowest:  role("surfaceContainerLowest", "#101315")
    readonly property color surfaceContainerLow:     role("surfaceContainerLow", "#1a1d1f")
    readonly property color surfaceContainer:        role("surfaceContainer", "#2c2f32")
    readonly property color surfaceContainerHigh:    role("surfaceContainerHigh", "#3d4144")
    readonly property color surfaceContainerHighest: role("surfaceContainerHighest", "#4b4e55")
    readonly property color inverseSurface:          role("inverseSurface", "#cacccc")
    readonly property color inverseOnSurface:        role("inverseOnSurface", "#101315")
    readonly property color surfaceTint:             role("surfaceTint", "#798186")
    readonly property color primary:                 role("primary", "#798186")
    readonly property color primaryContainer:        role("primaryContainer", "#1a1d1f")
    readonly property color secondary:               role("secondary", "#a8adb0")
    readonly property color secondaryContainer:      role("secondaryContainer", "#1a1d1f")
    readonly property color tertiary:                role("tertiary", "#de6145")
    readonly property color tertiaryContainer:       role("tertiaryContainer", "#1a1d1f")
    readonly property color error:                   role("error", "#de6145")
    readonly property color errorContainer:          role("errorContainer", "#1a1d1f")
    readonly property color outline:                 role("outline", "#565d60")
    readonly property color outlineVariant:          role("outlineVariant", "#343d41")

    // The "on" roles (see the class note): declared plain, bound via Binding so
    // the QML signal-handler grammar does not swallow their bindings.
    property color onSurface
    property color onSurfaceVariant
    property color onPrimary
    property color onPrimaryContainer
    property color onSecondary
    property color onSecondaryContainer
    property color onTertiary
    property color onTertiaryContainer
    property color onError
    property color onErrorContainer
    Binding { target: root; property: "onSurface";            value: root.role("onSurface", "#cacccc") }
    Binding { target: root; property: "onSurfaceVariant";     value: root.role("onSurfaceVariant", "#a8adb0") }
    Binding { target: root; property: "onPrimary";            value: root.role("onPrimary", "#101315") }
    Binding { target: root; property: "onPrimaryContainer";   value: root.role("onPrimaryContainer", "#a8adb0") }
    Binding { target: root; property: "onSecondary";          value: root.role("onSecondary", "#101315") }
    Binding { target: root; property: "onSecondaryContainer"; value: root.role("onSecondaryContainer", "#a8adb0") }
    Binding { target: root; property: "onTertiary";           value: root.role("onTertiary", "#101315") }
    Binding { target: root; property: "onTertiaryContainer";  value: root.role("onTertiaryContainer", "#de6145") }
    Binding { target: root; property: "onError";              value: root.role("onError", "#101315") }
    Binding { target: root; property: "onErrorContainer";     value: root.role("onErrorContainer", "#de6145") }

    // shadow and scrim are Material roles Ryoku paints with; always near-black.
    readonly property color shadow: role("shadow", "#000000")
    readonly property color scrim:  role("scrim", "#000000")

    // --- derived accent variants (Ryoku hover/active/gradient tints) ----------
    // No Material role covers a lightened/muted accent, so these track primary.
    readonly property color vermLit:     Qt.lighter(primary, 1.22)
    readonly property color vermDim:      Qt.darker(primary, 1.4)
    readonly property color vermDimDeep:  Qt.darker(primary, 2.2)
    readonly property color threadBg:     Qt.rgba(primary.r, primary.g, primary.b, 0.13)

    // --- overlay tints (Ryoku selection/hover fills over the surface) ---------
    readonly property color frameBg:     Qt.rgba(primary.r, primary.g, primary.b, 0.10)
    readonly property color frameBorder: Qt.rgba(onSurface.r, onSurface.g, onSurface.b, 0.18)
    // A dim tone for out-of-scope elements, e.g. out-of-month calendar days.
    readonly property color ghost:       Qt.darker(onSurfaceVariant, 1.6)

    // --- smart contrast ink (WCAG) --------------------------------------------
    // Sidebar ink (text, icons) must stay legible on whatever surface a palette
    // paints: any of the named presets (raw daemon palettes, uncontrasted), a
    // light or dark wallpaper palette, or a translucent panel where the
    // wallpaper bleeds through. These are pure math on colours already in
    // scope, so bindings stay cheap; each resolves an ink role against the
    // background it truly sits on and corrects ONLY when the role fails, so a
    // well-behaved palette is returned untouched and renders exactly as before.

    // WCAG relative luminance of an opaque colour.
    function relLum(c) {
        function lin(u) { return u <= 0.04045 ? u / 12.92 : Math.pow((u + 0.055) / 1.055, 2.4); }
        return 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b);
    }

    // WCAG contrast ratio between two opaque colours (1..21).
    function contrastRatio(a, b) {
        var la = relLum(a), lb = relLum(b);
        return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
    }

    // Composite a colour (honouring its own alpha) over an opaque background,
    // so an alpha-tinted fill resolves to the flat tone the eye actually reads.
    function blend(over, base) {
        var a = over.a;
        return Qt.rgba(over.r * a + base.r * (1 - a),
                       over.g * a + base.g * (1 - a),
                       over.b * a + base.b * (1 - a), 1);
    }

    // The background the sidebar ink truly sits on: the panel surface, and when
    // the frame is translucent (frameOpacity < 1) that surface composited over
    // the live wallpaper tone bleeding through it. Opaque frame -> just surface.
    readonly property color effectiveSurface: windowOpacity >= 0.999
        ? surface
        : blend(Qt.rgba(surface.r, surface.g, surface.b, windowOpacity), Scheme.wallpaperTone)

    // Smart ink: keep `role` when it already clears `minRatio` against `bg`
    // (the common case, a sound palette is untouched), else nudge it toward the
    // pole `bg` needs (white on a dark bg, black on a light one) until it
    // clears. An accent is lightened/darkened in place, keeping its hue for the
    // first steps; only a pathological pairing walks to a near-neutral. Pure
    // black or white clears >= 4.58:1 on any bg, so the walk always lands.
    function inkOn(bg, role, minRatio) {
        var target = (minRatio === undefined) ? 4.5 : minRatio;
        if (contrastRatio(role, bg) >= target)
            return role;
        var pole = relLum(bg) > 0.179 ? 0 : 1;
        var r = role.r, g = role.g, b = role.b;
        for (var i = 0; i < 12; ++i) {
            r += (pole - r) * 0.18;
            g += (pole - g) * 0.18;
            b += (pole - b) * 0.18;
            var c = Qt.rgba(r, g, b, 1);
            if (contrastRatio(c, bg) >= target)
                return c;
        }
        return Qt.rgba(pole, pole, pole, 1);
    }

    // The best neutral ink for a background at `minRatio` (default 4.5), seeded
    // from onSurface so a good palette returns its own text colour verbatim.
    function ink(bg, minRatio) {
        return inkOn(bg, onSurface, minRatio === undefined ? 4.5 : minRatio);
    }

    // --- flame identity (the RASHIN mark; a fixed Ryoku accent) ---------------
    readonly property color flameGlow: "#ff9e64"
    readonly property color flameCore: "#ffd2bf"

    // Hard offset for the brutalist drop shadow (opaque, no blur).
    readonly property int shadowOffset: 3

    // --- sizing (compiled defaults; frameOpacity is the Config knob) ----------
    readonly property int  radiusWidget: 8
    readonly property int  radiusWindow: 8
    readonly property int  borderWidth: 2
    readonly property int  paddingSm: 4
    readonly property int  paddingMd: 8
    readonly property int  paddingLg: 16
    readonly property int  iconSm: 16
    readonly property int  iconMd: 24
    readonly property int  iconLg: 32
    readonly property real windowOpacity: Config.frameOpacity

    // --- fonts ----------------------------------------------------------------
    // Primary is the shell UI face; empty resolves to the platform default sans,
    // matching the reference's inherited system font. secondary/tertiary are
    // override slots that inherit primary until a theme sets them.
    readonly property string fontPrimary:   Config.fontFamily.length > 0 ? Config.fontFamily : "Space Grotesk"
    readonly property string fontSecondary: fontPrimary
    readonly property string fontTertiary:  fontPrimary
    readonly property int fontSm: 14
    readonly property int fontMd: 16
    readonly property int fontLg: 18
    readonly property int fontXl: 26
    readonly property int fontXxl: 32
    readonly property int fontXxxl: 48
    // Ryoku-specific faces: a serif display, a monospace, and a CJK face.
    readonly property string display: "Fraunces"
    readonly property string mono:    "JetBrainsMono Nerd Font"
    readonly property string fontJp:  "Noto Sans CJK JP"

    // brand mark, user-overridable via ~/.config/ryoku/brand.json (Shell ->
    // Global). BrandMark renders `mark`, or `markSource` (an image) when set.
    readonly property string mark: Ui.UiPreferences.brandMark(Config.markText)
    readonly property string markSource: Config.markImage
    readonly property bool markTint: Config.markTint

    /**
     * MPRIS trackArtists arrives as a JS array from some players and as a
     * plain string from others (Spotify); calling join on the string throws
     * and kills the whole binding. Handles both, falls back to trackArtist.
     */
    function joinArtists(artists, single) {
        if (artists && typeof artists.join === "function" && artists.length > 0)
            return artists.join(", ");
        if (artists && String(artists).length > 0)
            return String(artists);
        return single ? String(single) : "";
    }
}

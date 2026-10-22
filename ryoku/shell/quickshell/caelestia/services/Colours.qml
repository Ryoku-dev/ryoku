pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Images
import Caelestia.Config
import Caelestia.Services
import caelestia.services
import caelestia.utils

Singleton {
    id: root

    property bool showPreview
    property string scheme: "dynamic"
    property string flavour: "default"
    property string variant: "default"
    property real intensity: 1.0
    readonly property real maxIntensity: 2.0
    readonly property real intensityFraction: intensity / maxIntensity
    property string previewScheme: ""
    property string previewFlavour: ""
    property string previewVariant: ""
    readonly property bool light: showPreview ? previewLight : currentLight
    property bool currentLight
    property bool previewLight
    readonly property M3Palette palette: showPreview ? preview : current
    readonly property M3TPalette tPalette: M3TPalette {}
    readonly property M3Palette current: M3Palette {}
    readonly property M3Palette preview: M3Palette {}
    readonly property Transparency transparency: Transparency {}
    readonly property alias wallLuminance: analyser.luminance

    function updatePaletteManager(): void {
        const p = root.palette;
        PaletteManager.update(
            {
                "m3primary_paletteKeyColor":          p.m3primary_paletteKeyColor,
                "m3secondary_paletteKeyColor":        p.m3secondary_paletteKeyColor,
                "m3tertiary_paletteKeyColor":         p.m3tertiary_paletteKeyColor,
                "m3neutral_paletteKeyColor":          p.m3neutral_paletteKeyColor,
                "m3neutral_variant_paletteKeyColor":  p.m3neutral_variant_paletteKeyColor,
                "m3background":                       p.m3background,
                "m3onBackground":                     p.m3onBackground,
                "m3surface":                          p.m3surface,
                "m3surfaceDim":                       p.m3surfaceDim,
                "m3surfaceBright":                    p.m3surfaceBright,
                "m3surfaceContainerLowest":           p.m3surfaceContainerLowest,
                "m3surfaceContainerLow":              p.m3surfaceContainerLow,
                "m3surfaceContainer":                 p.m3surfaceContainer,
                "m3surfaceContainerHigh":             p.m3surfaceContainerHigh,
                "m3surfaceContainerHighest":          p.m3surfaceContainerHighest,
                "m3onSurface":                        p.m3onSurface,
                "m3surfaceVariant":                   p.m3surfaceVariant,
                "m3onSurfaceVariant":                 p.m3onSurfaceVariant,
                "m3inverseSurface":                   p.m3inverseSurface,
                "m3inverseOnSurface":                 p.m3inverseOnSurface,
                "m3outline":                          p.m3outline,
                "m3outlineVariant":                   p.m3outlineVariant,
                "m3shadow":                           p.m3shadow,
                "m3scrim":                            p.m3scrim,
                "m3surfaceTint":                      p.m3surfaceTint,
                "m3primary":                          p.m3primary,
                "m3onPrimary":                        p.m3onPrimary,
                "m3primaryContainer":                 p.m3primaryContainer,
                "m3onPrimaryContainer":               p.m3onPrimaryContainer,
                "m3inversePrimary":                   p.m3inversePrimary,
                "m3secondary":                        p.m3secondary,
                "m3onSecondary":                      p.m3onSecondary,
                "m3secondaryContainer":               p.m3secondaryContainer,
                "m3onSecondaryContainer":             p.m3onSecondaryContainer,
                "m3tertiary":                         p.m3tertiary,
                "m3onTertiary":                       p.m3onTertiary,
                "m3tertiaryContainer":                p.m3tertiaryContainer,
                "m3onTertiaryContainer":              p.m3onTertiaryContainer,
                "m3error":                            p.m3error,
                "m3onError":                          p.m3onError,
                "m3errorContainer":                   p.m3errorContainer,
                "m3onErrorContainer":                 p.m3onErrorContainer,
                "m3success":                          p.m3success,
                "m3onSuccess":                        p.m3onSuccess,
                "m3successContainer":                 p.m3successContainer,
                "m3onSuccessContainer":               p.m3onSuccessContainer,
                "m3primaryFixed":                     p.m3primaryFixed,
                "m3primaryFixedDim":                  p.m3primaryFixedDim,
                "m3onPrimaryFixed":                   p.m3onPrimaryFixed,
                "m3onPrimaryFixedVariant":            p.m3onPrimaryFixedVariant,
                "m3secondaryFixed":                   p.m3secondaryFixed,
                "m3secondaryFixedDim":                p.m3secondaryFixedDim,
                "m3onSecondaryFixed":                 p.m3onSecondaryFixed,
                "m3onSecondaryFixedVariant":          p.m3onSecondaryFixedVariant,
                "m3tertiaryFixed":                    p.m3tertiaryFixed,
                "m3tertiaryFixedDim":                 p.m3tertiaryFixedDim,
                "m3onTertiaryFixed":                  p.m3onTertiaryFixed,
                "m3onTertiaryFixedVariant":           p.m3onTertiaryFixedVariant,
            },
            root.light,
            root.transparency.enabled,
            root.transparency.base,
            root.transparency.layers,
            root.wallLuminance
        );
    }

    property bool cooldownPending

    property real lastBaseTransparency

    property bool schemeLoaded: false

    property int schemeRetryCount: 0

    property int maxSchemeRetries: 20

    property int startupSchemePollCount: 0

    property int maxStartupSchemePolls: 120

    function getLuminance(c: color): real {
        if (c.r == 0 && c.g == 0 && c.b == 0)
            return 0;
        return Math.sqrt(0.299 * (c.r ** 2) + 0.587 * (c.g ** 2) + 0.114 * (c.b ** 2));
    }

    function alterColour(c: color, a: real, layer: int): color {
        const luminance = getLuminance(c);

        const offset = (!light || layer == 1 ? 1 : -layer / 2) * (light ? 0.2 : 0.3) * (1 - transparency.base) * (1 + wallLuminance * (light ? (layer == 1 ? 3 : 1) : 2.5));
        const scale = (luminance + offset) / luminance;
        const r = Math.max(0, Math.min(1, c.r * scale));
        const g = Math.max(0, Math.min(1, c.g * scale));
        const b = Math.max(0, Math.min(1, c.b * scale));

        return Qt.rgba(r, g, b, a);
    }

    function layer(c: color, layer: var): color {
        if (!transparency.enabled)
            return c;

        return layer === 0 ? Qt.alpha(c, transparency.base) : alterColour(c, transparency.layers, layer ?? 1);
    }

    function on(c: color): color {
        if (c.hslLightness < 0.5)
            return Qt.hsla(c.hslHue, c.hslSaturation, 0.9, 1);
        return Qt.hsla(c.hslHue, c.hslSaturation, 0.1, 1);
    }

    function harmonizeWith(designColor: color, sourceColor: color): color {
        let fromHue = designColor.hslHue;
        let toHue = sourceColor.hslHue;

        let diff = toHue - fromHue;
        if (diff > 0.5) diff -= 1.0;
        else if (diff < -0.5) diff += 1.0;

        const cap = 15.0 / 360.0;
        let rotation = diff * 0.5;
        if (rotation > cap) rotation = cap;
        else if (rotation < -cap) rotation = -cap;

        let outputHue = fromHue + rotation;
        if (outputHue < 0) outputHue += 1.0;
        if (outputHue > 1) outputHue -= 1.0;

        return Qt.hsla(outputHue, designColor.hslSaturation, designColor.hslLightness, designColor.a);
    }

    function harmonize(designColor: color): color {
        return harmonizeWith(designColor, root.palette.m3primary);
    }

    function load(data: string, isPreview: bool): void {
        const raw = (data || "").trim();
        if (raw === "") {
            if (!isPreview)
                scheduleSchemeReload();
            return;
        }

        const colours = isPreview ? preview : current;
        let scheme = null;
        try {
            scheme = JSON.parse(raw);
        } catch (e) {
            if (!isPreview)
                scheduleSchemeReload();
            return;
        }

        if (!scheme || !scheme.colours) {
            if (!isPreview)
                scheduleSchemeReload();
            return;
        }

        if (!isPreview) {
            root.scheme = (scheme.name || "").trim();
            root.flavour = (scheme.flavour || "").trim();
            root.variant = (scheme.variant || "").trim();
            root.currentLight = scheme.mode === "light";

            root.showPreview = false;

            // Absent, null, and a value the range does not take all mean the same thing here:
            // what is in effect is the palette the engine produced. 0 is a real setting (a grey
            // palette), so this cannot lean on falsiness either.
            const intensity = Number(scheme.intensity ?? NaN);
            root.intensity = Number.isFinite(intensity) ? Math.min(root.maxIntensity, Math.max(0, intensity)) : 1.0;
        } else {
            root.previewScheme = (scheme.name || "").trim();
            root.previewFlavour = (scheme.flavour || "").trim();
            root.previewVariant = (scheme.variant || "").trim();
            root.previewLight = scheme.mode === "light";
        }

        applyColours(colours, scheme.colours);

        if (!isPreview) {
            root.schemeLoaded = true;
            root.schemeRetryCount = 0;
        }
    }

    function loadRyoku(data: string): void {
        let palette = null;
        try {
            palette = JSON.parse((data || "").trim());
        } catch (e) {
            scheduleSchemeReload();
            return;
        }
        if (!palette || !palette.surface) {
            scheduleSchemeReload();
            return;
        }

        const colours = {};
        for (const [role, hex] of Object.entries(palette)) {
            const term = /^color(\d+)$/.exec(role);
            colours[term ? `term${term[1]}` : role] = String(hex).replace(/^#/, "");
        }
        const dark = getLuminance(Qt.color(palette.surface)) < 0.5;
        load(JSON.stringify({ name: "ryoku", flavour: "default", variant: "default", mode: dark ? "dark" : "light", colours: colours }), false);
    }

    function scheduleSchemeReload(): void {
        if (root.schemeLoaded)
            return;
        if (root.schemeRetryCount >= root.maxSchemeRetries)
            return;
        if (!schemeRetryTimer.running)
            schemeRetryTimer.start();
    }

    /// Fold a scheme's `colours` into a palette. A file leaves the leading `#` off and names a
    /// terminal role plainly, so the role a palette holds is not the key the file uses.
    function applyColours(palette: M3Palette, colours: var): void {
        for (const [role, colour] of Object.entries(colours)) {
            const propName = role.startsWith("term") ? role : `m3${role}`;
            if (palette.hasOwnProperty(propName))
                palette[propName] = colour.startsWith("#") ? colour : `#${colour}`;
        }
    }

    function previewNamed(name: string, flavour: string, colours: var, light: bool): void {
        if (!colours)
            return;
        root.previewScheme = name;
        root.previewFlavour = flavour;
        root.previewVariant = root.variant;
        root.previewLight = light;
        applyColours(root.preview, colours);
        root.showPreview = true;
    }

    function clearPreview(): void {
        root.showPreview = false;
    }

    // Light and dark are the Hub's curated presets: the Hub writes the palette this
    // file watches, so the shell follows the change like any other.
    function setMode(mode: string): void {
        if (mode === "light" || mode === "dark")
            Quickshell.execDetached(["ryoku-hub", "desktop", "scheme", mode]);
    }

    // shortcut: the Ryoku palette has no intensity knob and reseeding is the theme
    // daemon's job, so these do nothing; wire them if the daemon grows the controls.

    function setIntensity(fraction: real): void {}

    function reseedScheme(): void {}

    function reloadHyprRules(): void {
        // Layer rules are Hyprland-only; KWin handles blur via effects.
                    return;

        let rule, trEnabled;
        if (Kwin.usingLua) {
            rule = `eval hl.layer_rule({ match = { namespace = "caelestia-drawers" }, %1 = %2 })`;
            trEnabled = transparency.enabled;
        } else {
            rule = "keyword layerrule %1 %2, match:namespace caelestia-drawers";
            trEnabled = transparency.enabled ? 1 : 0;
        }
        Kwin.extras.batchMessage([rule.arg("blur").arg(trEnabled), rule.arg("ignore_alpha").arg(Math.max(0, transparency.base - 0.03))]);
    }

    function requestReloadHyprRules(): void {
        if (cooldownTimer.running) {
            root.cooldownPending = true;
        } else {
            root.reloadHyprRules();
            cooldownTimer.restart();
        }
    }

    Component.onCompleted: {
        root.requestReloadHyprRules()
        Qt.callLater(updatePaletteManager)
        scheduleSchemeReload()
        startupSchemePollTimer.start()
        reseedTimer.start()
    }

    Connections {
        function onConfigReloaded(): void {
            root.reloadHyprRules();
        }

        target: Kwin
    }



    // The palette is the one the Ryoku theme daemon writes for the whole desktop: a flat
    // map of Material roles, plus color0 to color15 for the terminal, as "#rrggbb".
    FileView {
        id: schemeFile

        path: `${Quickshell.env("XDG_CACHE_HOME") || `${Quickshell.env("HOME")}/.cache`}/ryoku/colors.json`
        watchChanges: true
        onFileChanged: reload()
        onLoaded: root.loadRyoku(text())
    }

    Connections {
        target: SchemeLoader

        function onCurrentSchemeChanged(): void {
            schemeFile.reload();
        }
    }

    Timer {
        id: reseedTimer

        interval: 2500
        repeat: false
        onTriggered: root.reseedScheme()
    }

    Timer {
        id: schemeRetryTimer

        interval: 350
        repeat: false
        onTriggered: {
            if (root.schemeLoaded)
                return;
            if (root.schemeRetryCount >= root.maxSchemeRetries)
                return;
            root.schemeRetryCount += 1;
            schemeFile.reload();
            if (!root.schemeLoaded)
                restart();
        }
    }

    Timer {
        id: startupSchemePollTimer

        interval: 500
        repeat: true
        onTriggered: {
            if (root.schemeLoaded) {
                stop();
                return;
            }
            if (root.startupSchemePollCount >= root.maxStartupSchemePolls) {
                stop();
                return;
            }
            root.startupSchemePollCount += 1;
            schemeFile.reload();
        }
    }

    ImageAnalyser {
        id: analyser

        source: Wallpapers.current
        onLuminanceChanged: Qt.callLater(root.updatePaletteManager)
    }

    Connections {
        target: root.palette

        function onM3primaryChanged(): void { Qt.callLater(root.updatePaletteManager); }

        function onM3backgroundChanged(): void { Qt.callLater(root.updatePaletteManager); }
    }

    onLightChanged: Qt.callLater(updatePaletteManager)

    onShowPreviewChanged: Qt.callLater(updatePaletteManager)



    Timer {
        id: cooldownTimer

        interval: 30
        onTriggered: {
            if (root.cooldownPending) {
                root.cooldownPending = false;
                root.reloadHyprRules();
                restart();
            }
        }
    }

    Timer {
        id: cAnimCompleteTimer

        interval: Tokens.anim.durations.expressiveSlowEffects
        onTriggered: root.requestReloadHyprRules()
    }

    component Transparency: QtObject {
        readonly property bool enabled: Tokens.transparency.enabled && !(GameMode.enabled && GlobalConfig.utilities.gameMode.disableShellTransparency)
        readonly property real base: Math.max(0, Math.min(1, Tokens.transparency.base - (root.light ? 0.1 : 0)))
        readonly property real layers: Math.max(0, Math.min(1, Tokens.transparency.layers))

        onEnabledChanged: {
            if (enabled)
                root.requestReloadHyprRules();
            else
                cAnimCompleteTimer.start();
        }
        onBaseChanged: {
            if (root.lastBaseTransparency > base)
                root.requestReloadHyprRules();
            else
                cAnimCompleteTimer.start();
            root.lastBaseTransparency = base;
        }
    }

    component M3TPalette: QtObject {
        readonly property color m3primary_paletteKeyColor:         PaletteManager.tPalette["m3primary_paletteKeyColor"] ?? root.palette.m3primary_paletteKeyColor
        readonly property color m3secondary_paletteKeyColor:       PaletteManager.tPalette["m3secondary_paletteKeyColor"] ?? root.palette.m3secondary_paletteKeyColor
        readonly property color m3tertiary_paletteKeyColor:        PaletteManager.tPalette["m3tertiary_paletteKeyColor"] ?? root.palette.m3tertiary_paletteKeyColor
        readonly property color m3neutral_paletteKeyColor:         PaletteManager.tPalette["m3neutral_paletteKeyColor"] ?? root.palette.m3neutral_paletteKeyColor
        readonly property color m3neutral_variant_paletteKeyColor: PaletteManager.tPalette["m3neutral_variant_paletteKeyColor"] ?? root.palette.m3neutral_variant_paletteKeyColor
        readonly property color m3background:                      PaletteManager.tPalette["m3background"] ?? root.palette.m3background
        readonly property color m3onBackground:                    PaletteManager.tPalette["m3onBackground"] ?? root.palette.m3onBackground
        readonly property color m3surface:                         PaletteManager.tPalette["m3surface"] ?? root.palette.m3surface
        readonly property color m3surfaceDim:                      PaletteManager.tPalette["m3surfaceDim"] ?? root.palette.m3surfaceDim
        readonly property color m3surfaceBright:                   PaletteManager.tPalette["m3surfaceBright"] ?? root.palette.m3surfaceBright
        readonly property color m3surfaceContainerLowest:          PaletteManager.tPalette["m3surfaceContainerLowest"] ?? root.palette.m3surfaceContainerLowest
        readonly property color m3surfaceContainerLow:             PaletteManager.tPalette["m3surfaceContainerLow"] ?? root.palette.m3surfaceContainerLow
        readonly property color m3surfaceContainer:                PaletteManager.tPalette["m3surfaceContainer"] ?? root.palette.m3surfaceContainer
        readonly property color m3surfaceContainerHigh:            PaletteManager.tPalette["m3surfaceContainerHigh"] ?? root.palette.m3surfaceContainerHigh
        readonly property color m3surfaceContainerHighest:         PaletteManager.tPalette["m3surfaceContainerHighest"] ?? root.palette.m3surfaceContainerHighest
        readonly property color m3onSurface:                       PaletteManager.tPalette["m3onSurface"] ?? root.palette.m3onSurface
        readonly property color m3surfaceVariant:                  PaletteManager.tPalette["m3surfaceVariant"] ?? root.palette.m3surfaceVariant
        readonly property color m3onSurfaceVariant:                PaletteManager.tPalette["m3onSurfaceVariant"] ?? root.palette.m3onSurfaceVariant
        readonly property color m3inverseSurface:                  PaletteManager.tPalette["m3inverseSurface"] ?? root.palette.m3inverseSurface
        readonly property color m3inverseOnSurface:                PaletteManager.tPalette["m3inverseOnSurface"] ?? root.palette.m3inverseOnSurface
        readonly property color m3outline:                         PaletteManager.tPalette["m3outline"] ?? root.palette.m3outline
        readonly property color m3outlineVariant:                  PaletteManager.tPalette["m3outlineVariant"] ?? root.palette.m3outlineVariant
        readonly property color m3shadow:                          PaletteManager.tPalette["m3shadow"] ?? root.palette.m3shadow
        readonly property color m3scrim:                           PaletteManager.tPalette["m3scrim"] ?? root.palette.m3scrim
        readonly property color m3surfaceTint:                     PaletteManager.tPalette["m3surfaceTint"] ?? root.palette.m3surfaceTint
        readonly property color m3primary:                         PaletteManager.tPalette["m3primary"] ?? root.palette.m3primary
        readonly property color m3onPrimary:                       PaletteManager.tPalette["m3onPrimary"] ?? root.palette.m3onPrimary
        readonly property color m3primaryContainer:                PaletteManager.tPalette["m3primaryContainer"] ?? root.palette.m3primaryContainer
        readonly property color m3onPrimaryContainer:              PaletteManager.tPalette["m3onPrimaryContainer"] ?? root.palette.m3onPrimaryContainer
        readonly property color m3inversePrimary:                  PaletteManager.tPalette["m3inversePrimary"] ?? root.palette.m3inversePrimary
        readonly property color m3secondary:                       PaletteManager.tPalette["m3secondary"] ?? root.palette.m3secondary
        readonly property color m3onSecondary:                     PaletteManager.tPalette["m3onSecondary"] ?? root.palette.m3onSecondary
        readonly property color m3secondaryContainer:              PaletteManager.tPalette["m3secondaryContainer"] ?? root.palette.m3secondaryContainer
        readonly property color m3onSecondaryContainer:            PaletteManager.tPalette["m3onSecondaryContainer"] ?? root.palette.m3onSecondaryContainer
        readonly property color m3tertiary:                        PaletteManager.tPalette["m3tertiary"] ?? root.palette.m3tertiary
        readonly property color m3onTertiary:                      PaletteManager.tPalette["m3onTertiary"] ?? root.palette.m3onTertiary
        readonly property color m3tertiaryContainer:               PaletteManager.tPalette["m3tertiaryContainer"] ?? root.palette.m3tertiaryContainer
        readonly property color m3onTertiaryContainer:             PaletteManager.tPalette["m3onTertiaryContainer"] ?? root.palette.m3onTertiaryContainer
        readonly property color m3error:                           PaletteManager.tPalette["m3error"] ?? root.palette.m3error
        readonly property color m3onError:                         PaletteManager.tPalette["m3onError"] ?? root.palette.m3onError
        readonly property color m3errorContainer:                  PaletteManager.tPalette["m3errorContainer"] ?? root.palette.m3errorContainer
        readonly property color m3onErrorContainer:                PaletteManager.tPalette["m3onErrorContainer"] ?? root.palette.m3onErrorContainer
        readonly property color m3success:                         PaletteManager.tPalette["m3success"] ?? root.palette.m3success
        readonly property color m3onSuccess:                       PaletteManager.tPalette["m3onSuccess"] ?? root.palette.m3onSuccess
        readonly property color m3successContainer:                PaletteManager.tPalette["m3successContainer"] ?? root.palette.m3successContainer
        readonly property color m3onSuccessContainer:              PaletteManager.tPalette["m3onSuccessContainer"] ?? root.palette.m3onSuccessContainer
        readonly property color m3primaryFixed:                    PaletteManager.tPalette["m3primaryFixed"] ?? root.palette.m3primaryFixed
        readonly property color m3primaryFixedDim:                 PaletteManager.tPalette["m3primaryFixedDim"] ?? root.palette.m3primaryFixedDim
        readonly property color m3onPrimaryFixed:                  PaletteManager.tPalette["m3onPrimaryFixed"] ?? root.palette.m3onPrimaryFixed
        readonly property color m3onPrimaryFixedVariant:           PaletteManager.tPalette["m3onPrimaryFixedVariant"] ?? root.palette.m3onPrimaryFixedVariant
        readonly property color m3secondaryFixed:                  PaletteManager.tPalette["m3secondaryFixed"] ?? root.palette.m3secondaryFixed
        readonly property color m3secondaryFixedDim:               PaletteManager.tPalette["m3secondaryFixedDim"] ?? root.palette.m3secondaryFixedDim
        readonly property color m3onSecondaryFixed:                PaletteManager.tPalette["m3onSecondaryFixed"] ?? root.palette.m3onSecondaryFixed
        readonly property color m3onSecondaryFixedVariant:         PaletteManager.tPalette["m3onSecondaryFixedVariant"] ?? root.palette.m3onSecondaryFixedVariant
        readonly property color m3tertiaryFixed:                   PaletteManager.tPalette["m3tertiaryFixed"] ?? root.palette.m3tertiaryFixed
        readonly property color m3tertiaryFixedDim:                PaletteManager.tPalette["m3tertiaryFixedDim"] ?? root.palette.m3tertiaryFixedDim
        readonly property color m3onTertiaryFixed:                 PaletteManager.tPalette["m3onTertiaryFixed"] ?? root.palette.m3onTertiaryFixed
        readonly property color m3onTertiaryFixedVariant:          PaletteManager.tPalette["m3onTertiaryFixedVariant"] ?? root.palette.m3onTertiaryFixedVariant
    }

    component M3Palette: QtObject {
        property color m3primary_paletteKeyColor: "#a8627b"
        property color m3secondary_paletteKeyColor: "#8e6f78"
        property color m3tertiary_paletteKeyColor: "#986e4c"
        property color m3neutral_paletteKeyColor: "#807477"
        property color m3neutral_variant_paletteKeyColor: "#837377"
        property color m3background: "#191114"
        property color m3onBackground: "#efdfe2"
        property color m3surface: "#191114"
        property color m3surfaceDim: "#191114"
        property color m3surfaceBright: "#403739"
        property color m3surfaceContainerLowest: "#130c0e"
        property color m3surfaceContainerLow: "#22191c"
        property color m3surfaceContainer: "#261d20"
        property color m3surfaceContainerHigh: "#31282a"
        property color m3surfaceContainerHighest: "#3c3235"
        property color m3onSurface: "#efdfe2"
        property color m3surfaceVariant: "#514347"
        property color m3onSurfaceVariant: "#d5c2c6"
        property color m3inverseSurface: "#efdfe2"
        property color m3inverseOnSurface: "#372e30"
        property color m3outline: "#9e8c91"
        property color m3outlineVariant: "#514347"
        property color m3shadow: "#000000"
        property color m3scrim: "#000000"
        property color m3surfaceTint: "#ffb0ca"
        property color m3primary: "#ffb0ca"
        property color m3onPrimary: "#541d34"
        property color m3primaryContainer: "#6f334a"
        property color m3onPrimaryContainer: "#ffd9e3"
        property color m3inversePrimary: "#8b4a62"
        property color m3secondary: "#e2bdc7"
        property color m3onSecondary: "#422932"
        property color m3secondaryContainer: "#5a3f48"
        property color m3onSecondaryContainer: "#ffd9e3"
        property color m3tertiary: "#f0bc95"
        property color m3onTertiary: "#48290c"
        property color m3tertiaryContainer: "#b58763"
        property color m3onTertiaryContainer: "#000000"
        property color m3error: "#ffb4ab"
        property color m3onError: "#690005"
        property color m3errorContainer: "#93000a"
        property color m3onErrorContainer: "#ffdad6"
        property color m3success: "#B5CCBA"
        property color m3onSuccess: "#213528"
        property color m3successContainer: "#374B3E"
        property color m3onSuccessContainer: "#D1E9D6"
        property color m3primaryFixed: "#ffd9e3"
        property color m3primaryFixedDim: "#ffb0ca"
        property color m3onPrimaryFixed: "#39071f"
        property color m3onPrimaryFixedVariant: "#6f334a"
        property color m3secondaryFixed: "#ffd9e3"
        property color m3secondaryFixedDim: "#e2bdc7"
        property color m3onSecondaryFixed: "#2b151d"
        property color m3onSecondaryFixedVariant: "#5a3f48"
        property color m3tertiaryFixed: "#ffdcc3"
        property color m3tertiaryFixedDim: "#f0bc95"
        property color m3onTertiaryFixed: "#2f1500"
        property color m3onTertiaryFixedVariant: "#623f21"
        property color term0: "#353434"
        property color term1: "#ff4c8a"
        property color term2: "#ffbbb7"
        property color term3: "#ffdedf"
        property color term4: "#b3a2d5"
        property color term5: "#e98fb0"
        property color term6: "#ffba93"
        property color term7: "#eed1d2"
        property color term8: "#b39e9e"
        property color term9: "#ff80a3"
        property color term10: "#ffd3d0"
        property color term11: "#fff1f0"
        property color term12: "#dcbc93"
        property color term13: "#f9a8c2"
        property color term14: "#ffd1c0"
        property color term15: "#ffffff"
    }
}

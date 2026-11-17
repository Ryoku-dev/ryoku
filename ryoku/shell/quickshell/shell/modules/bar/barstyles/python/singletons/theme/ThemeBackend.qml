pragma Singleton
import QtQuick
import shell.services
import "../../"
import Ryoku.Ui.Singletons as Ui

Item {
    id: root

    // Serpantinum writes its glyphs into ordinary text, so the style's face must be a Nerd Font. The global
    // font is used when it is one; otherwise the bar falls back to JetBrainsMono Nerd Font, which
    // ryoku-desktop ships on every edition, rather than drawing other fonts' private-use glyphs.
    readonly property string shippedNerdFont: "JetBrainsMono Nerd Font"
    readonly property string globalFont: Config.globalFontFamily || ""
    property string fontFamily: /Nerd Font|\bNF[MP]?\b/.test(root.globalFont) ? root.globalFont : root.shippedNerdFont
    readonly property string iconFont: "JetBrainsMono Nerd Font Mono"
    readonly property bool iconFontReady: true

    property var overrides: Config.getSetting("theme", {})
    property int borderRadius: overrides.borderRadius !== undefined ? Number(overrides.borderRadius) : 8
    readonly property int clampedBorderRadius: {
        const r = borderRadius;
        return Math.floor(r <= 24 ? r : 24 + Math.pow(r - 24, 0.55));
    }

    readonly property real configuredOpacity: {
        const value = overrides.elementOpacity !== undefined ? Number(overrides.elementOpacity) : 100;
        return isNaN(value) ? 1.0 : Math.max(0.2, Math.min(1.0, value / 100.0));
    }
    readonly property bool blurBackground: overrides.blurBackground === true
    readonly property string blurStyle: overrides.blurStyle === "liquid" ? "liquid" : "blur"
    readonly property real liquidGlassOpacity: 0.1
    readonly property bool liquidGlass: blurBackground && blurStyle === "liquid"
    readonly property real elementOpacity: liquidGlass ? liquidGlassOpacity : configuredOpacity
    readonly property bool glassActive: blurBackground && elementOpacity < 0.999

    readonly property var palette: (overrides && overrides.colors && overrides.matugen !== true) ? overrides.colors : ({})
    readonly property string dockLauncherColorMode: {
        const mode = overrides.dockLauncherColorMode;
        return mode === "accent" || mode === "custom" ? mode : "bar";
    }
    readonly property string dockLauncherCustomColor: {
        const value = overrides.dockLauncherCustomColor;
        return typeof value === "string" && /^#[0-9a-fA-F]{6}$/.test(value)
            ? value : Ui.Tokens.paper.toString();
    }
    readonly property color dockLauncherBase: dockLauncherColorMode === "accent"
        ? Ui.Tokens.sun
        : (dockLauncherColorMode === "custom" ? dockLauncherCustomColor : Ui.Tokens.paper)
    readonly property color dockLauncherText: Theme.inkOn(dockLauncherBase, Ui.Tokens.ink, 4.5)
    readonly property color dockLauncherSubtext0: Theme.inkOn(dockLauncherBase, Ui.Tokens.inkDim, 4.5)
    readonly property color dockLauncherSurface0: dockLauncherColorMode === "bar"
        ? Ui.Tokens.paperLift : mix(dockLauncherBase, dockLauncherText, 0.10)
    readonly property color dockLauncherSurface1: mix(dockLauncherBase, dockLauncherText, 0.18)
    readonly property color dockLauncherSurface2: mix(dockLauncherBase, dockLauncherText, 0.28)
    readonly property color dockLauncherAccent: Theme.inkOn(dockLauncherBase, Ui.Tokens.sun, 3.0)
    readonly property color dockLauncherOnAccent: Theme.inkOn(dockLauncherAccent, dockLauncherText, 4.5)

    function mix(a, b, t) {
        return Qt.rgba(a.r + (b.r - a.r) * t,
                       a.g + (b.g - a.g) * t,
                       a.b + (b.b - a.b) * t,
                       a.a + (b.a - a.a) * t);
    }

    function slot(name, fallback) {
        const value = root.palette[name];
        return value !== undefined && value !== null && String(value).length > 0 ? value : fallback;
    }

    property color base: slot("base", Theme.surfaceContainerLowest)
    property color mantle: slot("mantle", Theme.surfaceContainerLow)
    property color crust: slot("crust", Theme.surface)
    // Liquid glass is clear over the wallpaper, so text on it is always white.
    readonly property color text: liquidGlass ? "#ffffff" : slot("text", Theme.onSurface)
    readonly property color subtext0: liquidGlass ? "#ffffff" : slot("subtext0", Theme.onSurfaceVariant)
    readonly property color subtext1: liquidGlass ? "#ffffff" : slot("subtext1", mix(Theme.onSurfaceVariant, Theme.onSurface, 0.5))
    property color surface0: slot("surface0", Theme.surfaceContainer)
    property color surface1: slot("surface1", Theme.surfaceContainerHigh)
    property color surface2: slot("surface2", Theme.surfaceContainerHighest)
    property color overlay0: slot("overlay0", Theme.outline)
    property color overlay1: slot("overlay1", mix(Theme.outline, Theme.outlineVariant, 0.5))
    property color overlay2: slot("overlay2", mix(Theme.outlineVariant, Theme.onSurfaceVariant, 0.5))
    property color blue: slot("blue", Theme.primary)
    property color sapphire: slot("sapphire", mix(Theme.primary, Theme.tertiary, 0.4))
    property color peach: slot("peach", Theme.tertiary)
    property color green: slot("green", mix(Theme.secondary, Theme.tertiary, 0.3))
    property color red: slot("red", Theme.error)
    property color mauve: slot("mauve", mix(Theme.primary, Theme.error, 0.35))
    property color pink: slot("pink", mix(Theme.tertiary, Theme.error, 0.45))
    property color yellow: slot("yellow", mix(Theme.secondary, Theme.tertiary, 0.5))
    property color maroon: slot("maroon", mix(Theme.error, Theme.outline, 0.35))
    property color teal: slot("teal", mix(Theme.secondary, Theme.primary, 0.35))

    readonly property color primary: root.mauve
    readonly property color surfaceVariant: root.surface0
    readonly property color primaryContainer: Theme.primaryContainer
    readonly property color secondaryContainer: Theme.secondaryContainer
    readonly property color onSurface: root.text
    readonly property color onSecondaryContainer: Theme.onSecondaryContainer
    readonly property color lavender: mix(root.blue, root.mauve, 0.45)
    readonly property color flamingo: mix(root.peach, root.pink, 0.5)

    readonly property var matugenColors: ({
        "primary": root.mauve.toString(),
        "primary_container": root.surface0.toString(),
        "on_primary_container": root.text.toString(),
        "secondary": root.peach.toString(),
        "tertiary": root.teal.toString()
    })

    function faded(color) {
        return Qt.alpha(color, root.elementOpacity);
    }

    function reloadColors() {
        root.updateColors();
    }

    function updateColors() {
        root.overrides = Config.getSetting("theme", {});
    }

    Connections {
        target: Config
        function onSettingsLoaded() { root.updateColors(); }
        function onRawSettingsChanged() { root.updateColors(); }
    }

    Component.onCompleted: updateColors()
}

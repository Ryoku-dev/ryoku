pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

/**
 * The live wallpaper palette, active while Match wallpaper is on. The theme
 * daemon is the only author of colour math in Ryoku: on every wallpaper change
 * it writes ~/.cache/ryoku/colors.json carrying the full Material role set
 * (camelCase keys) beside the sixteen terminal colours, already contrast
 * corrected for light and dark images alike. This singleton is a thin reader:
 * every role is consumed verbatim, no derivation, so the shell can never
 * disagree with the terminal or the apps about what the palette is.
 *
 * The compiled constants below (Everforest Dark) only paint the first frames
 * of a fresh install, before the daemon's first write.
 */
Singleton {
    id: w

    // The daemon's palette, parsed straight from the file text. JsonAdapter
    // cannot carry it: half the role names start with "on", which QML's
    // signal-handler grammar rejects as adapter property names.
    property var roles: ({})
    property var committedRoles: ({})
    property var previewRoles: ({})
    function refreshRoles() {
        try {
            const t = file.text();
            committedRoles = t && t.length ? (JSON.parse(t) || {}) : {};
        } catch (e) {
            committedRoles = {};
        }
        roles = Object.keys(previewRoles).length ? previewRoles : committedRoles;
    }
    function refreshPreview() {
        try { previewRoles = JSON.parse(previewFile.text() || "{}") || {}; }
        catch (e) { previewRoles = {}; }
        roles = Object.keys(previewRoles).length ? previewRoles : committedRoles;
    }

    // Verbatim role when present, compiled default before the first write.
    function pick(role, fallback) {
        return (role && role.length > 0) ? role : fallback;
    }

    readonly property color surface:                 pick(roles.surface, "#1e2326")
    readonly property color surfaceVariant:          pick(roles.surfaceVariant, "#232a2e")
    readonly property color surfaceContainerLowest:  pick(roles.surfaceContainerLowest, "#1e2326")
    readonly property color surfaceContainerLow:     pick(roles.surfaceContainerLow, "#232a2e")
    readonly property color surfaceContainer:        pick(roles.surfaceContainer, "#272e33")
    readonly property color surfaceContainerHigh:    pick(roles.surfaceContainerHigh, "#2e383c")
    readonly property color surfaceContainerHighest: pick(roles.surfaceContainerHighest, "#374145")
    readonly property color surfaceTint:             pick(roles.surfaceTint, "#a7c080")
    readonly property color inverseSurface:          pick(roles.inverseSurface, "#d3c6aa")
    readonly property color inverseOnSurface:        pick(roles.inverseOnSurface, "#1e2326")

    // The raw wallpaper tone (the colour that bleeds through a translucent
    // panel); Theme composites the surface over it for effective-ink checks.
    readonly property color wallpaperTone: pick(roles.background, "#1e2326")


    // The "on" roles: declared plain and bound via Binding elements. QML's
    // signal-handler grammar eats an inline `onSurface: <expr>` (see the same
    // note in Theme.qml), which left these black for as long as the old
    // derivation code existed; Binding sidesteps the grab.
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

    Binding { target: w; property: "onSurface"; value: w.pick(w.roles.onSurface, "#d3c6aa") }
    Binding { target: w; property: "onSurfaceVariant"; value: w.pick(w.roles.onSurfaceVariant, "#9da9a0") }
    Binding { target: w; property: "onPrimary"; value: w.pick(w.roles.onPrimary, "#1e2326") }
    Binding { target: w; property: "onPrimaryContainer"; value: w.pick(w.roles.onPrimaryContainer, "#d3c6aa") }
    Binding { target: w; property: "onSecondary"; value: w.pick(w.roles.onSecondary, "#1e2326") }
    Binding { target: w; property: "onSecondaryContainer"; value: w.pick(w.roles.onSecondaryContainer, "#d3c6aa") }
    Binding { target: w; property: "onTertiary"; value: w.pick(w.roles.onTertiary, "#1e2326") }
    Binding { target: w; property: "onTertiaryContainer"; value: w.pick(w.roles.onTertiaryContainer, "#d3c6aa") }
    Binding { target: w; property: "onError"; value: w.pick(w.roles.onError, "#1e2326") }
    Binding { target: w; property: "onErrorContainer"; value: w.pick(w.roles.onErrorContainer, "#d3c6aa") }

    readonly property color primary:              pick(roles.primary, "#a7c080")
    readonly property color primaryContainer:     pick(roles.primaryContainer, "#3c4841")
    readonly property color secondary:            pick(roles.secondary, "#7fbbb3")
    readonly property color secondaryContainer:   pick(roles.secondaryContainer, "#324a49")
    readonly property color tertiary:             pick(roles.tertiary, "#83c092")
    readonly property color tertiaryContainer:    pick(roles.tertiaryContainer, "#3a4d44")
    readonly property color error:                pick(roles.error, "#e67e80")
    readonly property color errorContainer:       pick(roles.errorContainer, "#514045")

    readonly property color outline:        pick(roles.outline, "#565d60")
    readonly property color outlineVariant: pick(roles.outlineVariant, "#3d484d")

    // Shadow and scrim are always near-black regardless of the wallpaper.
    readonly property color shadow: "#000000"
    readonly property color scrim:  "#000000"

    FileView {
        id: file
        path: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/colors.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: w.refreshRoles()
    }

    FileView {
        id: previewFile
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/dev/null") + "/ryoku-wall-preview.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: w.refreshPreview()
    }

    Component.onCompleted: refreshRoles()
}

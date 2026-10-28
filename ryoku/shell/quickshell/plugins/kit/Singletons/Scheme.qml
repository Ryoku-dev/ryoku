pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// Thin reader of the daemon palette for the plugin kit. The theme daemon is the
// sole author of colour in Ryoku: a fixed named theme publishes its palette into
// shell.json's themePalette key, and follow-the-wallpaper mode writes the live
// palette to ~/.cache/ryoku/colors.json. This resolves each role the pill's
// way -- a named scheme wins, then the live wallpaper palette, then the compiled
// default -- so a plugin's card, ink and accent retint on ANY scheme change
// (static named theme or wallpaper-follow) with no colour math of its own.
// base/elevated/deep/line map onto the palette's surface depth ramp; accent is
// the primary role verbatim.
Singleton {
    id: root

    // Follow-the-wallpaper master (theme.json), the static named scheme's palette
    // (shell.json themePalette; null for the dynamic variants) and the live
    // wallpaper roles (colors.json). All three are parsed straight from the file
    // text: half the role names start with "on" (which JsonAdapter's
    // signal-handler grammar rejects), a removed themePalette key only reads as
    // absent from raw text, and a bare JsonAdapter does not repopulate reliably
    // for a lazily-created singleton.
    property bool matchWallpaper: true
    property var namedScheme: null
    property var wall: ({})
    property var preview: ({})

    // A palette value is only usable when it is a non-empty hex string; a null,
    // missing or half-written role falls through to the next layer, so a partial
    // colors.json (mid-write) or a scheme missing a role never paints black.
    function usable(v) { return typeof v === "string" && v.length > 0; }

    // Resolve one role through the layer chain: a selected named scheme wins,
    // then the live wallpaper palette while Match wallpaper is on, then the base.
    function role(key, base) {
        if (usable(preview[key]))
            return preview[key];
        if (namedScheme && usable(namedScheme[key]))
            return namedScheme[key];
        if (matchWallpaper && usable(wall[key]))
            return wall[key];
        return base;
    }

    // --- surface depth ramp (the plugin card sits on these) -------------------
    readonly property color base:     role("surface", "#16110b")
    readonly property color elevated: role("surfaceContainer", "#1b150e")
    readonly property color deep:     role("surfaceContainerLowest", "#0f0c07")
    readonly property color line:     role("outlineVariant", "#3d484d")

    // --- accent: the primary role, verbatim (the daemon curates it) -----------
    readonly property color accent: role("primary", "#e2342a")

    // --- ink, resolved from the palette so text reads on the themed card.
    // Declared plain and set via Binding: QML's signal-handler grammar parses an
    // inline `onSurface:` as a change-handler for the sibling `surface` property
    // and silently drops the binding (the transparent-black trap), so a Binding
    // element assigns the role by name instead. ---
    property color onSurface
    property color onSurfaceVariant
    Binding { target: root; property: "onSurface";        value: root.role("onSurface", "#e6dccb") }
    Binding { target: root; property: "onSurfaceVariant"; value: root.role("onSurfaceVariant", "#8f8770") }

    function refreshWall() {
        try {
            const t = wallFile.text();
            root.wall = t && t.length ? (JSON.parse(t) || {}) : {};
        } catch (e) {
            root.wall = {};
        }
    }
    function refreshNamed() {
        var pal = null;
        try {
            const t = shellFile.text();
            if (t) {
                const o = JSON.parse(t);
                if (o && typeof o.themePalette === "object" && o.themePalette !== null)
                    pal = o.themePalette;
            }
        } catch (e) {
            pal = null;
        }
        root.namedScheme = pal;
    }
    function refreshMatch() {
        try {
            const t = themeFile.text();
            const o = t ? JSON.parse(t) : null;
            // Default ON when theme.json is absent or omits the key, matching
            // services/Config.qml and the daemon's matchWallpaperOn: a fresh box
            // follows the wallpaper. Only an explicit false locks it off.
            root.matchWallpaper = (o && typeof o.followWallpaper === "boolean") ? o.followWallpaper : true;
        } catch (e) {
            root.matchWallpaper = true;
        }
    }

    FileView {
        id: wallFile
        path: (Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")) + "/ryoku/colors.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.refreshWall()
    }
    FileView {
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/dev/null") + "/ryoku-wall-preview.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try { root.preview = JSON.parse(text() || "{}") || {} }
            catch (e) { root.preview = {} }
        }
    }
    FileView {
        id: shellFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.refreshNamed()
    }
    FileView {
        id: themeFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.refreshMatch()
    }

    Component.onCompleted: {
        refreshWall();
        refreshNamed();
        refreshMatch();
    }
}

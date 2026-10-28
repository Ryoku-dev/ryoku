pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons as Ui

// Thin reader of the daemon palette for the visualiser. The theme daemon is the
// sole author of colour in Ryoku: a fixed named theme publishes its palette into
// shell.json's themePalette key, and follow-the-wallpaper mode writes the live
// palette to ~/.cache/ryoku/colors.json. This reads both and resolves each
// role exactly the way the pill's Theme does -- a named scheme wins, then the
// live wallpaper palette, then the compiled default -- so the spectrum retints
// on ANY scheme change, static named theme or wallpaper-follow. The compiled
// defaults (Everforest) only paint the first frames before the daemon's first
// write.
//
// The spectrum has no panel under it, so a role is used as published and only
// re-lit when the wallpaper behind would swallow it.
Singleton {
    id: root

    // The follow-the-wallpaper master (theme.json, the single colour source the
    // daemon and window borders also read).
    property bool matchWallpaper: true

    // The static named scheme's palette (shell.json themePalette; null for the
    // dynamic Default/Wallpaper variants) and the live wallpaper roles
    // (colors.json). Both are parsed straight from the file text: half the role
    // names start with "on", which JsonAdapter's signal-handler grammar rejects,
    // a removed themePalette key only reads as absent from raw text, and a bare
    // JsonAdapter does not repopulate reliably for a lazily-created singleton.
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

    // The palette roles as published. The spectrum paints in the accent the rest
    // of the shell uses, not a second palette: pushing every band a fixed step
    // off the wallpaper is what used to send it near-white on a dark picture and
    // yellow on a scheme whose secondary is yellow.
    readonly property color accent: role("primary", "#a7c080")

    // The accent is used as published, so the spectrum is the same colour the
    // bar and the Hub wear. Re-lighting it through a tonal ramp rebuilds the
    // colour from hue and saturation alone, which drains the chroma and lands
    // near-white; that only happens when the wallpaper behind would otherwise
    // swallow the accent whole.
    readonly property int minSep: 18
    readonly property real accentL: Ui.Ink.lstar(root.accent)

    function baseOn(bgL, dir) {
        if (Math.abs(root.accentL - bgL) >= root.minSep)
            return root.accent;
        return Ui.Ink.at("primary", root.accent,
                         Math.max(26, Math.min(90, bgL + dir * root.minSep)));
    }

    // A narrow walk either side of that colour: bass sits deeper than treble
    // without the sweep leaving the hue.
    function colorAt(t, bgL, dir) {
        var c = root.baseOn(bgL, dir);
        var f = 1 + (Math.max(0, Math.min(1, t)) - 0.5) * 0.36;
        return f >= 1 ? Qt.lighter(c, f) : Qt.darker(c, 1 / f);
    }

    // Ink reaches the renderer through here. A file that imports the module
    // loses the local Scheme singleton to QtQuick's own Scheme type, so this
    // is the one place in the config that imports it.
    readonly property var inkTones: Ui.Ink.tones
    readonly property real wallLstar: Ui.Ink.wallLstar
    function lstarAt(nx, ny, nw, nh) { return Ui.Ink.lstarAt(nx, ny, nw, nh); }
    function side(bgL) { return Ui.Ink.side(bgL); }

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

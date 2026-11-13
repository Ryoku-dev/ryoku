pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons as Ui

Singleton {
    id: root

    property alias matchWallpaper: themeAdapter.followWallpaper
    property string fontFamily: "Geologica"
    property real fontScale: 1.0

    function refreshFonts() {
        const family = Ui.ShellSettings.get("fontFamily", null);
        root.fontFamily = typeof family === "string" ? family : "Geologica";
        const scale = Ui.ShellSettings.get("fontScale", null);
        root.fontScale = scale !== null && isFinite(Number(scale)) ? Number(scale) : 1.0;
    }
    Connections {
        target: Ui.ShellSettings
        function onChanged() {
            const paths = Ui.ShellSettings.lastPaths;
            if (Ui.ShellSettings.touches("fontFamily", paths) || Ui.ShellSettings.touches("fontScale", paths))
                root.refreshFonts();
        }
    }
    Component.onCompleted: root.refreshFonts()

    // brand: the desktop's mark + name, user-overridable from Ryoku Settings ->
    // Shell -> Global. a small cross-cutting identity master (like theme.json).
    // markText is the glyph/short-text seal (default 力); markImage an optional
    // image path that wins over the text; markTint recolours a single-colour
    // image to the accent; name is the wordmark ("Ryoku") shown in chrome copy.
    // Ryoku's own apps (the Hub, ryo* apps) never read this and keep the 力 brand.
    property alias markText:  brandAdapter.markText
    property alias markImage: brandAdapter.markImage
    property alias markTint:  brandAdapter.markTint
    property alias brandName: brandAdapter.name

    // Current wallpaper path, so the workspace cells can render the real desktop
    // background behind their window previews. The daemon writes the absolute
    // path (+ newline) to ~/.local/state/ryoku-wallpaper on every change; watch
    // it and expose a file:// url (empty until it resolves).
    // The workspace cells render this as a still Image backdrop, so a live (video)
    // wallpaper has no frame to decode: feeding its path to Image just logs
    // "Unsupported image format" per cell. Treat a video (.mp4/.webm/.mkv/.mov) as
    // no backdrop -- the cell falls back to its flat fill -- matching the
    // Session.wallIsVideo extension check.
    property string wallpaper: {
        var t = wallFile.text().trim();
        if (t.length === 0) return "";
        var p = t.toLowerCase();
        if (p.endsWith(".mp4") || p.endsWith(".webm") || p.endsWith(".mkv") || p.endsWith(".mov")) return "";
        return "file://" + t;
    }
    FileView {
        id: wallFile
        path: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku-wallpaper"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
    }

    // The colour-source master (single source shared with the daemon, window
    // borders, and shell chrome): true = follow the wallpaper palette.
    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter { id: themeAdapter; property bool followWallpaper: true }
    }

    // brand identity master (mark + name), shared with doctor and the
    // Hub's Shell -> Global editor. read-only here; the always-on pill seeds it.
    FileView {
        id: brandFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter {
            id: brandAdapter
            property string markText: "力"
            property string markImage: ""
            property bool markTint: true
            property string name: "Ryoku"
        }
    }
}

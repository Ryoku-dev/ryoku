pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// Icons: the shell's icon resolver. The shell used to load every app and theme
// icon as an image://icon/<name> URL, which routed through Qt's icon engine
// (libqsvgicon under QIcon) and re-rasterised on a fractional-scale change --
// a path that raced the dock's icon warm-up and segfaulted the surface when a
// monitor's DPR arrived after the surfaces first mapped. This singleton loads a
// flat name -> absolute-path index once from `ryoku-shell icons` (icons.go, which
// walks the theme inheritance chain), and path() hands QML a plain file:// URL so
// the ordinary image plugin decodes the SVG/PNG, size-stable across DPR changes.
Singleton {
    id: root

    // The loaded index: { "<name>": "/abs/path", ... }. Empty until the first
    // `ryoku-shell icons` run lands.
    property var index: ({})
    // Bumped on every (re)load. A binding that calls path()/has() reads rev inside
    // the function, so it re-evaluates the moment a fresh index arrives.
    property int rev: 0

    // Resolve the binary the way the shell does elsewhere: a checkout runs it out
    // of RYOKU_SHELL_DIR/ipc, a deployed box finds it on PATH.
    readonly property string _shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string _bin: (root._shellDir && root._shellDir.length > 0)
        ? root._shellDir + "/ipc/ryoku-shell"
        : "ryoku-shell"

    // path(name, fallback): Quickshell.iconPath's semantics, resolved against the
    // index instead of Qt's icon engine.
    //   - name an absolute path or file:// URL -> returned as a file:// URL
    //   - image://icon/x (a tray/plugin-supplied value) -> resolve x
    //   - found in the index -> "file://" + path
    //   - not found and fallback === true -> ""
    //   - fallback a string -> resolve that name the same way ("" if empty/unresolved)
    function path(name, fallback) {
        void root.rev; // re-resolve when the index (re)loads
        return root._resolve(name, fallback);
    }

    function _resolve(name, fallback) {
        var n = (name === undefined || name === null) ? "" : String(name);
        if (n.length > 0) {
            if (n.indexOf("file://") === 0)
                return n;
            if (n.charAt(0) === "/")
                return "file://" + n;
            if (n.indexOf("image://icon/") === 0)
                n = n.substring(13); // "image://icon/".length
            var hit = root.index[n];
            if (hit)
                return "file://" + hit;
        }
        if (fallback === true)
            return "";
        if (typeof fallback === "string" && fallback.length > 0)
            return root._resolve(fallback, true);
        return "";
    }

    // has(name): true when the name resolves to a real file (an absolute path, a
    // file:// URL, or an indexed name). Mirrors Quickshell.hasThemeIcon.
    function has(name) {
        void root.rev;
        var n = (name === undefined || name === null) ? "" : String(name);
        if (n.length === 0)
            return false;
        if (n.charAt(0) === "/" || n.indexOf("file://") === 0)
            return true;
        if (n.indexOf("image://icon/") === 0)
            n = n.substring(13);
        var hit = root.index[n];
        return hit !== undefined && hit !== "";
    }

    // appIcon(appId): the icon for a window, resolved from the id its compositor
    // reports. That id is often not an icon name (a startup class, a flatpak
    // reverse-domain id, a Steam or Wine identifier), so each step asks a shape
    // the theme might name, most reliable first.
    function appIcon(appId) {
        void root.rev;
        var n = (appId === undefined || appId === null) ? "" : String(appId).trim();
        if (n.length === 0)
            return "";
        var candidates = [];
        var entry = DesktopEntries.heuristicLookup(n);
        if (entry && entry.icon)
            candidates.push(entry.icon);
        candidates.push(n);
        if (/^steam_app_(\d+)$/.test(n))
            candidates.push(n.replace(/^steam_app_/, "steam_icon_"));
        var stem = n.replace(/\.(desktop|exe)$/i, "");
        candidates.push(stem);
        candidates.push(stem.toLowerCase());
        candidates.push(stem.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase());
        candidates.push(stem.replace(/([a-z0-9])([A-Z])/g, "$1_$2").toLowerCase());
        candidates.push(stem.replace(/_/g, "-").toLowerCase());
        candidates.push(stem.replace(/-/g, "_").toLowerCase());
        for (var i = 0; i < candidates.length; ++i) {
            var hit = root.path(candidates[i], true);
            if (hit !== "")
                return hit;
        }
        return "";
    }

    // Rebuild the index. Coalesced: a run in flight is left to finish.
    function reload() {
        if (!loader.running)
            loader.running = true;
    }

    Process {
        id: loader
        command: [root._bin, "icons"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var obj = JSON.parse(this.text);
                    root.index = (obj && obj.icons) ? obj.icons : ({});
                } catch (e) {
                    // Keep the last good index on a parse/exec error.
                }
                root.rev++;
            }
        }
    }

    Component.onCompleted: root.reload()

    // A desktop-DB change (an app installed, updated, or removed) can add or fix
    // an icon; re-index, debounced so a burst of changes costs one rebuild.
    Timer {
        id: appsDebounce
        interval: 2000
        onTriggered: root.reload()
    }
    Connections {
        target: DesktopEntries
        function onApplicationsChanged() { appsDebounce.restart(); }
    }

    // The active icon theme is read from the GTK settings; re-index when it
    // changes so an icon-theme switch retunes the whole shell live.
    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/gtk-3.0/settings.ini"
        watchChanges: true
        printErrors: false
        onFileChanged: root.reload()
    }
}

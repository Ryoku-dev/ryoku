pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// Where Ryoku keeps things, resolved once for every surface that needs to know.
//
// The recordings directory used to be worked out in three places that disagreed:
// ryoku-cmd-record resolved it properly, the pill's deck hardcoded
// $HOME/Videos/Recordings, and the Hub's Recording page said so in prose. Set a
// custom XDG_VIDEOS_DIR and the writer and the list parted company.
//
// Resolution order, matching ryoku-cmd-record's recordings_dir():
// the RYOKU_SHELL_RECORDINGS_DIR env override, then the Hub's own setting in
// recording.json, then ${XDG_VIDEOS_DIR:-$HOME/Videos}/Recordings.
Singleton {
    id: paths

    readonly property string home: Quickshell.env("HOME") || ""

    // The Pictures folder. A localized desktop keeps it in ~/Imágenes, ~/Bilder
    // and so on, and the session exports that as XDG_PICTURES_DIR at login, so
    // anything that builds ~/Pictures by hand grows a second, empty folder.
    readonly property string picturesDir: {
        const xdg = String(Quickshell.env("XDG_PICTURES_DIR") || "");
        const clean = xdg.replace(/\/+$/, "");
        return (clean.length > 0 && clean !== paths.home) ? clean : paths.home + "/Pictures";
    }

    readonly property string videosDir: {
        const xdg = Quickshell.env("XDG_VIDEOS_DIR");
        return (xdg && xdg.length > 0) ? xdg : paths.home + "/Videos";
    }

    // The one directory every recording lands in and every list reads.
    readonly property string recordingsDir: {
        const env = Quickshell.env("RYOKU_SHELL_RECORDINGS_DIR");
        if (env && env.length > 0)
            return env;
        if (paths.configuredDir.length > 0)
            return paths.configuredDir;
        return paths.videosDir + "/Recordings";
    }

    // The Screen capture page's `directory` key; empty means "wherever the default
    // says", so a user who never touches it keeps following XDG_VIDEOS_DIR.
    property string configuredDir: ""

    function refresh() {
        try {
            const t = cfgFile.text();
            const o = t && t.length ? JSON.parse(t) : null;
            const d = o && typeof o.directory === "string" ? o.directory.trim() : "";
            paths.configuredDir = d;
        } catch (e) {
            paths.configuredDir = "";
        }
    }

    FileView {
        id: cfgFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (paths.home + "/.config")) + "/ryoku/recording.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: paths.refresh()
    }

    Component.onCompleted: refresh()
}

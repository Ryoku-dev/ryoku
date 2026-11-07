import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "ShellIndex.js" as ShellIndex

FileView {
    id: file

    property var index: ShellIndex.empty()
    readonly property string shellRoot: {
        const shellDir = String(Quickshell.env("RYOKU_SHELL_DIR") || "");
        if (shellDir.length > 0)
            return shellDir + "/quickshell/shell";
        return String(Quickshell.env("XDG_CONFIG_HOME")
            || (Quickshell.env("HOME") + "/.config")) + "/quickshell/shell";
    }

    function requestSync() {
        Spawn.run(["ryoku-shell", "shells", "sync"]);
    }

    path: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state"))
        + "/ryoku/store/shells.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: file.index = ShellIndex.parse(text())
    onLoadFailed: file.index = ShellIndex.empty()
}

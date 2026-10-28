//@ pragma DefaultEnv QSG_RENDER_LOOP = threaded

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

ShellRoot {
    id: root
    property var pins: []
    property bool framePainted: false

    readonly property var firstScreen: Quickshell.screens.length > 0 ? Quickshell.screens[0] : null
    readonly property string pinsDir: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku/pins"

    function parseLines(text) {
        var out = [];
        if (!text) return out;
        var lines = text.split("\n");
        for (var i = 0; i < lines.length; i++) {
            var p = lines[i].trim();
            if (p.length > 0) out.push(p);
        }
        return out;
    }

    /**
     * Reconciles the model against the current PNG list. Existing paths keep their
     * object identity so a live pin holds its dragged position and zoom, while new
     * paths become fresh slots. An empty directory means the last pin is gone, so
     * the host quits rather than idle holding the lock.
     */
    function rebuild(paths) {
        var byPath = {};
        for (var i = 0; i < pins.length; i++) byPath[pins[i].path] = pins[i];
        var next = [];
        for (var j = 0; j < paths.length; j++) {
            var p = paths[j];
            next.push(byPath[p] ? byPath[p] : { path: p, index: j });
        }
        pins = next;
        if (pins.length === 0) Qt.quit();
    }

    function relist() { lister.running = true; }

    /** Deletes one pin's PNG, drops it, and re-lists to reconcile the model. */
    function removePin(path) {
        Quickshell.execDetached(["rm", "-f", path]);
        var next = [];
        for (var i = 0; i < pins.length; i++)
            if (pins[i].path !== path) next.push(pins[i]);
        pins = next;
        if (pins.length === 0) Qt.quit();
        else relist();
    }

    Process {
        id: lister
        command: ["sh", "-c", "ls -1 \"$1\"/pin-*.png 2>/dev/null || true", "sh", root.pinsDir]
        stdout: StdioCollector { id: listOut }
        onExited: root.rebuild(root.parseLines(listOut.text))
    }

    // The pin PNGs are the whole state. .poke changes on every new pin, so a
    // watch on it plus a startup listing keeps the model current with no JSON.
    FileView {
        id: pokeFile
        path: root.pinsDir + "/.poke"
        watchChanges: true
        printErrors: false
        onLoaded: root.relist()
        onFileChanged: reload()
    }

    Component.onCompleted: {
        Quickshell.watchFiles = !Quickshell.shellDir.startsWith(
            (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/quickshell/");
        root.relist();
    }

    Variants {
        model: root.pins

        Pin {
            required property var modelData
            entry: modelData
            screen: root.firstScreen
            onPainted: root.framePainted = true
            onClosed: (path) => root.removePin(path)
        }
    }

    Timer {
        id: watchdog
        interval: 15000
        running: true
        onTriggered: {
            if (root.framePainted) return;
            console.error("ryopin: no frame rendered 15s after launch, graphics init failed, giving up");
            Quickshell.execDetached(["notify-send", "-a", "ryoku", I18n.tr("ryopin could not draw a pin"),
                I18n.tr("graphics init failed, likely GPU memory pressure")]);
            quitFallback.start();
            Qt.quit();
        }
    }

    Timer {
        id: quitFallback
        interval: 3000
        onTriggered: Qt.quit()
    }
}

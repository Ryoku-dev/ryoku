//@ pragma DefaultEnv QSG_RENDER_LOOP = threaded

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/pinlayout.js" as PinLayout

ShellRoot {
    id: root
    property var pins: []
    property bool framePainted: false
    property bool hidden: false
    property string focusedPin: ""
    property string tracingPath: ""
    signal blinkRequested(string path)
    signal exitTracingRequested(string path)

    function currentTracingPath() {
        for (var i = pins.length - 1; i >= 0; --i)
            if (pins[i].tracing && pins[i].path === tracingPath) return tracingPath;
        for (var j = pins.length - 1; j >= 0; --j)
            if (pins[j].tracing) return pins[j].path;
        return "";
    }
    property bool relistPending: false
    readonly property string pinsDir: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku/pins"

    function outputFor(entry) {
        return PinLayout.outputFor(entry.placement, Quickshell.screens, entry.fallbackOutput);
    }
    function pinsFor(output) {
        return pins.filter(function(entry) { return root.outputFor(entry) === output; });
    }
    function focusOne(path) { focusedPin = focusedPin === path ? "" : path; }
    function restore() { hidden = false; focusedPin = ""; }
    function rebuild(text) {
        var byPath = {};
        for (var i = 0; i < pins.length; ++i) byPath[pins[i].path] = pins[i];
        var next = [];
        var lines = text.trim().split("\n");
        for (var j = 0; j < lines.length; ++j) {
            if (!lines[j]) continue;
            try {
                var row = JSON.parse(lines[j]);
                if (!row.path) continue;
                next.push(byPath[row.path] || {
                    path: row.path, placement: row.placement || {}, index: next.length,
                    fallbackOutput: Wm.focusedOutput, geometry: null,
                    alpha: 1, tracing: false, blinked: false
                });
            } catch (error) { console.warn("ryopin: invalid pin record", error); }
        }
        pins = next;
        if (focusedPin && !next.some(function(entry) { return entry.path === root.focusedPin; })) focusedPin = "";
        if (!pins.length) Qt.quit();
    }
    function relist() {
        if (lister.running) relistPending = true;
        else lister.running = true;
    }
    function removePin(path) {
        Quickshell.execDetached(["rm", "-f", "--", path, path.replace(/\.png$/, ".json")]);
        pins = pins.filter(function(entry) { return entry.path !== path; });
        if (focusedPin === path) focusedPin = "";
        if (!pins.length) Qt.quit();
    }

    IpcHandler {
        target: "ryopin"
        function blink(): void { root.blinkRequested(root.currentTracingPath()); }
        function exitTracing(): void { root.exitTracingRequested(root.currentTracingPath()); }
    }
    Process {
        id: lister
        command: ["sh", "-c", "for p in \"$1\"/pin-*.png; do [ -f \"$p\" ] || continue; jq -c --arg path \"$p\" '{path:$path,placement:.}' \"${p%.png}.json\" 2>/dev/null || jq -cn --arg path \"$p\" '{path:$path}'; done", "sh", root.pinsDir]
        stdout: StdioCollector { id: listOut }
        onExited: {
            root.rebuild(listOut.text);
            if (root.relistPending) {
                root.relistPending = false;
                root.relist();
            }
        }
    }
    FileView {
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
        model: Quickshell.screens
        PinOutput {
            required property var modelData
            screen: modelData
            controller: root
            entries: root.pinsFor(modelData.name)
            onPainted: root.framePainted = true
        }
    }
    Timer {
        interval: 15000
        running: !root.framePainted
        onTriggered: {
            console.error("ryopin: no frame rendered 15s after launch");
            Quickshell.execDetached(["notify-send", "-a", "ryoku", I18n.tr("ryopin could not draw a pin"),
                I18n.tr("graphics init failed, likely GPU memory pressure")]);
            Qt.quit();
        }
    }
}

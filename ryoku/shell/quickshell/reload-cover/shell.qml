pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

ShellRoot {
    id: root
    Component.onCompleted: Quickshell.watchFiles = !Quickshell.shellDir.startsWith(
        (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/quickshell/")

    readonly property string token: Quickshell.env("RYOKU_RELOAD_COVER_TOKEN")
    readonly property string mode: Quickshell.env("RYOKU_RELOAD_COVER_MODE")
    // Login arrives behind the greeter's closed curtain. Map boot covers closed
    // too; replaying the reload close would expose the half-built desktop.
    property string phase: mode === "boot" ? "hold" : "closing"
    property bool startClose: false
    property bool finishQueued: false
    property int mappedCount: 0
    property var mappedOutputs: ({})
    readonly property string brandPath: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/brand.json"
    readonly property var reloadCover: brandAdapter.reloadCover || ({ path: "", name: "", kind: "default", bytes: 0 })

    FileView {
        path: root.brandPath
        blockLoading: true
        printErrors: false
        JsonAdapter {
            id: brandAdapter
            property var reloadCover: ({ path: "", name: "", kind: "default", bytes: 0 })
        }
    }


    function mapped(name: string): void {
        if (!mappedOutputs[name]) {
            mappedOutputs[name] = true;
            mappedCount += 1;
        }
        if (mappedCount >= Quickshell.screens.length && phase === "closing")
            startClose = true;
    }
    function finish(value: string): bool {
        if (value !== token || phase === "opening" || phase === "failed")
            return false;
        if (phase === "hold")
            phase = "opening";
        else
            finishQueued = true;
        return true;
    }
    function fail(value: string): bool {
        if (value !== token || phase === "opening")
            return false;
        phase = "failed";
        return true;
    }

    Timer {
        interval: 520
        running: root.startClose && root.phase === "closing"
        onTriggered: {
            root.phase = "hold";
            if (root.finishQueued)
                root.phase = "opening";
        }
    }
    // The hold deadline: a reload is a supervised daemon restart, so 16.5 s
    // is enough; a boot waits on the whole session chain (unit, daemon,
    // wallpaper, every surface), so it gets a much longer hold before the
    // cover gives up and releases the screen to a bare desktop.
    Timer {
        interval: root.mode === "boot" ? 45000 : 16500
        running: root.phase !== "opening" && root.phase !== "failed"
        onTriggered: root.phase = "failed"
    }
    Timer {
        interval: 1800
        running: root.phase === "failed"
        onTriggered: root.phase = "opening"
    }
    Timer {
        interval: 560
        running: root.phase === "opening"
        onTriggered: Qt.quit()
    }

    IpcHandler {
        target: "reload-cover"
        function mapped(value: string): bool {
            return value === root.token && root.mappedCount >= Quickshell.screens.length;
        }
        function covered(value: string): bool { return value === root.token && root.phase === "hold"; }
        function status(value: string): string { return value === root.token ? root.phase : ""; }
        function finish(value: string): bool { return root.finish(value); }
        function fail(value: string): bool { return root.fail(value); }
    }

    Variants {
        model: Quickshell.screens
        delegate: Component {
            ReloadCover {
                required property var modelData
                targetScreen: modelData
                mode: root.mode
                phase: root.phase
                startClose: root.startClose
                reloadCover: root.reloadCover
                onMapped: root.mapped(targetScreen.name)
            }
        }
    }
}

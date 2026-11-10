pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property bool started: false
    property bool collectionReady: false
    readonly property bool refreshing: collector.running
    signal collected(int exitCode)

    readonly property string collectorPath: decodeURIComponent(
        String(Qt.resolvedUrl("../bin/collect-usage")).replace(/^file:\/\//, ""))
    readonly property var collectorCommand: {
        const custom = String(Quickshell.env("RYOKU_AI_USAGE_COLLECTOR") || "")
        return custom.length > 0 ? [root.collectorPath, custom] : [root.collectorPath]
    }

    function refresh(): void {
        root.started = true
        if (collector.running) return
        retryTimer.stop()
        collector.running = true
        pollTimer.restart()
    }

    Process {
        id: collector
        command: root.collectorCommand
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 0) root.collectionReady = true
            if (exitCode === 75) retryTimer.restart()
            root.collected(exitCode)
        }
    }

    // A singleton survives destruction of the Island page that first used it.
    Timer {
        id: pollTimer
        interval: 15000
        repeat: true
        running: root.started
        onTriggered: root.refresh()
    }
    Timer {
        id: retryTimer
        interval: 2000
        onTriggered: root.refresh()
    }
}

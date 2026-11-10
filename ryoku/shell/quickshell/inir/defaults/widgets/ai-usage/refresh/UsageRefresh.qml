pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property bool collectionReady: false
    property int watchers: 0
    readonly property bool refreshing: collector.running
    signal collected(int exitCode)

    readonly property string collectorPath: decodeURIComponent(
        String(Qt.resolvedUrl("../bin/collect-usage")).replace(/^file:\/\//, ""))
    readonly property var collectorCommand: {
        const custom = String(Quickshell.env("RYOKU_AI_USAGE_COLLECTOR") || "")
        return custom.length > 0 ? [root.collectorPath, custom] : [root.collectorPath]
    }

    function watch(on: bool): void {
        root.watchers = Math.max(0, root.watchers + (on ? 1 : -1))
        if (root.watchers === 0) {
            pollTimer.stop()
            retryTimer.stop()
        } else if (!pollTimer.running) {
            pollTimer.start()
        }
    }

    function refresh(): void {
        if (collector.running) return
        retryTimer.stop()
        collector.running = true
        if (root.watchers > 0) pollTimer.restart()
    }

    Process {
        id: collector
        command: root.collectorCommand
        onExited: (exitCode, exitStatus) => {
            if (exitCode === 0) root.collectionReady = true
            if (exitCode === 75 && root.watchers > 0) retryTimer.restart()
            root.collected(exitCode)
        }
    }

    // collectors rescan whole agent histories, so nothing polls while no card is on screen
    Timer {
        id: pollTimer
        interval: 15000
        repeat: true
        onTriggered: root.refresh()
    }
    Timer {
        id: retryTimer
        interval: 2000
        onTriggered: root.refresh()
    }
}

pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Blobs
import "../utils/menupoll.js" as MenuPoll

Singleton {
    id: root

    property var owners: []
    readonly property bool active: root.owners.length > 0
    function setActive(owner, on) {
        root.owners = MenuPoll.setOwnership(root.owners, owner, on);
    }

    property bool _statsActive: false
    function _setStatsActive(on) {
        root._statsActive = on;
    }

    readonly property alias monitor: monitor
    property real cpu: monitor.cpuPercent / 100
    property real mem: monitor.memoryPercent / 100
    property real memUsedGiB: monitor.memoryUsedGiB
    property real memTotalGiB: monitor.memoryTotalGiB
    property real tempC: monitor.cpuTemp
    readonly property bool hasTemp: monitor.cpuTempAvailable
    property var cpuHistory: monitor.cpuHistory

    SystemMonitor {
        id: monitor
        samplePeriodMs: 1500
        historyLimit: 48
        includeVirtualNetwork: true
        // GPU usage comes from the daemon's stats feed, so the shell needn't load NVIDIA's libraries.
        gpu: false
        // Every reading here feeds something on screen, so nothing samples
        // while the session is locked or the displays are off.
        active: (root.active || root._statsActive) && !Presence.dark
    }
}

pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Blobs
import "../utils/menupoll.js" as MenuPoll

Singleton {
    id: root

    property var activeOwners: []
    readonly property bool active: activeOwners.length > 0
    readonly property bool analysing: root.active && !Perf.pillFrozen
    readonly property int bars: 40
    readonly property int fps: 30
    readonly property list<real> levels: feed.levels
    readonly property real energy: feed.energy
    readonly property bool silent: feed.silent

    function setActive(owner, enabled) {
        activeOwners = MenuPoll.setOwnership(activeOwners, owner, enabled);
    }

    CavaFeed {
        id: feed
        active: root.analysing
        bars: root.bars
        framerate: root.fps
        method: "pipewire"
        source: Audio.musicSink !== "" ? Audio.musicSink + ".monitor" : "auto"
        autosens: true
        sensitivity: 150
        noiseReduction: 25
        peakFollow: true
        staleMs: 120
    }
}

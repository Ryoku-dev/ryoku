pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Blobs
import shell.services
import "." as VizCfg

Singleton {
    id: root

    readonly property bool active: VizCfg.Config.enabled && Perf.visualizerResident
    readonly property int bars: VizCfg.Config.maxBars
    readonly property int fps: VizCfg.Config.fps
    property int placementHolds: 0

    readonly property bool analysing: root.active
        && (!Perf.visualizerFrozen || root.placementHolds > 0)
    readonly property list<real> levels: feed.levels
    readonly property real energy: feed.energy
    readonly property bool silent: feed.silent
    readonly property bool running: feed.running

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
        staleMs: 260
    }
}

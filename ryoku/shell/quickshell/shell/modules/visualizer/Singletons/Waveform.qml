pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Blobs
import shell.services
import "." as VizCfg

Singleton {
    id: root

    readonly property bool active: VizCfg.Config.enabled && Perf.visualizerResident
        && VizCfg.Config.anyLine && (!Perf.visualizerFrozen || VizCfg.Spectrum.placementHolds > 0)
    readonly property list<real> samples: capture.samples
    readonly property bool running: capture.running
    readonly property bool silent: capture.silent

    WaveCapture {
        id: capture
        source: Audio.musicSink
        active: root.active
    }
}

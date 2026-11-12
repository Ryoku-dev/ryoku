pragma Singleton
import Quickshell
import shell.services

// Thin forwarder to the shared performance policy. The desktop-widget layer reads
// the derived shadow and blur switches to drop its per-tile drop shadows and the
// frosted-glass blur pass, and reduceMotion to collapse the drag-guide animations
// to an instant cut. The policy itself -- the panels' glass and shadows and
// Animations' reduce motion in shell.json, folded with low power, the power
// profile and Game Mode -- lives once in shell.services.Perf, so there is no
// second file watcher here.
Singleton {
    readonly property bool lowPower: Perf.lowPower
    readonly property bool shadowsDisabled: Perf.shadowsDisabled
    readonly property bool reduceMotion: Perf.reduceMotion
    readonly property bool blurDisabled: Perf.blurDisabled
}

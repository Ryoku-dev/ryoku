pragma Singleton
import Quickshell
import shell.services

// Thin forwarder to the shared performance policy. The visualiser reads the
// derived freeze/blur switches; the policy (performance.json folded with the
// active power profile and battery) lives once in shell.services.Perf, so Power
// Saver (or lowPowerMode) freezes the idle wave and drops the bloom without a
// second performance.json watcher here.
Singleton {
    readonly property bool lowPower: Perf.lowPower
    readonly property bool visualizerFrozen: Perf.visualizerFrozen
    readonly property bool visualizerHardFrozen: Perf.visualizerHardFrozen
    readonly property bool visualizerResident: Perf.visualizerResident
    readonly property bool blurDisabled: Perf.blurDisabled
}

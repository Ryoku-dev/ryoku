pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Laptop battery state from the daemon's "battery" topic (UPower read in Go).
// A desktop without a battery reports present=false so the rail omits the
// display-only status chip. The wire carries raw device facts; every label,
// bucket, and signed rate is derived here, exactly as when the read lived in
// this file. The physical-battery pick (the synthetic display device drops
// health and presence on a limited cell) now lives daemon-side.
Singleton {
    id: root

    // UPower device states, mirrored from the spec for the derivations.
    readonly property int stateUnknown: 0
    readonly property int stateCharging: 1
    readonly property int stateDischarging: 2
    readonly property int stateFullyCharged: 4

    property bool present: false
    // True once the daemon's first battery frame has arrived: the readout is
    // loaded, so alert logic can trust the percentage. The QML UPower wrapper
    // carried the same gate as batDev.ready.
    property bool ready: false
    property real pctRaw: 0
    property int state: stateUnknown
    property bool onAcRaw: true
    property real energyRate: 0
    property real energyFull: 0
    property bool healthSupported: false
    property real healthRaw: 0
    property real timeToEmpty: 0
    property real timeToFull: 0

    readonly property real frac: Math.max(0, Math.min(1, pctRaw / 100))
    readonly property int pct: Math.round(frac * 100)

    readonly property bool charging: state === stateCharging
    readonly property bool full: state === stateFullyCharged || pct >= 100
    readonly property bool discharging: state === stateDischarging
    readonly property bool low: !charging && pct <= 20

    // Real line-power state. UPower's global OnBattery tracks the mains
    // adapter, not the cell, so a full battery idling on AC still reads as
    // on-AC and pulling the plug flips immediately. This is the signal for
    // "may I enter a high-performance mode", which charging/discharging
    // cannot answer on a machine that holds the cell at a charge limit.
    // No battery reads as AC.
    readonly property bool onAc: onAcRaw

    readonly property real rateW: !present ? 0
        : (discharging ? -energyRate : (charging ? energyRate : 0))
    readonly property real capacityWh: present ? energyFull : 0

    readonly property int health: healthSupported ? Math.round(healthRaw) : 0

    readonly property bool hasTime: charging ? timeToFull > 0
        : (discharging ? timeToEmpty > 0 : false)
    readonly property string timeStr: charging ? fmt(timeToFull)
        : (discharging ? fmt(timeToEmpty) : "")

    readonly property string stateLabel: charging ? I18n.tr("Charging")
        : (full ? I18n.tr("On AC · Full")
        : (discharging ? I18n.tr("Discharging") : I18n.tr("On AC")))

    function fmt(sec) {
        var s = Math.max(0, Math.round(sec));
        var h = Math.floor(s / 3600);
        var m = Math.floor((s % 3600) / 60);
        if (h > 0)
            return I18n.tr("%1h %2m").arg(h).arg(m);
        return I18n.tr("%1m").arg(m);
    }

    function apply(frame) {
        if (!frame)
            return;
        root.ready = true;
        root.present = frame.present === true;
        if (typeof frame.pct === "number")
            root.pctRaw = frame.pct;
        if (typeof frame.state === "number")
            root.state = frame.state;
        root.onAcRaw = frame.on_ac === true;
        if (typeof frame.energy_rate === "number")
            root.energyRate = frame.energy_rate;
        if (typeof frame.energy_full === "number")
            root.energyFull = frame.energy_full;
        root.healthSupported = frame.health_supported === true;
        if (typeof frame.health === "number")
            root.healthRaw = frame.health;
        if (typeof frame.time_to_empty === "number")
            root.timeToEmpty = frame.time_to_empty;
        if (typeof frame.time_to_full === "number")
            root.timeToFull = frame.time_to_full;
    }

    BusTopic {
        id: topicFeed
        topic: "battery"
        onUpdated: root.apply(topicFeed.data)
    }
}

pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Services.UPower
import Ryoku.Ui.Singletons

// Laptop battery state from UPower's display device. A desktop without a
// battery reports present=false so the rail omits the display-only status chip.
// The fuller state remains available to the preserved sidebar logic.
Singleton {
    id: root

    readonly property var dev: UPower.displayDevice

    // The synthetic displayDevice omits Capacity (health); pull it from the real
    // physical battery in the device list instead.
    readonly property var batDev: {
        var list = UPower.devices ? UPower.devices.values : [];
        for (var i = 0; i < list.length; i++) {
            if (list[i] && list[i].isLaptopBattery && list[i].isPresent)
                return list[i];
        }
        return dev;
    }

    // Level and charge state follow the aggregate, not one pack: on a
    // two-battery machine the first physical battery can sit parked by its
    // controller ("Not charging" at 99 %) for hours while the other pack
    // carries the load, and binding to it froze the whole readout. The
    // synthetic device has its own failure mode (some upower versions drop
    // ready/isLaptopBattery once a cell sits fully charged on AC), so an
    // unusable aggregate falls back to the active physical pack, then to the
    // first-battery pick that presence and health still use.
    readonly property var levelDev: {
        if (dev && dev.ready !== false
                && dev.state !== UPowerDeviceState.Unknown)
            return dev;
        var fallback = null;
        var list = UPower.devices ? UPower.devices.values : [];
        for (var i = 0; i < list.length; i++) {
            const d = list[i];
            if (!d || !d.isLaptopBattery || !d.isPresent) continue;
            if (d.state === UPowerDeviceState.Discharging
                    || d.state === UPowerDeviceState.Charging)
                return d;
            if (fallback === null) fallback = d;
        }
        return fallback ?? batDev;
    }

    // presence comes from the physical battery pick: the synthetic display
    // device drops ready/isLaptopBattery on some upower versions once the
    // cell sits fully charged on AC, which blanked every battery readout.
    readonly property bool present: batDev !== null && batDev.isLaptopBattery && batDev.isPresent
    readonly property real frac: levelDev ? Math.max(0, Math.min(1, levelDev.percentage)) : 0
    readonly property int pct: Math.round(frac * 100)
    readonly property int state: levelDev ? levelDev.state : UPowerDeviceState.Unknown

    readonly property bool charging: state === UPowerDeviceState.Charging
    readonly property bool full: state === UPowerDeviceState.FullyCharged || pct >= 100
    readonly property bool discharging: state === UPowerDeviceState.Discharging
    readonly property bool low: !charging && pct <= 20

    // Real line-power state. UPower's global OnBattery tracks the mains adapter,
    // not the cell, so a full battery idling on AC still reads as on-AC and
    // pulling the plug flips immediately. This is the signal for "may I enter a
    // high-performance mode", which charging/discharging cannot answer on a
    // machine that holds the cell at a charge limit. No battery reads as AC.
    readonly property bool onAc: !UPower.onBattery

    readonly property real rateW: !levelDev ? 0
        : (discharging ? -levelDev.changeRate : (charging ? levelDev.changeRate : 0))
    readonly property real capacityWh: levelDev ? levelDev.energyCapacity : 0

    readonly property bool healthSupported: batDev ? batDev.healthSupported : false
    readonly property int health: batDev ? Math.round(batDev.healthPercentage) : 0

    readonly property bool hasTime: !levelDev ? false
        : (charging ? levelDev.timeToFull > 0 : (discharging ? levelDev.timeToEmpty > 0 : false))
    readonly property string timeStr: !levelDev ? ""
        : (charging ? fmt(levelDev.timeToFull) : (discharging ? fmt(levelDev.timeToEmpty) : ""))

    // "On AC" is a claim about the adapter, not the pack: a machine can sit
    // fully charged on battery power (both packs parked by their controllers),
    // so only onAc earns the prefix.
    readonly property string stateLabel: charging ? I18n.tr("Charging")
        : (full ? (onAc ? I18n.tr("On AC · Full") : I18n.tr("Full"))
        : (discharging ? I18n.tr("Discharging")
        : (onAc ? I18n.tr("On AC") : I18n.tr("Not charging"))))

    function fmt(sec) {
        var s = Math.max(0, Math.round(sec));
        var h = Math.floor(s / 3600);
        var m = Math.floor((s % 3600) / 60);
        if (h > 0)
            return I18n.tr("%1h %2m").arg(h).arg(m);
        return I18n.tr("%1m").arg(m);
    }
}

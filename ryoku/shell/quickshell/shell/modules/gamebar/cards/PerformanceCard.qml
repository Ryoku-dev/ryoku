pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import shell.services
import ".."
import Ryoku.Ui.Singletons
import "../look"
import "../../../utils/units.js" as Units

// Performance: the machine's headroom while a game runs, read from the one
// sysinfo sampler. The rate here is the display's compositor mode, never an
// in-game frame counter; Ryoku does not read one, so it does not claim one.
GameBarCard {
    id: card

    property string monitorName: ""
    property string displayMode: ""

    title: I18n.tr("Performance")

    readonly property var mon: Sysinfo.monitor
    readonly property int hz: Units.hz(card.displayMode)
    readonly property string resolution: {
        const base = card.displayMode.split("@")[0];
        if (base.length > 0)
            return base.replace("x", " x ");
        const out = Wm.outputByName(card.monitorName);
        return out ? out.width + " x " + out.height : "n/a";
    }

    function readMode(clear) {
        if (clear)
            card.displayMode = "";
        if (modeRead.running)
            modeRead.running = false;
        modeRestart.restart();
    }

    Component.onCompleted: {
        Sysinfo.setActive(card, true);
        StatsFeed.setActive(card, true);
        card.readMode(true);
    }
    onMonitorNameChanged: card.readMode(true)
    Component.onDestruction: {
        Sysinfo.setActive(card, false);
        StatsFeed.setActive(card, false);
    }

    // A game can switch resolution or refresh while the card stays up. Watch
    // frames carry no mode, so re-read the full state now and then; the read
    // takes a few milliseconds. Skip it while the surface is unmapped (a shot or
    // a recording) or the session is dark, and in the Power Saver tier, where a
    // pinned card is not worth waking anything for.
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !Perf.saver

    Timer {
        interval: 10000
        repeat: true
        running: card.polling
        onRunningChanged: if (running) card.readMode(false)
        onTriggered: card.readMode(false)
    }

    // restarting the process in the same tick may coalesce into no new run, so
    // let the property settle first.
    Timer {
        id: modeRestart
        interval: 1
        onTriggered: if (!modeRead.running) modeRead.running = true
    }

    // The watch frames carry no mode, so the full state read is the one place
    // the current mode is reported. The Timer above refreshes it while the card
    // is up, so a mode switch by the game does not leave a stale reading.
    Process {
        id: modeRead
        command: ["ryoku", "wm", "state"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const doc = JSON.parse(text);
                    const list = doc.outputs || [];
                    for (let i = 0; i < list.length; i++) {
                        if (list[i].name === card.monitorName && list[i].mode) {
                            card.displayMode = String(list[i].mode);
                            return;
                        }
                    }
                } catch (e) {
                    card.displayMode = "";
                }
            }
        }
    }

    Column {
        width: parent.width
        spacing: 3 * card.s

        GameBarDataRow {
            s: card.s
            width: parent.width
            label: I18n.tr("Display")
            value: card.resolution
        }
        GameBarDataRow {
            s: card.s
            width: parent.width
            label: I18n.tr("Refresh")
            value: card.hz > 0 ? card.hz + " Hz" : "n/a"
        }
        GameBarDataRow {
            s: card.s
            width: parent.width
            label: I18n.tr("Temperature")
            value: card.mon.gpuTempAvailable ? Math.round(card.mon.gpuTemp) + " C"
                : (card.mon.cpuTempAvailable ? Math.round(card.mon.cpuTemp) + " C" : "n/a")
        }

        GameBarMeter {
            s: card.s
            width: parent.width
            label: I18n.tr("CPU")
            value: Units.pct(card.mon.cpuPercent / 100)
            fraction: card.mon.cpuPercent / 100
        }
        GameBarMeter {
            s: card.s
            width: parent.width
            label: I18n.tr("GPU")
            value: card.mon.gpuAvailable ? Units.pct(card.mon.gpuPercent / 100) : "n/a"
            fraction: card.mon.gpuAvailable ? card.mon.gpuPercent / 100 : 0
            tint: Theme.tertiary
        }
        GameBarMeter {
            s: card.s
            width: parent.width
            label: I18n.tr("Memory")
            value: Units.gib(card.mon.memoryUsedGiB) + " / " + Units.gib(card.mon.memoryTotalGiB)
            fraction: card.mon.memoryPercent / 100
        }
        GameBarMeter {
            s: card.s
            width: parent.width
            label: I18n.tr("VRAM")
            value: card.mon.gpuMemoryAvailable
                ? Units.gib(card.mon.gpuMemoryUsedGiB) + " / " + Units.gib(card.mon.gpuMemoryTotalGiB)
                : "n/a"
            fraction: card.mon.gpuMemoryAvailable && card.mon.gpuMemoryTotalGiB > 0
                ? card.mon.gpuMemoryUsedGiB / card.mon.gpuMemoryTotalGiB : 0
            tint: Theme.tertiary
        }

        GameBarSpark {
            width: parent.width
            height: 34 * card.s
            values: card.mon.cpuHistory
            color: GameBarLook.accent
        }

        Text {
            width: parent.width
            text: I18n.tr("Display refresh, not a game's frame rate.")
            color: GameBarLook.subtext
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.smallie * card.s
        }
    }
}

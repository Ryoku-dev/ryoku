pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import shell.services
import ".."
import Ryoku.Ui.Singletons
import Lithic
import "../look"
import "../../../utils/units.js" as Units

GameBarCard {
    id: card

    property string monitorName: ""
    property string displayMode: ""

    title: I18n.tr("Performance")

    readonly property var mon: Sysinfo.monitor
    readonly property int hz: Units.hz(card.displayMode)
    readonly property string tempUnit: Units.tempUnit(Config.weatherUnit, Qt.locale().name)
    readonly property string resolution: {
        const base = card.displayMode.split("@")[0];
        if (base.length > 0)
            return base.replace("x", " x ");
        if (card.monitorName.length === 0)
            return I18n.tr("n/a");
        const out = Wm.outputByName(card.monitorName);
        return out ? out.width + " x " + out.height : I18n.tr("n/a");
    }

    function readMode(clear) {
        if (clear)
            card.displayMode = "";
        if (modeRead.running)
            modeRead.running = false;
        if (card.polling)
            modeRestart.restart();
    }

    function syncFeed() {
        Sysinfo.setActive(card, card.polling);
        StatsFeed.setActive(card, card.polling);
    }

    Component.onCompleted: {
        card.syncFeed();
        card.readMode(true);
    }
    onMonitorNameChanged: card.readMode(true)
    Component.onDestruction: {
        Sysinfo.setActive(card, false);
        StatsFeed.setActive(card, false);
    }

    readonly property bool saverIdle: Perf.saver && card.pinned && !GameBarStore.open
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !card.saverIdle
    onPollingChanged: {
        card.syncFeed();
        if (!card.polling) {
            modeRestart.stop();
            modeRead.running = false;
        }
    }

    Timer {
        interval: 10000
        repeat: true
        running: card.polling
        onRunningChanged: if (running) card.readMode(false)
        onTriggered: card.readMode(false)
    }

    Timer {
        id: modeRestart
        interval: 1
        onTriggered: if (card.polling && !modeRead.running) modeRead.running = true
    }

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
        spacing: card.space2

        Grid {
            width: parent.width
            columns: 2
            columnSpacing: card.space3
            rowSpacing: card.space3

            Column {
                width: (parent.width - card.space3) / 2
                spacing: card.space1

                Label {
                    text: I18n.tr("Processor")
                    typeRole: "caption"
                    inkRole: "fg2"
                }
                Label {
                    text: Units.pct(card.mon.cpuPercent / 100)
                    typeRole: "row"
                    numeric: true
                }
                BladeMeter {
                    width: parent.width
                    value: card.mon.cpuPercent / 100
                }
            }

            Column {
                width: (parent.width - card.space3) / 2
                spacing: card.space1

                Label {
                    text: I18n.tr("Graphics")
                    typeRole: "caption"
                    inkRole: "fg2"
                }
                Label {
                    text: card.mon.gpuAvailable ? Units.pct(card.mon.gpuPercent / 100) : I18n.tr("n/a")
                    typeRole: "row"
                    numeric: true
                }
                BladeMeter {
                    width: parent.width
                    value: card.mon.gpuAvailable ? card.mon.gpuPercent / 100 : 0
                }
            }

            Column {
                width: (parent.width - card.space3) / 2
                spacing: card.space1

                Label {
                    text: I18n.tr("Memory")
                    typeRole: "caption"
                    inkRole: "fg2"
                }
                Label {
                    text: Units.gib(card.mon.memoryUsedGiB) + " / " + Units.gib(card.mon.memoryTotalGiB)
                    typeRole: "row"
                    numeric: true
                }
                BladeMeter {
                    width: parent.width
                    value: card.mon.memoryPercent / 100
                }
            }

            Column {
                width: (parent.width - card.space3) / 2
                spacing: card.space1

                Label {
                    text: I18n.tr("Video memory")
                    typeRole: "caption"
                    inkRole: "fg2"
                }
                Label {
                    text: card.mon.gpuMemoryAvailable
                        ? Units.gib(card.mon.gpuMemoryUsedGiB) + " / " + Units.gib(card.mon.gpuMemoryTotalGiB)
                        : I18n.tr("n/a")
                    typeRole: "row"
                    numeric: true
                }
                BladeMeter {
                    width: parent.width
                    value: card.mon.gpuMemoryAvailable && card.mon.gpuMemoryTotalGiB > 0
                        ? card.mon.gpuMemoryUsedGiB / card.mon.gpuMemoryTotalGiB : 0
                }
            }
        }

        EngravedChart {
            width: parent.width
            height: 44 * card.s
            values: card.mon.cpuHistory
            minimum: 0
            maximum: 1
            emptyText: ""
            Accessible.name: I18n.tr("Processor history")
        }

        GameBarDataRow {
            width: parent.width
            label: I18n.tr("Display")
            value: card.resolution
        }
        GameBarDataRow {
            width: parent.width
            label: I18n.tr("Refresh")
            value: card.hz > 0 ? card.hz + " Hz" : I18n.tr("n/a")
        }
        GameBarDataRow {
            width: parent.width
            label: I18n.tr("Temperature")
            value: card.mon.gpuTempAvailable ? Units.temp(card.mon.gpuTemp, card.tempUnit)
                : (card.mon.cpuTempAvailable ? Units.temp(card.mon.cpuTemp, card.tempUnit) : I18n.tr("n/a"))
        }

        Label {
            width: parent.width
            text: I18n.tr("Display refresh, not a game's frame rate.")
            typeRole: "caption"
            inkRole: "fg3"
        }
    }
}

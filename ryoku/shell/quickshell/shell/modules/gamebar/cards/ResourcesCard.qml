pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import ".."
import Ryoku.Ui.Singletons
import Lithic
import "../look"

GameBarCard {
    id: card

    title: I18n.tr("Resources")

    readonly property var mon: Sysinfo.monitor

    property var gpuHistory: []
    property var memHistory: []
    readonly property int tabIndex: Math.max(0, Math.min(2, GameBarStore.resourceTab))

    readonly property var tabs: [I18n.tr("Processor"), I18n.tr("Graphics"), I18n.tr("Memory")]

    readonly property real fraction: card.tabIndex === 0 ? card.mon.cpuPercent / 100
        : card.tabIndex === 1 ? (card.mon.gpuAvailable ? card.mon.gpuPercent / 100 : 0)
        : card.mon.memoryPercent / 100

    readonly property bool unavailable: card.tabIndex === 1 && !card.mon.gpuAvailable

    readonly property var history: card.tabIndex === 0 ? card.mon.cpuHistory
        : card.tabIndex === 1 ? card.gpuHistory : card.memHistory

    readonly property string total: card.tabIndex === 2
        ? I18n.tr("of %1 GiB").arg(Math.round(card.mon.memoryTotalGiB))
        : I18n.tr("of 100%")

    readonly property string reading: card.unavailable ? I18n.tr("n/a")
        : card.tabIndex === 2 ? Math.round(card.mon.memoryUsedGiB * 10) / 10 + " GiB"
        : Math.round(card.fraction * 100) + "%"

    function push(list, value) {
        const next = (list || []).slice();
        next.push(value);
        while (next.length > 48)
            next.shift();
        return next;
    }

    function syncFeed() {
        Sysinfo.setActive(card, card.polling);
        StatsFeed.setActive(card, card.polling);
    }

    Component.onCompleted: card.syncFeed()
    Component.onDestruction: {
        Sysinfo.setActive(card, false);
        StatsFeed.setActive(card, false);
    }

    readonly property bool saverIdle: Perf.saver && card.pinned && !GameBarStore.open
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !card.saverIdle
    onPollingChanged: card.syncFeed()

    Timer {
        interval: 1500
        running: card.polling
        repeat: true
        onTriggered: {
            card.gpuHistory = card.push(card.gpuHistory, card.mon.gpuPercent / 100);
            card.memHistory = card.push(card.memHistory, card.mon.memoryPercent / 100);
        }
    }

    Item {
        anchors.fill: parent

        Tabs {
            id: tabs
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: parent.top
            model: card.tabs
            currentIndex: card.tabIndex
            onActivated: index => GameBarStore.resourceTab = index
        }

        Item {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.top: tabs.bottom
            anchors.topMargin: card.space2
            anchors.bottom: parent.bottom

            Column {
                id: readingColumn
                width: 92 * card.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 2 * card.s

                Label {
                    text: card.reading
                    typeRole: "section"
                    numeric: true
                }
                Label {
                    text: card.unavailable ? I18n.tr("Reading unavailable") : card.total
                    typeRole: "detail"
                    inkRole: "fg3"
                    wrapMode: Text.WordWrap
                }
            }

            EngravedChart {
                anchors.left: readingColumn.right
                anchors.leftMargin: card.space4
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                values: card.history
                minimum: 0
                maximum: 1
                lineInk: GameBarTone.ink
                baselineInk: GameBarTone.line
                emptyText: I18n.tr("Gathering a reading")
                Accessible.name: card.tabs[card.tabIndex]
            }
        }
    }
}

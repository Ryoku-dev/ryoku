pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import shell.services
import ".."
import Ryoku.Ui.Singletons
import "../look"

// Resources, cut like end-4's overlay widget: a tab per resource, the reading
// in huge numerals over "of %1", and the graph on the secondary container
// filling the rest. The sampler has no history for GPU and memory, so the card
// keeps its own short window for the two tabs it has to draw itself.
GameBarCard {
    id: card

    title: I18n.tr("Resources")

    readonly property var mon: Sysinfo.monitor

    property var gpuHistory: []
    property var memHistory: []
    readonly property int tabIndex: Math.max(0, Math.min(2, GameBarStore.resourceTab))

    readonly property var tabs: [
        { label: I18n.tr("CPU"), icon: "cpu" },
        { label: I18n.tr("GPU"), icon: "bolt" },
        { label: I18n.tr("RAM"), icon: "archive" }
    ]

    readonly property real fraction: card.tabIndex === 0 ? card.mon.cpuPercent / 100
        : card.tabIndex === 1 ? (card.mon.gpuAvailable ? card.mon.gpuPercent / 100 : 0)
        : card.mon.memoryPercent / 100

    readonly property bool unavailable: card.tabIndex === 1 && !card.mon.gpuAvailable

    readonly property var history: card.tabIndex === 0 ? card.mon.cpuHistory
        : card.tabIndex === 1 ? card.gpuHistory : card.memHistory

    readonly property string total: card.tabIndex === 0 ? "100%"
        : card.tabIndex === 1 ? "100%"
        : Math.round(card.mon.memoryTotalGiB) + " GiB"

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

    // The graphs only need history while someone can see them: a shot or a
    // recording unmaps the overlay, and the session can be dark or in the Power
    // Saver tier. Stop copying samples then, like the other polling cards, and
    // release the samplers themselves so a pinned card does not keep them alive.
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !Perf.saver
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

    ColumnLayout {
        anchors.fill: parent
        spacing: 8 * card.s

        GameBarTabs {
            Layout.fillWidth: true
            Layout.preferredHeight: 42 * card.s
            s: card.s
            model: card.tabs
            currentIndex: card.tabIndex
            onPicked: index => GameBarStore.resourceTab = index
        }

        RowLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 12 * card.s

            ColumnLayout {
                spacing: 2 * card.s

                Text {
                    text: card.unavailable ? "n/a"
                        : card.tabIndex === 2 ? Math.round(card.mon.memoryUsedGiB * 10) / 10 + " GiB"
                        : Math.round(card.fraction * 100) + "%"
                    color: GameBarLook.fg
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.huge * card.s
                }

                Text {
                    text: card.unavailable
                        ? I18n.tr("GPU reading unavailable")
                        : I18n.tr("of %1").arg(card.total)
                    color: GameBarLook.subtext
                    font.family: GameBarLook.face
                    font.pixelSize: GameBarLook.smallie * card.s
                }

                Item {
                    Layout.fillHeight: true
                }
            }

            Rectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                radius: GameBarLook.radiusSmall * card.s
                color: GameBarLook.toggled
                clip: true

                GameBarSpark {
                    anchors.fill: parent
                    values: card.history
                    color: GameBarLook.accent
                }
            }
        }
    }
}

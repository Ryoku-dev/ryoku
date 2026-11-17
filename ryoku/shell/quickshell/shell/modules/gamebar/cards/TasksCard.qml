pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import ".."
import Ryoku.Ui.Singletons
import Lithic
import "../look"
import "../../../utils/tasks.js" as Tasks

GameBarCard {
    id: card

    property var rows: []
    readonly property string user: Quickshell.env("USER") || Quickshell.env("LOGNAME") || ""

    title: I18n.tr("Tasks")

    function refresh() {
        if (card.polling && !scan.running)
            scan.running = true;
    }

    Component.onCompleted: card.refresh()

    readonly property bool saverIdle: Perf.saver && card.pinned && !GameBarStore.open
    readonly property bool polling: (!card.host || (card.host.live && !card.host.suppressed))
        && !Presence.dark && !card.saverIdle

    Timer {
        interval: 2000
        repeat: true
        running: card.polling
        onRunningChanged: if (running) card.refresh()
        onTriggered: card.refresh()
    }

    Process {
        id: scan
        command: ["python3", Quickshell.shellPath("modules/gamebar/list_tasks.py")]
        stdout: StdioCollector {
            onStreamFinished: card.rows = Tasks.top(text, 5, card.user, Date.now())
        }
    }

    Process {
        id: ender
        onExited: (code, status) => {
            if (code === 0)
                settle.restart();
        }
    }

    Timer {
        id: settle
        interval: 700
        onTriggered: card.refresh()
    }

    Column {
        width: parent.width
        spacing: 2 * card.s

        Item {
            width: parent.width
            height: 20 * card.s

            Label {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Process")
                typeRole: "group"
                inkRole: "fg3"
            }

            Label {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CPU") + "    " + I18n.tr("Memory")
                typeRole: "group"
                inkRole: "fg3"
            }
        }

        Repeater {
            model: card.rows

            delegate: GameBarTaskRow {
                required property var modelData
                width: parent.width
                task: modelData
                protectedName: Tasks.isProtected(modelData.name)
                onEndRequested: (pid, started) => {
                    if (pid <= 0 || ender.running)
                        return;
                    ender.command = ["python3",
                        Quickshell.shellPath("modules/gamebar/end_process.py"),
                        String(pid), String(started)];
                    ender.running = true;
                }
            }
        }

        Label {
            visible: card.rows.length === 0
            width: parent.width
            topPadding: card.space2
            text: I18n.tr("Reading processes")
            typeRole: "detail"
            inkRole: "fg3"
            horizontalAlignment: Text.AlignHCenter
        }
    }
}

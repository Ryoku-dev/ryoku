pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import shell.services
import ".."
import Ryoku.Ui.Singletons
import "../look"
import "../../../utils/tasks.js" as Tasks

// Tasks: the five busiest processes by average CPU, with a two-step End. The
// protected set is the desktop itself, so the list cannot offer to kill the
// compositor or the shell it floats above.
GameBarCard {
    id: card

    property var rows: []
    readonly property string user: Quickshell.env("USER") || Quickshell.env("LOGNAME") || ""

    title: I18n.tr("Tasks")

    function refresh() {
        if (!ps.running)
            ps.running = true;
    }

    Component.onCompleted: card.refresh()

    // A pinned card keeps the surface mapped while the strip is closed, but a
    // shot or a recording unmaps it and the session can be dark. Do not scan
    // processes the user cannot see. Power Saver rests only a card left pinned
    // behind a closed bar; an open bar is being looked at.
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
        id: ps
        command: ["ps", "-eo", "user:64=,pid=,comm=,cputimes=,etimes=,rss="]
        stdout: StdioCollector {
            onStreamFinished: card.rows = Tasks.top(text, 5, card.user, Date.now())
        }
    }

    // The list refreshes every two seconds, so a pid can already belong to a
    // new process by the time Stop is clicked. The helper opens a pidfd before
    // it checks the start time, so the signal cannot reach a process that took
    // the pid after the row was confirmed.
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
            height: 22 * card.s

            Text {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Process")
                color: GameBarLook.subtext
                font.family: GameBarLook.face
                font.pixelSize: GameBarLook.smallie * card.s
            }
            Text {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CPU    MEM")
                color: GameBarLook.subtext
                font.family: GameBarLook.face
                font.pixelSize: GameBarLook.smallie * card.s
            }
        }

        Repeater {
            model: card.rows

            delegate: GameBarTaskRow {
                required property var modelData
                s: card.s
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

        Text {
            visible: card.rows.length === 0
            width: parent.width
            topPadding: 10 * card.s
            text: I18n.tr("Reading processes")
            color: GameBarLook.subtext
            horizontalAlignment: Text.AlignHCenter
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.small * card.s
        }
    }
}

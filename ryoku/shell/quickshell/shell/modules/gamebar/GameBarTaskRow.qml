pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import "../../components"
import Ryoku.Ui.Singletons
import "look"
import "../../utils/units.js" as Units

// One process line. End arms on the first press and only fires on the second,
// so a stray click cannot SIGTERM anything; processes the desktop needs never
// offer the control at all.
Item {
    id: root

    property real s: 1
    property var task: null
    property bool protectedName: false

    signal endRequested(int pid, real started)

    property bool confirming: false
    property int confirmingPid: 0
    property real confirmingStart: 0

    implicitHeight: 32 * root.s
    implicitWidth: 276 * root.s

    // A pid can be reused once its process exits; the start time disarms the
    // button unless the same process is still the one being confirmed.
    onTaskChanged: {
        if (!root.confirming)
            return;
        if (!root.task || root.task.pid !== root.confirmingPid
            || Math.abs((root.task.started || 0) - root.confirmingStart) > 2)
            root.confirming = false;
    }

    Text {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(0, cpu.x - 12 * root.s)
        text: root.task ? root.task.name : ""
        color: GameBarLook.fg
        font.family: GameBarLook.face
        font.pixelSize: GameBarLook.small * root.s
        elide: Text.ElideRight
    }

    Text {
        id: cpu
        anchors.right: mem.left
        anchors.rightMargin: 10 * root.s
        anchors.verticalCenter: parent.verticalCenter
        width: 40 * root.s
        text: root.task ? Units.pct(root.task.cpu / 100) : ""
        color: GameBarLook.subtext
        font.family: GameBarLook.face
        font.pixelSize: GameBarLook.smallie * root.s
        horizontalAlignment: Text.AlignRight
    }

    Text {
        id: mem
        anchors.right: actions.left
        anchors.rightMargin: 10 * root.s
        anchors.verticalCenter: parent.verticalCenter
        width: 54 * root.s
        text: root.task ? Math.round(root.task.memMiB) + " MB" : ""
        color: GameBarLook.subtext
        font.family: GameBarLook.face
        font.pixelSize: GameBarLook.smallie * root.s
        horizontalAlignment: Text.AlignRight
    }

    Row {
        id: actions
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: 4 * root.s

        Text {
            visible: root.protectedName && !root.confirming
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("core")
            color: GameBarLook.subtext
            font.family: GameBarLook.face
            font.pixelSize: GameBarLook.smallie * root.s
        }

        GameBarTextButton {
            visible: !root.protectedName && !root.confirming
            s: root.s
            compact: true
            label: I18n.tr("End")
            onClicked: {
                root.confirmingPid = root.task ? root.task.pid : 0;
                root.confirmingStart = root.task ? root.task.started || 0 : 0;
                root.confirming = true;
            }
        }

        GameBarTextButton {
            visible: root.confirming
            s: root.s
            compact: true
            label: I18n.tr("Stop")
            danger: true
            onClicked: {
                if (root.confirmingPid > 0)
                    root.endRequested(root.confirmingPid, root.confirmingStart);
                root.confirming = false;
                root.confirmingPid = 0;
                root.confirmingStart = 0;
            }
        }

        GameBarTextButton {
            visible: root.confirming
            s: root.s
            compact: true
            quiet: true
            label: I18n.tr("No")
            onClicked: {
                root.confirming = false;
                root.confirmingPid = 0;
                root.confirmingStart = 0;
            }
        }
    }
}

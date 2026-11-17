pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import Ryoku.Ui.Singletons
import "../../components"
import Lithic
import "look"
import "../../utils/units.js" as Units

Item {
    id: root

    property var task: null
    property bool protectedName: false

    signal endRequested(int pid, real started)

    property bool confirming: false
    property int confirmingPid: 0
    property real confirmingStart: 0

    readonly property real _scale: Lithic.scaleOf(root)

    implicitHeight: 32 * root._scale
    implicitWidth: 276 * root._scale

    onTaskChanged: {
        if (!root.confirming)
            return;
        if (!root.task || root.task.pid !== root.confirmingPid
            || (root.task.started || 0) !== root.confirmingStart)
            root.confirming = false;
    }

    Label {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: Math.max(0, cpu.x - 8 * root._scale)
        text: root.task ? root.task.name : ""
        textFormat: Text.PlainText
        elide: Text.ElideRight
        typeRole: "detail"
        ink: GameBarTone.ink
    }

    Label {
        id: cpu
        anchors.right: mem.left
        anchors.rightMargin: 8 * root._scale
        anchors.verticalCenter: parent.verticalCenter
        width: 38 * root._scale
        text: root.task ? Units.pct(root.task.cpu / 100) : ""
        typeRole: "mono"
        inkRole: "fg2"
        numeric: true
        horizontalAlignment: Text.AlignRight
    }

    Label {
        id: mem
        anchors.right: actions.left
        anchors.rightMargin: 8 * root._scale
        anchors.verticalCenter: parent.verticalCenter
        width: 50 * root._scale
        text: root.task ? Math.round(root.task.memMiB) + " MB" : ""
        typeRole: "mono"
        inkRole: "fg2"
        numeric: true
        horizontalAlignment: Text.AlignRight
    }

    Row {
        id: actions
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: 4 * root._scale

        Item {
            width: coreLabel.implicitWidth
            height: coreLabel.implicitHeight
            visible: root.protectedName && !root.confirming

            Label {
                id: coreLabel
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("core")
                typeRole: "caption"
                inkRole: "fg3"
            }
        }

        Plaque {
            visible: !root.protectedName && !root.confirming
            small: true
            kind: "quiet"
            text: I18n.tr("End")
            onClicked: {
                root.confirmingPid = root.task ? root.task.pid : 0;
                root.confirmingStart = root.task ? root.task.started || 0 : 0;
                root.confirming = true;
            }
        }

        Plaque {
            visible: root.confirming
            small: true
            kind: "dangerLine"
            text: I18n.tr("Stop")
            onClicked: {
                if (root.confirmingPid > 0)
                    root.endRequested(root.confirmingPid, root.confirmingStart);
                root.confirming = false;
                root.confirmingPid = 0;
                root.confirmingStart = 0;
            }
        }

        Plaque {
            visible: root.confirming
            small: true
            kind: "quiet"
            text: I18n.tr("No")
            onClicked: {
                root.confirming = false;
                root.confirmingPid = 0;
                root.confirmingStart = 0;
            }
        }
    }
}

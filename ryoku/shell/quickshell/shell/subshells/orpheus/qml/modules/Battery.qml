import QtQuick
import shell.services
import "../kit" as K

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: Battery.present

    readonly property real bodyWidth: 22 * K.Style.scale
    readonly property real bodyHeight: 11 * K.Style.scale
    readonly property real terminalWidth: 2 * K.Style.scale
    readonly property real terminalHeight: 5 * K.Style.scale
    readonly property bool critical: !Battery.charging && Battery.pct <= 15
    readonly property color levelColor: critical ? K.Style.alert : K.Style.ink
    readonly property string hintText: Battery.pct + "% battery, "
        + (Battery.charging ? "charging" : Battery.full ? "full"
            : Battery.discharging ? "discharging" : "on AC")
        + (Battery.hasTime ? ", " + Battery.timeStr : "")

    implicitWidth: present ? batteryRow.implicitWidth + K.Style.hitPad * 2 : 0
    implicitHeight: K.Style.barH
    scale: batteryPress.pressScale

    Row {
        id: batteryRow

        anchors.centerIn: parent
        spacing: K.Style.s1

        Item {
            width: root.bodyWidth + root.terminalWidth + K.Style.scale
            height: K.Style.barH

            Rectangle {
                id: batteryBody

                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: root.bodyWidth
                height: root.bodyHeight
                radius: 3.5 * K.Style.scale
                color: "transparent"
                border.width: Math.max(1, K.Style.scale)
                border.color: K.Style.inkDim

                Rectangle {
                    anchors.left: parent.left
                    anchors.leftMargin: Math.max(2, 2 * K.Style.scale)
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.max(0, (parent.width - Math.max(4, 4 * K.Style.scale))
                        * Math.max(0, Math.min(1, Battery.pct / 100)))
                    height: Math.max(1, parent.height - Math.max(4, 4 * K.Style.scale))
                    radius: Math.min(2 * K.Style.scale, height / 2)
                    color: root.levelColor

                    Behavior on width {
                        enabled: !K.Style.still
                        NumberAnimation {
                            duration: K.Style.effects
                            easing.type: Easing.BezierSpline
                            easing.bezierCurve: K.Style.effectsCurve
                        }
                    }
                }
            }

            Rectangle {
                anchors.left: batteryBody.right
                anchors.leftMargin: K.Style.scale
                anchors.verticalCenter: batteryBody.verticalCenter
                width: root.terminalWidth
                height: root.terminalHeight
                radius: width / 2
                color: K.Style.inkDim
            }
        }

        K.Label {
            anchors.verticalCenter: parent.verticalCenter
            text: Battery.pct + "%"
            role: "barSub"
            tone: root.critical ? "alert" : "dim"
            numeric: true
        }
    }

    K.Press {
        id: batteryPress
        radius: K.Style.radius
        onClicked: mouse => root.scene.toggleCard("power", root)
    }

    K.Hint {
        target: root
        text: root.hintText
        shown: batteryPress.hovered
    }
}

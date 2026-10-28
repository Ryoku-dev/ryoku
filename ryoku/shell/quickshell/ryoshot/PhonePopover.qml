import QtQuick
import QtQuick.Layouts
import "Singletons"
import Ryoku.Ui.Singletons

Item {
    id: pop

    property var devices: []

    signal picked(string deviceId, string deviceName)
    signal closeRequested()

    focus: visible
    Keys.onEscapePressed: pop.closeRequested()

    readonly property int arrow: 7
    implicitWidth: card.implicitWidth
    implicitHeight: card.implicitHeight + arrow

    Rectangle {
        id: card
        width: parent.width
        height: parent.height - pop.arrow
        radius: Theme.radius
        color: Theme.panel
        border.color: Theme.hair
        border.width: 1
        implicitWidth: 230
        implicitHeight: body.implicitHeight + 24

        ColumnLayout {
            id: body
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: 12
            anchors.rightMargin: 12
            spacing: 6

            Text {
                text: I18n.tr("Send to phone")
                color: Theme.ink
                font.family: Theme.ui
                font.pixelSize: 13
                font.weight: Font.DemiBold
            }

            Repeater {
                model: pop.devices

                Rectangle {
                    required property var modelData

                    Layout.fillWidth: true
                    Layout.preferredHeight: 36
                    radius: 6
                    color: deviceMouse.containsMouse ? Theme.hover : "transparent"

                    Text {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 8
                        anchors.rightMargin: 8
                        text: modelData.name
                        elide: Text.ElideRight
                        color: Theme.ink
                        font.family: Theme.ui
                        font.pixelSize: 12
                    }

                    MouseArea {
                        id: deviceMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        onClicked: pop.picked(String(modelData.deviceId), String(modelData.name))
                    }
                }
            }
        }
    }

    Canvas {
        anchors.top: card.bottom
        anchors.horizontalCenter: parent.horizontalCenter
        width: 14
        height: pop.arrow
        onPaint: {
            var ctx = getContext("2d");
            ctx.reset();
            ctx.beginPath();
            ctx.moveTo(0, 0);
            ctx.lineTo(width, 0);
            ctx.lineTo(width / 2, height);
            ctx.closePath();
            ctx.fillStyle = Theme.panel;
            ctx.fill();
        }
    }
}

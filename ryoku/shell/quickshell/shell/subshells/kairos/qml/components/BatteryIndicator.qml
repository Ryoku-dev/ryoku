pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import shell.barkit as Pill

// The Kairos clock island's power readout, in the hovered island's top-left
// corner, opposite the gear. A laptop shows a drawn battery with the charge
// percentage under it; a desktop has no cell to read, so it shows the
// all-inclusive glyph instead -- the island says "plugged in forever".
Item {
    id: root

    required property color ink
    // 0 hidden to 1 shown, driven by the island's clock expansion so the
    // resting pill stays a bare clock.
    property real reveal: 0

    readonly property bool laptop: Battery.present
    readonly property int pct: Battery.pct
    readonly property bool low: Battery.low
    readonly property bool charging: Battery.charging

    // Charging reads green with a bolt; there is no green role in the theme, so
    // the readout carries its own soft green against the island's near-black.
    readonly property color chargeColor: "#8fd08f"
    readonly property color battColor:
        charging ? chargeColor : (low ? Theme.error : ink)

    implicitWidth: 24
    implicitHeight: laptop ? 24 : 22

    visible: reveal > 0.01
    opacity: reveal

    Pill.MaterialIcon {
        visible: !root.laptop
        anchors.centerIn: parent
        text: "all_inclusive"
        color: root.ink
        opacity: 0.75
        font.pixelSize: 18
    }

    Column {
        visible: root.laptop
        anchors.centerIn: parent
        spacing: 2

        Item {
            id: batt
            width: 18
            height: 9
            anchors.horizontalCenter: parent.horizontalCenter

            readonly property real ratio: Math.max(0, Math.min(1, root.pct / 100))

            Rectangle {
                id: body
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: 17
                height: 9
                radius: 2.5
                color: "transparent"
                border.width: 1.2
                border.color: root.battColor
                opacity: 0.85

                Rectangle {
                    visible: root.charging
                    anchors.fill: parent
                    anchors.margins: 1.8
                    radius: 1.2
                    color: Qt.rgba(root.chargeColor.r, root.chargeColor.g, root.chargeColor.b, 0.35)
                }

                Pill.MaterialIcon {
                    visible: root.charging
                    anchors.centerIn: parent
                    text: "bolt"
                    fill: 1
                    color: Theme.shadow
                    font.pixelSize: 8
                }

                Rectangle {
                    anchors.left: parent.left
                    anchors.top: parent.top
                    anchors.bottom: parent.bottom
                    anchors.margins: 1.8
                    width: Math.max(batt.ratio > 0 ? 1.5 : 0, (parent.width - 3.6) * batt.ratio)
                    radius: 1.2
                    color: root.battColor
                    visible: !root.charging
                }
            }

            Rectangle {
                anchors.left: body.right
                anchors.leftMargin: -0.5
                anchors.verticalCenter: parent.verticalCenter
                width: 2.5
                height: 5
                radius: 1.2
                color: root.battColor
                opacity: 0.85
            }
        }

        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.pct + "%"
            color: root.ink
            font.family: Theme.mono
            font.pixelSize: 10
            opacity: 0.85
        }
    }
}

import QtQuick
import Ryoku.Ui.Singletons

// One small control above whatever skin is loaded: the layout the keys type
// now, and a click or the shortcut to change it.
Item {
    id: badge

    required property var feed
    property string shortcut: ""
    property color accent: "#ffb59b"

    readonly property real unit: Math.max(1, Math.min(width, height) / 720)

    visible: feed.switchable
    z: 10000
    anchors.left: parent.left
    anchors.bottom: parent.bottom
    anchors.margins: Math.round(24 * unit)
    width: row.implicitWidth + Math.round(28 * unit)
    height: Math.round(40 * unit)

    Rectangle {
        anchors.fill: parent
        radius: Math.round(8 * badge.unit)
        color: "#b3000000"
        border.width: 1
        border.color: hit.containsMouse || hit.pressed ? badge.accent : "#33ffffff"
        Behavior on border.color { ColorAnimation { duration: 140 } }
    }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: Math.round(12 * badge.unit)

        Text {
            anchors.verticalCenter: parent.verticalCenter
            color: "#ffffff"
            font.pixelSize: Math.round(15 * badge.unit)
            font.weight: Font.DemiBold
            text: badge.feed.active.length > 0 ? badge.feed.active : I18n.tr("Keyboard layout")
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            visible: badge.shortcut.length > 0
            color: "#ffffff"
            opacity: 0.55
            font.pixelSize: Math.round(12 * badge.unit)
            text: badge.shortcut
        }
    }

    MouseArea {
        id: hit
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: badge.feed.cycle()
    }
}

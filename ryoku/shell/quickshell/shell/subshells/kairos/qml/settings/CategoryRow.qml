import QtQuick
import "../kit"

// a settings category: its own tonal colour in a circle, its name, and what it is set to now
Item {
    id: row

    property string icon: ""
    property color hue: Md.accent
    property string headline: ""
    property string supporting: ""
    property string position: "only"
    property bool selected: false
    signal clicked()

    readonly property bool topOuter: shape.topOuter
    readonly property bool bottomOuter: shape.bottomOuter
    readonly property color ink: row.selected ? Md.onAccentContainer : Md.ink

    implicitWidth: 320
    implicitHeight: 66

    Accessible.role: Accessible.Button
    Accessible.name: row.headline
    Accessible.selected: row.selected

    Segment {
        id: shape
        anchors.fill: parent
        position: row.position
        color: row.selected ? Md.accentContainer : Md.container
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    StateLayer {
        topLeftRadius: shape.topLeftRadius
        topRightRadius: shape.topRightRadius
        bottomLeftRadius: shape.bottomLeftRadius
        bottomRightRadius: shape.bottomRightRadius
        tint: row.ink
        onActivated: row.clicked()
    }

    Rectangle {
        id: disc
        anchors { left: parent.left; leftMargin: 14; verticalCenter: parent.verticalCenter }
        width: 40
        height: 40
        radius: 20
        color: row.hue

        Icon {
            anchors.centerIn: parent
            name: row.icon
            size: 22
            filled: true
            color: Md.base
        }
    }

    Column {
        anchors {
            left: disc.right; leftMargin: 14
            right: parent.right; rightMargin: 16
            verticalCenter: parent.verticalCenter
        }
        spacing: 1

        Label {
            width: parent.width
            role: "title"
            font.weight: Font.Medium
            color: row.ink
            text: row.headline
        }
        Label {
            width: parent.width
            visible: text.length > 0
            role: "caption"
            font.pixelSize: 13
            color: row.selected ? row.ink : Md.inkMuted
            opacity: row.selected ? 0.8 : 1
            text: row.supporting
        }
    }
}

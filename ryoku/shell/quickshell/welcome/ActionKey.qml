import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "Singletons"

Rectangle {
    id: root

    property string chord: ""
    property string title: ""
    property string description: ""
    property var command: []
    readonly property bool actionable: command && command.length > 0
    signal runCommand(var argv)

    height: 74
    radius: Tokens.radius
    color: tap.pressed && actionable ? Tokens.tint16 : (hover.hovered && actionable ? Tokens.tint5 : "transparent")
    border.width: Tokens.border
    border.color: hover.hovered && actionable ? Tokens.lineStrong : Tokens.line

    Behavior on color { ColorAnimation { duration: Motion.snap } }
    Behavior on border.color { ColorAnimation { duration: Motion.snap } }

    Keycap {
        id: key
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s3
        anchors.verticalCenter: parent.verticalCenter
        text: root.chord
        us: 0.55
        width: implicitWidth
        height: implicitHeight
        layer.enabled: false
        dark: !Tokens.light
        motionEnabled: !Motion.reduce
    }

    Column {
        anchors.left: key.right
        anchors.right: parent.right
        anchors.leftMargin: Tokens.s3
        anchors.rightMargin: Tokens.s3
        anchors.verticalCenter: parent.verticalCenter
        spacing: 2

        Text {
            width: parent.width
            text: root.title
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody
            font.weight: Font.DemiBold
            wrapMode: Text.WordWrap
        }
        Text {
            width: parent.width
            text: root.description
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall
            wrapMode: Text.WordWrap
        }
    }

    HoverHandler {
        id: hover
        enabled: root.actionable
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        id: tap
        enabled: root.actionable
        onTapped: {
            key.pulse++
            root.runCommand(root.command)
        }
    }
}

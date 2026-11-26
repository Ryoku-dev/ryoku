import QtQuick
import Lithic

// A filter in play, written into the search line
CutRect {
    id: token

    property string text: ""
    property string glyph: ""
    property color dot: "transparent"
    signal dropped()

    readonly property bool hovered: area.containsMouse
    height: 28 * Theme.scale
    width: row.implicitWidth + 16 * Theme.scale
    cut: 4 * Theme.scale
    fill: token.hovered ? Qt.darker(Lithic.container, 1.08) : Lithic.container

    scale: 0.6
    opacity: 0
    Component.onCompleted: { scale = 1; opacity = 1 }
    Behavior on scale { enabled: !Lithic.reduceMotion; NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack; easing.overshoot: 1.6 } }
    Behavior on opacity { NumberAnimation { duration: Theme.fast } }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 6 * Theme.scale
        CutRect {
            visible: token.dot.a > 0
            anchors.verticalCenter: parent.verticalCenter
            width: 10 * Theme.scale; height: width; cut: 2 * Theme.scale
            fill: token.dot
        }
        Glyph {
            visible: token.glyph.length > 0
            anchors.verticalCenter: parent.verticalCenter
            name: token.glyph; size: 14; ink: Lithic.containerInk
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: token.text
            font.family: Lithic.font.sans
            font.weight: Font.Medium
            font.pixelSize: Theme.fs(13)
            color: Lithic.containerInk
            renderType: Text.NativeRendering
        }
        Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: "close"; size: 13
            ink: Lithic.alpha(Lithic.containerInk, token.hovered ? 1 : 0.6)
        }
    }
    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: token.dropped()
    }
}

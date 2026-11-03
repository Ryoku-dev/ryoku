// A quiet action (session, keyboard layout, sleep, restart, power off): plain at rest, a fill on hover.
import QtQuick 2.15
import "."

Item {
    id: action
    property real d: 1
    property string glyph: ""
    property string label: ""
    property color ink: "#ffffff"
    property color hover: Qt.rgba(1, 1, 1, 0.18)
    property real size: 40
    property real fontSize: 15
    property string fontFamily: "Inter"
    property string symbolFont: ""
    signal activated()

    readonly property real pad: Math.round((action.label.length > 0 ? 14 : 0) * action.d)
    implicitWidth: action.label.length > 0 ? row.implicitWidth + 2 * action.pad : action.size
    implicitHeight: action.size

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: action.hover
        opacity: area.pressed ? 1 : area.containsMouse ? 0.7 : 0
        Behavior on opacity { NumberAnimation { duration: 120; easing.type: Easing.OutCubic } }
    }
    Row {
        id: row
        anchors.centerIn: parent
        spacing: Math.round(8 * action.d)
        MSymbol {
            anchors.verticalCenter: parent.verticalCenter
            text: action.glyph
            symFont: action.symbolFont
            iconSize: Math.round(action.size * 0.5)
            iconColor: action.ink
        }
        LoginText {
            visible: action.label.length > 0
            anchors.verticalCenter: parent.verticalCenter
            text: action.label
            color: action.ink
            font.family: action.fontFamily
            font.pixelSize: action.fontSize
            font.weight: Font.Medium
        }
    }
    MouseArea {
        id: area
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: action.activated()
    }
}

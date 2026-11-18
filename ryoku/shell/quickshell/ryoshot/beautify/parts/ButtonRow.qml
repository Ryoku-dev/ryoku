import QtQuick
import Lithic
import "../../look"
import "../.."

// libadwaita's button row: a boxed-list row that is one flat button, its icon
// and label centred.
Item {
    id: br

    property real u: 1
    property string icon: ""
    property string text: ""
    signal clicked()

    width: parent ? parent.width : implicitWidth
    implicitHeight: Math.round(50 * br.u)
    height: implicitHeight
    opacity: br.enabled ? 1 : Lithic.disabledOpacity

    Rectangle {
        anchors.fill: parent
        bottomLeftRadius: 12 * br.u
        bottomRightRadius: 12 * br.u
        color: tap.pressed ? Look.press : hover.hovered ? Look.hover : Lithic.alpha(Look.hover, 0)
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }
    Row {
        anchors.centerIn: parent
        spacing: Math.round(8 * br.u)
        Icon {
            visible: br.icon.length > 0
            anchors.verticalCenter: parent.verticalCenter
            name: br.icon
            size: Math.round(16 * br.u)
            tint: Look.fg
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: br.text
            color: Look.fg
            font.family: Look.sans
            font.pixelSize: Math.round(14 * br.u)
            font.weight: Font.DemiBold
            renderType: Text.CurveRendering
        }
    }
    HoverHandler {
        id: hover
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        id: tap
        onTapped: br.clicked()
    }
}

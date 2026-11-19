import QtQuick
import Lithic
import "../../look"

// A group of the panel under its title: Lithic numbers it in mono caps along a
// hairline rule ("02 BACKGROUND"), Material names it in the primary, Adwaita
// sets an AdwPreferencesGroup caption over a boxed list, Fluent a body-strong
// heading over its cards. boxed puts the rows in Adwaita's boxed list.
Item {
    id: sec

    property real u: 1
    property int number: 0
    property string title: ""
    property bool boxed: true
    default property alias content: list.content

    implicitHeight: body.y + body.height
    height: implicitHeight

    readonly property bool hasTitle: sec.title.length > 0
    readonly property real headH: !sec.hasTitle ? 0
        : Math.round((Look.lithic ? 16 : Look.material ? 30 : Look.adwaita ? 26 : 30) * sec.u)
    readonly property real headGap: !sec.hasTitle ? 0 : Math.round((Look.lithic ? 10 : Look.material ? 4 : 2) * sec.u)

    Item {
        id: head
        visible: sec.hasTitle
        width: sec.width
        height: sec.headH

        Row {
            id: lithicHead
            visible: Look.lithic
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(6 * sec.u)
            Text {
                text: (sec.number < 10 ? "0" : "") + sec.number
                color: Look.sel
                font.family: Look.mono
                font.pixelSize: Math.round(9 * sec.u)
                font.weight: Font.Bold
                font.letterSpacing: Look.tracking * sec.u
                renderType: Text.CurveRendering
            }
            Text {
                text: sec.title.toUpperCase()
                color: Look.fg2
                font.family: Look.mono
                font.pixelSize: Math.round(9 * sec.u)
                font.weight: Font.Bold
                font.letterSpacing: Look.tracking * sec.u
                renderType: Text.CurveRendering
            }
        }
        Rectangle {
            visible: Look.lithic
            anchors.left: lithicHead.right
            anchors.leftMargin: Math.round(8 * sec.u)
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: Math.max(1, Math.round(sec.u))
            color: Look.vein
        }

        Text {
            visible: !Look.lithic
            anchors.left: parent.left
            anchors.leftMargin: Look.adwaita ? Math.round(2 * sec.u) : 0
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Math.round((Look.adwaita ? 6 : 6) * sec.u)
            text: sec.title
            color: Look.material ? Look.sel : Look.fg
            font.family: Look.sans
            font.pixelSize: Math.round((Look.adwaita ? 12.5 : 14) * sec.u)
            font.weight: Look.material ? Font.Medium : Look.adwaita ? Font.Bold : Font.DemiBold
            renderType: Text.CurveRendering
        }
    }

    Item {
        id: body
        y: sec.headH + sec.headGap
        width: sec.width
        height: list.height

        // Adwaita's boxed list: the card with its faint shade outline
        Rectangle {
            visible: Look.adwaita && sec.boxed
            anchors.fill: parent
            radius: 12 * sec.u
            color: Look.planes.card
            border.width: Math.max(1, Math.round(sec.u))
            border.color: Lithic.alpha(Look.fg, Look.dark ? 0.07 : 0.1)
            antialiasing: true
        }

        List {
            id: list
            width: parent.width
            u: sec.u
            boxed: sec.boxed
        }
    }
}

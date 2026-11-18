import QtQuick
import Lithic

// A tooltip in the design's own card: Lithic's cut plate with a mono key hint,
// Material's plain tooltip in the inverse tone, Adwaita's and Fluent's small
// rounded cards. It never takes input; the owner places it and sets shown.
Item {
    id: root

    property real u: 1
    property string text: ""
    property string keyHint: ""
    property bool shown: false

    implicitWidth: Math.ceil(row.implicitWidth + 2 * root.padX)
    implicitHeight: Math.ceil(Math.max(row.implicitHeight + 2 * root.padY, (Look.material ? 28 : 24) * root.u))
    width: implicitWidth
    height: implicitHeight
    enabled: false
    visible: opacity > 0
    opacity: root.shown && root.text.length > 0 ? 1 : 0
    Behavior on opacity {
        NumberAnimation {
            duration: Look.dur(root.shown ? 120 : 80)
            easing.type: Easing.OutCubic
        }
    }

    readonly property real padX: (Look.lithic ? 8 : Look.adwaita ? 10 : 8) * root.u
    readonly property real padY: 4 * root.u

    Surface {
        anchors.fill: parent
        u: root.u
        cut: Look.cutSm
        radius: (Look.adwaita ? 6 : 4) * root.u
        fill: Look.material ? Look.fg : Look.plane
        stroke: Look.material ? "transparent" : Look.planeStroke
        elevation: 1
        acrylic: false
    }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 6 * root.u

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            color: Look.material ? Look.ground : Look.fg
            font.family: Look.sans
            font.pixelSize: Math.round(12 * root.u)
            renderType: Text.CurveRendering
        }
        Text {
            visible: root.keyHint.length > 0
            anchors.verticalCenter: parent.verticalCenter
            text: Look.lithic ? root.keyHint.toUpperCase() : root.keyHint
            color: Look.material ? Lithic.alpha(Look.ground, 0.7) : Look.fg3
            font.family: Look.lithic ? Look.mono : Look.sans
            font.pixelSize: Math.round((Look.lithic ? 9.5 : 11.5) * root.u)
            font.letterSpacing: Look.lithic ? 0.6 * root.u : 0
            renderType: Text.CurveRendering
        }
    }
}

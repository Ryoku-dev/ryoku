import QtQuick
import Lithic

// A small value chip, such as the delay a capture waits: Lithic's mono figure
// on a cut well, Material's tonal chip, Adwaita's and Fluent's rounded pills.
Item {
    id: root

    property real u: 1
    property string text: ""
    property bool strong: false

    implicitWidth: Math.ceil(label.implicitWidth + (Look.lithic ? 10 : 14) * root.u)
    implicitHeight: Math.round((Look.material ? 24 : 18) * root.u)
    width: implicitWidth
    height: implicitHeight

    readonly property color fill: root.strong ? Look.sel
        : Look.material ? Look.container
        : Look.lithic ? Look.well
        : Lithic.alpha(Look.fg, 0.1)
    readonly property color ink: root.strong ? Look.onSel
        : Look.material ? Look.containerInk
        : Look.fg

    CutRect {
        visible: Look.lithic
        anchors.fill: parent
        design: "lithic"
        cut: 2 * root.u
        fill: root.fill
    }
    Rectangle {
        visible: !Look.lithic
        anchors.fill: parent
        radius: Look.material ? 8 * root.u : height / 2
        color: root.fill
    }
    Text {
        id: label
        anchors.centerIn: parent
        text: root.text
        color: root.ink
        font.family: Look.lithic ? Look.mono : Look.sans
        font.pixelSize: Math.round((Look.lithic ? 9.5 : 11.5) * root.u)
        font.weight: Look.lithic ? Font.Bold : Font.DemiBold
        font.features: ({ "tnum": 1 })
        renderType: Text.CurveRendering
    }
}

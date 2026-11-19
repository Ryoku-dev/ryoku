pragma ComponentBehavior: Bound

import QtQuick
import Lithic

// A small button. quiet draws nothing at rest; line is the design's secondary
// (a hairline in Lithic and Fluent, a tonal fill in Material and Adwaita);
// accent is the one filled action.
Shape {
    id: root

    property string text: ""
    property string glyph: ""
    property string tier: "quiet"
    property bool current: false
    readonly property bool hovered: hover.hovered
    readonly property real inset: Math.round((root.text.length > 0 ? 10 : 6) * root.s)
    readonly property color ink: root.tier === "accent" ? Lithic.onSel
        : root.tier === "line" && Look.material ? Lithic.containerInk : Lithic.fg
    readonly property color rest: root.tier === "accent" ? Lithic.sel
        : root.tier !== "line" ? "transparent"
        : Look.material ? Lithic.container
        : Look.adwaita ? Lithic.alpha(Lithic.fg, Look.dark ? 0.1 : 0.08)
        : "transparent"
    readonly property bool lined: root.tier === "line" && (Look.lithic || Look.fluent)

    signal clicked()

    cutSize: Look.cutRow
    implicitHeight: Math.round(28 * root.s)
    implicitWidth: Math.ceil(label.implicitWidth + root.inset * 2)
    height: implicitHeight
    width: implicitWidth
    fill: root.tier === "accent" ? (tap.pressed ? Lithic.pressRow(root.rest) : hover.hovered ? Lithic.hoverFill(root.rest) : root.rest)
        : tap.pressed ? Look.press
        : hover.hovered || root.current ? (Qt.colorEqual(root.rest, "transparent") ? Look.plate : Lithic.hoverFill(root.rest))
        : root.rest
    strokeColor: root.lined ? (Look.lithic ? Lithic.vein : Look.capStroke) : "transparent"
    strokeWidth: root.lined ? 1 : 0
    opacity: root.enabled ? 1 : Lithic.disabledOpacity

    Behavior on fill {
        ColorAnimation { duration: Look.dur(Look.stateMs) }
    }

    Row {
        id: label
        anchors.centerIn: parent
        anchors.verticalCenterOffset: tap.pressed && Look.lithic ? 1 : 0
        spacing: Math.round(6 * root.s)

        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.glyph.length > 0
            s: root.s
            name: root.glyph
            size: 16
            ink: root.ink
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.text.length > 0
            text: root.text
            color: root.ink
            font.family: Look.sans
            font.pixelSize: Math.round(12 * root.s)
            font.weight: Font.Medium
            renderType: Text.NativeRendering
        }
    }

    HoverHandler {
        id: hover
        enabled: root.enabled
        cursorShape: Qt.PointingHandCursor
    }
    TapHandler {
        id: tap
        enabled: root.enabled
        onTapped: root.clicked()
    }
}

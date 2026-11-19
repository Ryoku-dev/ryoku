import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../look"
import "../../designs"

// What the stage says about the composition: the output's pixel size, and the
// padding while an edge is dragged. Lithic draws them as a drawing does, with
// crop marks at the corners and dimension lines carrying mono plaques; the
// others put the figure on their own chip (Material's tonal chip, Adwaita's
// osd pill, a Fluent tooltip card).
Item {
    id: readout

    required property var chrome
    // the composition and the shot on screen, in this item's px
    property rect area: Qt.rect(0, 0, 0, 0)
    property rect card: Qt.rect(0, 0, 0, 0)
    property real outW: 0
    property real outH: 0
    property bool hd: false
    property bool showSize: false
    // the edge being dragged ("" none) and the padding it sets
    property string side: ""
    property int padding: 0

    readonly property real u: readout.chrome.u
    readonly property string sizeText: Math.round(readout.outW) + " \u00d7 " + Math.round(readout.outH)
        + (readout.hd ? "  \u00d72" : "")

    // ---- Lithic ----
    Item {
        anchors.fill: parent
        visible: Look.lithic
        opacity: readout.showSize ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Look.dur(160); easing.type: Easing.OutCubic } }

        LithicMarks {
            anchors.fill: parent
            chrome: readout.chrome
            rx: readout.area.x
            ry: readout.area.y
            rw: readout.area.width
            rh: readout.area.height
            length: 12
        }
        LithicDimension {
            anchors.fill: parent
            chrome: readout.chrome
            from: readout.area.x
            to: readout.area.x + readout.area.width
            at: readout.area.y - 16 * readout.u
            label: Math.round(readout.outW) + (readout.hd ? " \u00d72" : "")
        }
        LithicDimension {
            anchors.fill: parent
            chrome: readout.chrome
            vertical: true
            from: readout.area.y
            to: readout.area.y + readout.area.height
            at: readout.area.x - 16 * readout.u
            label: Math.round(readout.outH) + (readout.hd ? " \u00d72" : "")
        }
    }
    // the padding, measured from the shot's edge to the background's, beside
    // the grip so the pointer never covers it
    LithicDimension {
        anchors.fill: parent
        visible: Look.lithic && readout.side !== ""
        chrome: readout.chrome
        readonly property bool across: readout.side === "t" || readout.side === "b"
        vertical: across
        from: readout.side === "l" ? readout.area.x
            : readout.side === "r" ? readout.card.x + readout.card.width
            : readout.side === "t" ? readout.area.y
            : readout.card.y + readout.card.height
        to: readout.side === "l" ? readout.card.x
            : readout.side === "r" ? readout.area.x + readout.area.width
            : readout.side === "t" ? readout.card.y
            : readout.area.y + readout.area.height
        at: across ? readout.area.x + readout.area.width / 2 + 28 * readout.u
            : readout.area.y + readout.area.height / 2 - 28 * readout.u
        label: String(readout.padding)
    }

    // ---- the round designs ----
    ValueChip {
        visible: !Look.lithic
        u: readout.u
        shown: readout.showSize && !Look.lithic
        x: Math.round(readout.area.x + readout.area.width / 2 - width / 2)
        y: Math.round(readout.area.y + readout.area.height + 14 * readout.u)
        text: readout.sizeText
        chipHeight: Look.material ? 32 : 28
        radius: Look.material ? 8 : Look.fluent ? 4 : -1
        padX: Look.material ? 14 : 12
        fill: Look.material ? Look.container : Look.adwaita ? readout.osd : Look.plane
        ink: Look.material ? Look.containerInk : Look.adwaita ? readout.osdInk : Look.fg
        stroke: Look.fluent ? Look.planeStroke : "transparent"
        shadow: !Look.material
        fontSize: Look.material ? 13 : 12
        fontWeight: Look.fluent ? Font.Normal : Font.DemiBold
    }
    readonly property color osd: Lithic.alpha(Lithic.scrim, 0.78)
    readonly property color osdInk: readout.luma(Look.fg) > readout.luma(Look.ground) ? Look.fg : Look.ground
    function luma(c) { return 0.2126 * c.r + 0.7152 * c.g + 0.0722 * c.b; }

    ValueChip {
        id: padChip
        visible: !Look.lithic
        u: readout.u
        shown: readout.side !== "" && !Look.lithic
        readonly property real gap: 18 * readout.u
        x: Math.round(readout.side === "r" ? readout.area.x + readout.area.width + padChip.gap
            : readout.side === "l" ? readout.area.x - padChip.gap - width
            : readout.area.x + readout.area.width / 2 - width / 2)
        y: Math.round(readout.side === "b" ? readout.area.y + readout.area.height + padChip.gap
            : readout.side === "t" ? readout.area.y - padChip.gap - height
            : readout.area.y + readout.area.height / 2 - height / 2)
        text: Look.adwaita ? I18n.tr("%1 px").arg(readout.padding) : String(readout.padding)
        chipHeight: Look.material ? 32 : 28
        radius: Look.material ? -1 : Look.fluent ? 4 : -1
        padX: 12
        fill: Look.material ? Look.fg : Look.adwaita ? readout.osd : Look.plane
        ink: Look.material ? Look.ground : Look.adwaita ? readout.osdInk : Look.fg
        stroke: Look.fluent ? Look.planeStroke : "transparent"
        shadow: !Look.material
        fontSize: Look.material ? 14 : 12
        fontWeight: Look.fluent ? Font.Normal : Font.DemiBold
    }
}

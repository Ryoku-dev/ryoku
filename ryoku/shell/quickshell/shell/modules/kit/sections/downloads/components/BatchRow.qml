pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../../../ask/look"
import "../../../parts"

// One link of a batch: its tick, what the engine read (picture, title, site,
// size) or how far the reading is, and the format it will take. row: {id,
// state: "waiting"|"resolving"|"ready"|"failed", ticked, title, sub, thumb,
// glyph, reason, format, formatNote, own, canLess, canMore, spread}.
Item {
    id: root

    property real s: 1
    property var row: ({})
    property bool selected: false

    readonly property real pad: Math.round(12 * root.s)
    readonly property string state_: String(root.row.state || "")
    readonly property bool failed: root.state_ === "failed"
    readonly property bool ticked: root.row.ticked === true && !root.failed
    readonly property color quiet: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.78) : Lithic.fg2
    readonly property string formatText: String(root.row.format || "")

    Symbol {
        id: tick
        x: root.pad
        anchors.verticalCenter: parent.verticalCenter
        width: Math.round(20 * root.s)
        s: root.s
        name: root.failed ? "error" : root.ticked ? "check_box" : "check_box_outline_blank"
        size: 18
        ink: root.failed ? Lithic.jasper : root.ticked ? (root.selected ? Look.plateMark : Lithic.sel) : Lithic.fg3
        chosen: root.ticked
    }
    Picture {
        id: picture
        x: tick.x + tick.width + Math.round(10 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        width: Math.round(60 * root.s)
        height: Math.round(34 * root.s)
        s: root.s
        cutSize: Look.cutToken
        source: String(root.row.thumb || "")
        glyph: String(root.row.glyph || "link")
        glyphSize: 18
        opacity: root.ticked || root.failed ? 1 : 0.5
    }

    Line {
        id: title
        x: picture.x + picture.width + Math.round(12 * root.s)
        y: Math.round(8 * root.s)
        width: Math.max(0, control.x - x - Math.round(14 * root.s))
        s: root.s
        size: 14
        text: String(root.row.title || "")
        ink: !root.ticked && !root.failed ? Lithic.fg3 : root.selected ? Look.plateInk : Lithic.fg
        elide: root.state_ === "ready" ? Text.ElideRight : Text.ElideMiddle
    }
    Line {
        x: title.x
        y: Math.round(28 * root.s)
        width: title.width
        visible: root.state_ !== "resolving"
        s: root.s
        size: 12
        text: root.failed ? String(root.row.reason || "") : String(root.row.sub || "")
        ink: root.failed ? Lithic.jasper : root.quiet
    }
    Row {
        x: title.x
        y: Math.round(29 * root.s)
        visible: root.state_ === "resolving"
        spacing: Math.round(10 * root.s)

        Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12; text: I18n.tr("Reading"); ink: root.quiet }
        Busy { anchors.verticalCenter: parent.verticalCenter; width: Math.round(84 * root.s); s: root.s; running: root.state_ === "resolving" }
    }

    Row {
        id: control
        x: parent.width - root.pad - width
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(4 * root.s)

        // a list of several: all of it, or only its first
        Shape {
            anchors.verticalCenter: parent.verticalCenter
            visible: String(root.row.spread || "").length > 0
            width: Math.ceil(spread.implicitWidth + 16 * root.s)
            height: Math.round(22 * root.s)
            s: root.s
            cutSize: Look.cutToken
            fill: Look.cap
            strokeColor: Look.capStroke
            strokeWidth: Qt.colorEqual(Look.capStroke, "transparent") ? 0 : 1

            Line { id: spread; anchors.centerIn: parent; s: root.s; size: 11.5; text: String(root.row.spread || ""); ink: root.selected ? Look.plateInk : Lithic.fg }
        }
        Item { visible: String(root.row.spread || "").length > 0 && root.formatText.length > 0; width: Math.round(6 * root.s); height: 1 }

        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.formatText.length > 0
            opacity: root.selected && root.row.canLess === true ? 1 : 0
            s: root.s
            name: "chevron_left"
            size: 18
            ink: root.selected ? Look.plateInk : Lithic.fg2
        }
        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.row.own === true
            s: root.s
            name: "keep"
            size: 13
            ink: Lithic.fg3
        }
        Line {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.formatText.length > 0
            s: root.s
            size: 13
            text: root.formatText
            ink: !root.ticked ? Lithic.fg3 : root.selected ? Look.plateInk : Lithic.fg
        }
        Line {
            anchors.verticalCenter: parent.verticalCenter
            visible: String(root.row.formatNote || "").length > 0
            s: root.s
            size: 12
            figures: true
            text: " " + String(root.row.formatNote || "")
            ink: root.quiet
        }
        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.formatText.length > 0
            opacity: root.selected && root.row.canMore === true ? 1 : 0
            s: root.s
            name: "chevron_right"
            size: 18
            ink: root.selected ? Look.plateInk : Lithic.fg2
        }
    }
}

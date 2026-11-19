pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../../../ask/look"
import "../../../parts"

// One transfer in the queue: what it is, where it stands, and its piece map
// where a progress bar would be. row: {id, owner, glyph, title, facts, factTone,
// fills, tone, reason}; the list owns the selection plate.
Item {
    id: root

    property real s: 1
    property var row: ({})
    property bool selected: false

    readonly property real pad: Math.round(12 * root.s)
    readonly property real textX: Math.round(42 * root.s)
    readonly property string reason: String(root.row.reason || "")
    readonly property string factTone: String(root.row.factTone || "")
    readonly property color factInk: root.factTone === "error" ? Lithic.jasper
        : root.factTone === "paused" ? Lithic.amber
        : root.factTone === "done" ? Lithic.jade
        : root.factTone === "idle" ? Lithic.fg3
        : root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.8) : Lithic.fg2

    Symbol {
        x: root.pad
        y: Math.round(8 * root.s)
        width: Math.round(20 * root.s)
        height: width
        s: root.s
        name: String(root.row.glyph || "draft")
        size: 18
        ink: root.row.tone === "error" ? Lithic.jasper : root.selected ? Look.plateMark : Lithic.fg2
        chosen: root.selected
        Behavior on color {
            ColorAnimation { duration: Look.dur(Look.stateMs) }
        }
    }
    Line {
        x: root.textX
        y: Math.round(7 * root.s)
        width: Math.max(0, facts.x - x - Math.round(12 * root.s))
        s: root.s
        size: 14
        text: String(root.row.title || "")
        ink: root.selected ? Look.plateInk : Lithic.fg
    }
    Line {
        id: facts
        x: parent.width - root.pad - width
        y: Math.round(9 * root.s)
        width: Math.min(implicitWidth, parent.width * 0.5)
        s: root.s
        size: 12
        figures: true
        text: String(root.row.facts || "")
        ink: root.factInk
    }
    PieceMap {
        x: root.textX
        y: Math.round(31 * root.s)
        visible: root.reason.length === 0
        s: root.s
        owner: String(root.row.owner || "")
        available: Math.max(0, root.width - root.textX - root.pad)
        pieces: root.row.fills || []
        tone: String(root.row.tone || "")
    }
    Line {
        x: root.textX
        y: Math.round(27 * root.s)
        width: Math.max(0, parent.width - root.textX - root.pad)
        visible: root.reason.length > 0
        s: root.s
        size: 12
        text: root.reason
        ink: root.selected && Look.material ? Lithic.alpha(Look.plateInk, 0.8) : Lithic.fg2
    }
}

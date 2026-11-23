import QtQuick
import "../kit" as K

// A calculation: the result set huge in tabular figures, the expression under
// it, and Copy as the primary track (the engine copies and the foot flashes).
Column {
    id: root

    property var lead: null
    readonly property var row: root.lead ? root.lead.row : null

    spacing: K.Style.s3

    Item {
        width: root.width
        height: result.implicitHeight

        K.Label {
            id: result
            width: parent.width
            y: root.lead ? root.lead.rise : 0
            text: root.row ? String(root.row.title || "") : ""
            role: "display"
            numeric: true
            fontSizeMode: Text.HorizontalFit
            minimumPixelSize: K.Style.headline
        }
    }

    K.Label {
        width: root.width
        visible: text.length > 0
        text: root.row ? String(root.row.subtitle || "") : ""
        role: "body"
        tone: "dim"
        numeric: true
        wrapMode: Text.Wrap
        maximumLineCount: 2
        elide: Text.ElideRight
    }

    ActionList {
        width: root.width
        items: root.lead ? root.lead.actionItems : []
        focusIndex: root.lead ? root.lead.actionFocus : -1
        scroller: root.lead ? root.lead.scroller : null
        onActivated: index => root.lead.runAt(index)
    }
}

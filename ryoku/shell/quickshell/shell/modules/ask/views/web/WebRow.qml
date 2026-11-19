pragma ComponentBehavior: Bound

import QtQuick
import "../../bar"
import "../../look"

// One row of the Web list in the list's own geometry: the dictionary's card for
// a definition, the bar's result row for everything else. row.last marks the
// end of a section, where a boxed list draws no rule.
Item {
    id: root

    property real s: 1
    property var row: ({})
    property bool selected: false

    Loader {
        anchors.fill: parent
        sourceComponent: root.row.kind === "definition" ? card : plain
    }

    Component {
        id: plain
        ResultRow {
            s: root.s
            row: root.row
            selected: root.selected
        }
    }
    Component {
        id: card
        DefinitionCard {
            s: root.s
            row: root.row
            selected: root.selected
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: Math.round(44 * root.s)
        height: 1
        color: Look.divider
        visible: Look.boxed && root.row.last !== true
    }
}

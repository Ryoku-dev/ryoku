import QtQuick

// A hairline between groups, across the row (vertical) or the column, with pad
// logical px of air on either side of it.
Item {
    id: root

    property real u: 1
    property bool vertical: true
    property real length: 20
    property real pad: 0

    readonly property real line: Math.max(1, Math.round(root.u))
    implicitWidth: root.vertical ? root.line + 2 * Math.round(root.pad * root.u) : Math.round(root.length * root.u)
    implicitHeight: root.vertical ? Math.round(root.length * root.u) : root.line + 2 * Math.round(root.pad * root.u)
    width: implicitWidth
    height: implicitHeight

    Rectangle {
        anchors.centerIn: parent
        width: root.vertical ? root.line : parent.width
        height: root.vertical ? parent.height : root.line
        color: Look.divider
    }
}

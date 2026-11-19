import QtQuick
import "../../look"

// The rows of a group, stacked as the design stacks them: Adwaita's boxed list
// flush with a hairline between rows, Fluent's cards a few px apart (or, ruled,
// an expander's rows flush under hairlines), Material's and Lithic's rows with
// air between them.
Item {
    id: list

    property real u: 1
    property bool boxed: false
    property bool ruled: false
    readonly property bool flush: (Look.adwaita && list.boxed) || (Look.fluent && list.ruled)
    default property alias content: col.data

    implicitHeight: col.implicitHeight
    height: implicitHeight

    Column {
        id: col
        width: list.width
        spacing: Math.round((list.flush ? 0 : Look.fluent ? 4 : Look.material ? 6 : 12) * list.u)
    }

    Repeater {
        model: list.flush ? Math.max(0, col.visibleChildren.length - 1) : 0
        delegate: Rectangle {
            required property int index
            readonly property Item below: col.visibleChildren[index + 1] || null
            y: below ? below.y : 0
            width: list.width
            height: Math.max(1, Math.round(list.u))
            color: Look.fluent ? Look.planes.cardStroke : Look.divider
        }
    }
}

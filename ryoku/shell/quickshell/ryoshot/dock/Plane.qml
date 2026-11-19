pragma ComponentBehavior: Bound

import QtQuick
import "../look"

// What the dock stands on: Lithic's two stacked cut strips (top-left and
// bottom-right cut, the drafting caption), Material's two floating pills, one
// Adwaita toolbar card, one Fluent acrylic CommandBar.
Item {
    id: root

    property real u: 1
    property real stripHeight: 36
    property real stripGap: 4
    readonly property bool strips: Look.lithic || Look.material

    Repeater {
        model: root.strips ? 2 : 0
        Surface {
            required property int index
            y: index * (root.stripHeight + root.stripGap)
            width: root.width
            height: root.stripHeight
            u: root.u
            cutTopLeft: Look.lithic ? Look.cutMd : 99
            cutTopRight: Look.lithic ? 0 : 99
            cutBottomRight: Look.lithic ? Look.cutMd : 99
            cutBottomLeft: Look.lithic ? 0 : 99
            radius: Look.material ? root.stripHeight / 2 : -1
            elevation: Look.material ? 3 : 2
        }
    }

    Surface {
        visible: !root.strips
        anchors.fill: parent
        u: root.u
        radius: (Look.adwaita ? 12 : 8) * root.u
        elevation: 3
    }
}

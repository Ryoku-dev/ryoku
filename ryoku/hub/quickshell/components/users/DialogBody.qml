import QtQuick
import Lithic

// a dialog's body that scrolls once it is taller than the window allows
Item {
    id: root

    property real maxHeight: 480
    default property alias content: column.data
    property alias spacing: column.spacing
    readonly property alias flickable: flick

    implicitWidth: column.implicitWidth
    implicitHeight: Math.min(column.implicitHeight, Math.max(120, root.maxHeight))

    // keeps a focused field, or what a flow reports, in view
    function reveal(item) {
        if (!item)
            return;
        // positions settle on the next polish; settle them now so the item's place is current
        if (item.forceLayout)
            item.forceLayout();
        column.forceLayout();
        var y = item.mapToItem(column, 0, 0).y;
        if (y < flick.contentY)
            flick.contentY = Math.max(0, y - LithicTokens.space.s2);
        else if (y + item.height > flick.contentY + flick.height)
            flick.contentY = Math.min(flick.contentHeight - flick.height, y + item.height - flick.height + LithicTokens.space.s2);
    }

    Flickable {
        id: flick
        anchors.fill: parent
        anchors.rightMargin: LithicTokens.space.s3
        contentWidth: width
        contentHeight: column.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        // room for focus rings inside the clip
        Column {
            id: column
            x: LithicTokens.size.focusRingGap + LithicTokens.size.focusRingWidth
            width: flick.width - x * 2
            topPadding: x
            bottomPadding: x
            spacing: LithicTokens.space.s4
        }
    }

    ScrollRail {
        id: rail
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }
}

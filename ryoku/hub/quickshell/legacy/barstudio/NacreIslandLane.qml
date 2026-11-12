pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    required property string islandId
    required property var items
    required property var labelFor
    property int dropIndex: -1
    property bool dragPreview: false
    readonly property Item markerChip: {
        if (chips.count === 0 || root.dropIndex < 0)
            return null;
        return chips.itemAt(Math.min(root.dropIndex, chips.count - 1));
    }
    readonly property real markerX: {
        if (!root.markerChip)
            return content.x;
        const edge = root.dropIndex < chips.count
            ? content.x + root.markerChip.x - Tokens.s1 / 2
            : content.x + root.markerChip.x + root.markerChip.width + Tokens.s1 / 2;
        return Math.max(Tokens.s1, Math.min(root.width - Tokens.s1 - 2, edge));
    }
    readonly property real markerY: root.markerChip
        ? content.y + root.markerChip.y : content.y
    readonly property real markerHeight: root.markerChip
        ? root.markerChip.height : 32
    signal moved(string widgetId, string sourceIsland, string targetIsland, int targetIndex)

    objectName: "nacre-island-" + root.islandId
    height: Math.max(72, 34 + content.implicitHeight + Tokens.s2)
    radius: Tokens.radius
    color: root.dragPreview ? Tokens.tint10 : "transparent"
    border.width: Tokens.border
    border.color: root.dragPreview ? Tokens.lineStrong : Tokens.line

    function insertionIndex(x, y) {
        const localX = x - content.x;
        const localY = y - content.y;
        for (let index = 0; index < chips.count; index++) {
            const chip = chips.itemAt(index);
            if (chip && (localY < chip.y + chip.height / 2
                    || (localY <= chip.y + chip.height && localX < chip.x + chip.width / 2)))
                return index;
        }
        return root.items.length;
    }

    function showDropPreview(x, y) {
        root.dragPreview = true;
        root.dropIndex = root.insertionIndex(x, y);
    }

    function hideDropPreview() {
        root.dragPreview = false;
        root.dropIndex = -1;
    }

    Text {
        anchors { top: parent.top; left: parent.left; margins: Tokens.s2 }
        text: I18n.tr(root.islandId.toUpperCase())
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackLabel
    }

    Flow {
        id: content
        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s2; topMargin: 34 }
        spacing: Tokens.s1

        Repeater {
            id: chips
            model: root.items
            delegate: NacreWidgetChip {
                required property string modelData
                required property int index
                widgetId: modelData
                label: root.labelFor(modelData)
                sourceIsland: root.islandId
                sourceIndex: index
            }
        }
    }

    Text {
        objectName: "nacre-empty-drop-" + root.islandId
        anchors.centerIn: parent
        visible: root.items.length === 0
        text: I18n.tr("DROP WIDGET")
        color: root.dragPreview ? Tokens.ink : Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackLabel
    }

    Rectangle {
        objectName: "nacre-drop-marker-" + root.islandId
        x: root.markerX
        y: root.markerY
        width: 2
        height: root.markerHeight
        radius: 1
        visible: root.dragPreview && root.items.length > 0 && root.dropIndex >= 0
        color: Tokens.sun
        z: 2
    }

    DropArea {
        id: drop
        anchors.fill: parent
        keys: ["nacre-widget"]
        onEntered: drag => root.showDropPreview(drag.x, drag.y)
        onPositionChanged: drag => root.showDropPreview(drag.x, drag.y)
        onExited: root.hideDropPreview()
        onDropped: event => {
            const index = root.insertionIndex(event.x, event.y);
            root.hideDropPreview();
            root.moved(
                event.source.widgetId,
                event.source.sourceIsland,
                root.islandId,
                index
            );
        }
    }
}

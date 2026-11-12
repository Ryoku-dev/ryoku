import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    required property string widgetId
    required property string label
    required property string sourceIsland
    required property int sourceIndex
    property bool dragging: false

    width: Math.min(144, text.implicitWidth + 20)
    height: 32
    z: drag.active ? 10 : 0

    Rectangle {
        id: visual

        objectName: "nacre-widget-visual-" + root.widgetId
        width: root.width
        height: root.height
        radius: Tokens.radius
        color: drag.active ? Tokens.bone : hover.hovered ? Tokens.tint10 : "transparent"
        border.width: Tokens.border
        border.color: drag.active ? Tokens.bone : Tokens.line

        Drag.active: root.dragging
        Drag.source: root
        Drag.keys: ["nacre-widget"]
        Drag.hotSpot.x: width / 2
        Drag.hotSpot.y: height / 2

        Text {
            id: text
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; margins: 10 }
            text: root.label.toUpperCase()
            color: visual.Drag.active ? Tokens.inkOnBone : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: 10
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
            elide: Text.ElideRight
            horizontalAlignment: Text.AlignHCenter
        }

        HoverHandler { id: hover; cursorShape: Qt.OpenHandCursor }
        DragHandler {
            id: drag
            target: visual
            onActiveChanged: {
                if (active) {
                    root.dragging = true;
                } else {
                    visual.Drag.drop();
                    root.dragging = false;
                    visual.x = 0;
                    visual.y = 0;
                }
            }
        }
    }
}

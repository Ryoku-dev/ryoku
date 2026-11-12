pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Rectangle {
    id: root

    required property var items
    required property var labelFor
    property bool removalPreview: false
    signal removed(string widgetId)

    objectName: "nacre-palette"
    height: Math.max(72, 34 + content.implicitHeight + Tokens.s2)
    radius: Tokens.radius
    color: root.removalPreview ? Tokens.tint10 : "transparent"
    border.width: Tokens.border
    border.color: root.removalPreview ? Tokens.lineStrong : Tokens.line

    function showRemovalPreview() {
        root.removalPreview = true;
    }

    function hideRemovalPreview() {
        root.removalPreview = false;
    }

    Text {
        anchors { top: parent.top; left: parent.left; margins: Tokens.s2 }
        text: root.removalPreview ? I18n.tr("REMOVE WIDGET") : I18n.tr("UNUSED")
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
            model: root.items
            delegate: NacreWidgetChip {
                required property string modelData
                required property int index
                widgetId: modelData
                label: root.labelFor(modelData)
                sourceIsland: ""
                sourceIndex: index
            }
        }
    }

    Text {
        anchors.centerIn: parent
        visible: root.items.length === 0
        text: I18n.tr("ALL WIDGETS PLACED")
        color: Tokens.inkFaint
        font.family: Tokens.mono
        font.pixelSize: Tokens.fTiny
        font.letterSpacing: Tokens.trackLabel
    }

    DropArea {
        id: drop
        anchors.fill: parent
        keys: ["nacre-widget"]
        onEntered: root.showRemovalPreview()
        onExited: root.hideRemovalPreview()
        onDropped: event => {
            root.hideRemovalPreview();
            root.removed(event.source.widgetId);
        }
    }
}

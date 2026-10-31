pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    required property PickerState state
    readonly property CardField field: state.field
    readonly property bool active: state.collection === "workshop" && !!field && !!state.view
    property int modelRevision: 0

    function entryAt(row, revision) {
        return root.state.view.get(row)
    }

    function rectAt(row, currentRect, stageRect, scrollPosition, fieldWidth, fieldHeight) {
        return root.field.rectOf(row)
    }

    clip: true
    visible: active

    Connections {
        target: root.state.view
        function onModelReset() { root.modelRevision++ }
        function onDataChanged() { root.modelRevision++ }
        function onLayoutChanged() { root.modelRevision++ }
        function onCountChanged() { root.modelRevision++ }
        function onCollectionChanged() { root.modelRevision++ }
    }

    Repeater {
        model: root.active ? root.state.view.count : 0

        delegate: Item {
            id: cardLabel
            required property int index
            readonly property var entry: root.entryAt(cardLabel.index, root.modelRevision)
            readonly property rect cardRect: root.rectAt(cardLabel.index,
                root.field.currentRect, root.field.stageRect, root.field.scrollPosition,
                root.field.width, root.field.height)
            readonly property bool missingPreview: !cardLabel.entry.thumb

            x: cardRect.x
            y: cardRect.y
            width: cardRect.width
            height: cardRect.height
            visible: root.active && width > 4 && height > 4
                && x + width > 0 && y + height > 0 && x < root.width && y < root.height
            clip: true

            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: Math.min(parent.height, titleText.implicitHeight + 12 * Theme.scale)
                visible: parent.width > 36 * Theme.scale && parent.height > 22 * Theme.scale
                color: Theme.withAlpha(Theme.background, 0.68)

                Text {
                    id: titleText
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: 10 * Theme.scale
                    anchors.rightMargin: 10 * Theme.scale
                    text: cardLabel.entry.name || cardLabel.entry.key || ""
                    elide: Text.ElideRight
                    font.family: Theme.sans
                    font.weight: Font.DemiBold
                    font.pixelSize: Theme.fontBody
                    color: Theme.surfaceText
                    renderType: Text.NativeRendering
                }
            }

            Rectangle {
                anchors.centerIn: parent
                visible: cardLabel.missingPreview
                    && parent.width > 80 * Theme.scale && parent.height > 52 * Theme.scale
                radius: Theme.radius
                color: Theme.withAlpha(Theme.background, 0.58)
                border.width: 1
                border.color: Theme.withAlpha(Theme.outline, 0.38)
                implicitWidth: noPreviewText.implicitWidth + 14 * Theme.scale
                implicitHeight: noPreviewText.implicitHeight + 8 * Theme.scale

                Text {
                    id: noPreviewText
                    anchors.centerIn: parent
                    text: I18n.tr("No preview")
                    font.family: Theme.sans
                    font.weight: Font.Medium
                    font.pixelSize: Theme.fontFine
                    color: Theme.withAlpha(Theme.surfaceText, 0.86)
                    renderType: Text.NativeRendering
                }
            }
        }
    }
}

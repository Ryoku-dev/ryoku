pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import Ryoku.FrameBars

Column {
    id: root

    required property var config
    signal staged(var value)

    readonly property var normalized: NacreConfig.normalize(root.config)
    readonly property var islandIds: ["left", "center", "right"]
    readonly property int placedCount: root.normalized.islands.left.length
        + root.normalized.islands.center.length + root.normalized.islands.right.length
    readonly property int unusedCount: NacreConfig.unused(root.normalized).length

    spacing: Tokens.s3

    function label(id) {
        const item = NacreConfig.entry(id);
        return item ? I18n.tr(item.label) : id;
    }

    function moveWidget(widgetId, sourceIsland, targetIsland, targetIndex) {
        root.staged(NacreConfig.move(root.normalized, widgetId, sourceIsland, targetIsland, targetIndex));
    }

    function removeWidget(widgetId) {
        root.staged(NacreConfig.remove(root.normalized, widgetId));
    }

    function setAppearance(key, value) {
        root.staged(NacreConfig.setValue(root.normalized, key, value));
    }

    Column {
        width: parent.width
        spacing: Tokens.s2

        Repeater {
            model: root.islandIds
            delegate: NacreIslandLane {
                required property string modelData
                islandId: modelData
                items: root.normalized.islands[modelData]
                labelFor: root.label
                width: parent.width
                onMoved: (widgetId, sourceIsland, targetIsland, targetIndex) =>
                    root.moveWidget(widgetId, sourceIsland, targetIsland, targetIndex)
            }
        }
    }

    NacrePalette {
        width: parent.width
        items: NacreConfig.unused(root.normalized)
        labelFor: root.label
        onRemoved: widgetId => root.removeWidget(widgetId)
    }

    NacreAppearance {
        width: parent.width
        config: root.normalized
        onChanged: (key, value) => root.setAppearance(key, value)
    }
}

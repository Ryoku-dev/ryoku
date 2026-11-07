pragma ComponentBehavior: Bound

import QtQuick
import "../kit" as K
import "../modules/Registry.js" as Registry
import "." as Parts

Item {
    id: laneRoot

    required property var scene
    required property string name
    property var moduleIds: []

    readonly property real contentWidth: measureContentWidth()
    readonly property bool hintsSuppressed: scene.cardKey !== "" || scene.settingsOpen
    // While a drop morphs its width trails the content; keep the content where
    // the drop grows from so the clip eats evenly.
    readonly property real alignOffset: name === "center" ? Math.round((width - contentWidth) / 2)
        : name === "right" ? Math.round(width - contentWidth) : 0

    implicitWidth: contentWidth
    implicitHeight: K.Style.barH
    height: K.Style.barH

    function slotFor(moduleId) {
        for (let i = 0; i < slots.count; ++i) {
            const slot = slots.itemAt(i);
            if (slot && slot.moduleId === moduleId)
                return slot;
        }
        return null;
    }

    function priorReveal(index) {
        let reveal = 0;
        for (let i = 0; i < index; ++i) {
            const slot = slots.itemAt(i);
            if (slot)
                reveal = Math.max(reveal, slot.reveal);
        }
        return reveal;
    }

    function gapBefore(index) {
        if (index <= 0)
            return 0;
        const slot = slots.itemAt(index);
        return slot ? K.Style.barGap * slot.reveal * priorReveal(index) : 0;
    }

    function offsetFor(index) {
        let offset = 0;
        for (let i = 0; i < index; ++i) {
            const slot = slots.itemAt(i);
            if (slot)
                offset += slot.width;
            offset += gapBefore(i + 1);
        }
        return offset;
    }

    function measureContentWidth() {
        let total = 0;
        for (let i = 0; i < slots.count; ++i) {
            const slot = slots.itemAt(i);
            if (slot)
                total += slot.width;
            total += gapBefore(i);
        }
        return total;
    }

    Repeater {
        id: slots
        model: laneRoot.moduleIds

        Parts.Slot {
            required property int index
            required property var modelData

            readonly property var entry: Registry.find(moduleId)

            scene: laneRoot.scene
            lane: laneRoot.name
            moduleId: typeof modelData === "string" ? modelData : ""
            file: entry ? entry.file : ""
            x: laneRoot.alignOffset + laneRoot.offsetFor(index)
            y: 0
        }
    }
}

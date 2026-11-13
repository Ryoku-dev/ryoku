import QtQuick
import Lithic as L

L.EngravedChart {
    id: root

    property var levels: []
    property bool live: false
    property var previous: []
    property var targetValues: []
    property real blend: 1

    onLevelsChanged: {
        previous = values.slice();
        targetValues = levels.slice();
        blend = 0;
        blend = 1;
    }
    Behavior on blend {
        enabled: !L.Lithic.reduceMotion
        NumberAnimation { duration: 80; easing.type: Easing.Linear }
    }

    values: {
        const out = [];
        for (let i = 0; i < targetValues.length; i++) {
            const before = previous.length > i ? previous[i] : targetValues[i];
            out.push(before + (targetValues[i] - before) * blend);
        }
        return out;
    }
    // Activity is already relative to the microphone floor. Keep a fixed
    // range so startup and silence cannot be expanded into a high plateau.
    minimum: 0
    maximum: 1
    lineInk: live ? L.Lithic.fg : L.Lithic.fg3
    baselineInk: L.Lithic.vein
    emptyText: ""
    Accessible.name: L.Lithic.tr("Microphone level")
}

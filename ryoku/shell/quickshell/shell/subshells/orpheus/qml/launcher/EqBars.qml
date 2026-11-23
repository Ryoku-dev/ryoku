pragma ComponentBehavior: Bound

import QtQuick
import shell.services as Svc
import "../kit" as K

// Live equaliser bars on the row of the app that is playing. They read the
// analyser only while the vessel already runs it for the music edge, so a row
// never starts the analyser on its own. Otherwise they stand still.
Item {
    id: root

    property bool live: false
    property color tone: K.Style.ink

    readonly property bool reading: root.live && root.visible && K.Look.ambientMotion
        && Svc.AudioBars.analysing
    readonly property var bands: [3, 9, 17, 27]
    readonly property var still: [0.45, 0.85, 0.6, 0.35]

    implicitWidth: 4 * 3 + 3 * 2
    implicitHeight: 12

    Repeater {
        model: 4

        delegate: K.Shape {
            id: bar

            required property int index

            readonly property real amount: {
                if (!root.reading)
                    return root.still[bar.index];
                const levels = Svc.AudioBars.levels;
                const band = root.bands[bar.index];
                return band < levels.length ? Math.max(0.12, Math.min(1, levels[band])) : 0.12;
            }

            x: bar.index * 5
            width: 3
            height: Math.max(2, Math.round(root.height * bar.amount))
            y: root.height - height
            cutSize: K.Look.lithic ? 0 : 2
            fill: root.tone
        }
    }
}

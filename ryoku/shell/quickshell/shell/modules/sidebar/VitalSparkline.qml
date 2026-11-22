pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Blobs
import Lithic

Item {
    id: root

    required property var monitor
    required property bool active
    required property real s
    property var channels: ["cpu"]
    property var colors: [Lithic.sel]
    property bool fill: true
    property bool showGrid: false
    property bool detailPoints: false
    property real lineWidth: Math.max(1, Math.round(root.s))

    clip: true

    SystemGraph {
        anchors.fill: parent
        source: root.monitor
        active: root.active
        animated: !QsLook.still
        channels: root.channels
        colors: root.colors
        fill: root.fill
        showGrid: root.showGrid
        detailPoints: root.detailPoints
        gridColor: Lithic.vein
        lineWidth: root.lineWidth
    }
}

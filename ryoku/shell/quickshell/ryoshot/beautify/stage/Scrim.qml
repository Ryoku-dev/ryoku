import QtQuick
import "../../look"

// The studio's dim over an output's frozen frame. The capture's own scrim
// leaves the selection clear; this fills that hole to match, then deepens the
// whole output evenly, so the frame reads as one plane behind the studio.
Item {
    id: scrim

    // the selection's clear hole on this output, in local px, or null
    property var hole: null
    // 0 nothing, 1 the full studio dim
    property real level: 0

    readonly property real target: Look.material ? 0.70 : Look.adwaita ? 0.72 : Look.fluent ? 0.70 : 0.78
    readonly property real extra: 1 - (1 - scrim.target) / (1 - Look.scrimAlpha)

    visible: scrim.level > 0
    opacity: Math.max(0, Math.min(1, scrim.level))

    Rectangle {
        visible: scrim.hole !== null
        x: scrim.hole ? scrim.hole.x : 0
        y: scrim.hole ? scrim.hole.y : 0
        width: scrim.hole ? scrim.hole.w : 0
        height: scrim.hole ? scrim.hole.h : 0
        color: Look.scrim
    }
    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, scrim.extra)
    }
}

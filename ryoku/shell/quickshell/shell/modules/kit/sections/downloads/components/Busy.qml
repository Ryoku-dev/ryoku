import QtQuick
import Lithic
import "../../../../ask/look"

// Work with no measure yet (a link being read): a short lapis run crossing a
// track, only while it shows. Reduced motion holds it in the middle.
Item {
    id: root

    property real s: 1
    property bool running: true
    property real at: 0.5

    implicitWidth: Math.round(120 * root.s)
    implicitHeight: Math.max(3, Math.round(4 * root.s))
    clip: true

    Shape {
        anchors.fill: parent
        s: root.s
        cutSize: Look.lithic ? 0 : 2
        fill: Lithic.alpha(Lithic.fg, Look.dark ? 0.13 : 0.14)
    }
    Shape {
        x: (parent.width + width) * root.at - width
        width: Math.round(parent.width * 0.3)
        height: parent.height
        s: root.s
        cutSize: Look.lithic ? 0 : 2
        fill: Lithic.lapis
    }
    NumberAnimation on at {
        from: 0
        to: 1
        duration: 1100
        easing.type: Easing.InOutCubic
        loops: Animation.Infinite
        running: root.running && root.visible && !Look.still
    }
}

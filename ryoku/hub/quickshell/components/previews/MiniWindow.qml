import QtQuick
import QtQuick.Effects
import Lithic
import "../../Singletons"

// a window drawn small for a page's preview: a title strip and a few lines of content in the
// frame, corners, see-through and shadow the settings give it, scaled to the preview. It is an
// illustration of the settings, not a picture of the screen
Item {
    id: root

    // the settings, in real pixels and fractions; the preview scales the pixels
    property real scaleBy: 0.4
    property real radius: 0
    property real border: 0
    property color borderColor: Tone.accent
    property real see: 1
    property real dim: 0
    property bool shadow: false
    property real shadowRange: 0
    property real shadowX: 0
    property real shadowY: 0
    property color shadowColor: "#000000"
    property bool active: true

    readonly property real _r: Math.max(0, root.radius * root.scaleBy)
    readonly property real _b: root.border > 0 ? Math.max(1, root.border * root.scaleBy) : 0

    Behavior on radius { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    Behavior on border { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    Behavior on see { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    Behavior on dim { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    Behavior on shadowRange { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
    Behavior on borderColor { ColorAnimation { duration: Tone.move } }

    MultiEffect {
        source: body
        anchors.fill: body
        visible: root.shadow
        opacity: root.see
        shadowEnabled: true
        shadowColor: root.shadowColor
        shadowOpacity: Lithic.dark ? 0.9 : 0.45
        shadowBlur: Math.min(1, root.shadowRange / 60)
        shadowHorizontalOffset: root.shadowX * root.scaleBy
        shadowVerticalOffset: root.shadowY * root.scaleBy
        autoPaddingEnabled: true
    }

    Rectangle {
        id: body
        anchors.fill: parent
        radius: root._r
        color: Lithic.mix(Tone.card, Lithic.ground, Lithic.dark ? 0.15 : 0)
        border.width: root._b
        border.color: root.borderColor
        // with a shadow the effect draws the window and its shadow together
        visible: !root.shadow
        opacity: root.see
        layer.enabled: root.shadow

        Row {
            x: 8
            y: 7
            spacing: 4
            Repeater {
                model: 3
                Rectangle {
                    width: 5
                    height: 5
                    radius: 2.5
                    color: Lithic.alpha(Lithic.fg, root.active ? 0.5 : 0.25)
                }
            }
        }
        Column {
            x: 10
            y: 22
            width: parent.width - 20
            spacing: 5
            Repeater {
                model: [0.8, 0.55, 0.7, 0.4]
                Rectangle {
                    required property real modelData
                    width: parent.width * modelData
                    height: 4
                    radius: 2
                    color: Lithic.alpha(Lithic.fg, root.active ? 0.22 : 0.14)
                }
            }
        }
        // the dimming the other windows wear
        Rectangle {
            anchors.fill: parent
            radius: root._r
            color: "#000000"
            opacity: root.dim
        }
    }
}

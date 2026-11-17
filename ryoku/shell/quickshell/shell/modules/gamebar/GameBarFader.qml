import QtQuick
import Lithic
import "look"

Item {
    id: root

    property string glyph: "volume_up"
    property string label: ""
    property real value: 0
    property string valueLabel: ""
    property bool muted: false
    property bool available: true

    signal moved(real v)
    signal iconTapped()

    readonly property real _scale: Lithic.scaleOf(root)

    implicitHeight: 40 * root._scale

    Row {
        anchors.fill: parent
        spacing: 8 * root._scale

        Item {
            width: 32 * root._scale
            height: root.implicitHeight

            IconPlaque {
                anchors.verticalCenter: parent.verticalCenter
                small: true
                kind: root.muted ? "tonal" : "quiet"
                glyph: root.muted
                    ? (root.glyph === "mic" ? "mic_off" : "volume_off")
                    : root.glyph
                Accessible.name: root.label
                onClicked: root.iconTapped()
            }
        }

        Column {
            width: parent.width - 40 * root._scale
            spacing: 2 * root._scale

            Row {
                width: parent.width

                Label {
                    width: Math.max(0, parent.width - readout.width)
                    text: root.label
                    typeRole: "caption"
                    ink: root.available ? GameBarTone.ink2 : GameBarTone.ink3
                }

                Label {
                    id: readout
                    text: root.valueLabel
                    typeRole: "mono"
                    inkRole: "fg3"
                    numeric: true
                }
            }

            Slider {
                width: parent.width
                height: 22 * root._scale
                from: 0
                to: 1
                stepSize: 0.01
                enabled: root.available
                value: root.value
                plane: GameBarTone.pane
                Accessible.name: root.label
                onMoved: root.moved(value)
            }
        }
    }
}

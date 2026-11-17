pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Widgets
import shell.services
import Ryoku.Ui.Singletons
import Lithic
import "look"

Row {
    id: root

    property var stream: null

    readonly property var appAudio: root.stream ? root.stream.audio : null
    readonly property bool appMuted: root.appAudio ? root.appAudio.muted : false

    readonly property real _scale: Lithic.scaleOf(root)

    height: 40 * root._scale
    spacing: 8 * root._scale

    Item {
        width: 20 * root._scale
        height: root.height

        Item {
            anchors.verticalCenter: parent.verticalCenter
            width: 20 * root._scale
            height: 20 * root._scale

            IconImage {
                anchors.fill: parent
                source: Audio.streamIcon(root.stream)
                opacity: root.appMuted ? 0.4 : 1
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: if (root.appAudio) root.appAudio.muted = !root.appAudio.muted
            }
        }
    }

    Column {
        width: parent.width - 28 * root._scale
        spacing: 2 * root._scale

        Row {
            width: parent.width

            Label {
                width: Math.max(0, parent.width - level.width)
                text: Audio.streamName(root.stream)
                typeRole: "caption"
                ink: root.appMuted ? GameBarTone.ink3 : GameBarTone.ink2
            }

            Label {
                id: level
                text: root.appAudio
                    ? (root.appMuted ? I18n.tr("off") : Math.round(root.appAudio.volume * 100) + "%")
                    : ""
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
            value: root.appAudio ? root.appAudio.volume : 0
            plane: GameBarTone.pane
            onMoved: if (root.appAudio) root.appAudio.volume = value
        }
    }
}

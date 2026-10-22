pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell.Widgets
import Quickshell.Services.Pipewire
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

// One app in the mixer: its own icon (a click mutes it), its name over a thin slider,
// and its level. The popout draws the card the rows share.
Item {
    id: root

    required property PwNode node

    property bool muteOnIconClick: false

    readonly property bool muted: Audio.getAppMuted(root.node)
    readonly property real value: Audio.getAppVolume(root.node)
    readonly property string iconSource: Icons.getAppIcon(root.node?.properties["application.icon-name"] || root.node?.properties["application.process.binary"] || Audio.getStreamName(root.node), "audio-x-generic")

    Layout.fillWidth: true
    implicitHeight: row.implicitHeight + Tokens.padding.small * 2

    RowLayout {
        id: row

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.spacing.medium

        Item {
            implicitWidth: 36
            implicitHeight: 36

            StyledRect {
                anchors.fill: parent
                radius: Tokens.rounding.full
                color: Colours.layer(Colours.palette.m3surfaceContainerHighest, 2)
            }

            IconImage {
                anchors.centerIn: parent
                implicitSize: 22
                source: root.iconSource
                opacity: root.muted ? 0.35 : 1
            }

            MaterialIcon {
                visible: root.muted
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                anchors.margins: -2
                text: "volume_off"
                color: Colours.palette.m3error
                fontStyle: Tokens.font.icon.small
            }

            MouseArea {
                anchors.fill: parent
                enabled: root.muteOnIconClick
                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                onClicked: Audio.setAppMuted(root.node, !root.muted)
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: Tokens.spacing.extraSmall

            StyledText {
                Layout.fillWidth: true
                text: Audio.getStreamName(root.node)
                font: Tokens.font.body.small
                elide: Text.ElideRight
            }

            CustomMouseArea {
                function onWheel(event: WheelEvent): void {
                    const step = GlobalConfig.services.audioIncrement;
                    if (event.angleDelta.y > 0)
                        Audio.setAppVolume(root.node, Math.min(1, root.value + step));
                    else if (event.angleDelta.y < 0)
                        Audio.setAppVolume(root.node, Math.max(0, root.value - step));
                }

                Layout.fillWidth: true
                implicitHeight: 14

                StyledSlider {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    implicitHeight: parent.implicitHeight
                    radius: Tokens.rounding.full
                    value: root.value
                    fgColour: Colours.palette.m3primary
                    bgColour: Colours.tPalette.m3surfaceContainerHigh
                    onInteraction: v => Audio.setAppVolume(root.node, v)
                }
            }
        }

        StyledText {
            Layout.preferredWidth: 48
            text: root.muted ? qsTr("Muted") : Math.round(root.value * 100) + "%"
            color: Colours.palette.m3onSurfaceVariant
            font: Tokens.font.body.small
            horizontalAlignment: Text.AlignRight
        }
    }
}

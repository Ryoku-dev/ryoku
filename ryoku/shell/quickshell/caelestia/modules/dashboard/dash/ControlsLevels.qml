pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property ScreenState screenState

    readonly property Brightness.Monitor monitor: Brightness.getMonitorForScreen(root.screenState.modelData) ?? Brightness.getMonitor("active") ?? Brightness.monitors[0] ?? null
    readonly property real brightness: monitor?.effectiveBrightness ?? 0

    anchors.fill: parent
    implicitHeight: layout.implicitHeight + layout.anchors.margins * 2

    RowLayout {
        id: layout

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        spacing: Tokens.spacing.large

        ColumnLayout {
            visible: HostState.element("volume")
            Layout.fillWidth: true
            spacing: Tokens.spacing.extraSmall

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small

                IconButton {
                    icon: Icons.getVolumeIcon(Audio.volume, Audio.muted)
                    type: IconButton.Tonal
                    isRound: true
                    isToggle: true
                    checked: !Audio.muted
                    onClicked: {
                        if (Audio.sink?.ready && Audio.sink?.audio)
                            Audio.sink.audio.muted = !Audio.sink.audio.muted;
                    }
                }

                StyledText {
                    Layout.fillWidth: true
                    text: qsTr("Volume")
                    font: Tokens.font.body.small
                    elide: Text.ElideRight
                }

                StyledText {
                    text: Audio.muted ? qsTr("Muted") : `${Math.round(Audio.volume * 100)}%`
                    color: Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.body.small
                }
            }

            CustomMouseArea {
                Layout.fillWidth: true
                implicitHeight: Tokens.padding.medium * 2

                onWheel: event => {
                    if (event.angleDelta.y > 0)
                        Audio.incrementVolume();
                    else if (event.angleDelta.y < 0)
                        Audio.decrementVolume();
                }

                StyledSlider {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    implicitHeight: parent.implicitHeight

                    value: Audio.volume
                    onInteraction: value => Audio.setVolume(value)
                }
            }
        }

        ColumnLayout {
            visible: HostState.element("brightness")
            Layout.fillWidth: true
            spacing: Tokens.spacing.extraSmall

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.small

                IconButton {
                    icon: `brightness_${(Math.round(root.brightness * 6) + 1)}`
                    type: IconButton.Tonal
                    isRound: true
                    disabled: !root.monitor
                }

                StyledText {
                    Layout.fillWidth: true
                    text: qsTr("Brightness")
                    font: Tokens.font.body.small
                    elide: Text.ElideRight
                }

                StyledText {
                    text: `${Math.round(root.brightness * 100)}%`
                    color: Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.body.small
                }
            }

            CustomMouseArea {
                Layout.fillWidth: true
                implicitHeight: Tokens.padding.medium * 2

                onWheel: event => {
                    if (!root.monitor)
                        return;
                    if (event.angleDelta.y > 0)
                        root.monitor.stepBrightness(GlobalConfig.services.brightnessIncrement);
                    else if (event.angleDelta.y < 0)
                        root.monitor.stepBrightness(-GlobalConfig.services.brightnessIncrement);
                }

                StyledSlider {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    implicitHeight: parent.implicitHeight

                    enabled: root.monitor !== null
                    value: root.brightness
                    onInteraction: value => root.monitor?.setBrightness(value)
                }
            }
        }
    }
}

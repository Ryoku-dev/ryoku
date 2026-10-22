pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property Brightness.Monitor monitor
    required property DrawerVisibilities visibilities

    required property real volume
    required property bool muted
    required property real sourceVolume
    required property bool sourceMuted
    required property real brightness

    implicitWidth: layout.implicitWidth + Tokens.padding.large * 2
    implicitHeight: layout.implicitHeight + Tokens.padding.large * 2

    ColumnLayout {
        id: layout

        anchors.centerIn: parent
        spacing: Tokens.spacing.medium

        WrappedLoader {
            Layout.alignment: Qt.AlignHCenter
            shouldBeActive: Config.osd.enableVolume !== false && HostState.section("levels") && HostState.element("volume")
            targetWidth: Tokens.sizes.osd.sliderWidth
            targetHeight: Tokens.sizes.osd.sliderHeight

            sourceComponent: CustomMouseArea {
                function onWheel(event: WheelEvent) {
                    if (event.angleDelta.y > 0)
                        Audio.incrementVolume();
                    else if (event.angleDelta.y < 0)
                        Audio.decrementVolume();
                }

                implicitWidth: Tokens.sizes.osd.sliderWidth
                implicitHeight: Tokens.sizes.osd.sliderHeight

                FilledSlider {
                    anchors.fill: parent

                    icon: Icons.getVolumeIcon(value, root.muted)
                    value: root.volume
                    to: GlobalConfig.services.maxVolume
                    onMoved: Audio.setVolume(value)
                }
            }
        }

        WrappedLoader {
            Layout.alignment: Qt.AlignHCenter
            shouldBeActive: Config.osd.enableMicrophone && HostState.element("micMute") && (!Config.osd.enableBrightness || !root.visibilities.session)
            targetWidth: Tokens.sizes.osd.sliderWidth
            targetHeight: Tokens.sizes.osd.sliderHeight

            sourceComponent: CustomMouseArea {
                function onWheel(event: WheelEvent) {
                    if (event.angleDelta.y > 0)
                        Audio.incrementSourceVolume();
                    else if (event.angleDelta.y < 0)
                        Audio.decrementSourceVolume();
                }

                implicitWidth: Tokens.sizes.osd.sliderWidth
                implicitHeight: Tokens.sizes.osd.sliderHeight

                FilledSlider {
                    anchors.fill: parent

                    icon: Icons.getMicVolumeIcon(value, root.sourceMuted)
                    value: root.sourceVolume
                    to: GlobalConfig.services.maxVolume
                    onMoved: Audio.setSourceVolume(value)
                }
            }
        }

        WrappedLoader {
            Layout.alignment: Qt.AlignHCenter
            shouldBeActive: Config.osd.enableBrightness && HostState.section("levels") && HostState.element("brightness")
            targetWidth: Tokens.sizes.osd.sliderWidth
            targetHeight: Tokens.sizes.osd.sliderHeight

            sourceComponent: CustomMouseArea {
                function onWheel(event: WheelEvent) {
                    const monitor = root.monitor;
                    if (!monitor)
                        return;
                    if (event.angleDelta.y > 0)
                        monitor.stepBrightness(GlobalConfig.services.brightnessIncrement);
                    else if (event.angleDelta.y < 0)
                        monitor.stepBrightness(-GlobalConfig.services.brightnessIncrement);
                }

                implicitWidth: Tokens.sizes.osd.sliderWidth
                implicitHeight: Tokens.sizes.osd.sliderHeight

                FilledSlider {
                    anchors.fill: parent

                    icon: `brightness_${(Math.round(value * 6) + 1)}`
                    value: root.brightness
                    onMoved: root.monitor?.setBrightness(value)
                }
            }
        }
    }

    component WrappedLoader: Loader {
        required property bool shouldBeActive
        property real targetWidth: 0
        property real targetHeight: 0

        asynchronous: true
        Layout.preferredWidth: shouldBeActive ? targetWidth : 0
        Layout.preferredHeight: shouldBeActive ? targetHeight : 0
        opacity: shouldBeActive ? 1 : 0
        active: opacity > 0
        visible: active

        Behavior on Layout.preferredWidth {
            Anim {
                type: Anim.Emphasized
            }
        }

        Behavior on Layout.preferredHeight {
            Anim {
                type: Anim.Emphasized
            }
        }

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }
    }
}

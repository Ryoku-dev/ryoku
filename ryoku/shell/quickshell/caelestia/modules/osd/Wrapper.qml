pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property ShellScreen screen
    required property DrawerVisibilities visibilities
    required property bool sidebarOrSessionVisible
    property bool hovered
    readonly property Brightness.Monitor monitor: Brightness.getMonitorForScreen(root.screen)
    readonly property bool shouldBeActive: visibilities.osd && Config.osd.enabled && !(visibilities.utilities && Config.utilities.enabled) && !visibilities.overview
    // what the Controls page leaves of its Levels: no slider, and no pop-up for it
    readonly property bool volumeOn: HostState.section("levels") && HostState.element("volume")
    readonly property bool brightnessOn: HostState.section("levels") && HostState.element("brightness")
    readonly property bool micOn: HostState.element("micMute")
    property real offsetScale: shouldBeActive ? 0 : 1
    property real sidebarOffset: sidebarOrSessionVisible ? 12 : 0
    property real volume
    property bool muted
    property real sourceVolume
    property bool sourceMuted
    property real brightness

    function show(): void {
        visibilities.osd = true;
        timer.restart();
    }

    function refreshBrightness(): void {
        root.brightness = root.monitor?.effectiveBrightness ?? 0;
    }

    Component.onCompleted: {
        volume = Audio.volume;
        muted = Audio.muted;
        sourceVolume = Audio.sourceVolume;
        sourceMuted = Audio.sourceMuted;
        refreshBrightness();
    }
    visible: offsetScale < 1
    anchors.leftMargin: Config.bar.position === "right" ? (-implicitWidth - 5 - sidebarOffset) * offsetScale : 0
    anchors.rightMargin: Config.bar.position !== "right" ? (-implicitWidth - 5 - sidebarOffset) * offsetScale : 0
    implicitWidth: content.implicitWidth
    implicitHeight: content.implicitHeight
    opacity: 1 - offsetScale

    Behavior on offsetScale {
        Anim {}
    }
    Connections {
        function onMutedChanged(): void {
            if (root.volumeOn)
                root.show();
            root.muted = Audio.muted;
        }
        function onVolumeChanged(): void {
            if (root.volumeOn)
                root.show();
            root.volume = Audio.volume;
        }
        function onSourceMutedChanged(): void {
            if (root.micOn)
                root.show();
            root.sourceMuted = Audio.sourceMuted;
        }
        function onSourceVolumeChanged(): void {
            if (root.micOn)
                root.show();
            root.sourceVolume = Audio.sourceVolume;
        }

        target: Audio
    }
    Connections {
        function onBrightnessChanged(): void {
            if (root.brightnessOn)
                root.show();
            root.refreshBrightness();
        }

        function onDimmingChanged(): void {
            root.refreshBrightness();
        }

        target: root.monitor
    }
    Timer {
        id: timer

        interval: root.Config.osd.hideDelay
        onTriggered: {
            if (!root.hovered)
                root.visibilities.osd = false;
        }
    }
    Loader {
        id: content

        anchors.verticalCenter: parent.verticalCenter
        anchors.left: parent.left
        asynchronous: true
        active: root.shouldBeActive || root.visible
        sourceComponent: Content {
            monitor: root.monitor
            visibilities: root.visibilities
            volume: root.volume
            muted: root.muted
            sourceVolume: root.sourceVolume
            sourceMuted: root.sourceMuted
            brightness: root.brightness
        }
    }
}

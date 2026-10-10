pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    readonly property bool showVolume: SidebarState.elementVisible("volume")
    readonly property bool showBrightness: SidebarState.elementVisible("brightness")
    property bool volumeOpen: false
    property bool brightnessOpen: false
    property int externalBrightness: -1
    property real pendingBrightness: -1
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    readonly property string outputName: screen && screen.name ? String(screen.name) : Wm.focusedOutput
    readonly property string ddcBus: {
        const displays = Devices.ddcMonitors || [];
        for (let i = 0; i < displays.length; ++i) {
            if (String(displays[i].label) === root.outputName)
                return String(displays[i].bus);
        }
        return "";
    }
    readonly property bool brightnessAvailable: ddcBus !== "" ? externalBrightness >= 0 : Devices.backlightAvailable
    readonly property real brightnessValue: pendingBrightness >= 0 ? pendingBrightness
        : ddcBus !== "" ? Math.max(0, externalBrightness) / 100
        : Math.max(0, Devices.backlightPct) / 100
    readonly property bool volumeAvailable: !!(Audio.sink && Audio.sink.audio)
    readonly property bool volumeMuted: volumeAvailable && Audio.sink.audio.muted
    readonly property real volumeValue: volumeAvailable ? Math.min(1, Audio.sink.audio.volume) : 0

    implicitHeight: stack.implicitHeight
    height: implicitHeight
    Behavior on height {
        enabled: root.motionAllowed
        NumberAnimation { duration: SidebarState.motionDuration(Tokens.swap); easing.type: Tokens.ease }
    }

    function syncActive(): void {
        if (root.active && root.showBrightness) {
            Devices.startProbes(root);
            root.readExternalBrightness();
        } else {
            root.volumeOpen = false;
            root.brightnessOpen = false;
            root.writeBrightness();
            externalRead.running = false;
            Devices.stopProbes(root);
        }
    }

    function readExternalBrightness(): void {
        if (!root.active || !root.showBrightness || root.ddcBus === "" || externalRead.running)
            return;
        externalRead.command = ["timeout", "3", "ddcutil", "getvcp", "10", "--brief", "--bus", root.ddcBus];
        externalRead.running = true;
    }

    function setBrightness(value): void {
        root.pendingBrightness = Math.max(root.ddcBus !== "" ? 0.05 : 0.01, Math.min(1, value));
        if (root.ddcBus !== "")
            root.externalBrightness = Math.round(root.pendingBrightness * 100);
        else
            Devices.backlightPct = Math.round(root.pendingBrightness * 100);
        brightnessWrite.restart();
    }

    function writeBrightness(): void {
        if (root.pendingBrightness < 0)
            return;
        const percent = Math.round(root.pendingBrightness * 100);
        if (root.ddcBus !== "") {
            Devices.setBrightness(root.ddcBus, percent);
            root.externalBrightness = percent;
        } else if (Devices.backlightAvailable) {
            Devices.setBacklight(percent);
        }
        root.pendingBrightness = -1;
        brightnessWrite.stop();
    }

    onActiveChanged: root.syncActive()
    onShowBrightnessChanged: root.syncActive()
    onShowVolumeChanged: if (!root.showVolume) root.volumeOpen = false
    onDdcBusChanged: {
        root.externalBrightness = -1;
        root.readExternalBrightness();
    }
    Component.onCompleted: root.syncActive()
    Component.onDestruction: {
        root.writeBrightness();
        Devices.stopProbes(root);
    }

    Process {
        id: externalRead
        stdout: StdioCollector {
            onStreamFinished: {
                const value = Devices.parseBrightness(text);
                if (root.active && value >= 0)
                    root.externalBrightness = value;
            }
        }
    }

    Timer {
        id: brightnessWrite
        interval: 90
        onTriggered: root.writeBrightness()
    }

    Column {
        id: stack
        width: parent.width
        spacing: Tokens.s1 * root.s

        LevelSlider {
            id: volume
            visible: root.showVolume
            width: parent.width
            s: root.s
            label: I18n.tr("Volume")
            glyph: root.volumeMuted ? "volume_off" : value <= 0 ? "volume_mute" : value < 0.5 ? "volume_down" : "volume_up"
            value: root.volumeValue
            available: root.volumeAvailable
            muted: root.volumeMuted
            muteEnabled: true
            expanded: root.volumeOpen
            onAdjusted: value => {
                if (!root.volumeAvailable)
                    return;
                Audio.sink.audio.volume = value;
                if (value > 0)
                    Audio.sink.audio.muted = false;
            }
            onMuteRequested: if (root.volumeAvailable) Audio.sink.audio.muted = !Audio.sink.audio.muted
            onExpandedRequested: expanded => {
                root.volumeOpen = expanded;
                if (expanded)
                    root.brightnessOpen = false;
            }
        }

        Item {
            id: mixerClip
            width: parent.width
            height: root.volumeOpen ? mixer.implicitHeight : 0
            opacity: root.volumeOpen ? 1 : 0
            clip: true
            Behavior on height {
                enabled: root.motionAllowed
                NumberAnimation { duration: SidebarState.motionDuration(Tokens.swap); easing.type: Tokens.ease }
            }
            Behavior on opacity {
                enabled: root.motionAllowed
                NumberAnimation { duration: SidebarState.motionDuration(Tokens.move); easing.type: Tokens.ease }
            }
            MixerDrawer {
                id: mixer
                width: parent.width
                s: root.s
                active: root.active && root.volumeOpen
            }
        }

        LevelSlider {
            id: brightness
            visible: root.showBrightness
            width: parent.width
            s: root.s
            label: I18n.tr("Brightness")
            glyph: value < 0.34 ? "brightness_low" : value < 0.67 ? "brightness_medium" : "brightness_high"
            value: root.brightnessValue
            from: root.ddcBus !== "" ? 0.05 : 0.01
            available: root.brightnessAvailable
            expanded: root.brightnessOpen
            onAdjusted: value => root.setBrightness(value)
            onExpandedRequested: expanded => {
                root.brightnessOpen = expanded;
                if (expanded)
                    root.volumeOpen = false;
            }
        }

        Item {
            id: brightnessClip
            width: parent.width
            height: root.brightnessOpen ? brightnessDrawer.implicitHeight : 0
            opacity: root.brightnessOpen ? 1 : 0
            clip: true
            Behavior on height {
                enabled: root.motionAllowed
                NumberAnimation { duration: SidebarState.motionDuration(Tokens.swap); easing.type: Tokens.ease }
            }
            Behavior on opacity {
                enabled: root.motionAllowed
                NumberAnimation { duration: SidebarState.motionDuration(Tokens.move); easing.type: Tokens.ease }
            }
            BrightnessDrawer {
                id: brightnessDrawer
                width: parent.width
                s: root.s
                screen: root.screen
                active: root.active && root.brightnessOpen
                onLevelChanged: (name, percent) => {
                    if (name === root.outputName)
                        root.externalBrightness = percent;
                }
            }
        }
    }
}
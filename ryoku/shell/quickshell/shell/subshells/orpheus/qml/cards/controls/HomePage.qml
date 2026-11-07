pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import shell.services
import "../../kit" as K

Item {
    id: page

    required property var scene
    property var card: null
    property bool shown: false
    property bool live: false

    implicitWidth: 320
    implicitHeight: content.implicitHeight

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool wifiConnected: Network.wifiPresent && Network.wifiRadio
        && Network.wifiConnectivity === "Connected"
    readonly property var bluetoothDevice: {
        if (!adapter || !adapter.enabled)
            return null;
        const devices = Bluetooth.devices ? Bluetooth.devices.values : [];
        for (let index = 0; index < devices.length; ++index) {
            const device = devices[index];
            if (device && device.connected)
                return device;
        }
        return null;
    }
    readonly property string wifiSubtitle: {
        if (!Network.wifiPresent) return "Unavailable";
        if (!Network.wifiRadio) return "Off";
        return wifiConnected ? (Network.activeSsid || "Connected") : "Not connected";
    }
    readonly property string bluetoothSubtitle: {
        if (!adapter) return "Unavailable";
        if (!adapter.enabled) return "Off";
        return bluetoothDevice ? BtLink.label(bluetoothDevice) : "Not connected";
    }
    readonly property var sinkAudio: Audio.sink && Audio.sink.audio ? Audio.sink.audio : null
    readonly property real volume: sinkAudio ? sinkAudio.volume : 0
    readonly property bool volumeMuted: sinkAudio ? sinkAudio.muted : false
    readonly property string focusedDdcBus: {
        const monitors = Devices.ddcMonitors || [];
        for (let index = 0; index < monitors.length; ++index) {
            if (monitors[index] && monitors[index].label === page.scene.screenName)
                return String(monitors[index].bus || "");
        }
        return monitors.length > 0 && monitors[0] ? String(monitors[0].bus || "") : "";
    }
    readonly property bool brightnessAvailable: Devices.backlightAvailable || focusedDdcBus !== ""
    readonly property real brightnessValue: Devices.backlightAvailable
        ? Devices.backlightPct / 100 : 0.5
    readonly property var profileOptions: (PowerProfiles.profiles || []).map(function(name) {
        if (name === "power-saver")
            return { value: name, label: "Saver", icon: "battery_saver" };
        if (name === "performance")
            return { value: name, label: "Performance", icon: "bolt" };
        return { value: name, label: "Balanced", icon: "balance" };
    })

    function setVolume(value) {
        if (!sinkAudio)
            return;
        sinkAudio.volume = Math.max(0, Math.min(1, value));
        if (sinkAudio.muted && value > 0)
            sinkAudio.muted = false;
    }

    function setBrightness(value) {
        const percent = Math.round(Math.max(0.01, Math.min(1, value)) * 100);
        if (Devices.backlightAvailable)
            Devices.setBacklight(percent);
        else if (focusedDdcBus !== "")
            Devices.setBrightness(focusedDdcBus, percent);
    }

    function setLive(enabled) {
        if (enabled === live)
            return;
        live = enabled;
        if (enabled) {
            Devices.startProbes(page);
            PowerProfiles.setActive(page, true);
            Session.watchers += 1;
        } else {
            Devices.stopProbes(page);
            PowerProfiles.setActive(page, false);
            Session.watchers = Math.max(0, Session.watchers - 1);
        }
    }

    onShownChanged: setLive(shown)
    Component.onCompleted: setLive(shown)
    Component.onDestruction: setLive(false)

    component ConnectionTile: Item {
        id: tile

        required property string icon
        required property string label
        required property string statusText
        property bool active: false
        property bool available: true
        signal toggled()
        signal opened()

        implicitHeight: 64
        opacity: available ? 1 : 0.38

        Rectangle {
            anchors.fill: parent
            radius: K.Style.plateRadius
            color: tile.active ? K.Style.bone : K.Style.plate
            border.width: K.Style.border
            border.color: tile.active ? K.Style.bone : K.Style.lineSoft

            Behavior on color {
                enabled: !K.Style.still
                ColorAnimation { duration: K.Style.snap }
            }
        }

        K.Glyph {
            anchors.left: parent.left
            anchors.leftMargin: K.Style.s3
            anchors.verticalCenter: parent.verticalCenter
            name: tile.icon
            size: 22
            tone: tile.active ? "onBone" : "dim"
            filled: tile.active
        }

        Column {
            anchors.left: parent.left
            anchors.leftMargin: K.Style.s7
            anchors.right: openArea.left
            anchors.rightMargin: K.Style.s2
            anchors.verticalCenter: parent.verticalCenter
            spacing: K.Style.s1

            K.Label {
                width: parent.width
                text: tile.label
                role: "bodyStrong"
                tone: tile.active ? "onBone" : "ink"
            }
            K.Label {
                width: parent.width
                text: tile.statusText
                role: "caption"
                tone: tile.active ? "onBone" : "muted"
            }
        }

        Item {
            id: openArea
            anchors.right: parent.right
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            width: 48

            Rectangle {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: K.Style.border
                height: 32
                color: tile.active ? K.Style.alpha(K.Style.inkOnBone, 0.22)
                    : K.Style.lineSoft
            }

            K.Glyph {
                anchors.centerIn: parent
                name: "chevron_right"
                size: K.Style.icon
                tone: tile.active ? "onBone" : "dim"
            }

            K.Press {
                enabled: tile.available
                radius: K.Style.plateRadius
                onClicked: tile.opened()
            }
        }

        Item {
            anchors.left: parent.left
            anchors.right: openArea.left
            anchors.top: parent.top
            anchors.bottom: parent.bottom

            K.Press {
                enabled: tile.available
                radius: K.Style.plateRadius
                onClicked: tile.toggled()
            }
        }
    }

    component QuickToggle: Item {
        id: toggle

        required property string icon
        required property string label
        property bool active: false
        property bool available: true
        signal toggled()

        implicitHeight: 72
        opacity: available ? 1 : 0.38
        scale: togglePress.pressScale

        Rectangle {
            anchors.fill: parent
            radius: K.Style.plateRadius
            color: toggle.active ? K.Style.bone : K.Style.plate
            border.width: K.Style.border
            border.color: toggle.active ? K.Style.bone : K.Style.lineSoft
        }

        K.Glyph {
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.top: parent.top
            anchors.topMargin: K.Style.s3
            name: toggle.icon
            size: 20
            tone: toggle.active ? "onBone" : "dim"
            filled: toggle.active
        }

        K.Label {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.bottomMargin: K.Style.s2
            text: toggle.label
            role: "caption"
            tone: toggle.active ? "onBone" : "ink"
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
        }

        K.Press {
            id: togglePress
            enabled: toggle.available
            radius: K.Style.plateRadius
            onClicked: toggle.toggled()
        }
    }

    component LevelRow: Item {
        id: level

        required property string icon
        required property string label
        property real value: 0
        property bool available: true
        property bool muted: false
        property bool toggleEnabled: false
        signal adjusted(real value)
        signal iconClicked()
        signal opened()

        implicitHeight: 56
        opacity: available ? 1 : 0.38

        Item {
            id: levelIcon
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 40
            height: 40

            Rectangle {
                anchors.fill: parent
                radius: K.Style.radius
                color: level.muted ? K.Style.bone : K.Style.plate
                border.width: K.Style.border
                border.color: level.muted ? K.Style.bone : K.Style.lineSoft
            }

            K.Glyph {
                anchors.centerIn: parent
                name: level.icon
                size: 20
                tone: level.muted ? "onBone" : "dim"
            }

            K.Press {
                enabled: level.available && level.toggleEnabled
                radius: K.Style.radius
                onClicked: level.iconClicked()
            }
        }

        K.Label {
            anchors.left: levelIcon.right
            anchors.leftMargin: K.Style.s3
            anchors.top: parent.top
            anchors.topMargin: K.Style.s1
            text: level.label
            role: "bodyStrong"
        }

        K.Label {
            anchors.right: levelMore.left
            anchors.rightMargin: K.Style.s2
            anchors.top: parent.top
            anchors.topMargin: K.Style.s1
            text: Math.round(level.value * 100) + "%"
            role: "micro"
            tone: level.muted ? "faint" : "ink"
            numeric: true
        }

        K.Slider {
            anchors.left: levelIcon.right
            anchors.leftMargin: K.Style.s3
            anchors.right: levelMore.left
            anchors.rightMargin: K.Style.s2
            anchors.bottom: parent.bottom
            value: level.value
            enabled: level.available
            onMoved: value => level.adjusted(value)
            onCommitted: value => level.adjusted(value)
        }

        K.IconButton {
            id: levelMore
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            icon: "chevron_right"
            variant: "plain"
            onClicked: level.opened()
        }
    }

    Column {
        id: content

        width: parent.width
        spacing: K.Style.s4

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                text: "Connect"
                role: "micro"
            }

            ConnectionTile {
                width: parent.width
                icon: "wifi"
                label: "Wi-Fi"
                statusText: page.wifiSubtitle
                active: page.wifiConnected
                available: Network.wifiPresent
                onToggled: Toggles.toggleWifi()
                onOpened: if (page.card) page.card.go("wifi", true)
            }

            ConnectionTile {
                width: parent.width
                icon: "bluetooth"
                label: "Bluetooth"
                statusText: page.bluetoothSubtitle
                active: page.bluetoothDevice !== null
                available: !!page.adapter
                onToggled: Toggles.toggleBt()
                onOpened: if (page.card) page.card.go("bluetooth", true)
            }

            Row {
                width: parent.width
                spacing: K.Style.s2

                QuickToggle {
                    width: (parent.width - K.Style.s2 * 3) / 4
                    icon: "do_not_disturb_on"
                    label: "DND"
                    active: Toggles.dnd
                    onToggled: Toggles.toggleDnd()
                }
                QuickToggle {
                    width: (parent.width - K.Style.s2 * 3) / 4
                    icon: "nightlight"
                    label: "Night light"
                    active: Toggles.nightOn
                    onToggled: Toggles.toggleNight()
                }
                QuickToggle {
                    width: (parent.width - K.Style.s2 * 3) / 4
                    icon: "coffee"
                    label: "Awake"
                    active: Toggles.keepAwake
                    onToggled: Toggles.toggleCaffeine()
                }
                QuickToggle {
                    width: (parent.width - K.Style.s2 * 3) / 4
                    icon: Toggles.micMuted ? "mic_off" : "mic"
                    label: "Mic"
                    active: Toggles.micMuted
                    available: Audio.source !== null
                    onToggled: Toggles.toggleMic()
                }
            }
        }

        Column {
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                text: "Levels"
                role: "micro"
            }

            LevelRow {
                width: parent.width
                icon: page.volumeMuted || page.volume <= 0 ? "volume_off" : "volume_up"
                label: "Volume"
                value: page.volume
                muted: page.volumeMuted
                toggleEnabled: true
                available: page.sinkAudio !== null
                onAdjusted: value => page.setVolume(value)
                onIconClicked: if (page.sinkAudio) page.sinkAudio.muted = !page.sinkAudio.muted
                onOpened: if (page.card) page.card.go("sound", true)
            }

            LevelRow {
                visible: page.brightnessAvailable
                width: parent.width
                icon: "light_mode"
                label: "Brightness"
                value: page.brightnessValue
                available: page.brightnessAvailable
                onAdjusted: value => page.setBrightness(value)
                onOpened: if (page.card) page.card.go("display", true)
            }
        }

        Column {
            visible: PowerProfiles.available
            width: parent.width
            spacing: K.Style.s2

            K.Label {
                text: "Power"
                role: "micro"
            }

            K.Segmented {
                width: parent.width
                model: page.profileOptions
                value: PowerProfiles.profile
                onPicked: value => PowerProfiles.setProfile(value)
            }
        }

        Item {
            width: parent.width
            implicitHeight: K.Style.rowH

            Column {
                anchors.left: parent.left
                anchors.right: sessionButton.left
                anchors.rightMargin: K.Style.s3
                anchors.verticalCenter: parent.verticalCenter
                spacing: K.Style.s1

                K.Label {
                    width: parent.width
                    text: Session.user
                    role: "bodyStrong"
                }
                K.Label {
                    width: parent.width
                    text: "Up " + Session.uptimeText
                    role: "caption"
                    tone: "muted"
                    numeric: true
                }
            }

            K.IconButton {
                id: sessionButton
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                size: 40
                width: 40
                height: 40
                icon: "power_settings_new"
                variant: "outline"
                onClicked: page.scene.showCard("session", {})
            }
        }
    }
}

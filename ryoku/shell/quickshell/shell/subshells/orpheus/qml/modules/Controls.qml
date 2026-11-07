pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import shell.services
import "../kit" as K
import "../settings" as S

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: glyphRow.implicitWidth > 0
    property real pressedX: 0

    readonly property var sinkAudio: Audio.sink && Audio.sink.audio
        ? Audio.sink.audio : null
    readonly property real volume: sinkAudio
        ? Math.max(0, Math.min(1, Number(sinkAudio.volume) || 0)) : 0
    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property int connectedDevices: {
        if (!adapter || !adapter.enabled || !Bluetooth.devices)
            return 0;
        const devices = Bluetooth.devices.values;
        let count = 0;
        for (let index = 0; index < devices.length; ++index) {
            if (devices[index] && devices[index].connected)
                ++count;
        }
        return count;
    }

    implicitWidth: present ? glyphRow.implicitWidth + K.Style.hitPad * 2 : 0
    implicitHeight: K.Style.barH

    function enabledItem(name) {
        const items = S.Prefs.statusItems || [];
        return items.indexOf(name) >= 0;
    }

    function networkGlyph() {
        if (Network.kind === "ethernet")
            return "lan";
        if (Network.kind !== "wifi")
            return "signal_wifi_off";
        if (Network.level > 0.75)
            return "signal_wifi_4_bar";
        if (Network.level > 0.5)
            return "network_wifi_3_bar";
        if (Network.level > 0.25)
            return "network_wifi_2_bar";
        return "network_wifi_1_bar";
    }

    function volumeGlyph() {
        if (!sinkAudio || sinkAudio.muted || volume <= 0)
            return "volume_off";
        if (volume < 0.34)
            return "volume_mute";
        if (volume < 0.67)
            return "volume_down";
        return "volume_up";
    }

    function networkName() {
        if (Network.kind === "ethernet")
            return "Ethernet";
        if (Network.kind === "wifi")
            return Network.activeSsid || "Wi-Fi";
        return Network.wifiRadio ? "Wi-Fi disconnected" : "Wi-Fi off";
    }

    function stepVolume(steps) {
        if (!sinkAudio || steps === 0)
            return;
        sinkAudio.volume = Math.max(0, Math.min(1, volume + steps * 0.05));
    }

    function hit(item, pointX) {
        if (!item.visible)
            return false;
        const left = glyphRow.x + item.x;
        return pointX >= left && pointX < left + item.width;
    }

    function handleRightClick(pointX) {
        if (hit(wifiItem, pointX)) {
            scene.openCard("controls", root, { page: "wifi" });
        } else if (hit(bluetoothItem, pointX)) {
            scene.openCard("controls", root, { page: "bluetooth" });
        } else if (hit(volumeItem, pointX)) {
            if (sinkAudio)
                sinkAudio.muted = !sinkAudio.muted;
        } else if (hit(microphoneItem, pointX)) {
            Toggles.toggleMic();
        }
    }

    Row {
        id: glyphRow
        scale: clusterPress.pressScale

        anchors.centerIn: parent
        spacing: K.Style.barGap

        // Sizes use the conditions, not `visible`: that reads the parent's
        // visibility too, and the slot stays hidden until this has a width.
        Item {
            id: wifiItem
            readonly property bool shown: root.enabledItem("wifi")
            visible: shown
            width: shown ? K.Style.barIcon : 0
            height: K.Style.barH

            K.Glyph {
                anchors.centerIn: parent
                name: root.networkGlyph()
                size: K.Style.barIcon
                tone: Network.kind === "" ? "faint" : "dim"
                filled: Network.kind !== ""
            }
        }

        Item {
            id: bluetoothItem
            readonly property bool shown: root.enabledItem("bluetooth") && root.adapter !== null
            visible: shown
            width: shown ? K.Style.barIcon : 0
            height: K.Style.barH

            K.Glyph {
                anchors.centerIn: parent
                name: !root.adapter || !root.adapter.enabled ? "bluetooth_disabled"
                    : root.connectedDevices > 0 ? "bluetooth_connected" : "bluetooth"
                size: K.Style.barIcon
                tone: root.adapter && root.adapter.enabled ? "dim" : "faint"
                filled: root.adapter && root.adapter.enabled
            }
        }

        Item {
            id: volumeItem
            readonly property bool shown: root.enabledItem("volume")
            visible: shown
            width: shown ? K.Style.barIcon : 0
            height: K.Style.barH

            K.Glyph {
                anchors.centerIn: parent
                name: root.volumeGlyph()
                size: K.Style.barIcon
                tone: root.sinkAudio ? "dim" : "faint"
                filled: root.sinkAudio && !root.sinkAudio.muted && root.volume > 0
            }
        }

        Item {
            id: microphoneItem
            readonly property bool shown: root.enabledItem("mic") && Toggles.micMuted
            visible: shown
            width: shown ? K.Style.barIcon : 0
            height: K.Style.barH

            K.Glyph {
                anchors.centerIn: parent
                name: "mic_off"
                size: K.Style.barIcon
                tone: "alert"
                filled: true
            }
        }
    }

    K.Press {
        id: clusterPress

        radius: K.Style.radius
        onDown: mouse => root.pressedX = mouse.x
        onClicked: mouse => root.scene.toggleCard("controls", root, { page: "home" })
        onRightClicked: root.handleRightClick(root.pressedX)
        wheelSteps: true
        onWheel: steps => root.stepVolume(steps)
    }

    K.Hint {
        target: root
        text: root.networkName() + ", volume " + Math.round(root.volume * 100) + "%"
        shown: clusterPress.hovered
    }
}

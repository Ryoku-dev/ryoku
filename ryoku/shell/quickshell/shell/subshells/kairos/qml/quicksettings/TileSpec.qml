pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import shell.services
import Ryoku.Ui.Singletons

// every tile reads a live source (daemon topics, pipewire, bluez, Flags), nothing polls
QtObject {
    id: spec

    required property string tileId
    property var pane: null

    readonly property var btDevice: {
        if (!BluetoothPower.enabled)
            return null;
        var devs = Bluetooth.devices ? Bluetooth.devices.values : [];
        for (var i = 0; i < devs.length; i++)
            if (devs[i] && devs[i].connected)
                return devs[i];
        return null;
    }
    readonly property string profile: String(PowerProfiles.profile || "balanced")

    function profileLabel(name) {
        if (name === "power-saver") return I18n.tr("Power saver");
        if (name === "performance") return I18n.tr("Performance");
        return I18n.tr("Balanced");
    }

    readonly property string label: {
        switch (spec.tileId) {
        case "wifi": return I18n.tr("Wi-Fi");
        case "bluetooth": return I18n.tr("Bluetooth");
        case "dnd": return I18n.tr("Do not disturb");
        case "night": return I18n.tr("Night light");
        case "power": return I18n.tr("Power mode");
        case "mic": return I18n.tr("Microphone");
        case "awake": return I18n.tr("Keep awake");
        case "shot": return I18n.tr("Screenshot");
        case "record": return I18n.tr("Screen record");
        case "game": return I18n.tr("Game mode");
        case "clipboard": return I18n.tr("Clipboard");
        case "wallpaper": return I18n.tr("Wallpaper");
        case "tailscale": return I18n.tr("Tailscale");
        }
        return spec.tileId;
    }

    readonly property string icon: {
        switch (spec.tileId) {
        case "wifi": return !Network.wifiRadio ? "wifi_off"
            : Network.wifiConnectivity === "Connected" ? "wifi" : "wifi_find";
        case "bluetooth": return !spec.active ? "bluetooth_disabled"
            : spec.btDevice ? "bluetooth_connected" : "bluetooth";
        case "dnd": return "do_not_disturb_on";
        case "night": return "nightlight";
        case "power": return spec.profile === "power-saver" ? "energy_savings_leaf"
            : spec.profile === "performance" ? "bolt" : "balance";
        case "mic": return Toggles.micMuted ? "mic_off" : "mic";
        case "awake": return "coffee";
        case "shot": return "screenshot_region";
        case "record": return Recorder.active ? "stop_circle" : "screen_record";
        case "game": return "sports_esports";
        case "clipboard": return "content_paste";
        case "wallpaper": return "wallpaper";
        case "tailscale": return "vpn_lock";
        }
        return "help";
    }

    readonly property bool active: {
        switch (spec.tileId) {
        case "wifi": return Network.wifiRadio;
        case "bluetooth": return BluetoothPower.enabled;
        case "dnd": return Flags.dnd;
        case "night": return Nightlight.on;
        case "power": return spec.profile !== "balanced";
        case "mic": return !Toggles.micMuted;
        case "awake": return Flags.keepAwake;
        case "record": return Recorder.active;
        case "game": return Flags.gameMode;
        case "tailscale": return Tailscale.up;
        }
        return false;
    }

    readonly property bool available: {
        switch (spec.tileId) {
        case "wifi": return Network.wifiPresent;
        case "bluetooth": return BluetoothPower.available && !BluetoothPower.hardBlocked;
        case "power": return PowerProfiles.available;
        case "mic": return !!(Audio.source && Audio.source.audio);
        case "record": return Recorder.available;
        case "tailscale": return Tailscale.available;
        }
        return true;
    }

    readonly property string secondary: {
        var on = I18n.tr("On"), off = I18n.tr("Off");
        switch (spec.tileId) {
        case "wifi":
            if (!Network.wifiPresent) return I18n.tr("Unavailable");
            if (!Network.wifiRadio) return off;
            if (Network.wifiConnectivity === "Connecting") return I18n.tr("Connecting…");
            return Network.wifiConnectivity === "Connected"
                ? (Network.activeSsid || I18n.tr("Connected")) : I18n.tr("Not connected");
        case "bluetooth":
            if (BluetoothPower.hardBlocked) return I18n.tr("Blocked by a switch");
            if (!BluetoothPower.available) return I18n.tr("Unavailable");
            if (!BluetoothPower.enabled) return off;
            return spec.btDevice ? BtLink.label(spec.btDevice) : I18n.tr("No devices connected");
        case "night": return Nightlight.on ? Nightlight.temperature + " K" : off;
        case "power": return spec.profileLabel(spec.profile);
        case "mic": return Toggles.micMuted ? I18n.tr("Muted") : on;
        case "record": return Recorder.active ? Recorder.elapsedText : I18n.tr("Start");
        case "shot": return I18n.tr("Select an area");
        case "clipboard": return I18n.tr("History");
        case "wallpaper": return I18n.tr("Pick a wallpaper");
        case "tailscale":
            if (!Tailscale.available) return I18n.tr("Unavailable");
            if (Tailscale.signedOut) return I18n.tr("Signed out");
            return Tailscale.up ? I18n.tr("Connected") : off;
        }
        return spec.active ? on : off;
    }

    readonly property string detail: {
        switch (spec.tileId) {
        case "wifi": return "wifi";
        case "bluetooth": return "bluetooth";
        case "night": return "display";
        case "mic": return "sound";
        }
        return "";
    }
    readonly property bool split: spec.tileId === "wifi" || spec.tileId === "bluetooth"

    function toggle() {
        switch (spec.tileId) {
        case "wifi": Network.setWifiEnabled(!Network.wifiRadio); break;
        case "bluetooth": BluetoothPower.toggle(); break;
        case "dnd": Toggles.toggleDnd(); break;
        case "night": Toggles.toggleNight(); break;
        case "power": spec.cycleProfile(); break;
        case "mic": Toggles.toggleMic(); break;
        case "awake": Toggles.toggleCaffeine(); break;
        case "game": Toggles.toggleGame(); break;
        case "tailscale":
            if (Tailscale.signedOut) Tailscale.authorise();
            else if (Tailscale.up) Tailscale.turnOff();
            else Tailscale.turnOn();
            break;
        case "record":
            if (Recorder.active) {
                Recorder.stop();
            } else {
                spec.closePane();
                Capture.recordTarget("monitor", Recorder.recordArgs());
            }
            break;
        case "shot":
            spec.closePane();
            Capture.shoot("region");
            break;
        case "clipboard": {
            spec.closePane();
            var st = ShellState.forActive();
            if (st)
                st.clipboardOpen = true;
            break;
        }
        case "wallpaper":
            spec.closePane();
            Quickshell.execDetached(["ryogami", "wallpaper", "ui"]);
            break;
        }
    }

    function open() {
        if (spec.detail.length > 0 && spec.pane)
            spec.pane.go(spec.detail);
    }

    function cycleProfile() {
        var list = PowerProfiles.profiles || [];
        if (list.length === 0)
            return;
        var i = list.indexOf(spec.profile);
        PowerProfiles.setProfile(list[(i + 1) % list.length]);
    }

    function closePane() {
        if (spec.pane)
            spec.pane.close();
    }
}

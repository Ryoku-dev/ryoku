pragma Singleton
pragma ComponentBehavior: Bound

import Quickshell
import Ryoku.Ui.Singletons as RyokuUi
import stage.modules.common

Singleton {
    id: root

    function canonicalizeMac(deviceOrMac): string {
        let raw = "";
        if (typeof deviceOrMac === "string")
            raw = deviceOrMac;
        else if (deviceOrMac)
            raw = deviceOrMac.address ?? deviceOrMac.mac ?? "";
        return String(raw).trim().replace(/-/g, ":").toUpperCase();
    }

    function findDeviceByMac(mac: string): var {
        const canonical = root.canonicalizeMac(mac);
        if (!canonical || !BluetoothStatus.available)
            return null;
        return BluetoothStatus.connectedDevices.find(device =>
            root.canonicalizeMac(device) === canonical) ?? null;
    }

    function isAudioDevice(device): bool {
        const icon = String(device?.icon ?? "").toLowerCase();
        return icon.includes("headset") || icon.includes("headphones")
            || icon.includes("audio") || icon.includes("speaker");
    }

    function providerForDevice(device): var {
        return device?.connected && root.isAudioDevice(device) ? "bluez" : null;
    }
    function providerForMac(mac: string): var {
        return root.providerForDevice(root.findDeviceByMac(mac));
    }
    function isEnhanced(device): bool { return false; }

    readonly property var activeDevice: {
        if (!BluetoothStatus.available || !BluetoothStatus.enabled)
            return null;
        const connected = BluetoothStatus.connectedDevices ?? [];
        return connected.find(device => device?.connected && root.isAudioDevice(device))
            ?? connected.find(device => device?.connected) ?? null;
    }
    readonly property string activeProvider: root.providerForDevice(root.activeDevice) ?? ""
    readonly property bool connected: root.activeDevice !== null && root.activeProvider !== ""

    readonly property var glanceDevice: {
        if (!BluetoothStatus.available || !BluetoothStatus.enabled)
            return null;
        const connected = BluetoothStatus.connectedDevices ?? [];
        return connected.find(device => device?.connected && root.isAudioDevice(device)
            && root.primaryBatteryPercent(device) !== null) ?? null;
    }
    readonly property int glancePercent: {
        const percent = root.primaryBatteryPercent(root.glanceDevice);
        return percent === null ? -1 : percent;
    }

    function supports(device, capability: string): bool { return false; }

    function batteryInfo(device): var {
        if (!device) {
            return {
                available: false,
                aggregate: null,
                components: [],
                left: null,
                right: null,
                case: null,
                device: null
            };
        }

        const available = Boolean(device.batteryAvailable) && Number(device.battery ?? -1) >= 0;
        const level = available
            ? Math.max(0, Math.min(100, Math.round(Number(device.battery) * 100))) : null;
        const component = {
            index: 1,
            id: "device",
            label: String(device.name || Translation.tr("Device")),
            level: level,
            status: available ? "discharging" : "unknown",
            available: available,
            charging: false,
            connected: Boolean(device.connected)
        };
        return {
            available: available,
            aggregate: level,
            components: available ? [component] : [],
            left: null,
            right: null,
            case: null,
            device: component
        };
    }

    function primaryBatteryPercent(device): var {
        const info = root.batteryInfo(device);
        return info.available ? info.aggregate : null;
    }

    function noiseControl(device): var {
        return {
            available: false,
            title: Translation.tr("Noise Control"),
            modes: [],
            currentMode: "off",
            currentModeLabel: Translation.tr("Normal"),
            currentModeIcon: "hearing",
            currentState: 1,
            hasAdaptive: false,
            hasTransparency: false,
            hasAnc: false
        };
    }

    function currentNoiseMode(device): string { return "off"; }
    function setNoiseMode(deviceOrMac, modeKey: string): bool { return false; }
    function cycleNoiseMode(device): bool { return false; }
    function conversationAwareness(device): var { return { available: false, enabled: false }; }
    function setConversationAwareness(deviceOrMac, enabled: bool): bool { return false; }
    function dynamicControls(device): list<var> { return []; }
    function indicators(device): list<var> { return []; }
    function sendSliderAction(deviceOrMac, boxIndex: int, value: int, isDragging: bool): bool { return false; }
    function sendCheckAction(deviceOrMac, boxIndex: int, checkIndex: int, state: bool): bool { return false; }
    function sendRadioAction(deviceOrMac, boxIndex: int, state: int): bool { return false; }
    function openDeviceSettings(deviceOrMac): bool {
        if (!root.canonicalizeMac(deviceOrMac))
            return false;
        RyokuUi.Spawn.run(["ryoku-shell", "hub", "open", "bluetooth"]);
        return true;
    }
}

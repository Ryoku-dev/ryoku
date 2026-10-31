pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io

Singleton {
    id: root

    readonly property var adapter: {
        if (!root.cliAvailable)
            return null;
        const adapters = Bluetooth.adapters.values;
        if (adapters.length === 0)
            return null;
        const preferred = Bluetooth.defaultAdapter;
        return preferred && adapters.indexOf(preferred) >= 0 ? preferred : adapters[0];
    }
    readonly property var devices: root.adapter !== null && Bluetooth.devices
        ? Bluetooth.devices.values : []
    readonly property bool enabled: root.adapter !== null ? root.adapter.enabled : root.cliEnabled
    readonly property bool available: root.adapter !== null || root.cliAvailable || root.blocked
    readonly property bool blocked: root.softBlocked || root.hardBlocked
        || root.adapter?.state === BluetoothAdapterState.Blocked
    property bool cliAvailable: false
    property bool cliEnabled: false
    property bool softBlocked: false
    property bool hardBlocked: false

    function setEnabled(on: bool): void {
        Quickshell.execDetached(["ryoku-bluetooth-power", on ? "on" : "off"]);
    }

    function toggle(): void {
        setEnabled(!enabled);
    }

    Component.onCompleted: adapterState.running = true

    onAdapterChanged: {
        rfkillState.running = true;
        adapterRefresh.restart();
    }

    Timer {
        id: adapterRefresh
        interval: 100
        onTriggered: {
            adapterState.running = false;
            adapterState.running = true;
        }
    }

    Process {
        id: adapterState
        command: ["bluetoothctl", "--timeout", "2", "show"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.cliAvailable = text.indexOf("Controller") >= 0
                    && text.indexOf("No default controller") < 0;
                root.cliEnabled = /Powered:[^\\n]*yes/i.test(text);
            }
        }
    }

    Process {
        id: adapterMonitor
        command: [
            "dbus-monitor", "--system",
            "type='signal',sender='org.bluez',interface='org.freedesktop.DBus.ObjectManager'",
            "type='signal',sender='org.bluez',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged',arg0='org.bluez.Adapter1'",
            "type='signal',sender='org.freedesktop.DBus',interface='org.freedesktop.DBus',member='NameOwnerChanged',arg0='org.bluez'"
        ]
        running: true
        stdout: SplitParser {
            onRead: adapterRefresh.restart()
        }
        onExited: adapterMonitorRestart.restart()
    }

    Timer {
        id: adapterMonitorRestart
        interval: 1000
        onTriggered: if (!adapterMonitor.running) adapterMonitor.running = true
    }

    Process {
        id: rfkillState
        command: ["rfkill", "-n", "-o", "TYPE,SOFT,HARD", "list", "bluetooth"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                const rows = text.trim().split("\n").map(line => line.trim().split(/\s+/))
                    .filter(fields => fields[0] === "bluetooth");
                root.softBlocked = rows.some(fields => fields[1] === "blocked");
                root.hardBlocked = rows.some(fields => fields[2] === "blocked");
            }
        }
    }
}

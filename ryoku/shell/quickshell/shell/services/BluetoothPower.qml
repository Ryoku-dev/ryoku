pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io

Singleton {
    id: root

    readonly property bool enabled: Bluetooth.defaultAdapter?.enabled ?? false
    readonly property bool available: Bluetooth.defaultAdapter !== null || root.blocked
    readonly property bool blocked: root.softBlocked || root.hardBlocked
        || Bluetooth.defaultAdapter?.state === BluetoothAdapterState.Blocked
    property bool softBlocked: false
    property bool hardBlocked: false

    function setEnabled(on: bool): void {
        Quickshell.execDetached(["ryoku-bluetooth-power", on ? "on" : "off"]);
    }

    function toggle(): void {
        setEnabled(!enabled);
    }

    Connections {
        target: Bluetooth
        function onDefaultAdapterChanged(): void {
            rfkillState.running = true;
        }
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

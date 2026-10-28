pragma Singleton
pragma ComponentBehavior: Bound

import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io
import QtQuick
import stage

Singleton {
    id: root

    readonly property bool available: Bluetooth.adapters.values.length > 0 || root.blocked
    readonly property bool enabled: Bluetooth.defaultAdapter?.enabled ?? false
    readonly property BluetoothDevice firstActiveDevice: Bluetooth.defaultAdapter?.devices.values.find(device => device.connected) ?? null
    readonly property int activeDeviceCount: Bluetooth.defaultAdapter?.devices.values.filter(device => device.connected).length ?? 0
    readonly property bool connected: Bluetooth.devices.values.some(d => d.connected)

    readonly property BluetoothAdapter adapter: Bluetooth.defaultAdapter
    readonly property string adapterId: root.adapter?.adapterId ?? ""
    readonly property string adapterName: root.adapter?.name ?? ""
    /**
     * True while any Bluetooth radio is killswitched. Platform rfkill switches
     * can remove the BlueZ adapter entirely, so the kernel state remains
     * authoritative while there is no `defaultAdapter`.
     */
    readonly property bool blocked: root.softBlocked || root.hardBlocked
        || root.adapter?.state === BluetoothAdapterState.Blocked
    property bool softBlocked: false
    property bool hardBlocked: false
    readonly property bool discovering: root.adapter?.discovering ?? false
    /** BlueZ publishes the adapter's address, but Quickshell does not expose it. */
    property string adapterAddress: ""

    function startDiscovery(): void {
        if (!root.adapter || !root.enabled || root.adapter.discovering)
            return;
        root.adapter.discovering = true;
    }

    function stopDiscovery(): void {
        if (!root.adapter?.discovering)
            return;
        root.adapter.discovering = false;
    }

    onAdapterChanged: rfkillState.running = true
    onAdapterIdChanged: root.readAdapterAddress()
    Component.onCompleted: {
        rfkillState.running = true;
        root.readAdapterAddress();
        root.resortDeviceLists();
    }

    function readAdapterAddress(): void {
        root.adapterAddress = "";
        if (root.adapterId.length === 0)
            return;
        // The path is built here rather than bound: a binding is re-evaluated
        // after `running` is set, so the process would start on the previous
        // adapter id — an empty one, the first time round.
        adapterAddressProc.command = ["busctl", "--system", "get-property", "org.bluez",
            `/org/bluez/${root.adapterId}`, "org.bluez.Adapter1", "Address"];
        adapterAddressProc.running = true;
    }

    Process {
        id: rfkillState
        command: ["rfkill", "-n", "-o", "TYPE,SOFT,HARD", "list", "bluetooth"]
        stdout: StdioCollector {
            onStreamFinished: {
                const rows = text.trim().split("\n").map(line => line.trim().split(/\s+/))
                    .filter(fields => fields[0] === "bluetooth");
                root.softBlocked = rows.some(fields => fields[1] === "blocked");
                root.hardBlocked = rows.some(fields => fields[2] === "blocked");
            }
        }
    }

    Process {
        id: adapterAddressProc
        stdout: StdioCollector {
            onStreamFinished: {
                const match = text.match(/"([0-9A-Fa-f:]+)"/);
                root.adapterAddress = match ? match[1].toUpperCase() : "";
            }
        }
    }

    // === Power control ===
    function setEnabled(on: bool): void {
        Quickshell.execDetached(["ryoku-bluetooth-power", on ? "on" : "off"]);
    }

    function toggle(): void {
        root.setEnabled(!root.enabled);
    }

    // === Connection tracking ===
    signal deviceConnected(BluetoothDevice device)
    signal deviceDisconnected(BluetoothDevice device)

    onDeviceConnected: device => SoundService.playEvent("devices", "device-added")
    onDeviceDisconnected: device => SoundService.playEvent("devices", "device-removed")

    property var _previousConnectedAddresses: []
    property bool _initialized: false

    // The timer below alone left a connection unannounced for up to 2 s, so the island's
    // connection strip landed after the earbuds bubble - and after the keyboard a headset's
    // AVRCP device registers. The count already watches every device's `connected`.
    onActiveDeviceCountChanged: Qt.callLater(root._checkConnectionChanges)

    // perf: this recompute+connection-check ran at a flat 500 ms (2 Hz) forever
    // while the radio was on, even at idle with nothing observing the lists. The
    // connect/disconnect *sounds* must keep working with no UI open, so the timer
    // stays running — but only ticks fast (500 ms) when someone is actually
    // looking (sidebar dashboard / Settings) or a scan is in progress. Otherwise
    // it drops to 2 s, which still catches connections promptly enough for the
    // sound cue while cutting idle wakeups 4×. The bar's status dot reads plain
    // `connected`/`enabled` bindings and needs no fast resort.
    Timer {
        interval: (GlobalStates.dashboardPanelOpen || GlobalStates.settingsOpen || root.discovering) ? 500 : 2000
        running: root.enabled
        repeat: true
        onTriggered: root._checkConnectionChanges()
    }

    function _checkConnectionChanges() {
        root.resortDeviceLists();

        const currentConnected = Bluetooth.devices.values.filter(d => d.connected);
        const currentAddresses = currentConnected.map(d => d.address);

        // Skip initial snapshot to avoid false positives on startup
        if (!_initialized) {
            _previousConnectedAddresses = currentAddresses;
            _initialized = true;
            return;
        }

        // Find newly connected devices
        for (const device of currentConnected) {
            if (!_previousConnectedAddresses.includes(device.address)) {
                root.deviceConnected(device);
            }
        }

        // Find disconnected devices
        for (const addr of _previousConnectedAddresses) {
            if (!currentAddresses.includes(addr)) {
                const device = Bluetooth.devices.values.find(d => d.address === addr);
                if (device) root.deviceDisconnected(device);
            }
        }

        _previousConnectedAddresses = currentAddresses;
    }

    function sortFunction(a, b) {
        // Ones with meaningful names before MAC addresses
        const macRegex = /^([0-9A-Fa-f]{2}-){5}[0-9A-Fa-f]{2}$/;
        const aIsMac = macRegex.test(a.name);
        const bIsMac = macRegex.test(b.name);
        if (aIsMac !== bIsMac)
            return aIsMac ? 1 : -1;

        // Alphabetical by name
        return a.name.localeCompare(b.name);
    }
    // These used to be live bindings reading `.connected`/`.paired`/`.name` off
    // every known device, which subscribes each list to every device's state.
    // During active discovery BlueZ can report several new or changed devices
    // a second, so each one forced three full filter+sort passes and handed
    // every listener (the Bluetooth tab's three Repeaters, the sidebar dialog,
    // quick toggles...) a brand-new array — freezing the tab the moment it
    // started scanning. Recomputed explicitly instead, on the same 500ms
    // cadence as the connection tracking above.
    property list<var> connectedDevices: []
    property list<var> pairedButNotConnectedDevices: []
    property list<var> unpairedDevices: []
    property list<var> friendlyDeviceList: []

    /**
     * The first connected phone, for the island's phone bubble. BlueZ names it by icon
     * ("phone"); "audio-headphones" contains the same word, hence the second test.
     */
    readonly property var phoneDevice: root.connectedDevices.find(d => {
        const icon = d?.icon ?? "";
        return icon.includes("phone") && !icon.includes("headphones");
    }) ?? null

    function resortDeviceLists(): void {
        const values = Bluetooth.devices.values;
        root.connectedDevices = values.filter(d => d.connected).sort(root.sortFunction);
        root.pairedButNotConnectedDevices = values.filter(d => d.paired && !d.connected).sort(root.sortFunction);
        root.unpairedDevices = values.filter(d => !d.paired && !d.connected).sort(root.sortFunction);
        root.friendlyDeviceList = [
            ...root.connectedDevices,
            ...root.pairedButNotConnectedDevices,
            ...root.unpairedDevices
        ];
    }
}

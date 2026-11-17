pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import Quickshell.Io
import Ryoku.Ui as RU
import shell.services
import "../../kit" as K
import "btrows.js" as Rows

Item {
    id: page

    required property var scene
    property var card: null
    property bool shown: false
    property string pairingAddress: ""
    property string pairError: ""

    implicitWidth: 320
    implicitHeight: content.implicitHeight

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool powered: !!adapter && adapter.enabled
    readonly property bool scanning: powered && adapter.discovering
    readonly property bool busy: pairProcess.running
    readonly property var byAddress: {
        const out = ({});
        if (!powered || !Bluetooth.devices)
            return out;
        for (const device of (Bluetooth.devices.values || []))
            if (device && device.address)
                out[device.address] = device;
        return out;
    }
    // a device with only an address for a name stays out of the list until it is named
    readonly property var wanted: {
        const out = [];
        for (const address in page.byAddress) {
            const device = page.byAddress[address];
            const saved = device.paired || device.bonded;
            const label = RU.BtName.label(device);
            if (device.connected || saved || !RU.BtName.looksLikeAddress(label))
                out.push({ address: address, label: label,
                    rank: device.connected ? 0 : (saved ? 1 : 2) });
        }
        return out;
    }
    onWantedChanged: syncRows()

    // keyed by address, so a row under the pointer survives the scan filling in
    ListModel { id: deviceRows }

    function syncRows() {
        const current = [];
        for (let i = 0; i < deviceRows.count; i++)
            current.push(deviceRows.get(i).address);
        const keys = Rows.order(current, page.wanted);
        for (let i = deviceRows.count - 1; i >= 0; i--)
            if (keys.indexOf(deviceRows.get(i).address) < 0)
                deviceRows.remove(i);
        for (let j = 0; j < keys.length; j++) {
            let at = -1;
            for (let k = j; k < deviceRows.count; k++) {
                if (deviceRows.get(k).address === keys[j]) {
                    at = k;
                    break;
                }
            }
            if (at < 0)
                deviceRows.insert(j, { address: keys[j] });
            else if (at !== j)
                deviceRows.move(at, j, 1);
        }
    }

    function updateDiscovery() {
        BluetoothDiscovery.setDiscovering(page, adapter, shown && powered);
    }

    function activate(device) {
        if (!device || busy)
            return;
        if (device.connected) {
            device.disconnect();
            return;
        }
        const address = String(device.address || "");
        if (!/^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/.test(address))
            return;
        pairError = "";
        pairingAddress = address;
        pairProcess.command = BtLink.linkCommand(address);
        pairProcess.running = false;
        pairProcess.running = true;
    }

    function subtitle(device) {
        if (!device)
            return "";
        return device.connected ? "Connected"
            : ((device.paired || device.bonded) ? "Saved" : BtLink.typeLabel(device));
    }

    onShownChanged: updateDiscovery()
    onPoweredChanged: updateDiscovery()
    onAdapterChanged: updateDiscovery()
    Component.onCompleted: {
        syncRows();
        updateDiscovery();
    }
    Component.onDestruction: BluetoothDiscovery.setDiscovering(page, adapter, false)

    Process {
        id: pairProcess
        command: []
        stdout: StdioCollector { id: pairOutput }
        onExited: code => {
            page.pairingAddress = "";
            if (code === 0) {
                page.pairError = "";
            } else {
                const lines = String(pairOutput.text || "").trim().split("\n");
                page.pairError = lines.length > 0 && lines[lines.length - 1].length > 0
                    ? lines[lines.length - 1] : "Could not connect";
            }
            Qt.callLater(page.updateDiscovery);
        }
    }

    Column {
        id: content

        width: parent.width
        spacing: K.Style.s3

        K.Label {
            visible: page.pairError.length > 0
            width: parent.width
            text: page.pairError
            role: "caption"
            tone: "alert"
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }

        K.Label {
            visible: !page.powered
            width: parent.width
            text: page.adapter
                ? "Turn Bluetooth on to see nearby devices."
                : "No Bluetooth adapter found."
            role: "body"
            tone: "variant"
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
        }

        Column {
            visible: page.powered
            width: parent.width
            spacing: K.Style.s2

            Repeater {
                model: deviceRows

                delegate: K.ListRow {
                    id: deviceRow

                    required property int index
                    required property string address
                    readonly property var device: page.byAddress[address] || null
                    readonly property bool pairing: page.busy
                        && page.pairingAddress === address
                    readonly property int batteryLevel: BtLink.batteryLevel(device)

                    width: parent.width
                    icon: BtLink.glyphFor(device)
                    title: BtLink.label(device)
                    subtitle: pairing ? "Connecting…" : page.subtitle(device)
                    selected: !!device && device.connected
                    enabled: !page.busy
                    divider: index > 0
                    onClicked: page.activate(device)

                    K.Label {
                        visible: deviceRow.batteryLevel >= 0
                        text: deviceRow.batteryLevel + "%"
                        role: "caption"
                        tone: deviceRow.selected ? "onBone" : "dim"
                        numeric: true
                    }

                    K.Button {
                        z: 2
                        text: deviceRow.pairing ? "Connecting…"
                            : (deviceRow.device && deviceRow.device.connected
                                ? "Disconnect" : "Connect")
                        variant: deviceRow.selected ? "filled" : "outline"
                        enabled: !page.busy
                        onClicked: page.activate(deviceRow.device)
                    }
                }
            }

            K.Label {
                visible: deviceRows.count === 0
                width: parent.width
                text: page.scanning
                    ? "Put a device in pairing mode."
                    : "No Bluetooth devices found."
                role: "body"
                tone: "variant"
            }
        }
    }
}

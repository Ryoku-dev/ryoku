pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import Quickshell.Io
import shell.services
import "../../kit" as K

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
    readonly property var devices: {
        if (!powered || !Bluetooth.devices)
            return [];
        const values = (Bluetooth.devices.values || []).filter(function(device) {
            return device && (device.connected || device.paired || device.bonded
                || String(device.name || "").length > 0
                || String(device.deviceName || "").length > 0);
        });
        values.sort(function(left, right) {
            const leftRank = left.connected ? 0 : (left.paired || left.bonded ? 1 : 2);
            const rightRank = right.connected ? 0 : (right.paired || right.bonded ? 1 : 2);
            if (leftRank !== rightRank)
                return leftRank - rightRank;
            return BtLink.label(left).localeCompare(BtLink.label(right));
        });
        return values;
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
    Component.onCompleted: updateDiscovery()
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
                model: page.devices

                delegate: K.ListRow {
                    id: deviceRow

                    required property int index
                    required property var modelData
                    readonly property string address: String(modelData && modelData.address || "")
                    readonly property bool pairing: page.busy
                        && page.pairingAddress === address
                    readonly property int batteryLevel: BtLink.batteryLevel(modelData)

                    width: parent.width
                    icon: BtLink.glyphFor(modelData)
                    title: BtLink.label(modelData)
                    subtitle: pairing ? "Connecting…" : page.subtitle(modelData)
                    selected: !!modelData && modelData.connected
                    enabled: !page.busy
                    divider: index > 0
                    onClicked: page.activate(modelData)

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
                            : (deviceRow.modelData && deviceRow.modelData.connected
                                ? "Disconnect" : "Connect")
                        variant: deviceRow.selected ? "filled" : "outline"
                        enabled: !page.busy
                        onClicked: page.activate(deviceRow.modelData)
                    }
                }
            }

            K.Label {
                visible: page.devices.length === 0
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

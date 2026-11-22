pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Lithic
import shell.services
import Ryoku.Ui.Singletons
import "../../ask/look"
import ".."

Item {
    id: root

    required property real s
    required property bool active
    signal backRequested()

    readonly property var adapter: BluetoothPower.adapter
    readonly property bool adapterAvailable: BluetoothPower.available
    readonly property bool adapterEnabled: BluetoothPower.enabled
    readonly property bool scanning: root.adapter !== null && root.adapterEnabled && root.adapter.discovering
    property var discoveryAdapter: null
    property string busyAddress: ""
    property string errorText: ""
    readonly property bool busy: linkProcess.running

    readonly property var devices: root.adapterEnabled
        ? BluetoothPower.devices.filter(device => !!device) : []
    readonly property var connectedDevices: root.devices.filter(device => device && device.connected)
    readonly property var pairedDevices: root.devices.filter(device => device && !device.connected && (device.paired || device.bonded))
    readonly property var discoveredDevices: root.devices.filter(device => device && !device.connected && !device.paired && !device.bonded)

    implicitHeight: content.implicitHeight
    height: implicitHeight

    function syncDiscovery(): void {
        const wanted = root.active && root.adapterEnabled;
        if (root.discoveryAdapter && (!wanted || root.discoveryAdapter !== root.adapter)) {
            BluetoothDiscovery.setDiscovering(root, root.discoveryAdapter, false);
            root.discoveryAdapter = null;
        }
        if (wanted && root.adapter && root.discoveryAdapter !== root.adapter) {
            root.discoveryAdapter = root.adapter;
            BluetoothDiscovery.setDiscovering(root, root.adapter, true);
        } else if (wanted && root.adapter) {
            BluetoothDiscovery.setDiscovering(root, root.adapter, true);
        }
    }

    function stopDiscovery(): void {
        if (root.discoveryAdapter)
            BluetoothDiscovery.setDiscovering(root, root.discoveryAdapter, false);
        root.discoveryAdapter = null;
    }

    function link(device): void {
        if (!device || root.busy)
            return;
        const address = String(device.address || "");
        if (!/^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/.test(address)) {
            root.errorText = I18n.tr("This device did not provide a usable Bluetooth address.");
            return;
        }
        root.errorText = "";
        root.busyAddress = address;
        linkProcess.command = BtLink.linkCommand(address);
        linkProcess.running = false;
        linkProcess.running = true;
    }

    function primaryAction(device): void {
        if (!device || root.busy)
            return;
        if (device.connected)
            device.disconnect();
        else
            root.link(device);
    }

    function deviceStatus(device): string {
        if (!device)
            return "";
        const battery = BtLink.batteryLevel(device);
        let status = device.connected ? I18n.tr("Connected")
            : (device.paired || device.bonded) ? I18n.tr("Paired")
            : BtLink.typeLabel(device);
        if (device.connected && battery >= 0)
            status += " · " + I18n.tr("%1% battery").arg(battery);
        if (device.trusted)
            status += " · " + I18n.tr("Trusted");
        return status;
    }

    onActiveChanged: root.syncDiscovery()
    onAdapterChanged: root.syncDiscovery()
    onAdapterEnabledChanged: root.syncDiscovery()
    Component.onCompleted: root.syncDiscovery()
    Component.onDestruction: root.stopDiscovery()

    Process {
        id: linkProcess
        command: []
        stdout: StdioCollector { id: linkOutput }
        stderr: StdioCollector { id: linkError }
        onExited: code => {
            root.busyAddress = "";
            if (code === 0) {
                root.errorText = "";
                return;
            }
            const combined = String(linkOutput.text || "") + "\n" + String(linkError.text || "");
            const lines = combined.trim().split("\n").filter(line => line.trim().length > 0);
            root.errorText = lines.length > 0 ? lines[lines.length - 1].trim()
                : I18n.tr("Could not connect to this device.");
        }
    }

    component DeviceRow: PageRow {
        id: deviceRow
        required property var device
        readonly property bool known: !!deviceRow.device && (deviceRow.device.paired || deviceRow.device.bonded)
        readonly property bool linked: !!deviceRow.device && deviceRow.device.connected
        readonly property bool connecting: root.busy
            && root.busyAddress === String(deviceRow.device ? deviceRow.device.address : "")

        s: root.s
        plated: true
        plateOn: deviceRow.linked
        glyph: BtLink.glyphFor(deviceRow.device)
        title: BtLink.label(deviceRow.device)
        meta: root.deviceStatus(deviceRow.device)

        CornerButton {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            subtle: true
            visible: deviceRow.known
            glyph: deviceRow.device && deviceRow.device.trusted ? "verified" : "verified_off"
            Accessible.name: deviceRow.device && deviceRow.device.trusted ? I18n.tr("Untrust") : I18n.tr("Trust")
            enabled: !root.busy
            onClicked: if (deviceRow.device) deviceRow.device.trusted = !deviceRow.device.trusted
        }
        CornerButton {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            subtle: true
            visible: deviceRow.known && !deviceRow.linked
            glyph: "delete"
            Accessible.name: I18n.tr("Forget")
            enabled: !root.busy
            onClicked: if (deviceRow.device) deviceRow.device.forget()
        }
        SidebarButton {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            compact: true
            text: deviceRow.connecting ? I18n.tr("Connecting…")
                : deviceRow.linked ? I18n.tr("Disconnect")
                : deviceRow.known ? I18n.tr("Connect")
                : I18n.tr("Pair")
            armed: !root.busy
            onAct: root.primaryAction(deviceRow.device)
        }
    }

    component DeviceGroup: Column {
        id: group
        required property string title
        required property var members
        required property string empty

        visible: root.adapterEnabled
        spacing: Math.round(8 * root.s)

        Item {
            width: group.width
            height: groupTitle.implicitHeight

            Micro {
                id: groupTitle
                anchors.left: parent.left
                anchors.right: groupCount.left
                anchors.rightMargin: Math.round(8 * root.s)
                s: root.s
                text: group.title
            }
            Micro {
                id: groupCount
                anchors.right: parent.right
                visible: group.members.length > 0
                s: root.s
                text: String(group.members.length)
                font.features: ({ "tnum": 1 })
            }
        }

        Column {
            id: list
            width: group.width
            spacing: QsLook.material || QsLook.fluent ? Math.round(2 * root.s) : 0

            Repeater {
                model: group.members
                delegate: DeviceRow {
                    required property var modelData
                    required property int index
                    width: list.width
                    device: modelData
                    first: index === 0
                    last: index === group.members.length - 1
                }
            }
        }

        Text {
            visible: group.members.length === 0
            width: group.width
            text: group.empty
            color: Lithic.fg3
            font.family: QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }
    }

    Column {
        id: content
        width: root.width
        spacing: Math.round(QsLook.gap * root.s)

        RadioRow {
            width: parent.width
            s: root.s
            text: I18n.tr("Bluetooth")
            detail: BluetoothPower.blocked ? I18n.tr("Blocked")
                : !root.adapterAvailable ? I18n.tr("No Bluetooth adapter")
                : !root.adapterEnabled ? I18n.tr("Off")
                : root.connectedDevices.length > 0 ? I18n.tr("%1 connected").arg(root.connectedDevices.length)
                : I18n.tr("On")
            glyph: root.adapterEnabled ? "bluetooth" : "bluetooth_disabled"
            on: root.adapterEnabled
            available: root.adapterAvailable
            searching: root.scanning && root.active
            canSearch: root.adapterEnabled
            onToggleRequested: BluetoothPower.toggle()
            onSearchRequested: {
                root.stopDiscovery();
                Qt.callLater(root.syncDiscovery);
            }
        }

        PageSearching {
            visible: root.adapterEnabled
            width: parent.width
            s: root.s
            running: root.scanning && root.active
            text: root.scanning ? I18n.tr("Looking for devices. Put the device in pairing mode and keep it nearby.")
                : I18n.tr("Discovery pauses when you leave this page.")
        }

        Text {
            visible: !root.adapterEnabled
            width: parent.width
            text: BluetoothPower.blocked ? I18n.tr("Bluetooth is blocked")
                : !root.adapterAvailable ? I18n.tr("This machine has no Bluetooth adapter")
                : I18n.tr("Bluetooth is off")
            color: Lithic.fg2
            font.family: QsLook.sans
            font.pixelSize: Math.round(13 * root.s)
            wrapMode: Text.WordWrap
            renderType: Text.NativeRendering
        }

        Text {
            visible: root.errorText !== ""
            width: parent.width
            text: root.errorText
            textFormat: Text.PlainText
            color: Lithic.jasper
            font.family: QsLook.sans
            font.pixelSize: Math.round(12 * root.s)
            font.weight: Font.Medium
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            renderType: Text.NativeRendering
        }

        DeviceGroup {
            width: parent.width
            title: I18n.tr("Connected")
            members: root.connectedDevices
            empty: I18n.tr("No devices are connected.")
        }
        DeviceGroup {
            width: parent.width
            title: I18n.tr("Paired")
            members: root.pairedDevices
            empty: I18n.tr("Paired devices stay here when they are disconnected.")
        }
        DeviceGroup {
            width: parent.width
            title: I18n.tr("Nearby")
            members: root.discoveredDevices
            empty: root.scanning ? I18n.tr("Nothing found yet.") : I18n.tr("No nearby devices found.")
        }
    }
}

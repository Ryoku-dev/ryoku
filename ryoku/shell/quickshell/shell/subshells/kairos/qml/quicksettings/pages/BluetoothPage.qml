pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io
import shell.services
import Ryoku.Ui.Singletons
import "../../kit"
import "../Segments.js" as Segments

Column {
    id: page

    property var host

    spacing: 8

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool btOn: page.adapter !== null && page.adapter.enabled
    readonly property bool scanning: page.btOn && page.adapter.discovering
    property bool wantScan: true

    readonly property var devices: {
        if (!page.btOn || !Bluetooth.devices) return [];
        var vals = Bluetooth.devices.values;
        var out = [];
        for (var i = 0; i < vals.length; i++) {
            var d = vals[i];
            if (!d) continue;
            if (d.connected || d.paired || d.bonded
                    || (d.name && d.name.length > 0)
                    || (d.deviceName && d.deviceName.length > 0)) out.push(d);
        }
        return out;
    }
    readonly property var connectedList: page.devices.filter(function (d) { return d.connected; })
    readonly property var savedList: page.devices.filter(function (d) { return (d.paired || d.bonded) && !d.connected; })
    readonly property var nearbyList: page.devices.filter(function (d) { return !d.paired && !d.bonded && !d.connected; })

    property string pairingAddr: ""
    property string pairError: ""
    readonly property bool busy: pairProc.running

    function glyphFor(d) {
        var g = BtLink.glyphFor(d);
        if (g === "gamepad") return "sports_esports";
        if (g === "phone") return "smartphone";
        if (g === "monitor") return "computer";
        return g;
    }
    function batteryGlyph(pct) {
        if (pct >= 95) return "battery_full";
        return "battery_" + Math.max(0, Math.min(6, Math.round(pct / 100 * 6))) + "_bar";
    }
    function statusOf(d) {
        if (!d) return "";
        if (page.busy && page.pairingAddr === String(d.address || "")) return I18n.tr("Connecting\u2026");
        if (d.connected) return I18n.tr("Connected");
        return BtLink.typeLabel(d);
    }
    function link(d) {
        if (!d || page.busy) return;
        var mac = String(d.address || "");
        if (!/^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/.test(mac)) return;
        page.pairError = "";
        page.pairingAddr = mac;
        pairProc.command = BtLink.linkCommand(mac);
        pairProc.running = false;
        pairProc.running = true;
    }
    function disconnect(d) {
        if (!d || page.busy) return;
        page.pairError = "";
        d.disconnect();
    }
    property string forgetArmed: ""
    // a device gone stale on the other end only pairs again once this side forgets it too
    function forget(d) {
        if (!d || page.busy) return;
        var key = String(d.address || "");
        if (page.forgetArmed !== key) {
            page.forgetArmed = key;
            disarmForget.restart();
            return;
        }
        page.forgetArmed = "";
        page.pairError = "";
        d.forget();
    }
    Timer {
        id: disarmForget
        interval: 3000
        onTriggered: page.forgetArmed = ""
    }
    function syncScan() {
        BluetoothDiscovery.setDiscovering(page, page.adapter, page.btOn && page.wantScan && !page.busy);
    }

    Process {
        id: pairProc
        command: []
        stdout: StdioCollector { id: pairOut }
        onExited: (code) => {
            page.pairingAddr = "";
            if (code === 0) return;
            var lines = String(pairOut.text || "").trim().split("\n");
            page.pairError = lines.length > 0 && lines[lines.length - 1].length > 0
                ? lines[lines.length - 1] : I18n.tr("Could not connect");
        }
    }

    Component.onCompleted: page.syncScan()
    Component.onDestruction: BluetoothDiscovery.setDiscovering(page, page.adapter, false)
    onBtOnChanged: page.syncScan()
    onWantScanChanged: page.syncScan()
    // the link helper stops every scan while it negotiates
    onBusyChanged: if (!page.busy) page.syncScan()

    component DeviceRow: ListItem {
        id: row
        required property var modelData
        required property int index
        property int count: 1
        readonly property int battery: BtLink.batteryLevel(row.modelData)
        width: parent ? parent.width : 0
        position: Segments.position(row.index, row.count)
        selected: row.modelData.connected === true
        clickable: !row.modelData.connected && !page.busy
        icon: page.glyphFor(row.modelData)
        headline: BtLink.label(row.modelData)
        supporting: page.statusOf(row.modelData)
        onClicked: page.link(row.modelData)

        Row {
            anchors.verticalCenter: parent.verticalCenter
            visible: row.battery >= 0
            spacing: 2

            Icon {
                anchors.verticalCenter: parent.verticalCenter
                name: page.batteryGlyph(row.battery)
                size: 18
                filled: true
                color: row.text
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                role: "caption"
                font.family: Md.mono
                color: row.text
                text: row.battery + "%"
            }
        }
        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            visible: row.modelData.connected === true
            enabled: !page.busy
            icon: "link_off"
            iconSize: 20
            tip: I18n.tr("Disconnect")
            onClicked: page.disconnect(row.modelData)
        }
        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            visible: row.modelData.paired === true || row.modelData.bonded === true
            enabled: !page.busy
            readonly property bool armed: page.forgetArmed === String(row.modelData.address || "")
            icon: armed ? "delete_forever" : "delete"
            iconSize: 20
            danger: true
            checked: armed
            tip: armed ? I18n.tr("Press again to forget") : I18n.tr("Forget")
            onClicked: page.forget(row.modelData)
        }
    }

    TopBar {
        width: parent.width
        title: I18n.tr("Bluetooth")
        onBack: page.host.back()

        IconButton {
            anchors.verticalCenter: parent.verticalCenter
            visible: page.btOn
            enabled: !page.busy
            icon: "bluetooth_searching"
            checked: page.scanning
            tip: page.scanning ? I18n.tr("Stop scanning") : I18n.tr("Scan for devices")
            onClicked: {
                page.wantScan = !page.scanning;
                page.syncScan();
            }
        }
        Switch {
            anchors.verticalCenter: parent.verticalCenter
            visible: BluetoothPower.available
            enabled: !BluetoothPower.hardBlocked
            checked: page.adapter ? page.adapter.enabled : false
            onToggled: v => BluetoothPower.setEnabled(v)
        }
    }

    Rectangle {
        width: parent.width
        visible: page.pairError.length > 0
        height: errorRow.implicitHeight + 24
        radius: Md.rXl
        color: Md.alpha(Md.error, 0.16)

        Row {
            id: errorRow
            anchors { left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter; leftMargin: 18; rightMargin: 8 }
            spacing: 12

            Icon {
                anchors.verticalCenter: parent.verticalCenter
                name: "error"
                size: 20
                color: Md.error
            }
            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - 20 - dismiss.width - 2 * parent.spacing
                role: "body"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                maximumLineCount: 3
                text: page.pairError
            }
            IconButton {
                id: dismiss
                anchors.verticalCenter: parent.verticalCenter
                icon: "close"
                iconSize: 20
                tip: I18n.tr("Dismiss")
                onClicked: page.pairError = ""
            }
        }
    }

    EmptyState {
        visible: !page.btOn && !BluetoothPower.available
        icon: "bluetooth_disabled"
        title: I18n.tr("No Bluetooth adapter")
        body: I18n.tr("No Bluetooth hardware is showing up. If this computer has it, check that the Bluetooth service is running.")
    }

    EmptyState {
        visible: !page.btOn && BluetoothPower.available && BluetoothPower.hardBlocked
        icon: "bluetooth_disabled"
        title: I18n.tr("Bluetooth is blocked")
        body: I18n.tr("A hardware switch or airplane mode key has turned the radio off. Flip it back to use Bluetooth.")
    }

    EmptyState {
        visible: !page.btOn && BluetoothPower.available && !BluetoothPower.hardBlocked
        icon: "bluetooth_disabled"
        title: BluetoothPower.softBlocked ? I18n.tr("Bluetooth is blocked") : I18n.tr("Bluetooth is off")
        body: BluetoothPower.softBlocked
            ? I18n.tr("Bluetooth is switched off in software. Turning it on unblocks it.")
            : I18n.tr("Turn it on to connect headphones, keyboards, controllers and other devices.")
        action: I18n.tr("Turn on Bluetooth")
        onActed: BluetoothPower.setEnabled(true)
    }

    Column {
        width: parent.width
        visible: page.btOn
        spacing: 8

        SectionLabel {
            visible: page.connectedList.length > 0
            text: I18n.tr("Connected")
        }
        Column {
            width: parent.width
            visible: page.connectedList.length > 0
            spacing: Md.seam

            Repeater {
                model: page.connectedList
                delegate: DeviceRow { count: page.connectedList.length }
            }
        }

        SectionLabel {
            visible: page.savedList.length > 0
            text: I18n.tr("Paired devices")
        }
        Column {
            width: parent.width
            visible: page.savedList.length > 0
            spacing: Md.seam

            Repeater {
                model: page.savedList
                delegate: DeviceRow { count: page.savedList.length }
            }
        }

        SectionLabel { text: I18n.tr("Available devices") }
        Column {
            width: parent.width
            spacing: Md.seam

            Repeater {
                model: page.nearbyList
                delegate: DeviceRow { count: page.nearbyList.length }
            }

            ListItem {
                width: parent.width
                visible: page.nearbyList.length === 0
                clickable: false
                icon: page.scanning ? "bluetooth_searching" : "bluetooth"
                headline: page.scanning ? I18n.tr("Looking for devices\u2026") : I18n.tr("No devices found")
                supporting: page.scanning ? I18n.tr("Put the device you want in pairing mode.")
                    : I18n.tr("Use scan up top to look again.")
                wrapSupporting: true
            }
        }
    }

    Item { width: 1; height: 4 }
}

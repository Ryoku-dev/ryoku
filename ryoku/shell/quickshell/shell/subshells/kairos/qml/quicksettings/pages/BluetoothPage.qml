pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Bluetooth
import Quickshell.Io
import shell.services
import shell.barkit as Pill
import Ryoku.Ui.Singletons
import "../kit" as K

Column {
    id: page

    property var host

    spacing: 12

    readonly property color fill: Theme.shadow
    readonly property color ink: Theme.ink(fill, 7)
    readonly property color accent: Theme.primary
    function dim(a) { return Qt.rgba(page.ink.r, page.ink.g, page.ink.b, a) }

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool btOn: page.adapter !== null && page.adapter.enabled
    readonly property bool scanning: page.btOn && page.adapter.discovering

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

    readonly property var rows: {
        var out = [];
        if (!page.btOn) return out;
        function section(label, list, trailing, hint) {
            out.push({ kind: "header", label: label, trailing: trailing, device: null });
            if (list.length === 0 && hint)
                out.push({ kind: "hint", label: hint, trailing: "", device: null });
            for (var i = 0; i < list.length; i++)
                out.push({ kind: "device", label: "", trailing: "", device: list[i] });
        }
        section(I18n.tr("Connected"), page.connectedList, "", "");
        section(I18n.tr("Saved"), page.savedList, "", "");
        section(I18n.tr("Nearby"), page.nearbyList,
            page.scanning ? I18n.tr("Scanning") : "",
            page.scanning ? I18n.tr("Looking for devices\u2026 put the device in pairing mode")
                          : I18n.tr("No devices found"));
        return out;
    }

    property string pairingAddr: ""
    property string pairError: ""
    readonly property bool busy: pairProc.running

    function statusOf(d) {
        if (!d) return "";
        if (d.connected) return I18n.tr("Connected");
        if (d.paired || d.bonded) return I18n.tr("Saved");
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
    function activate(d) {
        if (!d || page.busy) return;
        if (d.connected) d.disconnect();
        else page.link(d);
    }
    // Drop the pairing on the adapter. A device that has gone stale on the other
    // end only pairs again once this side forgets it too, which is the whole
    // reason to reach for it; BlueZ disconnects first, so a connected one works.
    function forget(d) {
        if (!d || page.busy) return;
        page.pairError = "";
        d.forget();
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

    Component.onCompleted: BluetoothDiscovery.setDiscovering(page, page.adapter, true)
    Component.onDestruction: BluetoothDiscovery.setDiscovering(page, page.adapter, false)
    onBtOnChanged: BluetoothDiscovery.setDiscovering(page, page.adapter, page.btOn)

    Item {
        width: parent.width
        height: 36

        K.QsBack {
            id: back
            anchors { left: parent.left; verticalCenter: parent.verticalCenter }
            onClicked: page.host.back()
        }
        Text {
            anchors { left: back.right; leftMargin: 12; verticalCenter: parent.verticalCenter }
            text: I18n.tr("Bluetooth")
            color: page.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 18
            font.weight: Font.DemiBold
        }
        K.QsSwitch {
            anchors { right: parent.right; verticalCenter: parent.verticalCenter }
            visible: BluetoothPower.available
            value: page.adapter ? page.adapter.enabled : false
            onToggled: v => BluetoothPower.setEnabled(v)
        }
    }

    Text {
        visible: page.pairError.length > 0
        width: parent.width
        text: page.pairError
        color: Theme.error
        font.family: Theme.fontPrimary
        font.pixelSize: 12
        wrapMode: Text.WordWrap
    }

    Text {
        visible: !page.btOn
        text: BluetoothPower.blocked ? I18n.tr("Bluetooth is blocked") : I18n.tr("Bluetooth is off")
        color: page.dim(0.55)
        font.family: Theme.fontPrimary
        font.pixelSize: 13
    }

    Column {
        visible: page.btOn
        width: parent.width
        spacing: 8

        Repeater {
            model: page.rows
            delegate: Item {
                id: rowItem
                required property var modelData
                width: parent.width
                height: rowItem.modelData.kind === "header" ? 30
                    : (rowItem.modelData.kind === "hint" ? 46 : 58)

                Text {
                    visible: rowItem.modelData.kind === "header"
                    anchors { left: parent.left; leftMargin: 4; verticalCenter: parent.verticalCenter }
                    text: rowItem.modelData.kind === "header" ? rowItem.modelData.label : ""
                    color: page.dim(0.55)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 12
                }
                Text {
                    visible: rowItem.modelData.kind === "header"
                        && String(rowItem.modelData.trailing || "").length > 0
                    anchors { right: parent.right; rightMargin: 4; verticalCenter: parent.verticalCenter }
                    text: rowItem.modelData.trailing || ""
                    color: page.accent
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11
                }
                Text {
                    visible: rowItem.modelData.kind === "hint"
                    anchors { left: parent.left; leftMargin: 4; right: parent.right; verticalCenter: parent.verticalCenter }
                    text: rowItem.modelData.kind === "hint" ? rowItem.modelData.label : ""
                    color: page.dim(0.45)
                    font.family: Theme.fontPrimary
                    font.pixelSize: 12
                    wrapMode: Text.WordWrap
                }

                Rectangle {
                    id: devRow
                    visible: rowItem.modelData.kind === "device"
                    anchors.fill: parent
                    radius: 18
                    color: page.dim(0.05)
                    readonly property var dev: rowItem.modelData.device
                    readonly property bool connecting: page.busy
                        && page.pairingAddr === String(dev ? dev.address : "")

                    Rectangle {
                        id: devDisc
                        anchors { left: parent.left; leftMargin: 10; verticalCenter: parent.verticalCenter }
                        width: 38
                        height: 38
                        radius: 19
                        color: Qt.rgba(page.accent.r, page.accent.g, page.accent.b, 0.18)
                        Pill.GlyphIcon {
                            anchors.centerIn: parent
                            width: 19
                            height: 19
                            name: BtLink.glyphFor(devRow.dev)
                            color: page.accent
                        }
                    }
                    Column {
                        anchors {
                            left: devDisc.right; leftMargin: 10
                            right: devActions.left; rightMargin: 10
                            verticalCenter: parent.verticalCenter
                        }
                        spacing: 1
                        Text {
                            width: parent.width
                            text: BtLink.label(devRow.dev)
                            color: page.ink
                            font.family: Theme.fontPrimary
                            font.pixelSize: 13
                            font.weight: Font.DemiBold
                            elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: page.statusOf(devRow.dev)
                            color: page.dim(0.6)
                            font.family: Theme.fontPrimary
                            font.pixelSize: 11
                            elide: Text.ElideRight
                        }
                    }
                    Row {
                        id: devActions
                        anchors { right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
                        spacing: 8
                        K.QsAction {
                            // Only a device the adapter has actually saved can be forgotten; one
                            // merely seen nearby has no pairing to drop.
                            visible: !!devRow.dev && (devRow.dev.paired || devRow.dev.bonded)
                            enabled: !page.busy
                            destructive: true
                            text: I18n.tr("Forget")
                            onClicked: page.forget(devRow.dev)
                        }
                        K.QsAction {
                            enabled: !page.busy
                            text: devRow.dev && devRow.dev.connected ? I18n.tr("Disconnect")
                                : (devRow.connecting ? I18n.tr("Connecting\u2026") : I18n.tr("Connect"))
                            onClicked: page.activate(devRow.dev)
                        }
                    }
                }
            }
        }
    }
}

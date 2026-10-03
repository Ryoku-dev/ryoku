pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import Quickshell.Io
import shell.services
import Ryoku.Ui.Singletons

// Bluetooth entry (contract 06 sec 2.7): a RevealerRow whose inert action button
// carries the adapter-state icon and whose label reports adapter presence and
// power; the reveal opens two service-ordered device lists split by paired state,
// and toggling the reveal drives adapter discovery. Devices and their live state
// come straight from Quickshell.Bluetooth with no client-side sort. The
// reference "alias" is Quickshell's writable device name, "trust" is its writable
// trusted property, and device/battery glyphs are translated to the nearest
// Material Symbols.
Item {
    id: root

    property real s: 1
    required property bool open

    readonly property var adapter: Bluetooth.defaultAdapter
    readonly property bool hasAdapter: root.adapter !== null
    readonly property bool adapterEnabled: root.hasAdapter && root.adapter.enabled

    // The models are the live BluetoothDevice objects in service order so each
    // row binds to its own state; only the split by paired is applied. Reading
    // d.paired inside the filter keeps the split reactive when a device pairs or
    // is forgotten without leaving the adapter's device set.
    readonly property var devices: (root.open && Bluetooth.devices) ? Bluetooth.devices.values : []
    readonly property var pairedDevices: root.devices.filter(d => d && d.paired)
    readonly property var discoveredDevices: root.devices.filter(d => d && !d.paired)

    implicitHeight: row.implicitHeight

    // Discovery is tied to the reveal edge (contract 06 sec 4). Closing the menu
    // collapses the panel and ends discovery; destruction releases ownership.
    onOpenChanged: {
        if (!root.open) {
            row.revealed = false;
            BluetoothDiscovery.setDiscovering(root, root.adapter, false);
        }
    }
    Component.onDestruction: BluetoothDiscovery.setDiscovering(root, root.adapter, false)

    // No adapter and a powered-off adapter share one Material glyph; the label
    // carries the distinction the reference draws between them.
    readonly property string stateIcon: root.adapterEnabled ? "bluetooth" : "bluetooth_disabled"
    readonly property string stateLabel: !root.hasAdapter ? I18n.tr("Bluetooth Hardware Missing")
        : root.adapterEnabled ? I18n.tr("Bluetooth") : I18n.tr("Bluetooth Disabled")

    // Translate the device's own BlueZ icon hint to the nearest Material Symbol,
    // falling back to the plain bluetooth glyph like the reference does.
    function deviceIcon(d) {
        var hint = (d && d.icon ? d.icon : "").toLowerCase();
        if (hint.indexOf("headset") >= 0 || hint.indexOf("headphone") >= 0)
            return "headphones";
        if (hint.indexOf("audio") >= 0 || hint.indexOf("speaker") >= 0)
            return "speaker";
        if (hint.indexOf("mouse") >= 0)
            return "mouse";
        if (hint.indexOf("keyboard") >= 0)
            return "keyboard";
        if (hint.indexOf("gaming") >= 0 || hint.indexOf("joypad") >= 0)
            return "sports_esports";
        if (hint.indexOf("phone") >= 0)
            return "smartphone";
        if (hint.indexOf("computer") >= 0 || hint.indexOf("laptop") >= 0)
            return "computer";
        if (hint.indexOf("watch") >= 0)
            return "watch";
        return "bluetooth";
    }

    // Battery arrives as a 0..1 fraction here; step it onto the Material
    // battery-bar glyphs (contract 06 sec 3 uses a stepped level icon).
    function batteryIcon(raw) {
        var pct = raw <= 1 ? raw * 100 : raw;
        if (pct > 95)
            return "battery_full";
        if (pct > 80)
            return "battery_6_bar";
        if (pct > 65)
            return "battery_5_bar";
        if (pct > 50)
            return "battery_4_bar";
        if (pct > 35)
            return "battery_3_bar";
        if (pct > 20)
            return "battery_2_bar";
        if (pct > 10)
            return "battery_1_bar";
        return "battery_alert";
    }

    // The reference per-device action set (contract 06 sec 2.7 table): only the
    // actions valid for the device's current state are offered, in table order.
    function deviceActions(d) {
        var a = [];
        if (!d)
            return a;
        if (d.paired && !d.connected)
            a.push({ label: I18n.tr("Connect"), act: "connect" });
        if (d.paired && d.connected)
            a.push({ label: I18n.tr("Disconnect"), act: "disconnect" });
        if (d.paired && !d.trusted)
            a.push({ label: I18n.tr("Trust"), act: "trust" });
        if (d.paired && d.trusted)
            a.push({ label: I18n.tr("Untrust"), act: "untrust" });
        if (!d.paired)
            a.push({ label: I18n.tr("Pair"), act: "pair" });
        if (d.paired)
            a.push({ label: I18n.tr("Forget"), act: "forget" });
        return a;
    }

    // Trust, untrust, disconnect and forget are plain Quickshell
    // BluetoothDevice members. Connect and pair are not: Device1.Pair and
    // Device1.Connect register no agent for BlueZ to authorise through, and
    // Connect on a bond BlueZ holds but the device has forgotten fails at once
    // with nothing clearing it (#144/#156). Both go through the one link
    // script instead, which brings an agent, retries, and rebuilds a dead bond.
    function runDeviceAction(d, act) {
        if (!d)
            return;
        switch (act) {
        case "connect":
        case "pair": root.link(d); break;
        case "disconnect": d.disconnect(); break;
        case "trust": d.trusted = true; break;
        case "untrust": d.trusted = false; break;
        case "forget": d.forget(); break;
        }
    }
    function link(d) {
        if (!d || linkProc.running)
            return;
        linkProc.command = BtLink.linkCommand(d.address);
        linkProc.running = false;
        linkProc.running = true;
    }
    Process { id: linkProc }

    RevealerRow {
        id: row
        width: root.width
        actionIconName: root.stateIcon
        actionSensitive: false

        middle: RevealerRowLabel {
            anchors.fill: parent
            label: root.stateLabel
        }

        onToggled: revealed => BluetoothDiscovery.setDiscovering(root, root.adapter, revealed)

        Column {
            width: parent.width
            spacing: 10

            Column {
                width: parent.width
                spacing: 10

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("Paired Devices")
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontLg
                    font.weight: Font.Bold
                }
                Text {
                    width: parent.width
                    visible: root.pairedDevices.length === 0
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("No Paired Devices")
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontMd
                }
                ListView {
                    width: parent.width
                    height: count > 0 ? Math.min(Math.max(contentHeight, 64 * root.s), 320 * root.s) : 0
                    clip: true
                    spacing: 10
                    model: root.pairedDevices
                    cacheBuffer: Math.max(0, height)
                    boundsBehavior: Flickable.StopAtBounds
                    delegate: deviceRowComponent
                }
            }

            Column {
                width: parent.width
                spacing: 10

                Text {
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("Discovered Devices")
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontLg
                    font.weight: Font.Bold
                }
                Text {
                    width: parent.width
                    visible: root.discoveredDevices.length === 0
                    horizontalAlignment: Text.AlignHCenter
                    text: I18n.tr("No Devices Found")
                    color: Theme.inkOn(Theme.effectiveSurface, Theme.onSurface)
                    font.family: Theme.fontPrimary
                    font.pixelSize: Theme.fontMd
                }
                ListView {
                    width: parent.width
                    height: count > 0 ? Math.min(Math.max(contentHeight, 64 * root.s), 400 * root.s) : 0
                    clip: true
                    spacing: 10
                    model: root.discoveredDevices
                    cacheBuffer: Math.max(0, height)
                    boundsBehavior: Flickable.StopAtBounds
                    delegate: deviceRowComponent
                }
            }
        }
    }

    // A device entry: collapsed to icon + alias with a battery glyph only while
    // connected, expanded to the reference primary actions for its state.
    Component {
        id: deviceRowComponent

        RevealerButton {
            id: devRow
            required property var modelData
            readonly property var dev: devRow.modelData

            width: parent.width
            iconName: root.deviceIcon(devRow.dev)
            label: BtLink.label(devRow.dev)
            secondaryIconName: (devRow.dev.connected && devRow.dev.batteryAvailable)
                ? root.batteryIcon(devRow.dev.battery) : ""

            Column {
                width: parent.width
                spacing: 8

                Repeater {
                    model: root.deviceActions(devRow.dev)
                    delegate: MenuButton {
                        id: actionButton
                        required property var modelData
                        selected: true
                        width: parent.width
                        minH: actionLabel.implicitHeight + actionButton.pad * 2
                        onClicked: root.runDeviceAction(devRow.dev, actionButton.modelData.act)
                        Text {
                            id: actionLabel
                            anchors.fill: parent
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            text: I18n.tr(actionButton.modelData.label)
                            color: actionButton.contentColor
                            font.family: Theme.fontPrimary
                            font.pixelSize: Theme.fontMd
                            font.weight: Font.Bold
                        }
                    }
                }
            }
        }
    }
}

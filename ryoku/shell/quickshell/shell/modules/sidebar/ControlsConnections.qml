pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Bluetooth
import Ryoku.Ui.Singletons
import shell.services

Item {
    id: root

    required property real s
    required property var screen
    required property bool active
    readonly property bool showWifi: SidebarState.elementVisible("wifi")
    readonly property bool showBluetooth: SidebarState.elementVisible("bluetooth")
    readonly property bool showEthernet: SidebarState.elementVisible("ethernet")
    readonly property bool showVpn: SidebarState.elementVisible("vpn")
    signal openPage(string page)

    readonly property var adapter: root.showBluetooth ? Bluetooth.defaultAdapter : null
    readonly property var connectedDevices: root.showBluetooth && Bluetooth.devices && Bluetooth.devices.values
        ? Bluetooth.devices.values.filter(device => device && device.connected) : []
    readonly property var activeAccessPoint: {
        if (!root.showWifi)
            return null;
        const aps = Network.accessPoints || [];
        for (let i = 0; i < aps.length; i++)
            if (aps[i] && aps[i].active)
                return aps[i];
        return null;
    }
    readonly property string wifiDetail: {
        if (!root.showWifi)
            return "";
        if (!Network.wifiPresent)
            return I18n.tr("No Wi-Fi adapter");
        if (!Network.wifiRadio)
            return I18n.tr("Radio off");
        if (!Network.activeSsid)
            return Network.wifiConnectivity === "Connecting" ? I18n.tr("Connecting…") : I18n.tr("Not connected");

        const strength = Math.round(Number(Network.wifi.strength || 0));
        const band = root.activeAccessPoint ? String(root.activeAccessPoint.band || "") : "";
        return band !== ""
            ? I18n.tr("%1 · %2 GHz · %3%").arg(Network.activeSsid).arg(band).arg(strength)
            : I18n.tr("%1 · %2%").arg(Network.activeSsid).arg(strength);
    }
    readonly property string bluetoothDetail: !root.adapter ? I18n.tr("No adapter")
        : !root.adapter.enabled ? I18n.tr("Radio off")
        : root.connectedDevices.length === 1 ? BtLink.label(root.connectedDevices[0])
        : root.connectedDevices.length > 1 ? I18n.tr("%1 connected").arg(root.connectedDevices.length)
        : I18n.tr("No devices connected")
    readonly property var statusChips: {
        const chips = [];
        if (root.showEthernet && Network.kind === "ethernet")
            chips.push({ glyph: "lan", label: I18n.tr("Ethernet") });
        if (root.showVpn && Network.vpnActive)
            chips.push({ glyph: "vpn_lock", label: Network.vpnName || I18n.tr("VPN") });
        return chips;
    }
    readonly property bool showStatusTile: root.statusChips.length > 0
    readonly property bool showStatusChips: false
    readonly property int tileCount: (root.showWifi ? 1 : 0) + (root.showBluetooth ? 1 : 0)
        + (root.showStatusTile ? 1 : 0)
    readonly property real tileWidth: root.tileCount > 0
        ? (tileRow.width - tileRow.spacing * (root.tileCount - 1)) / root.tileCount : 0
    readonly property string statusTileLabel: root.statusChips.length > 1
        ? I18n.tr("Ethernet + VPN") : root.statusChips.length === 1 ? root.statusChips[0].label : ""
    readonly property string statusTileDetail: root.statusChips.length > 1 ? root.statusChips[1].label : ""

    implicitHeight: root.tileCount > 0 ? tileRow.height : 0

    Row {
        id: tileRow
        width: parent.width
        height: (Tokens.rowH + Tokens.s3) * root.s
        spacing: Tokens.s2 * root.s

        ConnectionTile {
            visible: root.showWifi
            width: root.tileWidth
            height: tileRow.height
            s: root.s
            active: root.active
            label: I18n.tr("Wi-Fi")
            detail: root.wifiDetail
            glyph: Network.wifiRadio ? "wifi" : "wifi_off"
            kind: "wifi"
            connected: Network.activeSsid !== ""
            radioOn: Network.wifiRadio
            available: Network.wifiPresent
            strength: Network.level
            onSelected: root.openPage("wifi")
            onToggleRequested: Toggles.toggleWifi()
        }

        ConnectionTile {
            visible: root.showBluetooth
            width: root.tileWidth
            height: tileRow.height
            s: root.s
            active: root.active
            label: I18n.tr("Bluetooth")
            detail: root.bluetoothDetail
            glyph: root.adapter && root.adapter.enabled ? "bluetooth" : "bluetooth_disabled"
            kind: "bluetooth"
            connected: root.connectedDevices.length > 0
            radioOn: !!(root.adapter && root.adapter.enabled)
            available: root.adapter !== null
            connectedCount: root.connectedDevices.length
            onSelected: root.openPage("bluetooth")
            onToggleRequested: Toggles.toggleBt()
        }

        Rectangle {
            visible: root.showStatusTile
            width: root.tileWidth
            height: tileRow.height
            radius: Tokens.radius * root.s * 1.5
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineSoft
            Accessible.name: root.statusTileLabel
            Accessible.description: root.statusTileDetail

            Text {
                id: statusGlyph
                anchors.left: parent.left
                anchors.leftMargin: Tokens.s3 * root.s
                anchors.verticalCenter: parent.verticalCenter
                width: (Tokens.s5 - Tokens.s1) * root.s
                height: width
                text: root.statusChips.length > 1 ? "lan"
                    : root.statusChips.length === 1 ? root.statusChips[0].glyph : ""
                color: Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: (Tokens.s5 - Tokens.s1) * root.s
                horizontalAlignment: Text.AlignHCenter
                verticalAlignment: Text.AlignVCenter
                Accessible.ignored: true
            }

            Column {
                anchors.left: statusGlyph.right
                anchors.leftMargin: Tokens.s2 * root.s
                anchors.right: parent.right
                anchors.rightMargin: Tokens.s3 * root.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s1 * root.s

                Text {
                    width: parent.width
                    text: root.statusTileLabel
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }

                Text {
                    visible: root.statusTileDetail !== ""
                    width: parent.width
                    text: root.statusTileDetail
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    elide: Text.ElideRight
                }
            }
        }
    }

    Item {
        id: statusChipArea
        visible: root.showStatusChips
        y: tileRow.height + Tokens.s2 * root.s
        width: parent.width
        height: visible ? Tokens.ctlH * root.s : 0

        Row {
            anchors.right: parent.right
            height: parent.height
            spacing: Tokens.s2 * root.s

            Repeater {
                model: root.statusChips

                delegate: Rectangle {
                    id: chip
                    required property var modelData
                    implicitWidth: chipCopy.implicitWidth + Tokens.s2 * root.s * 2
                    width: implicitWidth
                    height: Tokens.ctlH * root.s
                    radius: Tokens.radius * root.s
                    color: Tokens.paperLift
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft

                    Row {
                        id: chipCopy
                        anchors.centerIn: parent
                        spacing: Tokens.s1 * root.s

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: chip.modelData.glyph
                            color: Tokens.inkDim
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: Tokens.fBody * root.s
                            Accessible.ignored: true
                        }

                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: chip.modelData.label
                            color: Tokens.inkDim
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny * root.s
                            font.weight: Font.Medium
                        }
                    }
                }
            }
        }
    }
}
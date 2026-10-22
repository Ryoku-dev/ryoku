pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property PopoutState popouts

    property string view: "wireless"
    property var passwordNetwork: null
    property bool showPasswordDialog: false
    property bool _isSidebarOpen: false

    readonly property var activeDeviceDetails: root.view === "wireless" ? Nmcli.wirelessDeviceDetails : Nmcli.ethernetDeviceDetails
    readonly property var activeDetails: {
        const d = root.activeDeviceDetails;
        if (!d || typeof d !== "object" || Object.keys(d).length === 0)
            return { visible: false, rows: [] };

        const dnsList = Array.isArray(d.dns) ? d.dns : [];
        return {
            visible: true,
            rows: [
                { label: qsTr("IP address"), value: d.ipAddress ?? "" },
                { label: qsTr("Subnet mask"), value: d.subnet ?? "" },
                { label: qsTr("Gateway"), value: d.gateway ?? "" },
                { label: qsTr("DNS"), value: dnsList.join(", ") },
                { label: qsTr("MAC address"), value: d.macAddress ?? "" }
            ]
        };
    }

    property real scaleOffset: 1.0
    property real fontScale: 1.0

    spacing: Tokens.spacing.medium * scaleOffset
    // With the sidebar open, match its width so the popout joins it (like the
    // narrower popouts do) instead of sticking out past it.
    width: _isSidebarOpen ? Math.max(300 * scaleOffset, (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased) : 400 * scaleOffset

    RowLayout {
        Layout.topMargin: Tokens.padding.small * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        Layout.rightMargin: Tokens.padding.small * root.scaleOffset
        Layout.fillWidth: true
        spacing: Tokens.spacing.small * root.scaleOffset

        StyledText {
            Layout.fillWidth: true
            text: qsTr("Network")
            font.weight: 500
            font.pointSize: Tokens.font.title.small.pointSize * root.fontScale
        }

        IconButton {
            icon: "settings"
            font: Tokens.font.icon.medium
            type: IconButton.Tonal
            isRound: true
            inactiveColour: Colours.tPalette.m3surfaceContainerHigh
            inactiveOnColour: Colours.palette.m3onSurfaceVariant
            onClicked: root.popouts.detachRequested("network")
        }
    }

    StyledRect {
        Layout.fillWidth: true
        implicitWidth: cardLayout.implicitWidth + Tokens.padding.medium * 2 * root.scaleOffset
        implicitHeight: cardLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.medium * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: cardLayout

            width: parent.width - Tokens.padding.medium * 2 * root.scaleOffset
            x: Tokens.padding.medium * root.scaleOffset
            y: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

    StyledText {
        visible: root.view === "wireless"

        Layout.topMargin: visible ? Tokens.padding.medium * root.scaleOffset : 0
        Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
        text: qsTr("Wireless")
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
    }

    StyledText {
        visible: root.view === "wireless"

        Layout.topMargin: visible ? Tokens.spacing.small * root.scaleOffset : 0
        Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
        text: qsTr("%1 networks available").arg(Nmcli.networks.length) // qmllint disable missing-property
        color: Colours.palette.m3onSurfaceVariant
        font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
    }

    Repeater {
        visible: root.view === "wireless"
        model: ScriptModel {
            values: [...Nmcli.networks].sort((a, b) => {
                if (a.active !== b.active)
                    return b.active - a.active;
                return b.strength - a.strength;
            }).slice(0, 8)
        }

        StyledRect {
            id: networkItem

            required property Nmcli.AccessPoint modelData
            readonly property bool isConnecting: Nmcli.connectingSsid === modelData?.ssid
            readonly property bool loading: networkItem.isConnecting

            Layout.fillWidth: true
            Layout.preferredWidth: 0
            implicitHeight: networkRow.implicitHeight + Tokens.padding.small * 2 * root.scaleOffset
            visible: root.view === "wireless"
            radius: Tokens.rounding.small * root.scaleOffset
            color: networkItem.modelData?.active ? Colours.tPalette.m3surfaceContainerHigh : "transparent"

            RowLayout {
                id: networkRow

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: Tokens.padding.medium * root.scaleOffset
                anchors.rightMargin: Tokens.padding.medium * root.scaleOffset
                spacing: Tokens.spacing.small * root.scaleOffset

                MaterialIcon {
                    text: Icons.getNetworkIcon(networkItem.modelData?.strength ?? 0)
                    color: networkItem.modelData?.active ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                MaterialIcon {
                    visible: networkItem.modelData?.isSecure ?? false
                    text: "lock"
                    color: Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.small.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    Layout.preferredWidth: 0
                    elide: Text.ElideRight
                    text: networkItem.modelData?.ssid ?? ""
                    color: networkItem.modelData?.active ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                    font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                }

                Item {
                    Layout.preferredWidth: Tokens.font.icon.medium.pointSize * root.scaleOffset
                    Layout.preferredHeight: width

                    CircularIndicator {
                        anchors.fill: parent
                        running: networkItem.loading
                    }

                    MaterialIcon {
                        anchors.centerIn: parent
                        text: networkItem.modelData?.active ? "check_circle" : "radio_button_unchecked"
                        color: networkItem.modelData?.active ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                        opacity: networkItem.loading ? 0 : 1
                    }
                }
            }

            StateLayer {
                anchors.fill: parent
                radius: networkItem.radius
                disabled: networkItem.loading || !Nmcli.wifiEnabled

                onClicked: {
                    if (networkItem.modelData?.active) {
                        Nmcli.disconnectFromNetwork();
                    } else if (networkItem.modelData) {
                        NetworkConnection.handleConnect(networkItem.modelData, null, network => {
                            const networkSnapshot = {
                                ssid: network.ssid,
                                bssid: network.bssid || "",
                                isSecure: network.isSecure ?? true,
                                strength: network.strength ?? 0
                            };
                            NetworkConnection.passwordNetwork = networkSnapshot;
                            root.passwordNetwork = networkSnapshot;
                            root.showPasswordDialog = true;
                            root.popouts.currentName = "wirelesspassword";
                        });

                    }
                }
            }
        }
    }

    StyledRect {
        visible: root.view === "wireless"

        Layout.topMargin: visible ? Tokens.spacing.small * root.scaleOffset : 0
        Layout.fillWidth: true
        implicitHeight: rescanBtn.implicitHeight + Tokens.padding.small * root.scaleOffset

        radius: Tokens.rounding.full * root.scaleOffset
        color: Colours.palette.m3primaryContainer

        StateLayer {
            color: Colours.palette.m3onPrimaryContainer
            disabled: Nmcli.scanning || !Nmcli.wifiEnabled
            onClicked: Nmcli.rescanWifi()
        }

        RowLayout {
            id: rescanBtn

            anchors.centerIn: parent
            spacing: Tokens.spacing.small * root.scaleOffset
            opacity: Nmcli.scanning ? 0 : 1

            MaterialIcon {
                id: scanIcon

                Layout.topMargin: Math.round(fontInfo.pointSize * 0.0575)
                animate: true
                text: "wifi_find"
                color: Colours.palette.m3onPrimaryContainer
                fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
            }

            StyledText {
                Layout.topMargin: -Math.round(scanIcon.fontInfo.pointSize * 0.0575)
                text: qsTr("Rescan networks")
                color: Colours.palette.m3onPrimaryContainer
                font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
            }

            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }

        CircularIndicator {
            anchors.centerIn: parent
            strokeWidth: Tokens.padding.extraSmall / 2 * root.scaleOffset
            bgColour: "transparent"
            implicitSize: parent.implicitHeight - Tokens.padding.large * root.scaleOffset
            running: Nmcli.scanning
        }
    }

    PopoutSection {
        Layout.fillWidth: true
        Layout.topMargin: Tokens.padding.small * root.scaleOffset
        scaleOffset: root.scaleOffset
        fontScale: root.fontScale
        title: qsTr("VPN")
        expanded: false

        StyledText {
            Layout.topMargin: Tokens.spacing.extraSmall * root.scaleOffset
            Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
            text: qsTr("%1 profiles available").arg(Nmcli.vpnConnections.length)
            color: Colours.palette.m3onSurfaceVariant
            font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
        }

        ListView {
            id: vpnList

            model: Nmcli.vpnConnections

            Layout.fillWidth: true
            Layout.leftMargin: Tokens.padding.extraSmall * root.scaleOffset
            Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset

            clip: true
            interactive: Nmcli.vpnConnections.length > 1
            boundsBehavior: Flickable.StopAtBounds
            spacing: Tokens.spacing.extraSmall * root.scaleOffset
            implicitHeight: Math.min(contentHeight, Tokens.sizes.bar.networkVpnListMaxHeight * root.scaleOffset)

            StyledScrollBar.vertical: StyledScrollBar {
                flickable: vpnList
            }

            add: Transition {
                NumberAnimation {
                    properties: "opacity"
                    from: 0
                    to: 1
                    duration: 140
                }
                NumberAnimation {
                    properties: "y"
                    duration: 180
                    easing.type: Easing.OutCubic
                }
            }
            remove: Transition {
                NumberAnimation {
                    properties: "opacity"
                    to: 0
                    duration: 100
                }
            }
            move: Transition {
                NumberAnimation {
                    properties: "y"
                    duration: 180
                    easing.type: Easing.OutCubic
                }
            }
            displaced: Transition {
                NumberAnimation {
                    properties: "y"
                    duration: 180
                    easing.type: Easing.OutCubic
                }
            }

            delegate: StyledRect {
                id: vpnItem

                required property var modelData
                readonly property bool loading: Nmcli.vpnPendingConnection === modelData?.name

                width: ListView.view.width
                implicitHeight: vpnRow.implicitHeight + Tokens.padding.small * 2 * root.scaleOffset
                radius: Tokens.rounding.small * root.scaleOffset
                color: vpnItem.modelData?.connected ? Colours.tPalette.m3surfaceContainerHigh : "transparent"

                RowLayout {
                    id: vpnRow

                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: Tokens.padding.medium * root.scaleOffset
                    anchors.rightMargin: Tokens.padding.medium * root.scaleOffset
                    spacing: Tokens.spacing.small * root.scaleOffset

                    MaterialIcon {
                        text: "vpn_key"
                        color: vpnItem.modelData?.connected ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                    }

                    StyledText {
                        Layout.fillWidth: true
                        Layout.preferredWidth: 0
                        elide: Text.ElideRight
                        text: vpnItem.modelData?.name ?? ""
                        color: vpnItem.modelData?.connected ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                        font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                    }

                    Item {
                        Layout.preferredWidth: Tokens.font.icon.medium.pointSize * root.scaleOffset
                        Layout.preferredHeight: width

                        CircularIndicator {
                            anchors.fill: parent
                            running: vpnItem.loading
                        }

                        MaterialIcon {
                            anchors.centerIn: parent
                            text: vpnItem.modelData?.connected ? "check_circle" : "radio_button_unchecked"
                            color: vpnItem.modelData?.connected ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                            fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                            opacity: vpnItem.loading ? 0 : 1
                        }
                    }
                }

                StateLayer {
                    anchors.fill: parent
                    radius: vpnItem.radius
                    disabled: vpnItem.loading

                    onClicked: {
                        if (vpnItem.modelData?.connected) {
                            Nmcli.disconnectVpn(vpnItem.modelData.name, () => {});
                        } else if (vpnItem.modelData?.name) {
                            Nmcli.connectVpn(vpnItem.modelData.name, () => {});
                        }
                    }
                }
            }
        }

        StyledText {
            visible: Nmcli.vpnConnections.length === 0
            Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
            text: qsTr("No VPN profiles found")
            color: Colours.palette.m3onSurfaceVariant
            font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
        }
    }

    StyledText {
        visible: root.view === "ethernet"

        Layout.topMargin: visible ? Tokens.padding.medium * root.scaleOffset : 0
        Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
        text: qsTr("Ethernet")
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
    }

    StyledText {
        visible: root.view === "ethernet"

        Layout.topMargin: visible ? Tokens.spacing.small * root.scaleOffset : 0
        Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
        text: qsTr("%1 devices available").arg(Nmcli.ethernetDevices.length)
        color: Colours.palette.m3onSurfaceVariant
        font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
    }

    Repeater {
        visible: root.view === "ethernet"
        model: ScriptModel {
            values: [...Nmcli.ethernetDevices].sort((a, b) => {
                if (a.connected !== b.connected)
                    return b.connected - a.connected;
                return (a.iface || "").localeCompare(b.iface || "");
            }).slice(0, 8)
        }

        StyledRect {
            id: ethernetItem

            required property Nmcli.EthernetDevice modelData
            readonly property bool loading: false

            Layout.fillWidth: true
            Layout.preferredWidth: 0
            implicitHeight: ethernetRow.implicitHeight + Tokens.padding.small * 2 * root.scaleOffset
            visible: root.view === "ethernet"
            radius: Tokens.rounding.small * root.scaleOffset
            color: ethernetItem.modelData?.connected ? Colours.tPalette.m3surfaceContainerHigh : "transparent"

            RowLayout {
                id: ethernetRow

                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: Tokens.padding.medium * root.scaleOffset
                anchors.rightMargin: Tokens.padding.medium * root.scaleOffset
                spacing: Tokens.spacing.small * root.scaleOffset

                MaterialIcon {
                    text: "cable"
                    color: ethernetItem.modelData?.connected ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    Layout.preferredWidth: 0
                    elide: Text.ElideRight
                    text: ethernetItem.modelData?.iface || qsTr("Unknown")
                    color: ethernetItem.modelData?.connected ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                    font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                }

                Item {
                    Layout.preferredWidth: Tokens.font.icon.medium.pointSize * root.scaleOffset
                    Layout.preferredHeight: width

                    CircularIndicator {
                        anchors.fill: parent
                        running: ethernetItem.loading
                    }

                    MaterialIcon {
                        anchors.centerIn: parent
                        text: ethernetItem.modelData?.connected ? "check_circle" : "radio_button_unchecked"
                        color: ethernetItem.modelData?.connected ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                        opacity: ethernetItem.loading ? 0 : 1
                    }
                }
            }

            StateLayer {
                anchors.fill: parent
                radius: ethernetItem.radius
                disabled: ethernetItem.loading

                onClicked: {
                    if (ethernetItem.modelData?.connected && ethernetItem.modelData?.connection) {
                        Nmcli.disconnectEthernet(ethernetItem.modelData.connection, () => {});
                    } else if (ethernetItem.modelData) {
                        Nmcli.connectEthernet(ethernetItem.modelData.connection || "", ethernetItem.modelData.iface || "", () => {});
                    }
                }
            }
        }
    }

    PopoutSection {
        visible: root.activeDetails.visible
        Layout.fillWidth: true
        Layout.topMargin: visible ? Tokens.padding.medium * root.scaleOffset : 0
        scaleOffset: root.scaleOffset
        fontScale: root.fontScale
        title: qsTr("Connection details")
        expanded: false

        Repeater {
            model: root.activeDetails.rows

            RowLayout {
                required property var modelData

                visible: (modelData?.value ?? "") !== ""

                Layout.fillWidth: true
                Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
                spacing: Tokens.spacing.small * root.scaleOffset

                StyledText {
                    text: modelData?.label ?? ""
                    font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                }

                StyledText {
                    Layout.fillWidth: true
                    horizontalAlignment: Text.AlignRight
                    text: modelData?.value ?? ""
                    color: Colours.palette.m3onSurfaceVariant
                    elide: Text.ElideRight
                    font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                }
            }
        }
    }
        }
    }

    Connections {
        function onActiveChanged(): void {
            if (root.showPasswordDialog && root.passwordNetwork && Nmcli.active && Nmcli.active.ssid === root.passwordNetwork.ssid) {
                root.showPasswordDialog = false;
                root.passwordNetwork = null;
            }
        }

        function onScanningChanged(): void {
            if (!Nmcli.scanning)
                scanIcon.rotation = 0;
        }

        target: Nmcli
    }

    Connections {
        function onCurrentNameChanged(): void {
            if (root.popouts.currentName !== "wirelesspassword" && root.showPasswordDialog) {
                root.showPasswordDialog = false;
                root.passwordNetwork = null;
            }
        }

        target: root.popouts
    }
}

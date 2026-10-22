pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell.Bluetooth
import Quickshell.Services.UPower
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

Item {
    id: root

    // the Dashboard shows the connection rows and the power profile as two cards
    property bool showConnections: true
    property bool showPower: true
    // a card with the whole row to itself sets the connections out in two columns
    property bool wide: false
    // beside a tall connections card the power profiles stand as a list, one row each
    property bool tall: false

    readonly property var btAdapter: Bluetooth.defaultAdapter // qmllint disable unresolved-type
    readonly property bool btEnabled: btAdapter?.enabled ?? false
    readonly property string btDetail: {
        if (!btAdapter)
            return qsTr("Unavailable");
        if (!btAdapter.enabled)
            return qsTr("Off");
        const connected = Bluetooth.devices.values.filter(d => d.connected); // qmllint disable unresolved-type
        if (connected.length === 1)
            return connected[0].name;
        if (connected.length > 1)
            return qsTr("%1 connected").arg(connected.length);
        return qsTr("Disconnected");
    }

    readonly property string vpnDetail: {
        if (GlobalConfig.utilities.vpn.selectedProvider.length === 0)
            return qsTr("No provider set");
        if (VPN.connecting)
            return qsTr("Connecting…");
        if (VPN.disconnecting)
            return qsTr("Disconnecting…");
        return VPN.connected ? VPN.providerName : qsTr("Disconnected");
    }

    readonly property var ethDevice: Nmcli.activeEthernet ?? (Nmcli.ethernetDevices.length > 0 ? Nmcli.ethernetDevices[0] : null)
    readonly property string ethDetail: {
        if (Nmcli.activeEthernet)
            return Nmcli.activeEthernet.iface || qsTr("Connected");
        if (Nmcli.ethernetDevices.length > 0)
            return qsTr("Disconnected");
        return qsTr("Unavailable");
    }

    anchors.fill: parent
    implicitHeight: layout.implicitHeight + layout.anchors.margins * 2

    ColumnLayout {
        id: layout

        anchors.fill: parent
        anchors.margins: Tokens.padding.large
        spacing: Tokens.spacing.small

        Item {
            visible: !root.showConnections && !root.tall
            Layout.fillHeight: true
        }

        GridLayout {
            visible: root.showConnections
            Layout.fillWidth: true
            columns: root.wide ? 2 : 1
            columnSpacing: Tokens.spacing.extraLarge
            rowSpacing: layout.spacing

            ConnectionRow {
                visible: root.showConnections && HostState.element("wifi")
                icon: Nmcli.wifiEnabled ? (Nmcli.active ? Icons.getNetworkIcon(Nmcli.active.strength, Nmcli.active.isSecure) : "wifi") : "wifi_off"
                label: qsTr("Wi-Fi")
                detail: !Nmcli.wifiEnabled ? qsTr("Off") : Nmcli.connectingSsid !== "" ? qsTr("Connecting…") : Nmcli.active?.ssid ? `${Nmcli.active.ssid} (${Nmcli.active.strength}%)` : qsTr("Disconnected")
                checked: Nmcli.wifiEnabled
                onToggled: Nmcli.toggleWifi()
            }

            ConnectionRow {
                visible: root.showConnections && HostState.element("bluetooth")
                icon: root.btEnabled ? "bluetooth" : "bluetooth_disabled"
                label: qsTr("Bluetooth")
                detail: root.btDetail
                checked: root.btEnabled
                enabled: root.btAdapter !== null
                onToggled: {
                    const adapter = Bluetooth.defaultAdapter; // qmllint disable unresolved-type
                    if (adapter)
                        adapter.enabled = !adapter.enabled;
                }
            }

            ConnectionRow {
                visible: root.showConnections && HostState.element("ethernet")
                icon: "cable"
                label: qsTr("Ethernet")
                detail: root.ethDetail
                checked: Nmcli.activeEthernet !== null
                enabled: Nmcli.ethernetDevices.length > 0
                onToggled: {
                    const dev = root.ethDevice;
                    if (!dev)
                        return;
                    if (dev.connected && dev.connection)
                        Nmcli.disconnectEthernet(dev.connection, () => {});
                    else
                        Nmcli.connectEthernet(dev.connection || "", dev.iface || "", () => {});
                }
            }

            ConnectionRow {
                visible: root.showConnections && HostState.element("vpn")
                icon: "vpn_key"
                label: qsTr("VPN")
                detail: root.vpnDetail
                checked: VPN.connected
                enabled: GlobalConfig.utilities.vpn.selectedProvider.length > 0 && !VPN.connecting && !VPN.disconnecting
                onToggled: VPN.toggle()
            }
        }

        RowLayout {
            visible: root.showPower
            Layout.fillWidth: true
            Layout.topMargin: root.showConnections ? Tokens.spacing.extraSmall : 0

            StyledText {
                Layout.fillWidth: true
                text: qsTr("Power Profile")
                font: Tokens.font.body.small
                color: Colours.palette.m3onSurfaceVariant
            }

            StyledText {
                text: {
                    const p = PowerProfiles.profile;
                    if (p === PowerProfile.PowerSaver)
                        return qsTr("Power Saver");
                    if (p === PowerProfile.Performance)
                        return qsTr("Performance");
                    return qsTr("Balanced");
                }
                font: Tokens.font.body.small
                color: Colours.palette.m3primary
            }
        }

        StyledRect {
            id: profiles

            visible: root.showPower && !root.tall
            Layout.fillWidth: true

            property string current: {
                const p = PowerProfiles.profile;
                if (p === PowerProfile.PowerSaver)
                    return saver.icon;
                if (p === PowerProfile.Performance)
                    return perf.icon;
                return balance.icon;
            }

            implicitHeight: Math.max(saver.implicitHeight, balance.implicitHeight, perf.implicitHeight) + Tokens.padding.small
            color: Colours.layer(Colours.palette.m3surfaceContainerHighest, 2)
            radius: Tokens.rounding.full

            StyledRect {
                id: indicator

                color: Colours.palette.m3primary
                radius: Tokens.rounding.full
                state: profiles.current

                states: [
                    State {
                        name: saver.icon
                        Fill {
                            item: saver
                        }
                    },
                    State {
                        name: balance.icon
                        Fill {
                            item: balance
                        }
                    },
                    State {
                        name: perf.icon
                        Fill {
                            item: perf
                        }
                    }
                ]

                transitions: Transition {
                    AnchorAnim {}
                }
            }

            Profile {
                id: saver

                anchors.verticalCenter: parent.verticalCenter
                anchors.left: parent.left
                anchors.leftMargin: Tokens.padding.extraSmall
                profile: PowerProfile.PowerSaver
                icon: "energy_savings_leaf"
            }

            Profile {
                id: balance

                anchors.centerIn: parent
                profile: PowerProfile.Balanced
                icon: "balance"
            }

            Profile {
                id: perf

                anchors.verticalCenter: parent.verticalCenter
                anchors.right: parent.right
                anchors.rightMargin: Tokens.padding.extraSmall
                profile: PowerProfile.Performance
                icon: "rocket_launch"
            }
        }

        Item {
            id: profileList

            readonly property Item activeRow: PowerProfiles.profile === PowerProfile.PowerSaver ? saverRow
                : PowerProfiles.profile === PowerProfile.Performance ? perfRow : balanceRow

            visible: root.showPower && root.tall
            Layout.fillWidth: true
            Layout.fillHeight: true
            implicitHeight: list.implicitHeight

            // the active profile's fill slides between the rows
            StyledRect {
                width: parent.width
                y: profileList.activeRow.y
                height: profileList.activeRow.height
                color: Colours.palette.m3primary
                radius: Tokens.rounding.large

                Behavior on y {
                    Anim {}
                }
            }

            ColumnLayout {
                id: list

                anchors.fill: parent
                spacing: Tokens.spacing.extraSmall

                ProfileRow {
                    id: saverRow

                    profile: PowerProfile.PowerSaver
                    icon: "energy_savings_leaf"
                    label: qsTr("Power Saver")
                    detail: qsTr("Battery first")
                }

                ProfileRow {
                    id: balanceRow

                    profile: PowerProfile.Balanced
                    icon: "balance"
                    label: qsTr("Balanced")
                    detail: qsTr("Default")
                }

                ProfileRow {
                    id: perfRow

                    profile: PowerProfile.Performance
                    icon: "rocket_launch"
                    label: qsTr("Performance")
                    detail: qsTr("Full speed")
                }
            }
        }

        Item {
            visible: !root.showConnections && !root.tall
            Layout.fillHeight: true
        }
    }

    component ProfileRow: Item {
        id: prow

        required property int profile
        required property string icon
        required property string label
        required property string detail
        readonly property bool active: PowerProfiles.profile === profile

        Layout.fillWidth: true
        Layout.fillHeight: true
        implicitHeight: rowContent.implicitHeight + Tokens.padding.small * 2

        StateLayer {
            radius: Tokens.rounding.large
            color: prow.active ? Colours.palette.m3onPrimary : Colours.palette.m3onSurface
            onClicked: PowerProfiles.profile = prow.profile
        }

        RowLayout {
            id: rowContent

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Tokens.padding.medium
            anchors.rightMargin: Tokens.padding.medium
            spacing: Tokens.spacing.small

            MaterialIcon {
                text: prow.icon
                fontStyle: Tokens.font.icon.medium
                color: prow.active ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                fill: prow.active ? 1 : 0
            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: 0

                StyledText {
                    Layout.fillWidth: true
                    text: prow.label
                    font: Tokens.font.body.small
                    color: prow.active ? Colours.palette.m3onPrimary : Colours.palette.m3onSurface
                    elide: Text.ElideRight
                }

                StyledText {
                    Layout.fillWidth: true
                    text: prow.detail
                    font: Tokens.font.label.small
                    color: prow.active ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                    elide: Text.ElideRight
                }
            }
        }
    }

    component ConnectionRow: RowLayout {
        id: row

        required property string icon
        required property string label
        required property string detail
        required property bool checked
        signal toggled

        Layout.fillWidth: true
        spacing: Tokens.spacing.small

        MaterialIcon {
            text: row.icon
            color: row.checked ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
            fontStyle: Tokens.font.icon.medium
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 0

            StyledText {
                Layout.fillWidth: true
                text: row.label
                font: Tokens.font.body.small
                elide: Text.ElideRight
            }

            StyledText {
                Layout.fillWidth: true
                text: row.detail
                color: Colours.palette.m3onSurfaceVariant
                font: Tokens.font.label.small
                elide: Text.ElideRight
            }
        }

        StyledSwitch {
            cLayer: 2
            checked: row.checked
            enabled: row.enabled
            onToggled: row.toggled()
        }
    }

    component Fill: AnchorChanges {
        required property Item item

        target: indicator
        anchors.left: item.left
        anchors.right: item.right
        anchors.top: item.top
        anchors.bottom: item.bottom
    }

    component Profile: Item {
        required property string icon
        required property int profile

        width: Math.max(implicitWidth, (profiles.width - Tokens.padding.extraSmall * 2) / 3)
        implicitWidth: profileIcon.implicitHeight + Tokens.padding.small
        implicitHeight: profileIcon.implicitHeight + Tokens.padding.small

        StateLayer {
            radius: Tokens.rounding.full
            color: profiles.current === parent.icon ? Colours.palette.m3onPrimary : Colours.palette.m3onSurface
            onClicked: PowerProfiles.profile = parent.profile
        }

        MaterialIcon {
            id: profileIcon

            anchors.centerIn: parent
            text: parent.icon
            fontStyle: Tokens.font.icon.medium
            color: profiles.current === text ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
            fill: profiles.current === text ? 1 : 0

            Behavior on fill {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }
    }
}

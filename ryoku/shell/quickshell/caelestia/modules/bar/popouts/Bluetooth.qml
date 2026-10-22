pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Bluetooth
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property PopoutState popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    width: Math.max(300 * scaleOffset, _isSidebarOpen ? (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased : 0)
    spacing: Tokens.spacing.small * scaleOffset

    StyledText {
        Layout.topMargin: Tokens.padding.medium * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        text: qsTr("Bluetooth")
        font.weight: 500
        font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
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

    PopoutToggleRow {
                scaleOffset: root.scaleOffset
                fontScale: root.fontScale
                label: qsTr("Enabled")
                checked: Bluetooth.defaultAdapter?.enabled ?? false // qmllint disable unresolved-type
                toggle.onToggled: {
                    const adapter = Bluetooth.defaultAdapter; // qmllint disable unresolved-type
                    if (adapter)
                        adapter.enabled = checked;
                }
            }

            PopoutToggleRow {
                scaleOffset: root.scaleOffset
                fontScale: root.fontScale
                label: qsTr("Discovering")
                checked: Bluetooth.defaultAdapter?.discovering ?? false // qmllint disable unresolved-type
                toggle.onToggled: {
                    const adapter = Bluetooth.defaultAdapter; // qmllint disable unresolved-type
                    if (adapter)
                        adapter.discovering = checked;
                }
            }

            StyledText {
                Layout.topMargin: Tokens.spacing.small * root.scaleOffset
                Layout.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
                text: {
                    const devices = Bluetooth.devices.values; // qmllint disable unresolved-type
                    let available = qsTr("%1 device%2 available").arg(devices.length).arg(devices.length === 1 ? "" : "s");
                    const connected = devices.filter(d => d.connected).length;
                    if (connected > 0)
                        available += qsTr(" (%1 connected)").arg(connected);
                    return available;
                }
                color: Colours.palette.m3onSurfaceVariant
                font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
            }

            Repeater {
                model: ScriptModel {
                    values: [...Bluetooth.devices.values].sort((a, b) => (b.connected - a.connected) || (b.paired - a.paired) || a.name.localeCompare(b.name)).slice(0, 5) // qmllint disable unresolved-type
                }

                Item {
                    id: device

                    required property BluetoothDevice modelData
                    readonly property bool loading: modelData.state === BluetoothDeviceState.Connecting || modelData.state === BluetoothDeviceState.Disconnecting // qmllint disable unresolved-type

                    Layout.fillWidth: true
                    implicitHeight: deviceRow.implicitHeight

                    StateLayer {
                        anchors.fill: parent
                        radius: Tokens.rounding.medium * root.scaleOffset
                        disabled: device.loading

                        onClicked: device.modelData.connected = !device.modelData.connected
                    }

                    ListRow {
                        id: deviceRow

                        anchors.fill: parent
                        rowScale: root.scaleOffset

                        MaterialIcon {
                            text: Icons.getBluetoothIcon(device.modelData.icon)
                            fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                        }

                        StyledText {
                            Layout.leftMargin: Tokens.spacing.extraSmall * root.scaleOffset
                            Layout.rightMargin: Tokens.spacing.extraSmall * root.scaleOffset
                            Layout.fillWidth: true
                            text: device.modelData.name
                            elide: Text.ElideRight
                            font.pointSize: Tokens.font.body.medium.pointSize * root.fontScale
                        }

                        RowLayout {
                            visible: device.modelData.state === BluetoothDeviceState.Connected  // qmllint disable unresolved-type
                            spacing: Tokens.spacing.extraSmall * root.scaleOffset

                            MaterialIcon {
                                text: device.modelData.batteryAvailable ? Icons.getBatteryIcon(device.modelData.battery) : "battery_alert"
                                color: device.modelData.batteryAvailable && device.modelData.battery < 0.2 ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
                                fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                            }

                            StyledText {
                                visible: device.modelData.batteryAvailable // qmllint disable unresolved-type
                                text: device.modelData.batteryAvailable ? qsTr("%1%").arg(Math.round(device.modelData.battery * 100)) : "" // qmllint disable unresolved-type
                                font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
                                color: device.modelData.batteryAvailable && device.modelData.battery < 0.2 ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
                            }
                        }

                        Item {
                            id: connectBtn

                            Layout.preferredWidth: Tokens.font.icon.medium.pointSize * root.scaleOffset
                            Layout.preferredHeight: width
                            visible: device.modelData.state === BluetoothDeviceState.Connected || device.loading

                            CircularIndicator {
                                anchors.fill: parent
                                running: device.loading
                            }

                            MaterialIcon {
                                anchors.centerIn: parent
                                animate: true
                                text: device.modelData.state === BluetoothDeviceState.Connected ? "link_off" : "link"
                                color: device.modelData.state === BluetoothDeviceState.Connected ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                                fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                                opacity: device.loading ? 0 : 1
                            }
                        }

                        Loader {
                            visible: status === Loader.Ready
                            asynchronous: true
                            active: device.modelData.bonded
                            sourceComponent: Item {
                                implicitWidth: connectBtn.implicitWidth
                                implicitHeight: connectBtn.implicitHeight

                                StateLayer {
                                    radius: Tokens.rounding.full * root.scaleOffset
                                    onClicked: device.modelData.forget()
                                }

                                MaterialIcon {
                                    anchors.centerIn: parent
                                    text: "delete"
                                    fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    IconTextButton {
        Layout.fillWidth: true
        inactiveColour: Colours.palette.m3primaryContainer
        inactiveOnColour: Colours.palette.m3onPrimaryContainer
        verticalPadding: Tokens.padding.small * root.scaleOffset
        text: qsTr("Open settings")
        icon: "settings"

        onClicked: root.popouts.detachRequested("bluetooth")
    }
}

import QtQuick
import QtQuick.Layouts
import Qt5Compat.GraphicalEffects
import stage
import stage.services
import stage.modules.common
import stage.modules.common.functions
import stage.modules.common.widgets
import stage.modules.ii.background.widgets
AbstractBackgroundWidget {
    id: root

    configEntryName: "devices_battery_list"
    readonly property int maxDevices: Config.options.background.widgets.devices_battery_list.maxDevices ?? 4
    readonly property int slotCount: Math.max(1, Math.min(4, root.maxDevices))
    readonly property bool showDeviceName: Config.options.background.widgets.devices_battery_list.showDeviceName ?? true
    readonly property bool showPercentage: Config.options.background.widgets.devices_battery_list.showPercentage ?? true

    // Fixed 2x1 Widget geometry
    implicitWidth: 492
    implicitHeight: 240

    // System theme tokens from WidgetColorScheme
    readonly property color cardBgColor: WidgetColorScheme.cardBgColor
    readonly property color pillBgColor: WidgetColorScheme.pillBgColor
    readonly property color pillFillColor: WidgetColorScheme.pillFillColor
    readonly property color textColorOnPillFill: WidgetColorScheme.textColorOnPillFill
    readonly property color textColorOnPillTrack: WidgetColorScheme.textColorOnPillTrack

    // Combined list of up to 4 battery devices (Laptop + KDE Connect Phone + Bluetooth devices)
    readonly property var deviceList: {
        let list = [];

        // 1. PC Laptop Battery
        if (Battery.available) {
            list.push({
                name: "Laptop",
                battery: Battery.percentage ?? 1.0,
                charging: Battery.isCharging || Battery.isPluggedIn,
                symbol: "laptop",
                accent: WidgetColorScheme.colors.colPrimary,
                onAccent: WidgetColorScheme.colors.colOnPrimary,
                container: WidgetColorScheme.colors.colPrimaryContainer,
                onContainer: WidgetColorScheme.colors.colOnPrimaryContainer,
                containerHover: WidgetColorScheme.colors.colPrimaryContainerHover
            });
        }

        // 2. KDE Connect Mobile Phone
        if (KdeConnectService.activeDevice && KdeConnectService.activeDevice.reachable) {
            list.push({
                name: KdeConnectService.activeDeviceDisplayName || "Mobile Phone",
                battery: (KdeConnectService.activeDevice.charge ?? 100) / 100.0,
                charging: KdeConnectService.activeDevice.isCharging ?? false,
                symbol: "smartphone",
                accent: WidgetColorScheme.colors.colTertiary,
                onAccent: WidgetColorScheme.colors.colOnTertiary,
                container: WidgetColorScheme.colors.colTertiaryContainer,
                onContainer: WidgetColorScheme.colors.colOnTertiaryContainer,
                containerHover: WidgetColorScheme.colors.colTertiaryContainerHover
            });
        }

        // 3. Connected Bluetooth Devices
        for (let i = 0; i < BluetoothStatus.connectedDevices.length; i++) {
            let bt = BluetoothStatus.connectedDevices[i];
            let devName = bt.name || "Bluetooth Device";
            let symbol = Icons.getBluetoothDeviceMaterialSymbol(bt.icon || "");

            let rawBattery = (bt.battery !== undefined && bt.battery !== null) ? bt.battery : (bt.batteryAvailable ? 0.80 : 0.80);
            let battVal = rawBattery > 1.0 ? rawBattery / 100.0 : rawBattery;

            list.push({
                name: devName,
                battery: battVal,
                charging: false,
                symbol: symbol !== "" ? symbol : "headphones",
                accent: WidgetColorScheme.colors.colSecondaryContainer,
                onAccent: WidgetColorScheme.colors.colOnSecondaryContainer,
                container: ColorUtils.mix(WidgetColorScheme.colors.colSecondary, WidgetColorScheme.colors.colSecondaryContainer, 0.5),
                onContainer: WidgetColorScheme.colors.colOnSecondary,
                containerHover: ColorUtils.mix(WidgetColorScheme.colors.colSecondary, WidgetColorScheme.colors.colSecondaryContainer, 0.7)
            });
        }

        return list;
    }

    StyledRectangularShadow {
        id: bgShadow
        target: cardBg
        visible: Config.options.background.widgets.enableShadows ?? true
    }

    Rectangle {
        id: cardBg
        anchors.fill: parent
        color: WidgetColorScheme.tintBackground(root.cardBgColor)
        radius: Appearance.rounding.windowRounding

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 14
            spacing: 8

            Repeater {
                model: root.slotCount

                delegate: Item {
                    id: rowSlot
                    Layout.fillWidth: true
                    Layout.fillHeight: true

                    readonly property var itemData: (root.deviceList.length > index) ? root.deviceList[index] : null
                    readonly property bool hasItem: itemData !== null
                    readonly property real fillPct: hasItem ? Math.min(1.0, Math.max(0.0, itemData.battery)) : 0.0
                    readonly property int displayPercent: Math.round(fillPct * 100)
                    readonly property color activeOnColor: hasItem ? itemData.onContainer : WidgetColorScheme.colors.colOnSurfaceVariant

                    // Pill Background Track
                    Rectangle {
                        id: pillTrack
                        anchors.fill: parent
                        radius: height / 2
                        color: hasItem ? (pillHover.hovered ? itemData.containerHover : itemData.container) : root.pillBgColor
                        opacity: hasItem ? 1.0 : 0.30

                        Behavior on color {
                            animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(pillTrack)
                        }

                        HoverHandler {
                            id: pillHover
                            enabled: rowSlot.hasItem
                        }

                        // Horizontal Liquid Fill Level Bar
                        Rectangle {
                            id: fillBar
                            anchors.left: parent.left
                            anchors.top: parent.top
                            anchors.bottom: parent.bottom
                            width: parent.width * rowSlot.fillPct
                            radius: pillTrack.radius
                            color: hasItem ? itemData.accent : root.pillFillColor

                            Behavior on width {
                                NumberAnimation { duration: 400; easing.type: Easing.OutCubic }
                            }
                        }

                        // Layer 1: Content Row on Unfilled Track (ColOnSurface)
                        RowLayout {
                            id: trackRow
                            anchors.fill: parent
                            anchors.leftMargin: 16
                            anchors.rightMargin: 16
                            spacing: 10

                            MaterialSymbol {
                                visible: rowSlot.hasItem
                                text: rowSlot.hasItem ? rowSlot.itemData.symbol : "devices"
                                iconSize: 22
                                fill: 1
                                color: hasItem ? itemData.onContainer : WidgetColorScheme.colors.colOnSurfaceVariant
                            }

                            StyledText {
                                Layout.fillWidth: true
                                visible: rowSlot.hasItem && root.showDeviceName
                                text: rowSlot.hasItem ? rowSlot.itemData.name : ""
                                font.pixelSize: 15
                                font.weight: Font.Medium
                                color: hasItem ? itemData.onContainer : WidgetColorScheme.colors.colOnSurfaceVariant
                                elide: Text.ElideRight
                            }

                            Item { Layout.fillWidth: true; visible: !rowSlot.hasItem || !root.showDeviceName }

                            Text {
                                visible: rowSlot.hasItem && root.showPercentage
                                text: rowSlot.displayPercent + "%"
                                color: hasItem ? itemData.onContainer : WidgetColorScheme.colors.colOnSurfaceVariant
                                font {
                                    pixelSize: 15
                                    weight: Font.Medium
                                    family: "Google Sans Flex"
                                }
                            }

                            Rectangle {
                                Layout.preferredWidth: 26
                                Layout.preferredHeight: 26
                                radius: width / 2
                                color: hasItem ? itemData.accent : "transparent"
                                visible: rowSlot.hasItem && rowSlot.itemData.charging

                                MaterialSymbol {
                                    anchors.centerIn: parent
                                    text: "bolt"
                                    iconSize: 16
                                    fill: 1
                                    color: itemData ? itemData.onAccent : WidgetColorScheme.colors.colOnPrimary
                                }
                            }
                        }

                        // Layer 2: Masked Fill Overlay Row (ColOnPrimary/ColOnSecondary/ColOnTertiary over liquid fill)
                        RowLayout {
                            id: fillRow
                            anchors.fill: parent
                            anchors.leftMargin: 16
                            anchors.rightMargin: 16
                            spacing: 10

                            layer.enabled: true
                            layer.smooth: true
                            layer.effect: OpacityMask {
                                // the mask is stretched over the whole layer, so it must be the row's size
                                maskSource: Item {
                                    width: fillRow.width
                                    height: fillRow.height

                                    Rectangle {
                                        x: fillBar.x - fillRow.x
                                        y: fillBar.y - fillRow.y
                                        width: fillBar.width
                                        height: fillBar.height
                                        radius: fillBar.radius
                                    }
                                }
                            }

                            MaterialSymbol {
                                visible: rowSlot.hasItem
                                text: rowSlot.hasItem ? rowSlot.itemData.symbol : "devices"
                                iconSize: 22
                                fill: 1
                                color: itemData ? itemData.onAccent : WidgetColorScheme.colors.colOnPrimary
                            }

                            StyledText {
                                Layout.fillWidth: true
                                visible: rowSlot.hasItem && root.showDeviceName
                                text: rowSlot.hasItem ? rowSlot.itemData.name : ""
                                font.pixelSize: 15
                                font.weight: Font.Medium
                                color: itemData ? itemData.onAccent : WidgetColorScheme.colors.colOnPrimary
                                elide: Text.ElideRight
                            }

                            Item { Layout.fillWidth: true; visible: !rowSlot.hasItem || !root.showDeviceName }

                            Text {
                                visible: rowSlot.hasItem && root.showPercentage
                                text: rowSlot.displayPercent + "%"
                                color: itemData ? itemData.onAccent : WidgetColorScheme.colors.colOnPrimary
                                font {
                                    pixelSize: 15
                                    weight: Font.Medium
                                    family: "Google Sans Flex"
                                }
                            }

                            Rectangle {
                                Layout.preferredWidth: 26
                                Layout.preferredHeight: 26
                                radius: width / 2
                                color: hasItem ? itemData.accent : "transparent"
                                visible: rowSlot.hasItem && rowSlot.itemData.charging

                                MaterialSymbol {
                                    anchors.centerIn: parent
                                    text: "bolt"
                                    iconSize: 16
                                    fill: 1
                                    color: itemData ? itemData.onAccent : WidgetColorScheme.colors.colOnPrimary
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

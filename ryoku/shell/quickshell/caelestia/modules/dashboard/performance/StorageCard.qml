import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.services

StyledRect {
    id: root

    property var monitor: null
    property real diskReadBytesPerSec: monitor?.diskReadBytesPerSec ?? 0
    property real diskWriteBytesPerSec: monitor?.diskWriteBytesPerSec ?? 0
    property real storageTemp: monitor?.storageTemp ?? 0
    property bool storageTempAvailable: monitor?.storageTempAvailable ?? false
    readonly property color accent: Colours.palette.m3secondary
    readonly property real percentage: Storage.primaryDisk?.perc ?? 0

    color: Colours.tPalette.m3surfaceContainer
    radius: Tokens.rounding.extraExtraLarge

    implicitWidth: layout.implicitWidth + layout.anchors.margins * 2
    implicitHeight: layout.implicitHeight + Tokens.padding.large * 2

    ServiceRef {
        service: Storage
    }

    ColumnLayout {
        id: layout

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.margins: Tokens.padding.extraLarge
        spacing: 0

        RowLayout {
            id: row

            Layout.alignment: Qt.AlignHCenter
            spacing: Tokens.spacing.large

            CircularProgress {
                fgColour: root.accent
                value: root.percentage
                implicitSize: usageColumn.implicitHeight + thickness + Tokens.padding.large * 2
                startAngle: -225
                sweepAngle: 270

                Behavior on clampedVal {
                    Anim {}
                }

                ColumnLayout {
                    id: usageColumn

                    anchors.centerIn: parent
                    spacing: 0

                    MaterialIcon {
                        Layout.alignment: Qt.AlignHCenter
                        text: "hard_drive"
                        color: root.accent
                        fontStyle: Tokens.font.icon.medium
                    }

                    StyledText {
                        Layout.alignment: Qt.AlignHCenter
                        text: Math.round(root.percentage * 100) + "%"
                        font: Tokens.font.title.builders.large.width(90).build()
                        color: root.accent
                    }

                    StyledText {
                        Layout.alignment: Qt.AlignHCenter
                        text: qsTr("Used")
                        font: Tokens.font.body.small
                        color: Colours.palette.m3onSurfaceVariant
                    }
                }
            }

            ColumnLayout {
                Layout.minimumWidth: Tokens.sizes.dashboard.perfStorageTextWidth
                spacing: Tokens.spacing.extraSmall

                StyledText {
                    text: qsTr("Storage")
                    font: Tokens.font.title.medium
                }

                StyledText {
                    text: {
                        if (!Storage.primaryDisk)
                            return qsTr("No disks detected");

                        const fmt = Units.formatKibUsage(Storage.primaryDisk.used, Storage.primaryDisk.total);
                        return fmt;
                    }
                    font: Tokens.font.body.large
                    color: root.accent
                }

                StyledText {
                    visible: Storage.primaryDisk !== null
                    text: `↓ ${Units.formatBytes(root.diskReadBytesPerSec, true)}  ↑ ${Units.formatBytes(root.diskWriteBytesPerSec, true)}`
                    font: Tokens.font.body.small
                    color: Colours.palette.m3onSurfaceVariant
                }

                RowLayout {
                    visible: root.storageTempAvailable && root.storageTemp > 0
                    spacing: Tokens.spacing.extraSmall

                    MaterialIcon {
                        text: root.storageTemp > 70 ? "thermometer_alert" : "thermometer"
                        color: root.storageTemp > 70 ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
                        fontStyle: Tokens.font.icon.small
                        fill: 1
                    }

                    StyledText {
                        text: Units.formatSensorTemp(root.storageTemp)
                        font: Tokens.font.body.small
                        color: Colours.palette.m3onSurfaceVariant
                    }
                }
            }
        }

        SplitButton {
            Layout.alignment: Qt.AlignHCenter

            type: SplitButton.Tonal
            disabled: !Storage.disks.length
            fallbackIcon: "storage"
            fallbackText: qsTr("No disks")
            menuOnTop: true
            minLeftWidth: row.implicitWidth * 0.6

            menuItems: disks.instances
            active: menuItems.find(m => m.modelData === Storage.primaryDisk) ?? menuItems[0] ?? null
            menu.onItemSelected: item => Storage.manualPrimaryDisk = (item as DiskItem).modelData

            Variants {
                id: disks

                model: Storage.disks

                DiskItem {}
            }
        }
    }

    component DiskItem: MenuItem {
        required property var modelData

        icon: modelData === Storage.primaryDisk ? "check" : ""
        text: modelData.mount
        activeIcon: "storage"
    }
}

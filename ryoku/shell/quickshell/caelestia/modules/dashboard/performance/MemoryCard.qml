import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.services

StyledRect {
    id: root

    property var monitor: null
    property real swapUsedGiB: monitor?.swapUsedGiB ?? 0
    property real swapTotalGiB: monitor?.swapTotalGiB ?? 0
    readonly property color accent: Colours.palette.m3tertiary

    color: Colours.tPalette.m3surfaceContainer
    radius: Tokens.rounding.medium

    implicitWidth: layout.implicitWidth + Tokens.padding.extraLargeIncreased * 2
    implicitHeight: layout.implicitHeight + Tokens.padding.large * 2

    ServiceRef {
        service: Memory
    }

    ColumnLayout {
        id: layout

        anchors.centerIn: parent
        spacing: Tokens.spacing.extraSmall

        RowLayout {
            Layout.leftMargin: -Tokens.padding.extraSmall
            spacing: Tokens.spacing.small

            MaterialIcon {
                text: "memory_alt"
                fill: 1
                color: root.accent
                fontStyle: Tokens.font.icon.builders.medium.weight(Font.DemiBold).build()
            }

            StyledText {
                text: qsTr("Memory")
                font: Tokens.font.title.medium
            }
        }

        CircularProgress {
            Layout.topMargin: Tokens.spacing.large
            Layout.alignment: Qt.AlignHCenter
            implicitSize: usageColumn.implicitHeight + thickness + Tokens.padding.largeIncreased * 2
            startAngle: -225
            sweepAngle: 270

            fgColour: root.accent
            value: Memory.percentage

            Behavior on clampedVal {
                Anim {}
            }

            ColumnLayout {
                id: usageColumn

                anchors.centerIn: parent
                anchors.verticalCenterOffset: Tokens.padding.extraSmall
                spacing: 0

                StyledText {
                    Layout.alignment: Qt.AlignHCenter
                    text: Math.round(Memory.percentage * 100) + "%"
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

        StyledText {
            Layout.alignment: Qt.AlignHCenter
            text: Units.formatKibUsage(Memory.used, Memory.total)
            font: Tokens.font.body.medium
        }

        ColumnLayout {
            visible: root.swapTotalGiB > 0
            Layout.fillWidth: true
            Layout.topMargin: Tokens.spacing.extraSmall
            spacing: Tokens.spacing.extraSmall

            StyledText {
                Layout.alignment: Qt.AlignHCenter
                text: qsTr("Swap: %1").arg(Units.formatKibUsage(root.swapUsedGiB * 1024 * 1024, root.swapTotalGiB * 1024 * 1024))
                font: Tokens.font.body.small
                color: Colours.palette.m3onSurfaceVariant
            }

            StyledProgressBar {
                Layout.fillWidth: true
                implicitWidth: 120
                implicitHeight: Tokens.padding.small
                value: root.swapTotalGiB > 0 ? root.swapUsedGiB / root.swapTotalGiB : 0
                fgColour: Colours.palette.m3secondary
            }
        }
    }
}

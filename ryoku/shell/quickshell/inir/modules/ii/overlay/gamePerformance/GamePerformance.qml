pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.iris.components
import inir.modules.iris.settings
import inir.modules.iris.style
import inir.modules.ii.overlay
import inir.services

StyledOverlayWidget {
    id: root

    title: Translation.tr("Game performance")
    minimumWidth: 430
    minimumHeight: 280
    property QtObject resourceMonitor: ResourceUsageMonitor { target: root }

    contentItem: OverlayBackground {
        radius: root.contentRadius
        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 14
            spacing: 10

            RowLayout {
                Layout.fillWidth: true
                IrisSquircle {
                    width: 38 * IrisStyle.density
                    height: width
                    tint: GameMode.active ? IrisStyle.identity.green : IrisStyle.identity.gray
                    glyph: "sports_esports"
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 1
                    IrisText { text: GameMode.active ? Translation.tr("Game mode is active") : Translation.tr("Game mode is off"); font.weight: IrisStyle.weight(Font.DemiBold) }
                    IrisText { text: Translation.tr("Live readings from Ryoku's resource monitor"); color: IrisStyle.muted; font.pixelSize: IrisStyle.typeMeta }
                }
                IrisButton { text: GameMode.active ? Translation.tr("Turn off") : Translation.tr("Turn on"); emphasized: !GameMode.active; onClicked: GameMode.toggle() }
            }

            GridLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                columns: 2
                columnSpacing: 8
                rowSpacing: 8
                Metric { label: Translation.tr("CPU"); value: Math.round(ResourceUsage.cpuUsage * 100) + "%"; detail: ResourceUsage.cpuTemp > 0 ? ResourceUsage.cpuTemp + " °C" : ""; tint: IrisStyle.identity.blue }
                Metric { label: Translation.tr("GPU"); value: Math.round(ResourceUsage.gpuUsage * 100) + "%"; detail: ResourceUsage.gpuTemp > 0 ? ResourceUsage.gpuTemp + " °C" : ""; tint: IrisStyle.identity.green }
                Metric { label: Translation.tr("Memory"); value: (ResourceUsage.memoryUsed / 1048576).toFixed(1) + " GB"; detail: ResourceUsage.maxAvailableMemoryString; tint: IrisStyle.identity.purple }
                Metric { label: Translation.tr("Swap"); value: (ResourceUsage.swapUsed / 1048576).toFixed(1) + " GB"; detail: ResourceUsage.maxAvailableSwapString; tint: IrisStyle.identity.orange }
            }
        }
    }

    component Metric: Rectangle {
        id: metric
        required property string label
        required property string value
        property string detail: ""
        required property color tint
        Layout.fillWidth: true
        Layout.fillHeight: true
        radius: IrisStyle.radiusRow
        color: IrisStyle.fillQuiet
        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 10
            spacing: 2
            IrisText { text: metric.label; color: metric.tint; font.pixelSize: IrisStyle.typeMeta; font.weight: IrisStyle.weight(Font.DemiBold) }
            IrisText { text: metric.value; font.family: IrisStyle.fontNumbers; font.pixelSize: IrisStyle.typeTitleLarge; font.weight: IrisStyle.weight(Font.Bold) }
            IrisText { text: metric.detail; color: IrisStyle.muted; font.pixelSize: IrisStyle.typeMeta }
        }
    }
}

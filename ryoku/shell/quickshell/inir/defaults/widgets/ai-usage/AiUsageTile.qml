pragma ComponentBehavior: Bound

// AI usage desktop tile — the quiet wallpaper face of the same module data.
// The bar face lives in IrisCompact.qml; both read UsageModel.
import QtQuick
import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.functions
import inir.modules.background.widgets

AbstractBackgroundWidget {
    id: root

    configEntryName: "custom.ai-usage"

    readonly property string tool: CustomWidgets.getConfigValue("ai-usage", "tool", "opencode")
    readonly property var labels: ({ opencode: "OpenCode", claude: "Claude", codex: "Codex" })
    readonly property string toolLabel: root.labels[root.tool] ?? root.tool

    implicitWidth: Math.round(172 * scaleFactor)
    implicitHeight: Math.round(64 * scaleFactor)
    resizableAxes: ({ uniform: "widgetScale" })
    resizeMinWidth: 120
    resizeMinHeight: 48

    UsageModel {
        id: usage
        tool: root.tool
    }

    Rectangle {
        anchors.fill: parent
        radius: root.cornerRadiusOverride >= 0 ? root.cornerRadiusOverride : Appearance.rounding.normal
        color: root.backgroundOpacity > 0 ? ColorUtils.applyAlpha(root.colText, root.backgroundOpacity) : "transparent"
        border {
            width: root.borderWidth
            color: ColorUtils.applyAlpha(root.colText, root.borderOpacity)
        }
    }

    Column {
        id: content
        spacing: Math.round(4 * root.scaleFactor)
        anchors.centerIn: parent

        Row {
            spacing: Math.round(6 * root.scaleFactor)
            anchors.horizontalCenter: parent.horizontalCenter

            MaterialSymbol {
                text: "neurology"
                iconSize: Math.round(16 * root.scaleFactor)
                color: root.colText
                anchors.verticalCenter: parent.verticalCenter
            }
            StyledText {
                text: root.toolLabel
                font.pixelSize: Math.round(Appearance.font.pixelSize.small * root.scaleFactor)
                color: ColorUtils.applyAlpha(root.colText, 0.65)
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        StyledText {
            text: root.usage.has ? root.usage.percent7d + "% used" : "—"
            font {
                pixelSize: Math.round(Appearance.font.pixelSize.large * root.scaleFactor)
                family: Appearance.font.family.numbers
            }
            color: root.colText
            anchors.horizontalCenter: parent.horizontalCenter
        }

        Rectangle {
            width: parent.width
            anchors.horizontalCenter: content.horizontalCenter
            height: Math.round(4 * root.scaleFactor)
            radius: height / 2
            color: ColorUtils.applyAlpha(root.colText, 0.15)
            visible: root.usage.has
            Rectangle {
                width: parent.width * Math.max(0, Math.min(100, root.usage.percent7d)) / 100
                height: parent.height
                radius: parent.radius
                color: root.colText
            }
        }
    }
}

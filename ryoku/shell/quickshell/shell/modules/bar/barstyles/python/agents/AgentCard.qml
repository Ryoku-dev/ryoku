import QtQuick
import QtQuick.Layouts
import "../"
import shell.services as Ryoku

Rectangle {
    id: root

    property string agentId: ""
    property real nowSec: 0

    readonly property var tool: Ryoku.AiUsage.tool(agentId)
    readonly property string planText: AgentsFormat.plan(tool.plan)
    readonly property string rateText: AgentsFormat.rate(tool.rate)

    implicitHeight: body.implicitHeight + Scaler.s(28)
    radius: ThemeBackend.borderRadius
    color: Qt.alpha(ThemeBackend.surface0, 0.55)

    ColumnLayout {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Scaler.s(14)
        spacing: Scaler.s(10)

        RowLayout {
            Layout.fillWidth: true
            spacing: Scaler.s(10)

            AgentMark {
                agentId: root.agentId
                Layout.preferredWidth: Scaler.s(26)
                Layout.preferredHeight: Scaler.s(26)
            }

            Text {
                text: AgentsFormat.agentName(root.agentId)
                font.family: ThemeBackend.fontFamily
                font.pixelSize: Scaler.s(16)
                font.weight: Font.Black
                color: ThemeBackend.text
            }

            Rectangle {
                visible: root.planText !== ""
                Layout.preferredHeight: Scaler.s(20)
                Layout.preferredWidth: planLabel.implicitWidth + Scaler.s(14)
                radius: height / 2
                color: Qt.alpha(ThemeBackend.mauve, 0.18)

                Text {
                    id: planLabel
                    anchors.centerIn: parent
                    text: root.planText
                    font.family: ThemeBackend.fontFamily
                    font.pixelSize: Scaler.s(11)
                    font.weight: Font.Bold
                    color: ThemeBackend.mauve
                }
            }

            Item { Layout.fillWidth: true }

            Rectangle {
                visible: root.tool.blocked || !root.tool.fresh
                Layout.preferredHeight: Scaler.s(22)
                Layout.preferredWidth: badge.implicitWidth + Scaler.s(16)
                radius: height / 2
                color: Qt.alpha(root.tool.blocked ? ThemeBackend.red : ThemeBackend.yellow, 0.18)

                Text {
                    id: badge
                    anchors.centerIn: parent
                    text: root.tool.blocked
                        ? AgentsFormat.tr("blocked", "Limit reached")
                        : AgentsFormat.tr("stale", "Last known")
                    font.family: ThemeBackend.fontFamily
                    font.pixelSize: Scaler.s(11)
                    font.weight: Font.Bold
                    color: root.tool.blocked ? ThemeBackend.red : ThemeBackend.yellow
                }
            }
        }

        Repeater {
            model: AgentsFormat.groups(root.tool)

            ColumnLayout {
                id: group
                required property var modelData
                Layout.fillWidth: true
                spacing: Scaler.s(8)

                Text {
                    visible: group.modelData.label !== ""
                    text: group.modelData.label
                    font.family: ThemeBackend.fontFamily
                    font.pixelSize: Scaler.s(12)
                    font.weight: Font.Bold
                    font.capitalization: Font.AllUppercase
                    color: ThemeBackend.overlay1
                }

                Repeater {
                    model: group.modelData.windows

                    UsageWindow {
                        required property var modelData
                        Layout.fillWidth: true
                        agentId: root.agentId
                        usage: modelData
                        general: group.modelData.general
                        limited: root.tool.blocked && (group.modelData.blocked || group.modelData.general)
                        nowSec: root.nowSec
                    }
                }
            }
        }

        RowLayout {
            Layout.fillWidth: true
            visible: root.tool.today > 0 || root.rateText !== ""
            spacing: Scaler.s(16)

            Text {
                visible: root.tool.today > 0
                text: AgentsFormat.tr("today", "Today  {n} tokens", { n: AgentsFormat.tokens(root.tool.today) })
                font.family: ThemeBackend.fontFamily
                font.pixelSize: Scaler.s(12)
                font.weight: Font.Bold
                color: ThemeBackend.text
            }

            Text {
                visible: text !== ""
                text: root.rateText
                font.family: ThemeBackend.fontFamily
                font.pixelSize: Scaler.s(12)
                color: ThemeBackend.subtext0
            }
        }

        ModelList {
            Layout.fillWidth: true
            models: (root.tool.models || []).slice(0, 4)
        }

        WeekChart {
            Layout.fillWidth: true
            recent: root.tool.recent
            nowSec: root.nowSec
        }
    }
}

import QtQuick
import QtQuick.Layouts
import Quickshell
import "../../../reusables"
import "../../../agents"
import "../../../"
import shell.services as Ryoku

Item {
    id: root

    property var module: null
    property var widget: module

    property string orientation: module ? module.orientation : "horizontal"
    readonly property bool isVertical: orientation === "vertical"

    readonly property bool isCompact: module ? module.isCompact : false
    readonly property var barWindow: module ? module.barWindow : null

    readonly property var agents: Ryoku.AiUsage.present
    readonly property bool isFaceVisible: agents.length > 0

    function cs(v) { return barWindow ? barWindow.cs(v) : v; }

    property real lengthPadding: isVertical ? cs(isCompact ? 10 : 12) : cs(isCompact ? 12 : 14)
    property real targetLength: (isVertical ? agentsLayout.implicitHeight : agentsLayout.implicitWidth) + (lengthPadding * 2)

    implicitWidth: isVertical ? (parent ? parent.width : 0) : targetLength
    implicitHeight: isVertical ? targetLength : (parent ? parent.height : 0)

    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        acceptedButtons: Qt.LeftButton | Qt.MiddleButton
        onClicked: (mouse) => {
            if (mouse.button === Qt.MiddleButton) Ryoku.AiUsage.refresh();
            else if (Caching.payloadDir)
                Quickshell.execDetached(["bash", Caching.payloadDir + "/scripts/qs_manager.sh", "toggle", "agents"]);
        }
    }

    AxisFlow {
        id: agentsLayout
        vertical: root.isVertical
        horizontalItemAlignment: Grid.AlignHCenter
        verticalItemAlignment: Grid.AlignVCenter
        anchors.centerIn: parent
        spacing: root.isVertical ? root.cs(root.isCompact ? 6 : 8) : root.cs(root.isCompact ? 10 : 12)

        Repeater {
            model: root.agents

            AxisFlow {
                id: agent
                required property string modelData
                readonly property var tool: Ryoku.AiUsage.tool(modelData)
                readonly property var values: AgentsFormat.headline(tool)

                vertical: root.isVertical
                horizontalItemAlignment: Grid.AlignHCenter
                verticalItemAlignment: Grid.AlignVCenter
                spacing: root.isVertical ? root.cs(root.isCompact ? 2 : 3) : root.cs(root.isCompact ? 4 : 5)
                opacity: tool.fresh ? 1.0 : 0.55

                AgentMark {
                    agentId: agent.modelData
                    width: root.cs(root.isCompact ? 15 : 17)
                    height: width
                }

                Repeater {
                    model: agent.values

                    Text {
                        required property int modelData
                        required property int index
                        text: (index > 0 && !root.isVertical ? "· " : "") + modelData + "%"
                        font.family: ThemeBackend.fontFamily
                        font.pixelSize: root.isVertical ? root.cs(root.isCompact ? 10 : 11) : root.cs(root.isCompact ? 13 : 14)
                        font.weight: index === 0 ? Font.Black : Font.Bold
                        color: index === 0
                            ? AgentsFormat.levelColor(Math.max.apply(null, agent.values), agent.tool.blocked)
                            : ThemeBackend.subtext0
                    }
                }
            }
        }
    }
}

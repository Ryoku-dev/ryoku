import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services
import "claudecode.js" as ClaudeCode

// One tool call of a Claude Code reply: header with the tool and a one-line
// summary, expandable to its input and result. A subagent card also lists
// the subagent's own tool calls and its report.
StyledRect {
    id: toolCard

    // A tool card from ClaudeCodeSession: { id, name, summary, result, isError,
    // done, ... } plus, for a subagent, agentType, progress, steps and totals.
    property var tool: ({})
    // Kept by the owner, so it survives the card being created again.
    property bool expanded: false
    readonly property bool isAgent: ClaudeCode.isAgentTool(tool.name)
    readonly property var steps: tool.steps || []
    // Collapsed running subagent: show just its latest steps.
    readonly property var visibleSteps: expanded ? steps : (!tool.done ? steps.slice(-3) : [])
    readonly property string home: Quickshell.env("HOME") || ""

    signal toggled

    function shortPaths(text: string): string {
        return ClaudeCode.shortPaths(text, home);
    }

    implicitHeight: toolCardCol.implicitHeight + Tokens.padding.small * 2
    radius: Tokens.rounding.small
    color: Colours.layer(Colours.tPalette.m3surfaceContainerHigh, 2)

    // Subagent cards get an accent bar down the left edge.
    StyledRect {
        visible: toolCard.isAgent
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.margins: 4
        width: 3
        radius: Tokens.rounding.full
        color: toolCard.tool.isError ? Colours.palette.m3error : (toolCard.tool.done ? Colours.palette.m3outlineVariant : Colours.palette.m3tertiary)

        Behavior on color { CAnim {} }
    }

    Column {
        id: toolCardCol

        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Tokens.padding.small
        anchors.leftMargin: toolCard.isAgent ? Tokens.padding.small + 8 : Tokens.padding.small
        spacing: Tokens.spacing.small

        Item {
            width: parent.width
            implicitHeight: toolHeader.implicitHeight

            RowLayout {
                id: toolHeader

                anchors.left: parent.left
                anchors.right: parent.right
                spacing: Tokens.spacing.small

                MaterialIcon {
                    text: toolCard.tool.isError ? "error" : (toolCard.isAgent ? (toolCard.tool.done ? "task_alt" : "smart_toy") : (toolCard.tool.done ? "check_circle" : "pending"))
                    color: toolCard.tool.isError ? Colours.palette.m3error : (toolCard.tool.done ? Colours.palette.m3primary : (toolCard.isAgent ? Colours.palette.m3tertiary : Colours.palette.m3onSurfaceVariant))
                    font: Tokens.font.icon.small

                    // A running subagent breathes so it reads as busy.
                    SequentialAnimation on opacity {
                        running: toolCard.isAgent && !toolCard.tool.done
                        loops: Animation.Infinite
                        onRunningChanged: if (!running) parent.opacity = 1

                        NumberAnimation { to: 0.35; duration: 700; easing.type: Easing.InOutSine }
                        NumberAnimation { to: 1; duration: 700; easing.type: Easing.InOutSine }
                    }
                }

                StyledText {
                    text: toolCard.isAgent ? (toolCard.tool.agentType || "subagent") : toolCard.tool.name
                    color: Colours.palette.m3onSurface
                    font: Tokens.font.label.medium
                }

                StyledText {
                    Layout.fillWidth: true
                    text: toolCard.shortPaths(toolCard.tool.summary)
                    color: Colours.palette.m3onSurfaceVariant
                    font: toolCard.isAgent ? Tokens.font.label.medium : Tokens.font.mono.small
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }

                MaterialIcon {
                    visible: toolCard.isAgent || (toolCard.tool.result || "") !== "" || (toolCard.tool.summary || "").length > 40
                    text: "expand_more"
                    color: Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.icon.small
                    rotation: toolCard.expanded ? 180 : 0

                    Behavior on rotation { NumberAnimation { duration: 150; easing.type: Easing.OutQuad } }
                }
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: toolCard.toggled()
            }
        }

        // Subagent: current step and running totals.
        StyledText {
            visible: toolCard.isAgent && text !== ""
            width: parent.width
            text: {
                const stats = ClaudeCode.agentStatsText(toolCard.tool);
                const step = !toolCard.tool.done ? toolCard.shortPaths(toolCard.tool.progress) : "";
                return step !== "" && stats !== "" ? step + " · " + stats : (step || stats);
            }
            color: Colours.palette.m3outline
            font: Tokens.font.label.small
            elide: Text.ElideRight
        }

        // Subagent: its own tool calls.
        Repeater {
            model: toolCard.visibleSteps

            RowLayout {
                required property var modelData

                width: toolCardCol.width
                spacing: Tokens.spacing.small

                MaterialIcon {
                    text: parent.modelData.isError ? "close" : (parent.modelData.done ? "check" : "more_horiz")
                    color: parent.modelData.isError ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.icon.small
                }

                StyledText {
                    text: parent.modelData.name
                    color: Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.label.small
                }

                StyledText {
                    Layout.fillWidth: true
                    text: toolCard.shortPaths(parent.modelData.summary)
                    color: Colours.palette.m3outline
                    font: Tokens.font.mono.small
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }
            }
        }

        StyledText {
            visible: !toolCard.expanded && !toolCard.tool.done && toolCard.steps.length > 3
            text: qsTr("+%1 earlier steps").arg(toolCard.steps.length - 3)
            color: Colours.palette.m3outline
            font: Tokens.font.label.small
        }

        // Subagent report, rendered as Markdown.
        ChatMarkdown {
            visible: toolCard.isAgent && toolCard.expanded && text !== ""
            text: toolCard.tool.result || ""
            maxWidth: parent.width
            color: toolCard.tool.isError ? Colours.palette.m3error : Colours.palette.m3onSurface
        }

        TextEdit {
            visible: toolCard.expanded && !toolCard.isAgent
            width: parent.width
            text: toolCard.shortPaths(toolCard.tool.summary) + ((toolCard.tool.result || "") !== "" ? "\n\n" + toolCard.tool.result : "")
            textFormat: Text.PlainText
            color: toolCard.tool.isError ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
            font: Tokens.font.mono.small
            wrapMode: Text.WrapAnywhere
            readOnly: true
            selectByMouse: true
            selectionColor: Colours.palette.m3primary
            selectedTextColor: Colours.palette.m3onPrimary
        }
    }
}

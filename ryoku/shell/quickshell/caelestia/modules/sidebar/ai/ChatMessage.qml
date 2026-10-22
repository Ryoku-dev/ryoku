import QtQuick
import QtQuick.Layouts
import Caelestia.Config
import caelestia.components
import caelestia.services
import "attachments.js" as AttachmentPaths

// One message of the AI chat, as a row of ChatStore.messages: the bubble with
// the thinking, the reply text with the tool cards placed where they were
// called, attachments and the usage line.
Item {
    id: root

    required property string msgId
    required property string text
    required property bool isUser
    required property bool isFinished
    required property string thoughtText
    required property string toolsJson
    required property string usageText
    required property string attachments
    required property bool isNew
    required property bool thoughtExpanded
    // Ids of the tool cards that are expanded, comma separated.
    required property string expandedTools

    // A reply is being thought about (only its tool calls are shown meanwhile).
    property bool thinking: false

    readonly property var tools: {
        if (!toolsJson)
            return [];
        try {
            return JSON.parse(toolsJson);
        } catch (e) {
            return [];
        }
    }
    // The reply text split at the points where tool calls were made:
    // each segment is the text written before its tool cards. Tools
    // saved without a position are shown first.
    readonly property var segments: {
        var out = [{ text: "", tools: [] }];
        var pos = 0;
        for (var i = 0; i < tools.length; i++) {
            var at = Math.max(pos, Math.min(tools[i].at || 0, text.length));
            var cur = out[out.length - 1];
            if (at > pos) {
                if (cur.tools.length > 0) {
                    cur = { text: "", tools: [] };
                    out.push(cur);
                }
                cur.text = text.substring(pos, at).trim();
                pos = at;
            }
            cur.tools.push(tools[i]);
        }
        var rest = text.substring(pos).trim();
        if (out[out.length - 1].tools.length > 0)
            out.push({ text: rest, tools: [] });
        else
            out[out.length - 1].text = rest;
        return out;
    }
    readonly property var attachmentList: attachments ? attachments.split("\n") : []
    readonly property var expandedToolIds: expandedTools ? expandedTools.split(",") : []

    // What is expanded, and whether the message has popped in yet, is kept in
    // the model so the message looks the same when its delegate is recreated.
    signal viewStateEdited(patch: var)

    function toggleTool(id: string): void {
        const ids = expandedToolIds.filter(t => t !== id);
        if (ids.length === expandedToolIds.length)
            ids.push(id);
        viewStateEdited({ "expandedTools": ids.join(",") });
    }

    // While a reply is still thinking only its tool calls are worth showing.
    visible: (!root.isFinished && root.thinking && root.tools.length === 0) ? false : (root.text !== "" || root.thoughtText !== "" || root.tools.length > 0 || root.attachmentList.length > 0)
    height: visible ? bubbleRect.height : 0

    scale: 0.0
    opacity: 0.0

    // Only a message that was just added pops in, once; one being
    // created again as it scrolls into view appears as is.
    Component.onCompleted: {
        if (isNew) {
            popInAnim.start();
            root.viewStateEdited({ "isNew": false });
        } else {
            scale = 1;
            opacity = 1;
        }
    }
    onIsFinishedChanged: {
        if (isFinished)
            popDoneAnim.start();
    }

    ParallelAnimation {
        id: popInAnim

        NumberAnimation { target: root; property: "scale"; from: 0.8; to: 1.0; duration: 300; easing.type: Easing.OutBack }
        NumberAnimation { target: root; property: "opacity"; from: 0.0; to: 1.0; duration: 200; easing.type: Easing.OutQuad }
    }

    SequentialAnimation {
        id: popDoneAnim

        NumberAnimation { target: root; property: "scale"; from: 1.0; to: 1.02; duration: 100; easing.type: Easing.OutQuad }
        NumberAnimation { target: root; property: "scale"; from: 1.02; to: 1.0; duration: 150; easing.type: Easing.OutSine }
    }

    StyledRect {
        id: bubbleRect

        readonly property real maxBubbleWidth: root.width * 0.85

        anchors.right: root.isUser ? parent.right : undefined
        anchors.left: root.isUser ? undefined : parent.left

        width: Math.min(maxBubbleWidth, bubbleLayout.implicitWidth + Tokens.padding.medium * 2 + 8)
        height: bubbleLayout.implicitHeight + Tokens.padding.medium * 2
        radius: Tokens.rounding.large
        color: root.isUser ? Colours.palette.m3primary : Colours.tPalette.m3surfaceContainer

        topLeftRadius: Tokens.rounding.large
        topRightRadius: Tokens.rounding.large
        bottomLeftRadius: root.isUser ? Tokens.rounding.large : 4
        bottomRightRadius: root.isUser ? 4 : Tokens.rounding.large

        Column {
            id: bubbleLayout

            property string delegateThought: root.thoughtText

            anchors.top: parent.top
            anchors.left: parent.left
            anchors.margins: Tokens.padding.medium
            spacing: Tokens.spacing.small

            Item {
                visible: bubbleLayout.delegateThought !== ""
                implicitWidth: thoughtRow.implicitWidth
                implicitHeight: thoughtRow.implicitHeight
                height: visible ? implicitHeight : 0

                Row {
                    id: thoughtRow

                    spacing: Tokens.spacing.small

                    Text {
                        text: qsTr("Thought Process")
                        color: Colours.palette.m3onSurfaceVariant
                        font: Tokens.font.body.small
                    }
                    MaterialIcon {
                        id: thoughtArrow

                        text: "expand_more"
                        color: Colours.palette.m3onSurfaceVariant
                        font: Tokens.font.icon.small
                        rotation: root.thoughtExpanded ? 180 : 0

                        Behavior on rotation { NumberAnimation { duration: 150; easing.type: Easing.OutQuad } }
                    }
                }
                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.viewStateEdited({ "thoughtExpanded": !root.thoughtExpanded })
                }
            }

            Item {
                id: thoughtContentWrapper

                width: thoughtContent.width
                height: root.thoughtExpanded ? thoughtContent.implicitHeight : 0
                clip: true

                Behavior on height { NumberAnimation { duration: 200; easing.type: Easing.InOutQuad } }

                TextEdit {
                    id: thoughtContent

                    property string fullThought: bubbleLayout.delegateThought
                    property bool cursorVisible: true

                    width: Math.min(implicitWidth, bubbleRect.maxBubbleWidth - Tokens.padding.medium * 2)
                    textFormat: Text.MarkdownText
                    text: root.isFinished ? fullThought : fullThought + (cursorVisible ? "▌" : "")
                    color: Colours.palette.m3onSurfaceVariant
                    font: Tokens.font.body.small
                    wrapMode: Text.Wrap
                    readOnly: true
                    selectByMouse: true
                    selectionColor: Colours.palette.m3primary
                    selectedTextColor: Colours.palette.m3onPrimary
                    opacity: root.thoughtExpanded ? 1.0 : 0.0

                    Timer {
                        running: !root.isFinished
                        repeat: true
                        interval: 400
                        onTriggered: thoughtContent.cursorVisible = !thoughtContent.cursorVisible
                    }

                    Behavior on opacity {
                        SequentialAnimation {
                            PauseAnimation { duration: root.thoughtExpanded ? 100 : 0 }
                            NumberAnimation { duration: 150; easing.type: Easing.InOutQuad }
                        }
                    }
                }
            }

            // Reply text with the tool calls made while producing it (Claude
            // Code), each card placed after the text written before it.
            Repeater {
                model: root.segments.length

                Column {
                    id: segmentItem

                    required property int index
                    readonly property var segment: root.segments[index] || ({ text: "", tools: [] })
                    readonly property bool isLast: index === root.segments.length - 1

                    visible: segment.text !== "" || segment.tools.length > 0
                    spacing: bubbleLayout.spacing

                    ChatMarkdown {
                        // Nothing to show yet (e.g. only tool calls so far): no
                        // empty line with a lone blinking cursor.
                        visible: text !== ""
                        text: segmentItem.segment.text
                        showCursor: !root.isFinished && segmentItem.isLast
                        maxWidth: bubbleRect.maxBubbleWidth - Tokens.padding.medium * 2
                        color: root.isUser ? Colours.palette.m3onPrimary : Colours.palette.m3onSurface
                    }

                    Repeater {
                        model: segmentItem.segment.tools.length

                        ToolCard {
                            required property int index

                            width: bubbleRect.maxBubbleWidth - Tokens.padding.medium * 2
                            tool: segmentItem.segment.tools[index] || ({})
                            expanded: root.expandedToolIds.indexOf(tool.id) !== -1
                            onToggled: root.toggleTool(tool.id)
                        }
                    }
                }
            }

            Repeater {
                model: root.attachmentList

                Row {
                    required property string modelData

                    spacing: Tokens.spacing.small

                    MaterialIcon {
                        text: AttachmentPaths.isImagePath(parent.modelData) ? "image" : "attach_file"
                        color: root.isUser ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                        font: Tokens.font.icon.small
                    }

                    StyledText {
                        width: Math.min(implicitWidth, bubbleRect.maxBubbleWidth - Tokens.padding.medium * 2 - 24)
                        text: parent.modelData.replace(/^.*\//, "")
                        color: root.isUser ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                        font: Tokens.font.label.small
                        elide: Text.ElideMiddle
                    }
                }
            }

            StyledText {
                visible: !root.isUser && root.isFinished && root.usageText !== ""
                width: Math.min(implicitWidth, bubbleRect.maxBubbleWidth - Tokens.padding.medium * 2)
                text: root.usageText
                color: Colours.palette.m3outline
                font: Tokens.font.label.small
                wrapMode: Text.Wrap
            }
        }
    }
}

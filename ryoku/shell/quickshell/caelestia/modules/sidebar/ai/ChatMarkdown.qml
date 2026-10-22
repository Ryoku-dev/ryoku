pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.services

// Markdown text for chat bubbles. Fenced code blocks are pulled out and shown
// as their own wrapping blocks: Qt's Markdown importer keeps code lines
// unbreakable, so long commands would otherwise run past the bubble.
Column {
    id: root

    property string text
    property color color: Colours.palette.m3onSurface
    property font font: Tokens.font.body.small
    property real maxWidth
    // Appends a blinking cursor to the last segment while a reply streams in.
    property bool showCursor
    property bool cursorVisible: true

    readonly property var segments: splitCodeBlocks(text)

    function splitCodeBlocks(src: string): var {
        const out = [];
        let buf = [];
        let fence = null;
        let lang = "";
        const flush = code => {
            const t = buf.join("\n");
            if (code || t.trim() !== "")
                out.push({
                    code: code,
                    lang: lang,
                    text: t
                });
            buf = [];
        };

        for (const line of src.split("\n")) {
            if (!fence) {
                const m = line.match(/^ {0,3}(`{3,}|~{3,})\s*(.*)$/);
                // A backtick fence's info string can't contain backticks, so
                // ```foo``` on one line is inline code, not a block.
                if (m && !(m[1][0] === "`" && m[2].includes("`"))) {
                    flush(false);
                    fence = new RegExp(`^\\s*${m[1][0] === "`" ? "`" : "~"}{${m[1].length},}\\s*$`);
                    lang = m[2].trim().split(/\s+/)[0];
                    continue;
                }
            } else if (fence.test(line)) {
                flush(true);
                fence = null;
                lang = "";
                continue;
            }
            buf.push(line);
        }
        // An unterminated fence (still streaming) is shown as code too.
        flush(fence !== null);
        return out;
    }

    spacing: Tokens.spacing.small

    Timer {
        running: root.showCursor
        repeat: true
        interval: 400
        onTriggered: root.cursorVisible = !root.cursorVisible
    }

    Repeater {
        // Bound to the count rather than the array so delegates survive while
        // the text streams in; each one reads its segment by index.
        model: root.segments.length

        Item {
            id: segment

            required property int index

            readonly property var seg: root.segments[index] ?? {}
            readonly property bool isCode: seg.code ?? false
            readonly property bool isLast: index === root.segments.length - 1
            readonly property string shownText: (seg.text ?? "") + (isLast && root.showCursor && root.cursorVisible ? "▌" : "")

            implicitWidth: isCode ? codeBlock.width : prose.width
            implicitHeight: isCode ? codeBlock.height : prose.height
            width: implicitWidth
            height: implicitHeight

            TextEdit {
                id: prose

                visible: !segment.isCode
                width: Math.min(implicitWidth, root.maxWidth)
                textFormat: Text.MarkdownText
                text: segment.isCode ? "" : segment.shownText
                color: root.color
                font: root.font
                wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                readOnly: true
                selectByMouse: true
                selectionColor: Colours.palette.m3primary
                selectedTextColor: Colours.palette.m3onPrimary

                MouseArea {
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.IBeamCursor
                    propagateComposedEvents: true
                    onPressed: mouse => mouse.accepted = false
                }
            }

            StyledRect {
                id: codeBlock

                readonly property real innerMax: root.maxWidth - Tokens.padding.small * 2

                visible: segment.isCode
                width: Math.min(root.maxWidth, Math.max(header.implicitWidth, code.implicitWidth) + Tokens.padding.small * 2)
                height: codeColumn.implicitHeight + Tokens.padding.small * 2
                radius: Tokens.rounding.small
                color: Qt.rgba(root.color.r, root.color.g, root.color.b, 0.08)

                Column {
                    id: codeColumn

                    anchors.fill: parent
                    anchors.margins: Tokens.padding.small
                    spacing: Tokens.spacing.extraSmall

                    Item {
                        id: header

                        implicitWidth: langLabel.implicitWidth + copyButton.implicitWidth + Tokens.spacing.small
                        width: parent.width
                        height: copyButton.implicitHeight

                        StyledText {
                            id: langLabel

                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            text: segment.seg.lang || "code"
                            color: root.color
                            opacity: 0.6
                            font: Tokens.font.label.small
                        }

                        IconButton {
                            id: copyButton

                            property bool copied

                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            type: IconButton.Text
                            font: Tokens.font.icon.small
                            icon: copied ? "check" : "content_copy"
                            inactiveOnColour: root.color
                            onClicked: {
                                Quickshell.clipboardText = segment.seg.text ?? "";
                                copied = true;
                                copiedTimer.restart();
                            }

                            Timer {
                                id: copiedTimer

                                interval: 1500
                                onTriggered: copyButton.copied = false
                            }
                        }
                    }

                    TextEdit {
                        id: code

                        width: Math.min(implicitWidth, codeBlock.innerMax)
                        textFormat: Text.PlainText
                        text: segment.isCode ? segment.shownText : ""
                        color: root.color
                        font: Tokens.font.mono.small
                        wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                        readOnly: true
                        selectByMouse: true
                        selectionColor: Colours.palette.m3primary
                        selectedTextColor: Colours.palette.m3onPrimary

                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.IBeamCursor
                            propagateComposedEvents: true
                            onPressed: mouse => mouse.accepted = false
                        }
                    }
                }
            }
        }
    }
}

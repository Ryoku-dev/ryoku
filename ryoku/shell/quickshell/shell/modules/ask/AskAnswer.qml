pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "look"
import "../../services/lib/markdown.js" as Md

// One Quick Ask turn: what was asked, what Hyakki is doing while it works, and
// the answer as it streams (Markdown; images are cut to their alt text and only
// web and mail links open). The things the answer itself offers (a command to
// copy, a file to open) sit under it as plaques.
Item {
    id: root

    required property real s
    required property HyakkiSession session
    property real maximumHeight: 420 * s

    readonly property var sessionData: session.state
    readonly property string answerText: String(sessionData.answer || "")
    readonly property bool boxed: !Look.lithic
    readonly property real side: Math.round(18 * root.s)
    // what the answer offers, as Hyakki sent it: {kind: copy|cmd|color|file|dir|url, label, value}
    readonly property var offers: sessionData.phase === "done" || sessionData.phase === "cancelled" ? (sessionData.actions || []) : []
    readonly property string workingLabel: {
        const tools = sessionData.tools || [];
        return tools.length > 0 ? String(tools[tools.length - 1].title || I18n.tr("Working")) : I18n.tr("Thinking");
    }

    signal hud(string text)

    implicitHeight: Math.min(root.maximumHeight, column.implicitHeight + Math.round(26 * root.s))

    function offerLabel(offer) {
        if (offer.kind === "file")
            return "nvim " + String(offer.label || offer.value || "");
        if (offer.kind === "dir")
            return I18n.tr("Open %1").arg(offer.label || offer.value || "");
        if (offer.kind === "cmd")
            return "$ " + String(offer.label || offer.value || "");
        return String(offer.label || "");
    }

    function offerGlyph(offer) {
        return offer.kind === "file" ? "edit_document" : offer.kind === "dir" ? "folder_open"
            : offer.kind === "url" ? "open_in_browser" : offer.kind === "cmd" ? "terminal" : "content_copy";
    }

    function fire(offer) {
        const kind = String(offer.kind || "");
        const value = String(offer.value || "");
        if (kind === "copy" || kind === "cmd" || kind === "color") {
            Quickshell.clipboardText = value;
            root.hud(I18n.tr("Copied"));
        } else if (kind === "file") {
            Spawn.runApp(["kitty", "-e", "nvim", value], { label: "kitty" });
        } else if (kind === "dir" || kind === "url") {
            Spawn.runApp(["xdg-open", value], { label: kind });
        }
    }

    function scrollBy(amount) {
        const limit = Math.max(0, flick.contentHeight - flick.height);
        flick.contentY = Math.max(0, Math.min(limit, flick.contentY + amount));
        flick.follow = flick.contentY >= limit - 2;
    }

    function line(delta) { root.scrollBy(delta * 48 * root.s); }
    function page(delta) { root.scrollBy(delta * (flick.height - 40 * root.s)); }

    Flickable {
        id: flick

        // stay with the newest line until the reader scrolls away from it
        property bool follow: true

        anchors.fill: parent
        contentWidth: width
        contentHeight: column.implicitHeight + Math.round(26 * root.s)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height + 1
        onMovementEnded: flick.follow = flick.contentY >= flick.contentHeight - flick.height - 2
        onContentHeightChanged: {
            if (flick.follow && root.session.busy)
                flick.contentY = Math.max(0, flick.contentHeight - flick.height);
        }

        Column {
            id: column
            x: root.side
            y: Math.round(12 * root.s)
            width: flick.width - root.side * 2
            spacing: Math.round(10 * root.s)

            Text {
                width: parent.width
                visible: text.length > 0
                text: String(root.sessionData.question || "")
                color: Lithic.fg
                font.family: Look.sans
                font.pixelSize: Math.round(15 * root.s)
                font.weight: Font.Medium
                wrapMode: Text.WordWrap
                maximumLineCount: 3
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }

            Column {
                width: parent.width
                visible: root.session.busy
                spacing: Math.round(3 * root.s)

                Micro {
                    width: parent.width
                    s: root.s
                    visible: (root.sessionData.tools || []).length === 0
                    text: I18n.tr("Thinking")
                    color: Lithic.fg2
                }
                Repeater {
                    model: root.session.busy ? (root.sessionData.tools || []).slice(-4) : []

                    delegate: Row {
                        id: toolRow
                        required property var modelData
                        readonly property string status: String(toolRow.modelData.status || "running").toLowerCase()
                        readonly property bool settled: toolRow.status === "done" || toolRow.status === "completed"
                        readonly property bool broken: toolRow.status === "failed" || toolRow.status === "error"
                        spacing: Math.round(8 * root.s)

                        Symbol {
                            anchors.verticalCenter: parent.verticalCenter
                            s: root.s
                            name: toolRow.broken ? "close" : toolRow.settled ? "check" : "arrow_right_alt"
                            size: 14
                            ink: toolRow.broken ? Lithic.jasper : toolRow.settled ? Lithic.jade : Lithic.lapis
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.min(implicitWidth, column.width - 30 * root.s)
                            text: String(toolRow.modelData.title || toolRow.modelData.toolKind || I18n.tr("Tool"))
                            color: Lithic.fg3
                            font.family: Look.sans
                            font.pixelSize: Math.round(12 * root.s)
                            elide: Text.ElideRight
                            renderType: Text.NativeRendering
                        }
                    }
                }
            }

            Shape {
                width: parent.width
                height: answer.implicitHeight + (root.boxed ? Math.round(24 * root.s) : 0)
                visible: root.answerText.length > 0
                s: root.s
                cutSize: Look.cutCard
                fill: root.boxed ? Look.card : "transparent"
                strokeColor: root.boxed ? Look.cardStroke : "transparent"
                strokeWidth: root.boxed && !Qt.colorEqual(Look.cardStroke, "transparent") ? 1 : 0

                TextEdit {
                    id: answer
                    x: root.boxed ? Math.round(14 * root.s) : 0
                    y: root.boxed ? Math.round(12 * root.s) : 0
                    width: parent.width - x * 2
                    text: Md.render(root.answerText, Look.prose(root.s, 14, root.boxed ? Look.card : Look.frame))
                    textFormat: TextEdit.RichText
                    color: Lithic.fg
                    selectionColor: Lithic.sel
                    selectedTextColor: Lithic.onSel
                    font.family: Look.sans
                    font.pixelSize: Math.round(14 * root.s)
                    wrapMode: TextEdit.Wrap
                    readOnly: true
                    selectByMouse: true
                    activeFocusOnPress: false
                    renderType: Text.NativeRendering
                    onLinkActivated: url => {
                        const safe = Md.safeUrl(url);
                        if (safe.length > 0)
                            Spawn.runApp(["xdg-open", safe], { label: "browser" });
                    }
                }
            }

            Text {
                width: parent.width
                visible: root.sessionData.needsAgent === true && text.length > 0
                text: String(root.sessionData.needsAgentReason || "")
                color: Lithic.fg2
                font.family: Look.sans
                font.pixelSize: Math.round(13 * root.s)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }

            Text {
                width: parent.width
                visible: text.length > 0
                text: String(root.sessionData.error || "")
                color: Lithic.jasper
                font.family: Look.sans
                font.pixelSize: Math.round(13 * root.s)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }

            Text {
                width: parent.width
                visible: root.sessionData.phase === "cancelled"
                text: I18n.tr("Stopped before the answer was finished.")
                color: Lithic.fg3
                font.family: Look.sans
                font.pixelSize: Math.round(13 * root.s)
                wrapMode: Text.WordWrap
                renderType: Text.NativeRendering
            }

            Flow {
                width: parent.width
                visible: root.offers.length > 0
                spacing: Math.round(6 * root.s)

                Repeater {
                    model: root.offers

                    delegate: Chip {
                        required property var modelData
                        s: root.s
                        tier: "line"
                        glyph: root.offerGlyph(modelData)
                        text: root.offerLabel(modelData)
                        onClicked: root.fire(modelData)
                    }
                }
            }
        }
    }

    ScrollRail {
        anchors.right: parent.right
        anchors.rightMargin: Math.round(2 * root.s)
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.topMargin: Math.round(4 * root.s)
        anchors.bottomMargin: Math.round(4 * root.s)
        flickable: flick
    }
}

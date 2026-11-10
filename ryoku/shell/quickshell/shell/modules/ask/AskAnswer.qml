pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

Item {
    id: root

    required property real s
    required property RashinSession session
    property bool active: false
    property int selectedChip: 0
    property string flash: ""
    readonly property var sessionData: session.state
    readonly property var chips: {
        if (session.busy)
            return [
                { kind: "agent", label: I18n.tr("Move to Agent"), value: "" },
                { kind: "cancel", label: I18n.tr("Cancel"), value: "" }
            ];
        if (sessionData.phase !== "done" && sessionData.phase !== "cancelled")
            return [];
        const rows = [];
        if (sessionData.needsAgent)
            rows.push({ kind: "agent", label: I18n.tr("Move to Agent"), value: "" });
        if (String(sessionData.answer || "").length > 0)
            rows.push({ kind: "copy", label: I18n.tr("Copy"), value: sessionData.answer });
        const actions = sessionData.actions || [];
        for (var i = 0; i < actions.length; i++)
            rows.push(actions[i]);
        if (!sessionData.needsAgent)
            rows.push({ kind: "agent", label: I18n.tr("Continue with Agent"), value: "" });
        rows.push({ kind: "open", label: I18n.tr("Open in Rashin"), value: "" });
        rows.push({ kind: "pin", label: I18n.tr("Pin bubble"), value: "" });
        return rows;
    }

    signal handoffRequested()
    signal pinRequested(string question, string answer)

    implicitHeight: content.implicitHeight

    function statusLabel(value) {
        const status = String(value || "running").toLowerCase();
        if (status === "done" || status === "completed")
            return I18n.tr("Done");
        if (status === "failed" || status === "error")
            return I18n.tr("Failed");
        if (status === "cancelled" || status === "canceled")
            return I18n.tr("Cancelled");
        if (status === "waiting" || status === "pending")
            return I18n.tr("Waiting");
        return I18n.tr("Running");
    }

    function move(delta) {
        selectedChip = Math.max(0, Math.min(chips.length - 1, selectedChip + delta));
    }

    function activate() {
        if (chips[selectedChip])
            fire(chips[selectedChip]);
    }

    function fire(chip) {
        const kind = String(chip.kind || "");
        const value = String(chip.value || "");
        if (kind === "copy" || kind === "cmd" || kind === "color") {
            Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" | wl-copy", "_", value]);
            flash = kind + "\u0000" + value;
            flashTimer.restart();
        } else if (kind === "file") {
            Spawn.runApp(["kitty", "-e", "nvim", value], { label: "kitty" });
        } else if (kind === "dir" || kind === "url") {
            Spawn.runApp(["xdg-open", value], { label: kind });
        } else if (kind === "agent") {
            handoffRequested();
        } else if (kind === "open") {
            Quickshell.execDetached(["rashin", "open", "--session", root.session.id]);
        } else if (kind === "pin") {
            pinRequested(String(sessionData.question || ""), String(sessionData.answer || ""));
        } else if (kind === "cancel") {
            session.cancel();
        }
    }

    function caption(chip) {
        if (flash === String(chip.kind || "") + "\u0000" + String(chip.value || ""))
            return I18n.tr("Copied");
        if (chip.kind === "file")
            return "nvim " + String(chip.label || chip.value || "");
        if (chip.kind === "dir")
            return I18n.tr("Open %1").arg(chip.label || chip.value || "");
        if (chip.kind === "cmd")
            return "$ " + String(chip.label || chip.value || "");
        return String(chip.label || "");
    }

    Timer {
        id: flashTimer
        interval: 1200
        onTriggered: root.flash = ""
    }

    Column {
        id: content
        width: parent.width
        spacing: Tokens.s3 * root.s

        Text {
            visible: root.session.busy
            text: root.sessionData.tools.length > 0 ? I18n.tr("Working with %1").arg(root.sessionData.tools[root.sessionData.tools.length - 1].title || I18n.tr("tools")) : I18n.tr("Thinking")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
        }

        Repeater {
            model: root.sessionData.tools || []
            delegate: Row {
                required property var modelData
                width: content.width
                spacing: Tokens.s2 * root.s
                Text {
                    text: root.statusLabel(modelData.status)
                    color: Tokens.inkMuted
                    font.family: Tokens.mono
                    font.pixelSize: Tokens.fTiny * root.s
                }
                Text {
                    width: parent.width - 90 * root.s
                    text: String(modelData.title || modelData.toolKind || I18n.tr("Tool"))
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall * root.s
                    elide: Text.ElideRight
                }
            }
        }

        TextEdit {
            width: parent.width
            visible: text.length > 0
            text: String(root.sessionData.answer || "").replace(/([^\n])\n(?!\n)/g, "$1  \n")
            textFormat: TextEdit.MarkdownText
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fBody * root.s
            wrapMode: TextEdit.Wrap
            readOnly: true
            selectByMouse: true
            selectionColor: Tokens.bone
            selectedTextColor: Tokens.inkOnBone
            onLinkActivated: url => Spawn.runApp(["xdg-open", url], { label: "browser" })
        }

        Text {
            width: parent.width
            visible: root.sessionData.needsAgent && String(root.sessionData.needsAgentReason || "").length > 0
            text: String(root.sessionData.needsAgentReason || "")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            font.italic: true
            wrapMode: Text.WordWrap
        }

        Text {
            width: parent.width
            visible: String(root.sessionData.error || "").length > 0
            text: String(root.sessionData.error || "")
            color: Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall * root.s
            wrapMode: Text.WordWrap
        }

        Flow {
            width: parent.width
            spacing: Tokens.s2 * root.s
            Repeater {
                model: root.chips
                delegate: Rectangle {
                    id: chip
                    required property var modelData
                    required property int index
                    readonly property bool current: index === root.selectedChip
                    width: chipText.implicitWidth + Tokens.s4 * root.s
                    height: Tokens.ctlH * root.s
                    radius: Tokens.radius * root.s
                    color: current ? Tokens.bone : chipTap.pressed ? Tokens.tint16 : chipHover.hovered ? Tokens.tint10 : Tokens.tint5
                    border.width: Tokens.border
                    border.color: current ? Tokens.bone : Tokens.lineStrong
                    Text {
                        id: chipText
                        anchors.centerIn: parent
                        text: root.caption(chip.modelData)
                        color: chip.current ? Tokens.inkOnBone : Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }
                    HoverHandler { id: chipHover; cursorShape: Qt.PointingHandCursor; onHoveredChanged: if (hovered) root.selectedChip = chip.index }
                    TapHandler { id: chipTap; onTapped: root.fire(chip.modelData) }
                }
            }
        }
    }
}

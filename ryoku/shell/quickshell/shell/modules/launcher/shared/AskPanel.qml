pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services as Svc
import "Singletons"
import Ryoku.Ui.Singletons

Item {
    id: root

    property real s: 1
    property string question: ""
    property bool sessionActive: false
    property bool resumeMode: false
    property int selectedChip: 0
    property int selectedRecent: 0
    property string flash: ""
    property var pendingBridgeAction: null
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    readonly property var bridge: bridgeLoader.item
    readonly property var sessionState: bridge ? bridge.session.state : ({ phase: "idle", busy: false, question: "", answer: "", actions: [], needsAgent: false, needsAgentReason: "", error: "", tools: [] })
    readonly property string phase: {
        if (!bridge)
            return "idle";
        if (bridge.client.ready)
            return String(sessionState.phase || "idle");
        return bridge.status === "connecting" ? "working" : "failed";
    }
    readonly property var recent: bridge ? bridge.session.sessions : []
    readonly property string working: {
        if (!bridge || bridge.status === "connecting")
            return I18n.tr("Connecting to Rashin");
        const tools = sessionState.tools || [];
        return tools.length > 0 ? String(tools[tools.length - 1].title || I18n.tr("Working")) : I18n.tr("Thinking");
    }
    readonly property string answerText: String(sessionState.answer || "")
    readonly property var answerImages: []
    readonly property var answerActions: sessionState.actions || []
    readonly property string answerId: bridge ? bridge.session.id : ""
    readonly property bool needsAgent: sessionState.needsAgent === true
    readonly property string needsAgentReason: String(sessionState.needsAgentReason || "")
    readonly property string errorText: {
        if (!bridge)
            return "";
        if (bridge.status === "off")
            return I18n.tr("Rashin is off. Turn Rashin on in Ryoku Settings > Rashin.");
        if (bridge.status === "not-installed")
            return I18n.tr("Rashin is not installed. Turn Rashin on in Ryoku Settings > Rashin.");
        if (bridge.status === "error")
            return bridge.client.errorText || I18n.tr("Rashin could not start. See %1").arg(bridge.client.logPath);
        return String(sessionState.error || bridge.errorText || "");
    }
    readonly property bool fromHistory: bridge ? bridge.fromHistory : false
    readonly property string askedQuestion: String(sessionState.question || "")
    readonly property bool busy: phase === "working" || sessionState.busy === true
    readonly property bool answerCurrent: phase === "done" && askedQuestion.length > 0
    readonly property var chips: {
        if (busy)
            return [
                { kind: "chat", value: "", label: I18n.tr("Continue in Agent") },
                { kind: "cancel", value: "", label: I18n.tr("Cancel") }
            ];
        if (phase !== "done")
            return [];
        const result = [];
        if (needsAgent)
            result.push({ kind: "chat", value: "", label: I18n.tr("Open Agent") });
        result.push({ kind: "copy", value: answerText, label: I18n.tr("Copy") });
        for (var i = 0; i < answerActions.length; i++)
            result.push(answerActions[i]);
        if (!needsAgent)
            result.push({ kind: "chat", value: "", label: I18n.tr("Open Agent") });
        return result;
    }

    signal finished()

    implicitHeight: col.implicitHeight

    Loader {
        id: bridgeLoader
        active: root.sessionActive
        sourceComponent: Component { LauncherAskBridge {} }
        onItemChanged: {
            if (item && root.pendingBridgeAction) {
                const action = root.pendingBridgeAction;
                root.pendingBridgeAction = null;
                action(item);
            }
        }
    }

    function useBridge(action) {
        sessionActive = true;
        if (bridgeLoader.item)
            action(bridgeLoader.item);
        else
            pendingBridgeAction = action;
    }

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

    function reset() {
        if (bridgeLoader.item)
            bridgeLoader.item.cancel();
        pendingBridgeAction = null;
        sessionActive = false;
        resumeMode = false;
        selectedChip = 0;
        selectedRecent = 0;
        flash = "";
    }

    function run() {
        const value = question.trim();
        if (value === "resume" || value === "recent") {
            openResume();
            return;
        }
        if (value.length === 0)
            return;
        resumeMode = false;
        selectedChip = 0;
        useBridge(function(target) { target.ask(value); });
    }

    function openResume() {
        resumeMode = true;
        selectedRecent = 0;
        useBridge(function(target) { target.recent(); });
    }

    function loadRecent(info) {
        if (!info || !info.id)
            return;
        resumeMode = false;
        selectedChip = 0;
        useBridge(function(target) { target.attach(info.id); });
    }

    function move(delta) {
        if (resumeMode) {
            if (recent.length > 0)
                selectedRecent = Math.max(0, Math.min(recent.length - 1, selectedRecent + delta));
        } else if (chips.length > 0) {
            selectedChip = Math.max(0, Math.min(chips.length - 1, selectedChip + delta));
        }
    }

    function activate() {
        if (resumeMode) {
            if (recent[selectedRecent])
                loadRecent(recent[selectedRecent]);
            return;
        }
        if (chips[selectedChip])
            fire(chips[selectedChip]);
    }

    function cancel() {
        if (bridge)
            bridge.cancel();
    }

    function fire(chip) {
        const kind = String(chip.kind || "");
        const value = String(chip.value || "");
        if (kind === "copy" || kind === "cmd" || kind === "color") {
            Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" | wl-copy", "_", value]);
            flash = kind + "\u0000" + value;
            flashTimer.restart();
            return;
        }
        if (kind === "cancel") {
            cancel();
            finished();
            return;
        }
        if (kind === "file")
            Spawn.runApp(["kitty", "-e", "nvim", value], { label: "kitty" });
        else if (kind === "dir" || kind === "url")
            Spawn.runApp(["xdg-open", value], { label: kind });
        else if (kind === "chat" && bridge) {
            bridge.handoff(function() {
                Svc.ShellState.requestSurfaceActive("ask#chat", undefined);
                root.finished();
            });
            return;
        }
        finished();
    }

    function chipCaption(chip) {
        if (flash === String(chip.kind || "") + "\u0000" + String(chip.value || ""))
            return I18n.tr("Copied");
        if (chip.kind === "file") return "nvim " + chip.label;
        if (chip.kind === "dir") return I18n.tr("Open %1").arg(chip.label);
        if (chip.kind === "cmd") return "$ " + chip.label;
        return chip.label;
    }

    Timer { id: flashTimer; interval: 1400; onTriggered: root.flash = "" }

    Column {
        id: col
        width: parent.width
        spacing: 8 * root.s

        Text {
            width: parent.width
            text: root.resumeMode ? I18n.tr("Rashin: recent asks") : I18n.tr("Rashin")
            color: Theme.faint
            font.family: Theme.mono
            font.pixelSize: Metrics.fontEyebrow * root.s
            font.letterSpacing: 1
        }

        Text {
            width: parent.width
            visible: root.phase === "idle" && !root.resumeMode
            text: root.question.trim().length === 0
                ? I18n.tr("Ask Rashin anything. Enter sends; \\resume recalls recent asks.")
                : I18n.tr("Enter to ask: %1").arg(root.question.trim())
            color: Theme.subtle
            font.family: Theme.font
            font.pixelSize: Metrics.fontSubtitle * root.s
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        Column {
            width: parent.width
            visible: root.resumeMode
            spacing: 3 * root.s
            Text {
                visible: root.recent.length === 0
                text: I18n.tr("No recent asks yet")
                color: Theme.faint
                font.family: Theme.font
                font.pixelSize: Metrics.fontSubtitle * root.s
            }
            Repeater {
                model: root.resumeMode ? root.recent : []
                delegate: Rectangle {
                    id: recentRow
                    required property var modelData
                    required property int index
                    width: parent.width
                    height: recentColumn.implicitHeight + 10 * root.s
                    radius: Theme.radius
                    color: index === root.selectedRecent ? Theme.tileBg : "transparent"
                    Column {
                        id: recentColumn
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.leftMargin: 10 * root.s
                        anchors.rightMargin: 10 * root.s
                        spacing: root.s
                        Text { width: parent.width; text: String(recentRow.modelData.title || recentRow.modelData.id || ""); color: recentRow.index === root.selectedRecent ? Theme.bright : Theme.subtle; font.family: Theme.font; font.pixelSize: Metrics.fontSubtitle * root.s; elide: Text.ElideRight }
                        Text { width: parent.width; text: String(recentRow.modelData.preview || "").replace(/\n/g, " "); color: Theme.faint; font.family: Theme.mono; font.pixelSize: Metrics.fontEyebrow * root.s; elide: Text.ElideRight }
                    }
                    HoverHandler { cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: root.loadRecent(recentRow.modelData) }
                }
            }
        }

        Row {
            visible: root.busy
            spacing: 8 * root.s
            Rectangle {
                width: 8 * root.s
                height: width
                radius: width / 2
                color: Theme.flameGlow
                anchors.verticalCenter: parent.verticalCenter
                SequentialAnimation on opacity {
                    running: root.busy
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.25; duration: 550; easing.type: Easing.InOutQuad }
                    NumberAnimation { to: 1; duration: 550; easing.type: Easing.InOutQuad }
                }
            }
            Text { text: root.working; color: Theme.subtle; font.family: Theme.mono; font.pixelSize: Metrics.fontSubtitle * root.s; elide: Text.ElideRight }
        }

        Repeater {
            model: root.sessionState.tools || []
            delegate: Text {
                required property var modelData
                width: col.width
                text: root.statusLabel(modelData.status) + "  " + String(modelData.title || I18n.tr("Tool"))
                color: Theme.faint
                font.family: Theme.mono
                font.pixelSize: Metrics.fontEyebrow * root.s
                elide: Text.ElideRight
            }
        }

        TextEdit {
            width: parent.width
            visible: root.phase === "done"
            text: root.answerText
            color: Theme.bright
            font.family: Theme.font
            font.pixelSize: Metrics.fontSubtitle * root.s
            wrapMode: TextEdit.Wrap
            readOnly: true
            selectByMouse: true
            selectionColor: Theme.vermDimDeep
            selectedTextColor: Theme.bright
        }

        Text { visible: root.phase === "done" && root.fromHistory; text: I18n.tr("From history"); color: Theme.faint; font.family: Theme.mono; font.pixelSize: Metrics.fontEyebrow * root.s }
        Text { width: parent.width; visible: root.needsAgent && root.needsAgentReason.length > 0; text: root.needsAgentReason; color: Theme.faint; font.family: Theme.font; font.pixelSize: Metrics.fontSubtitle * root.s; wrapMode: Text.WordWrap }
        Text { width: parent.width; visible: root.phase === "failed"; text: root.errorText; color: Theme.verm; font.family: Theme.mono; font.pixelSize: Metrics.fontSubtitle * root.s; wrapMode: Text.WordWrap; maximumLineCount: 3 }

        Flow {
            width: parent.width
            visible: root.chips.length > 0 && !root.resumeMode
            spacing: 6 * root.s
            Repeater {
                model: root.chips
                delegate: Rectangle {
                    id: chipBox
                    required property var modelData
                    required property int index
                    readonly property bool current: index === root.selectedChip
                    readonly property bool danger: modelData.kind === "cancel"
                    width: chipLabel.implicitWidth + 18 * root.s
                    height: chipLabel.implicitHeight + 10 * root.s
                    radius: Theme.radius
                    color: current || chipHover.hovered ? (danger ? Theme.vermDeep : Theme.verm) : "transparent"
                    border.color: current || chipHover.hovered ? (danger ? Theme.vermDeep : Theme.verm) : Theme.border
                    border.width: 1
                    Text { id: chipLabel; anchors.centerIn: parent; text: root.chipCaption(chipBox.modelData); color: chipBox.current || chipHover.hovered ? Theme.cardTop : Theme.subtle; font.family: Theme.mono; font.pixelSize: Metrics.fontEyebrow * root.s; font.letterSpacing: 1 }
                    HoverHandler { id: chipHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: root.fire(chipBox.modelData) }
                }
            }
        }

        Text {
            visible: root.chips.length > 0 || root.resumeMode
            text: root.resumeMode ? I18n.tr("↑↓ Pick · Enter recall · Esc back")
                : root.busy ? I18n.tr("↑↓ Pick · Enter fires · Esc cancels")
                : I18n.tr("↑↓ Walk chips · Enter fires · Type to re-ask · Esc dismisses")
            color: Theme.faint
            font.family: Theme.mono
            font.pixelSize: Metrics.fontEyebrow * root.s
        }
    }
}

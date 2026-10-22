pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.containers
import caelestia.components.controls
import caelestia.services
import "chatsessions.js" as Sessions

// The AI chat history: a searchable list of the store's chats, grouped by
// when they were last active, with pin, rename, copy and delete per chat, and
// clearing everything unpinned. Deletions can be undone for a few seconds.
//
// The list is rebuilt from the store when it changes, but only while the pane
// is shown, and not while a chat is being renamed (that would tear down the
// editor).
Item {
    id: root

    required property ChatStore store
    // The pane is shown.
    property bool active
    // The open chat is getting a reply: undoing a deletion leaves it open.
    property bool busy

    property string query: ""
    property string renamingId: ""
    property bool dirty: true
    // From the last rebuild: every chat the list can show, whatever the query.
    property int total: 0
    property int pinnedCount: 0
    property var unpinnedIds: []
    // { removed, previousChatId, switchedTo, label } while a deletion can be undone.
    property var undo: null

    readonly property ListModel rows: ListModel {}
    readonly property string home: Quickshell.env("HOME") || ""

    signal chatOpened(string chatId)
    signal newChatRequested
    // Before chats are removed, so whatever is running in them can be stopped.
    signal removing(var chatIds)
    // The history stays shown while the chat behind it changes: after the open
    // chat was deleted (chatId is the next one, or "" for a new chat), or when
    // undoing that.
    signal switchChat(string chatId)

    function markDirty(): void {
        dirty = true;
        if (active && !renamingId)
            rebuild();
    }

    function rebuild(): void {
        if (!store)
            return;
        dirty = false;
        const list = Sessions.historyRows(store.sessions, query, new Date());
        total = list.total;
        pinnedCount = list.pinned;
        unpinnedIds = list.unpinnedIds;

        rows.clear();
        for (let i = 0; i < list.rows.length; i++) {
            const r = list.rows[i];
            rows.append({
                "chatId": r.chatId,
                "title": r.title,
                "preview": (r.previewIsUser ? qsTr("You: ") : "") + r.preview,
                "pinned": r.pinned,
                "timeText": ageText(r.ts),
                "msgCount": r.msgCount,
                "isClaudeCode": r.isClaudeCode,
                "cwd": shortPath(r.cwd),
                "section": r.section
            });
        }
    }

    function sectionLabel(section: string): string {
        switch (section) {
        case "pinned":
            return qsTr("Pinned");
        case "today":
            return qsTr("Today");
        case "yesterday":
            return qsTr("Yesterday");
        case "week":
            return qsTr("Previous 7 days");
        case "month":
            return qsTr("Previous 30 days");
        default:
            return qsTr("Older");
        }
    }

    function ageText(ts: real): string {
        const age = Sessions.chatAge(ts, new Date());
        const d = new Date(ts);
        switch (age.kind) {
        case "now":
            return qsTr("just now");
        case "minutes":
            return qsTr("%1 min ago").arg(age.minutes);
        case "time":
            return Qt.formatTime(d, "hh:mm");
        case "weekday":
            return Qt.formatDateTime(d, "ddd hh:mm");
        case "date":
            return Qt.formatDate(d, "MMM d");
        case "fullDate":
            return Qt.formatDate(d, "yyyy-MM-dd");
        default:
            return "";
        }
    }

    function shortPath(p: string): string {
        if (home !== "" && (p === home || p.indexOf(home + "/") === 0))
            return "~" + p.substring(home.length);
        return p;
    }

    // Escapes text for Text.StyledText and emphasises occurrences of the search query.
    function highlight(text: string): string {
        const esc = s => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
        const out = esc(text || "");
        const q = query.trim();
        if (!q)
            return out;
        const pattern = esc(q).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
        return out.replace(new RegExp(pattern, "gi"), m => "<b><font color=\"" + Colours.palette.m3primary + "\">" + m + "</font></b>");
    }

    function togglePin(id: string): void {
        const s = store.session(id);
        if (s)
            store.setChatProps(id, { "pinned": !s.pinned });
    }

    function rename(id: string, title: string): void {
        store.rename(id, (title || "").trim().replace(/\s+/g, " "));
        renamingId = "";
        markDirty();
    }

    function copyAsMarkdown(id: string): void {
        const s = store.session(id);
        if (s)
            Quickshell.clipboardText = Sessions.asMarkdown(s);
    }

    // Removes chats, keeping them around for a short undo window.
    function removeChats(ids: var, label: string): void {
        removing(ids);
        const previousChatId = store.currentChatId;
        const removed = store.takeChats(ids);
        if (removed.length === 0)
            return;
        if (!store.current())
            switchChat(store.sessions.length > 0 ? store.sessions[0].id : "");
        undo = {
            "removed": removed,
            "previousChatId": previousChatId,
            "switchedTo": store.currentChatId,
            "label": label
        };
        undoTimer.restart();
    }

    function deleteChat(id: string): void {
        removeChats([id], qsTr("Chat deleted"));
    }

    // Pinned chats survive "clear"; with none pinned this wipes everything.
    function clearUnpinned(): void {
        if (dirty)
            rebuild();
        const ids = unpinnedIds;
        removeChats(ids, ids.length === 1 ? qsTr("1 chat cleared") : qsTr("%1 chats cleared").arg(ids.length));
    }

    function undoRemoval(): void {
        const u = undo;
        undo = null;
        undoTimer.stop();
        if (!u)
            return;
        store.restoreChats(u.removed);
        if (store.currentChatId === u.switchedTo && u.switchedTo !== u.previousChatId && store.session(u.previousChatId) && !busy)
            switchChat(u.previousChatId);
    }

    opacity: active ? 1 : 0
    visible: opacity > 0
    onActiveChanged: {
        if (active && dirty)
            rebuild();
    }

    Behavior on opacity { NumberAnimation { duration: 250; easing.type: Easing.InOutQuad } }

    Connections {
        function onRevisionChanged(): void {
            root.markDirty();
        }

        target: root.store
    }

    Timer {
        id: undoTimer

        interval: 8000
        onTriggered: root.undo = null
    }

    SearchBar {
        id: historySearch

        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        topPadding: Tokens.padding.medium
        bottomPadding: Tokens.padding.medium
        placeholderText: root.total === 1 ? qsTr("Search 1 chat") : qsTr("Search %1 chats").arg(root.total)
        bg.color: Colours.tPalette.m3surfaceContainerHigh

        onTextChanged: {
            root.query = text;
            historySearchDebounce.restart();
        }
        Keys.onReturnPressed: {
            if (rows.count > 0)
                root.chatOpened(rows.get(0).chatId);
        }
        Keys.onEscapePressed: event => {
            if (text) {
                clear();
                event.accepted = true;
            } else {
                event.accepted = false;
            }
        }
    }

    Timer {
        id: historySearchDebounce

        interval: 150
        onTriggered: root.rebuild()
    }

    StyledListView {
        id: historyList

        anchors.top: historySearch.bottom
        anchors.bottom: historyActions.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.topMargin: Tokens.spacing.small
        anchors.bottomMargin: Tokens.spacing.medium
        clip: true
        spacing: Tokens.spacing.small
        model: rows

        section.property: "section"
        section.delegate: StyledText {
            required property string section

            width: historyList.width
            topPadding: Tokens.spacing.medium
            bottomPadding: Tokens.spacing.small
            leftPadding: Tokens.padding.small
            text: root.sectionLabel(section)
            color: Colours.palette.m3primary
            font: Tokens.font.label.medium
        }

        ScrollBar.vertical: StyledScrollBar {
            flickable: historyList
        }

        delegate: Item {
            id: chatRow

            required property string chatId
            required property string title
            required property string preview
            required property bool pinned
            required property string timeText
            required property int msgCount
            required property bool isClaudeCode
            required property string cwd

            readonly property bool isCurrent: chatId === root.store.currentChatId
            readonly property bool renaming: root.renamingId === chatId
            property bool confirmDelete: false
            property bool copied: false
            readonly property bool showActions: (rowHover.hovered || confirmDelete) && !renaming

            width: historyList.width
            implicitHeight: card.implicitHeight

            onShowActionsChanged: {
                if (!showActions)
                    confirmDelete = false;
            }

            Timer {
                id: confirmDeleteReset

                interval: 3000
                onTriggered: chatRow.confirmDelete = false
            }

            Timer {
                id: copiedReset

                interval: 1500
                onTriggered: chatRow.copied = false
            }

            StyledRect {
                id: card

                anchors.left: parent.left
                anchors.right: parent.right
                implicitHeight: rowContent.implicitHeight + Tokens.padding.medium * 2
                radius: Tokens.rounding.medium
                color: chatRow.isCurrent ? Colours.palette.m3secondaryContainer : Colours.tPalette.m3surfaceContainerHigh

                HoverHandler {
                    id: rowHover
                }

                StateLayer {
                    radius: Tokens.rounding.medium
                    disabled: chatRow.renaming
                    onClicked: root.chatOpened(chatRow.chatId)
                }

                RowLayout {
                    id: rowContent

                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.leftMargin: Tokens.padding.medium
                    anchors.rightMargin: Tokens.padding.medium
                    spacing: Tokens.spacing.medium

                    StyledRect {
                        Layout.alignment: Qt.AlignTop
                        Layout.preferredWidth: 32
                        Layout.preferredHeight: 32
                        radius: Tokens.rounding.full
                        color: chatRow.isCurrent ? Colours.palette.m3primary : Colours.tPalette.m3surfaceContainerHighest

                        MaterialIcon {
                            anchors.centerIn: parent
                            text: chatRow.isClaudeCode ? "terminal" : "chat"
                            color: chatRow.isCurrent ? Colours.palette.m3onPrimary : Colours.palette.m3onSurfaceVariant
                            font: Tokens.font.icon.small
                        }
                    }

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 2

                        RowLayout {
                            Layout.fillWidth: true
                            Layout.preferredHeight: Math.max(titleText.implicitHeight, rowActions.implicitHeight)
                            spacing: Tokens.spacing.small

                            MaterialIcon {
                                visible: chatRow.pinned && !chatRow.renaming
                                text: "keep"
                                fill: 1
                                color: Colours.palette.m3primary
                                fontStyle: Tokens.font.icon.builders.small.scale(0.85).build()
                            }

                            StyledText {
                                id: titleText

                                Layout.fillWidth: true
                                visible: !chatRow.renaming
                                text: root.highlight(chatRow.title)
                                textFormat: Text.StyledText
                                color: chatRow.isCurrent ? Colours.palette.m3onSecondaryContainer : Colours.palette.m3onSurface
                                font: Tokens.font.title.small
                                elide: Text.ElideRight
                                maximumLineCount: 1
                            }

                            TextFieldBase {
                                id: renameField

                                Layout.fillWidth: true
                                visible: chatRow.renaming
                                leftPadding: Tokens.padding.small
                                rightPadding: Tokens.padding.small
                                topPadding: 2
                                bottomPadding: 2
                                font: Tokens.font.title.small

                                background: StyledRect {
                                    radius: Tokens.rounding.small
                                    color: Colours.tPalette.m3surfaceContainerHighest
                                    border.width: 1
                                    border.color: Colours.palette.m3primary
                                }

                                onVisibleChanged: {
                                    if (visible) {
                                        text = chatRow.title;
                                        forceActiveFocus();
                                        selectAll();
                                    }
                                }
                                onActiveFocusChanged: {
                                    if (!activeFocus && chatRow.renaming)
                                        root.rename(chatRow.chatId, text);
                                }
                                Keys.onReturnPressed: root.rename(chatRow.chatId, text)
                                Keys.onEnterPressed: root.rename(chatRow.chatId, text)
                                Keys.onEscapePressed: {
                                    root.renamingId = "";
                                    root.markDirty();
                                }
                            }

                            Row {
                                id: rowActions

                                visible: chatRow.showActions

                                IconButton {
                                    id: pinButton

                                    type: IconButton.Text
                                    font: Tokens.font.icon.small
                                    icon: chatRow.pinned ? "keep_off" : "keep"
                                    onClicked: root.togglePin(chatRow.chatId)

                                    Tooltip {
                                        target: pinButton
                                        text: chatRow.pinned ? qsTr("Unpin") : qsTr("Pin to top")
                                    }
                                }

                                IconButton {
                                    id: renameButton

                                    type: IconButton.Text
                                    font: Tokens.font.icon.small
                                    icon: "edit"
                                    onClicked: root.renamingId = chatRow.chatId

                                    Tooltip {
                                        target: renameButton
                                        text: qsTr("Rename")
                                    }
                                }

                                IconButton {
                                    id: copyButton

                                    type: IconButton.Text
                                    font: Tokens.font.icon.small
                                    icon: chatRow.copied ? "check" : "content_copy"
                                    onClicked: {
                                        root.copyAsMarkdown(chatRow.chatId);
                                        chatRow.copied = true;
                                        copiedReset.restart();
                                    }

                                    Tooltip {
                                        target: copyButton
                                        text: qsTr("Copy as Markdown")
                                    }
                                }

                                IconButton {
                                    id: deleteButton

                                    type: IconButton.Text
                                    font: Tokens.font.icon.small
                                    icon: chatRow.confirmDelete ? "delete_forever" : "delete"
                                    inactiveOnColour: chatRow.confirmDelete ? Colours.palette.m3error : Colours.palette.m3onSurfaceVariant
                                    onClicked: {
                                        if (chatRow.confirmDelete) {
                                            root.deleteChat(chatRow.chatId);
                                        } else {
                                            chatRow.confirmDelete = true;
                                            confirmDeleteReset.restart();
                                        }
                                    }

                                    Tooltip {
                                        target: deleteButton
                                        text: chatRow.confirmDelete ? qsTr("Click again to delete") : qsTr("Delete")
                                    }
                                }
                            }
                        }

                        StyledText {
                            Layout.fillWidth: true
                            visible: chatRow.preview !== ""
                            text: root.highlight(chatRow.preview)
                            textFormat: Text.StyledText
                            color: chatRow.isCurrent ? Colours.palette.m3onSecondaryContainer : Colours.palette.m3onSurfaceVariant
                            opacity: chatRow.isCurrent ? 0.8 : 1
                            font: Tokens.font.body.small
                            wrapMode: Text.Wrap
                            maximumLineCount: 2
                            elide: Text.ElideRight
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            Layout.topMargin: 2
                            spacing: Tokens.spacing.small

                            StyledText {
                                text: chatRow.timeText
                                color: Colours.palette.m3outline
                                font: Tokens.font.label.small
                            }

                            StyledText {
                                text: "·"
                                color: Colours.palette.m3outline
                                font: Tokens.font.label.small
                            }

                            StyledText {
                                text: chatRow.msgCount === 1 ? qsTr("1 message") : qsTr("%1 messages").arg(chatRow.msgCount)
                                color: Colours.palette.m3outline
                                font: Tokens.font.label.small
                            }

                            MaterialIcon {
                                visible: chatRow.cwd !== ""
                                Layout.leftMargin: Tokens.spacing.small
                                text: "folder"
                                color: Colours.palette.m3outline
                                fontStyle: Tokens.font.icon.builders.small.scale(0.8).build()
                            }

                            StyledText {
                                Layout.fillWidth: true
                                visible: chatRow.cwd !== ""
                                text: chatRow.cwd
                                color: Colours.palette.m3outline
                                font: Tokens.font.label.small
                                elide: Text.ElideMiddle
                            }

                            Item {
                                Layout.fillWidth: true
                                visible: chatRow.cwd === ""
                            }
                        }
                    }
                }
            }
        }
    }

    ColumnLayout {
        anchors.centerIn: historyList
        width: historyList.width - Tokens.padding.large * 2
        visible: rows.count === 0
        spacing: Tokens.spacing.small

        MaterialIcon {
            Layout.alignment: Qt.AlignHCenter
            text: root.query.trim() ? "search_off" : "forum"
            color: Colours.palette.m3outline
            font: Tokens.font.icon.extraLarge
        }

        StyledText {
            Layout.fillWidth: true
            horizontalAlignment: Text.AlignHCenter
            text: root.query.trim() ? qsTr("No chats match \"%1\"").arg(root.query.trim()) : qsTr("No saved chats yet")
            color: Colours.palette.m3onSurfaceVariant
            font: Tokens.font.body.medium
            wrapMode: Text.Wrap
        }

        StyledText {
            Layout.fillWidth: true
            horizontalAlignment: Text.AlignHCenter
            visible: !GlobalConfig.ai.saveChatHistory && !root.query.trim()
            text: qsTr("Saving chat history is turned off in AI settings")
            color: Colours.palette.m3outline
            font: Tokens.font.body.small
            wrapMode: Text.Wrap
        }
    }

    StyledRect {
        id: historyUndoBar

        anchors.bottom: historyActions.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottomMargin: Tokens.spacing.small
        z: 1
        implicitHeight: undoLayout.implicitHeight + Tokens.padding.small * 2
        radius: Tokens.rounding.medium
        color: Colours.palette.m3inverseSurface
        opacity: root.undo ? 1 : 0
        visible: opacity > 0

        Behavior on opacity { NumberAnimation { duration: 200; easing.type: Easing.OutCubic } }

        RowLayout {
            id: undoLayout

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Tokens.padding.large
            anchors.rightMargin: Tokens.padding.small
            spacing: Tokens.spacing.small

            StyledText {
                Layout.fillWidth: true
                text: root.undo ? root.undo.label : ""
                color: Colours.palette.m3inverseOnSurface
                font: Tokens.font.body.small
                elide: Text.ElideRight
            }

            StyledRect {
                Layout.preferredWidth: undoText.implicitWidth + Tokens.padding.large * 2
                Layout.preferredHeight: 28
                radius: 14

                StateLayer {
                    radius: 14
                    color: Colours.palette.m3inversePrimary
                    onClicked: root.undoRemoval()
                }

                StyledText {
                    id: undoText

                    anchors.centerIn: parent
                    text: qsTr("Undo")
                    color: Colours.palette.m3inversePrimary
                    font: Tokens.font.label.large
                }
            }
        }
    }

    RowLayout {
        id: historyActions

        readonly property int clearableCount: root.total - root.pinnedCount
        readonly property string clearIdleLabel: root.pinnedCount > 0 ? qsTr("Clear unpinned") : qsTr("Clear all")
        readonly property string clearHoldLabel: qsTr("Hold to clear")
        // Both buttons share one width so the count stays centred, and the
        // clear label can change without the button resizing.
        readonly property real buttonWidth: Math.max(clearAllIcon.implicitWidth + Tokens.spacing.small + Math.max(idleLabelMetrics.advanceWidth, holdLabelMetrics.advanceWidth), newChatLayout.implicitWidth) + Tokens.padding.large * 2

        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        spacing: Tokens.spacing.small

        TextMetrics {
            id: idleLabelMetrics

            font: Tokens.font.body.small
            text: historyActions.clearIdleLabel
        }

        TextMetrics {
            id: holdLabelMetrics

            font: Tokens.font.body.small
            text: historyActions.clearHoldLabel
        }

        // Press and hold to clear, so a stray click does nothing.
        StyledClippingRect {
            id: clearAllButton

            readonly property bool holding: clearLayer.pressed
            property real holdProgress: 0
            property bool showHoldHint: false

            Layout.preferredWidth: historyActions.buttonWidth
            Layout.preferredHeight: 32
            radius: 16
            visible: historyActions.clearableCount > 0
            color: "transparent"
            border.width: 1
            border.color: holding ? Colours.palette.m3error : Colours.palette.m3outlineVariant

            onHoldingChanged: {
                if (holding) {
                    holdAnim.restart();
                } else {
                    // Released too early: say why nothing happened.
                    if (holdProgress > 0 && holdProgress < 1) {
                        showHoldHint = true;
                        holdHintReset.restart();
                    }
                    holdAnim.stop();
                    holdProgress = 0;
                }
            }

            Timer {
                id: holdHintReset

                interval: 2000
                onTriggered: clearAllButton.showHoldHint = false
            }

            NumberAnimation {
                id: holdAnim

                target: clearAllButton
                property: "holdProgress"
                from: 0
                to: 1
                duration: 1200
                onFinished: {
                    if (clearAllButton.holding) {
                        clearAllButton.holdProgress = 0;
                        root.clearUnpinned();
                    }
                }
            }

            Rectangle {
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                width: parent.width * clearAllButton.holdProgress
                color: Colours.palette.m3errorContainer
            }

            StateLayer {
                id: clearLayer

                radius: 16
            }

            RowLayout {
                id: clearAllLayout

                anchors.centerIn: parent
                spacing: Tokens.spacing.small

                MaterialIcon {
                    id: clearAllIcon

                    text: "delete_sweep"
                    color: clearAllButton.holding ? Colours.palette.m3onErrorContainer : Colours.palette.m3error
                    font: Tokens.font.icon.small
                }
                Text {
                    text: clearAllButton.holding || clearAllButton.showHoldHint ? historyActions.clearHoldLabel : historyActions.clearIdleLabel
                    color: clearAllButton.holding ? Colours.palette.m3onErrorContainer : Colours.palette.m3error
                    font: Tokens.font.body.small
                }
            }

            Tooltip {
                target: clearAllButton
                text: qsTr("Press and hold to delete %1 chats").arg(historyActions.clearableCount)
            }
        }

        Item {
            Layout.fillWidth: true
        }

        StyledRect {
            id: newChatButton

            Layout.preferredWidth: historyActions.buttonWidth
            Layout.preferredHeight: 32
            radius: 16
            color: Colours.palette.m3primaryContainer

            StateLayer {
                radius: 16
                onClicked: root.newChatRequested()
            }

            RowLayout {
                id: newChatLayout

                anchors.centerIn: parent
                spacing: Tokens.spacing.small

                MaterialIcon {
                    text: "add"
                    color: Colours.palette.m3onPrimaryContainer
                    font: Tokens.font.icon.small
                }
                Text {
                    text: qsTr("New Chat")
                    color: Colours.palette.m3onPrimaryContainer
                    font: Tokens.font.body.small
                }
            }
        }
    }
}

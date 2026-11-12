pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Dialogs
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services

Rectangle {
    id: root

    required property real s
    required property var screen
    required property RashinClient client
    property bool active: false
    property string mode: "ask"
    property string tool: ""
    property string query: ""
    property real maximumHeight: 640 * s
    property bool historyOpen: false
    property bool modelOpen: false
    property var attachments: []
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    readonly property bool aiMode: mode === "ask" || mode === "chat"
    readonly property bool available: client.ready
    readonly property string headerLabel: mode === "chat" ? I18n.tr("Rashin // Agent")
        : mode === "tools" ? I18n.tr("Rashin // Tools")
        : mode === "web" ? I18n.tr("Rashin // Web") : I18n.tr("Rashin // Ask")
    readonly property real bodyBudget: Math.max(100 * s, maximumHeight - 190 * s)
    readonly property real selectedBodyHeight: !available && aiMode ? unavailable.implicitHeight
        : mode === "chat" ? chat.implicitHeight
        : mode === "tools" ? tools.implicitHeight
        : mode === "web" ? web.implicitHeight : answer.implicitHeight
    readonly property real drawerReserve: historyOpen || modelOpen ? Math.min(220 * s, maximumHeight * 0.34) : 0

    signal requestClose()
    signal modeRequested(string mode)
    signal bubblePinned(string question, string answer)

    implicitHeight: Math.min(maximumHeight, content.implicitHeight + drawerReserve + Tokens.s4 * s * 2)
    radius: Tokens.radius * s
    color: Tokens.paper
    border.width: Tokens.border
    border.color: Tokens.lineStrong
    clip: true

    RashinSession { id: askSession; client: root.client; kind: "ask" }
    RashinSession { id: agentSession; client: root.client; kind: "agent" }

    function focusField() { field.focusField(); }

    function prepareAgent() {
        if (client.ready && agentSession.id.length === 0)
            agentSession.loadLatest();
    }

    function requestMode(next) {
        if (["ask", "chat", "tools", "web"].indexOf(next) < 0 || next === mode)
            return;
        closeDrawers();
        modeRequested(next);
        if (next === "chat")
            Qt.callLater(prepareAgent);
        Qt.callLater(field.focusField);
    }

    function closeDrawers() {
        historyOpen = false;
        modelOpen = false;
        chat.modelMenuOpen = false;
    }

    function toggleHistory() {
        modelOpen = false;
        historyOpen = !historyOpen;
        if (!historyOpen || !client.ready)
            return;
        if (mode === "ask")
            askSession.list(20);
        else if (mode === "chat")
            agentSession.list(1);
    }

    function toggleModel() {
        if (mode !== "chat" || !client.ready)
            return;
        historyOpen = false;
        modelOpen = !modelOpen;
    }

    function relative(stamp) {
        const time = Date.parse(String(stamp || ""));
        if (isNaN(time))
            return "";
        const minutes = Math.max(0, Math.round((Date.now() - time) / 60000));
        if (minutes < 1) return I18n.tr("Just now");
        if (minutes < 60) return I18n.tr("%1m ago").arg(minutes);
        if (minutes < 1440) return I18n.tr("%1h ago").arg(Math.round(minutes / 60));
        return I18n.tr("%1d ago").arg(Math.round(minutes / 1440));
    }

    readonly property var historyRows: {
        const rows = [];
        const sessions = mode === "ask" ? askSession.sessions : agentSession.sessions;
        if (sessions.length > 0)
            rows.push({ section: mode === "ask" ? I18n.tr("Recent asks") : I18n.tr("Latest Agent session") });
        for (var i = 0; i < sessions.length; i++) {
            const info = sessions[i];
            rows.push({
                kind: "session",
                value: String(info.id || ""),
                label: String(info.title || info.id || ""),
                sub: String(info.preview || root.relative(info.updated)),
                active: String(info.id || "") === (mode === "ask" ? askSession.id : agentSession.id)
            });
        }
        return rows;
    }

    readonly property var modelRows: {
        const rows = [{ section: I18n.tr("Agent models") }];
        const models = agentSession.state.models || [];
        for (var i = 0; i < models.length; i++) {
            const model = models[i];
            rows.push({
                kind: "model",
                value: String(model.id || ""),
                label: String(model.name || model.id || ""),
                sub: String(model.description || ""),
                active: String(model.id || "") === String(agentSession.state.model || "")
            });
        }
        if (models.length === 0)
            rows.push({ kind: "note", disabled: true, label: I18n.tr("Models load with the harness"), sub: I18n.tr("Send an Agent message first") });
        return rows;
    }

    function pickHistory(row) {
        historyOpen = false;
        if (mode === "ask")
            askSession.attach(row.value);
        else
            agentSession.attach(row.value);
    }

    function pickModel(row) {
        modelOpen = false;
        if (row.kind === "model")
            agentSession.setModel(row.value);
    }

    function attachmentPath(url) {
        var value = String(url || "");
        if (value.indexOf("file://") === 0)
            value = value.slice(7);
        try { return decodeURIComponent(value); }
        catch (error) { return value; }
    }

    function submit(value) {
        const text = String(value || "").trim();
        if (text.length === 0 && historyOpen) {
            historyDrawer.activate();
            return;
        }
        if (text.length === 0 && modelOpen) {
            modelDrawer.activate();
            return;
        }
        closeDrawers();
        if (mode === "tools") {
            tools.activate(text);
        } else if (mode === "web") {
            web.activate();
        } else if (!client.ready) {
            return;
        } else if (mode === "chat") {
            agentSession.prompt(text);
            if (text.length > 0)
                query = "";
        } else {
            askSession.ask(text, attachments);
            if (text.length > 0) {
                query = "";
                attachments = [];
            }
        }
    }

    function move(delta) {
        if (historyOpen) historyDrawer.move(delta);
        else if (modelOpen) modelDrawer.move(delta);
        else if (mode === "tools") tools.move(delta);
        else if (mode === "web") web.move(delta);
        else if (mode === "ask") answer.move(delta);
    }

    function cycleMode() {
        const modes = ["ask", "chat", "tools", "web"];
        const index = Math.max(0, modes.indexOf(mode));
        requestMode(modes[(index + 1) % modes.length]);
    }

    function openRashinSettings() {
        Bus.run("hub open rashin");
        requestClose();
    }

    onModeChanged: if (mode === "chat") prepareAgent()
    onActiveChanged: if (active && mode === "chat") prepareAgent()

    Connections {
        target: root.client
        function onBecameReady() { if (root.mode === "chat") root.prepareAgent(); }
    }

    FileDialog {
        id: attachDialog
        title: I18n.tr("Attach images")
        fileMode: FileDialog.OpenFiles
        nameFilters: [I18n.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.gif *.bmp)"]
        onAccepted: {
            const next = [];
            for (var i = 0; i < selectedFiles.length; i++)
                next.push(root.attachmentPath(selectedFiles[i]));
            root.attachments = next;
        }
    }

    Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: Tokens.s4 * root.s
        spacing: Tokens.s3 * root.s

        Item {
            id: header
            width: parent.width
            height: 30 * root.s
            Image {
                id: seal
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: 16 * root.s
                height: width
                source: "image://icon/dev.ryoku.rashin"
                sourceSize: Qt.size(width, height)
            }
            Text {
                anchors.left: seal.right
                anchors.leftMargin: Tokens.s2 * root.s
                anchors.verticalCenter: parent.verticalCenter
                text: root.headerLabel
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fMicro * root.s
                font.letterSpacing: Tokens.trackMark
            }
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s1 * root.s
                Rectangle {
                    visible: root.aiMode
                    width: 30 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    color: root.historyOpen ? Tokens.bone : historyHover.hovered ? Tokens.tint10 : "transparent"
                    Text { anchors.centerIn: parent; text: "history"; color: root.historyOpen ? Tokens.inkOnBone : Tokens.inkDim; font.family: "Material Symbols Rounded"; font.pixelSize: Tokens.fBody * root.s }
                    HoverHandler { id: historyHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: root.toggleHistory() }
                }
                Rectangle {
                    visible: root.mode === "chat"
                    width: 30 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    color: root.modelOpen ? Tokens.bone : modelHover.hovered ? Tokens.tint10 : "transparent"
                    Text { anchors.centerIn: parent; text: "tune"; color: root.modelOpen ? Tokens.inkOnBone : Tokens.inkDim; font.family: "Material Symbols Rounded"; font.pixelSize: Tokens.fBody * root.s }
                    HoverHandler { id: modelHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: root.toggleModel() }
                }
                Rectangle {
                    visible: root.mode === "ask" && root.available
                    width: 30 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    color: attachHover.hovered ? Tokens.tint10 : "transparent"
                    Text { anchors.centerIn: parent; text: "attach_file"; color: Tokens.inkDim; font.family: "Material Symbols Rounded"; font.pixelSize: Tokens.fBody * root.s }
                    HoverHandler { id: attachHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: attachDialog.open() }
                }
            }
        }

        AskField {
            id: field
            width: parent.width
            s: root.s
            active: root.active
            mode: root.mode
            phase: root.mode === "chat" ? agentSession.state.phase : askSession.state.phase
            working: root.mode === "chat" ? I18n.tr("Agent working") : I18n.tr("Ask working")
            chatBusy: agentSession.busy
            chatActivity: agentSession.state.phase
            pendingHandoff: false
            horizontalNavigation: root.mode === "ask" && answer.chips.length > 0
            text: root.query
            onTextChanged: if (root.query !== text) root.query = text
            onSubmitted: value => root.submit(value)
            onPrefixRequested: (next, value) => { root.query = value; root.requestMode(next); }
            onTabRequested: root.cycleMode()
            onUpRequested: root.move(-1)
            onDownRequested: root.move(1)
            onLeftRequested: answer.move(-1)
            onRightRequested: answer.move(1)
            onNewChatRequested: { agentSession.reset(); root.query = ""; }
            onEscapeRequested: (root.historyOpen || root.modelOpen) ? root.closeDrawers() : root.requestClose()
            onHistoryRequested: root.toggleHistory()
            onModelRequested: root.toggleModel()
        }

        Flow {
            width: parent.width
            visible: root.mode === "ask" && root.attachments.length > 0
            spacing: Tokens.s2 * root.s
            Repeater {
                model: root.attachments
                delegate: Rectangle {
                    id: attachmentChip
                    required property string modelData
                    width: attachmentLabel.implicitWidth + Tokens.s5 * root.s
                    height: 26 * root.s
                    radius: Tokens.radius * root.s
                    color: Tokens.tint5
                    border.width: Tokens.border
                    border.color: Tokens.lineSoft
                    Text { id: attachmentLabel; anchors.centerIn: parent; width: Math.min(implicitWidth, 180 * root.s); text: attachmentChip.modelData.split("/").pop(); color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny * root.s; elide: Text.ElideMiddle }
                    TapHandler { onTapped: root.attachments = root.attachments.filter(function(path) { return path !== attachmentChip.modelData; }) }
                }
            }
        }

        Flickable {
            id: modeViewport
            width: parent.width
            height: Math.min(root.selectedBodyHeight, root.bodyBudget)
            contentWidth: width
            contentHeight: root.selectedBodyHeight
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            interactive: contentHeight > height
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded; visible: modeViewport.contentHeight > modeViewport.height + 1 }

            Item {
                id: unavailable
                width: modeViewport.width
                visible: root.aiMode && !root.available
                implicitHeight: unavailableColumn.implicitHeight
                Column {
                    id: unavailableColumn
                    width: parent.width
                    spacing: Tokens.s3 * root.s
                    Text {
                        width: parent.width
                        text: root.client.state === "not-installed"
                            ? I18n.tr("Rashin is not installed. Turn Rashin on in Ryoku Settings > Apps > Rashin.")
                            : root.client.state === "off"
                                ? I18n.tr("Rashin is off. Turn Rashin on in Ryoku Settings > Apps > Rashin.")
                                : root.client.state === "error"
                                    ? (root.client.errorText || I18n.tr("Rashin could not start. See %1").arg(root.client.logPath))
                                    : I18n.tr("Connecting to Rashin")
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        wrapMode: Text.WordWrap
                    }
                    Rectangle {
                        visible: root.client.state === "off" || root.client.state === "not-installed"
                        width: settingsLabel.implicitWidth + Tokens.s4 * root.s
                        height: Tokens.ctlH * root.s
                        radius: Tokens.radius * root.s
                        color: settingsTap.pressed ? Tokens.tint16 : Tokens.bone
                        Text { id: settingsLabel; anchors.centerIn: parent; text: I18n.tr("Open Rashin settings"); color: Tokens.inkOnBone; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall * root.s }
                        HoverHandler { cursorShape: Qt.PointingHandCursor }
                        TapHandler { id: settingsTap; onTapped: root.openRashinSettings() }
                    }
                }
            }

            AskAnswer {
                id: answer
                width: modeViewport.width
                visible: root.mode === "ask" && root.available
                s: root.s
                session: askSession
                active: root.active && visible
                onHandoffRequested: askSession.handoff(function(info) {
                    root.requestMode("chat");
                    agentSession.attach(info.id);
                })
                onPinRequested: (question, answerText) => root.bubblePinned(question, answerText)
            }

            AskChat {
                id: chat
                width: modeViewport.width
                visible: root.mode === "chat" && root.available
                s: root.s
                session: agentSession
                active: root.active && visible
                maximumHeight: root.bodyBudget
                onInputFocusRequested: root.focusField()
                onModelMenuOpenChanged: if (modelMenuOpen) { root.historyOpen = false; root.modelOpen = false; }
            }

            AskTools {
                id: tools
                width: modeViewport.width
                visible: root.mode === "tools"
                s: root.s
                active: root.active && visible
                query: root.query
                tool: root.tool
                maximumHeight: root.bodyBudget
                onAccepted: root.query = ""
            }

            AskWeb {
                id: web
                width: modeViewport.width
                visible: root.mode === "web"
                s: root.s
                active: root.active && visible
                query: root.query
            }
        }

        AskModeStrip { width: parent.width; s: root.s; mode: root.mode; onModeRequested: next => root.requestMode(next) }
    }

    MouseArea {
        anchors.fill: parent
        z: 10
        visible: root.historyOpen || root.modelOpen
        enabled: visible
        onPressed: root.closeDrawers()
    }

    AskDrawer {
        id: historyDrawer
        anchors.left: content.left
        anchors.right: content.right
        y: content.y + header.y + header.height + Tokens.s1 * root.s
        s: root.s
        open: root.historyOpen
        rows: root.historyRows
        emptyText: I18n.tr("Nothing here yet")
        maxHeight: Math.max(120 * root.s, root.drawerReserve)
        onActivated: row => root.pickHistory(row)
    }

    AskDrawer {
        id: modelDrawer
        anchors.left: content.left
        anchors.right: content.right
        y: content.y + header.y + header.height + Tokens.s1 * root.s
        s: root.s
        open: root.modelOpen
        rows: root.modelRows
        emptyText: I18n.tr("No models yet")
        maxHeight: Math.max(120 * root.s, root.drawerReserve)
        onActivated: row => root.pickModel(row)
    }
}

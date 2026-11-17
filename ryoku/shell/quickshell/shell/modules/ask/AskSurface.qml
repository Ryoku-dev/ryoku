pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import shell.services
import "../../services" as Services

Rectangle {
    id: root

    required property real s
    required property var screen
    required property Services.AskSession session
    property bool active: false
    property string mode: "ask"
    property string tool: ""
    property string query: ""
    property real maximumHeight: 640 * s
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real selectedBodyHeight: mode === "chat" ? chat.implicitHeight
        : mode === "tools" ? tools.implicitHeight
        : mode === "web" ? web.implicitHeight
        : answer.implicitHeight
    readonly property real bodyBudget: Math.max(90 * s,
        maximumHeight - 72 * s - 38 * s - (mode === "ask" ? 30 * s : 0) - Tokens.s7 * s)
    // A drawer hangs below the header and the surface is clipped, so the bar
    // grows by exactly the room the open drawer spills past its bottom edge,
    // never past the bar's own maximum height.
    readonly property real drawerReserve: {
        if (!historyOpen && !modelOpen)
            return 0;
        const drawer = historyOpen ? historyDrawer : modelDrawer;
        const base = content.implicitHeight + Tokens.s4 * s * 2;
        const spill = drawer.y + drawer.naturalHeight + Tokens.s4 * s - base;
        return Math.max(0, Math.min(spill, maximumHeight - base));
    }
    readonly property string headerLabel: mode === "chat" ? I18n.tr("RASHIN // AGENT")
        : mode === "tools" ? I18n.tr("RASHIN // TOOLS")
        : mode === "web" ? I18n.tr("RASHIN // WEB")
        : I18n.tr("RASHIN // ASK")
    readonly property string routeLabel: {
        if (session.phase === "done" && session.answerVia.length > 0)
            return session.answerVia;
        const lane = Needle.quickLane;
        const active = lane && lane.route ? String(lane.route) : "agent";
        const routes = lane && lane.routes ? lane.routes : [];
        for (let i = 0; i < routes.length; i++) {
            const route = routes[i];
            if (String(route.id || "") !== active)
                continue;
            const label = String(route.label || route.id || "");
            const sub = String(route.sub || "");
            if (active === "agent")
                return sub.length > 0 ? label + " · " + sub : label;
            return I18n.tr("Prowl · %1").arg(label);
        }
        return I18n.tr("Ask route");
    }

    signal requestClose()
    signal modeRequested(string mode)

    // The header drawers: recent chats and asks, and the model/lane picker.
    property bool historyOpen: false
    property bool modelOpen: false

    implicitHeight: Math.min(maximumHeight, content.implicitHeight + drawerReserve + Tokens.s4 * s * 2)
    radius: Tokens.radius * s
    color: Tokens.paper
    border.width: Tokens.border
    border.color: Tokens.lineStrong
    clip: true

    Behavior on implicitHeight {
        enabled: root.active && root.motionAllowed
        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
    }

    function focusField() {
        field.focusField();
    }

    function requestMode(next) {
        if (["ask", "chat", "tools", "web"].indexOf(next) < 0 || next === mode)
            return;
        closeDrawers();
        modeRequested(next);
        Qt.callLater(field.focusField);
    }

    function closeDrawers() {
        historyOpen = false;
        modelOpen = false;
        chat.modelMenuOpen = false;
    }

    function toggleHistory() {
        chat.modelMenuOpen = false;
        modelOpen = false;
        historyOpen = !historyOpen;
        if (historyOpen) {
            Needle.noteOpened();
            if (mode === "chat")
                Needle.loadSessions();
            else if (mode === "ask")
                Needle.loadRecentAsks();
        }
    }

    function toggleModel() {
        chat.modelMenuOpen = false;
        historyOpen = false;
        modelOpen = !modelOpen;
        if (modelOpen) {
            Needle.noteOpened();
            Needle.loadQuickLane();
        }
    }

    function handleEscape() {
        if (historyOpen || modelOpen || chat.modelMenuOpen)
            closeDrawers();
        else
            root.requestClose();
    }

    function pickHistory(row) {
        historyOpen = false;
        if (row.kind === "session")
            Needle.switchSession(row.value);
        else if (row.kind === "ask")
            session.loadRecent(row.value);
    }

    function pickModel(row) {
        modelOpen = false;
        if (row.kind === "lane")
            Needle.setQuickLane(row.value);
        else if (row.kind === "model")
            Needle.setModel(row.value);
    }

    // "3m ago" from an RFC3339 stamp; anything unreadable renders as "".
    function relTime(stamp) {
        const raw = String(stamp || "");
        if (raw.length === 0)
            return "";
        const t = Date.parse(raw);
        if (Number.isNaN(t))
            return "";
        const mins = Math.max(0, Math.round((Date.now() - t) / 60000));
        if (mins < 1)
            return I18n.tr("just now");
        if (mins < 60)
            return I18n.tr("%1m ago").arg(mins);
        const hours = Math.round(mins / 60);
        if (hours < 24)
            return I18n.tr("%1h ago").arg(hours);
        return I18n.tr("%1d ago").arg(Math.round(hours / 24));
    }

    // Each mode owns its own history and model namespace.
    readonly property var historyRows: {
        const rows = [];
        if (mode === "ask") {
            const asks = Needle.recentAsks || [];
            if (asks.length > 0)
                rows.push({ section: I18n.tr("Recent asks") });
            for (let i = 0; i < asks.length; i++) {
                const a = asks[i];
                rows.push({
                    kind: "ask", value: a,
                    label: String(a.q || ""),
                    sub: String(a.a || "").replace(/\n/g, " ")
                });
            }
            return rows;
        }
        if (mode === "chat") {
            const sessions = Needle.sessions || [];
            if (sessions.length > 0)
                rows.push({ section: I18n.tr("Agent sessions") });
            if (Needle.sessionsError.length > 0) {
                rows.push({
                    kind: "note", disabled: true,
                    label: I18n.tr("Rename failed"),
                    sub: Needle.sessionsError
                });
            }
            for (let i = 0; i < sessions.length; i++) {
                const sessionRow = sessions[i];
                rows.push({
                    kind: "session", value: String(sessionRow.id || ""),
                    label: String(sessionRow.title || sessionRow.id || ""),
                    sub: root.relTime(sessionRow.updatedAt),
                    active: String(sessionRow.id || "") === Needle.currentSession,
                    renamable: true
                });
            }
        }
        return rows;
    }

    readonly property var modelRows: {
        const rows = [];
        if (mode === "ask") {
            const lane = Needle.quickLane;
            rows.push({ section: I18n.tr("Ask routes") });
            const activeRoute = lane && lane.route ? String(lane.route) : "agent";
            const routes = lane && lane.routes ? lane.routes : [];
            for (let i = 0; i < routes.length; i++) {
                const route = routes[i];
                rows.push({
                    kind: "lane", value: String(route.id || ""),
                    label: String(route.label || route.id || ""),
                    sub: String(route.sub || ""),
                    active: String(route.id || "") === activeRoute
                });
            }
            return rows;
        }
        if (mode !== "chat")
            return rows;
        rows.push({ section: I18n.tr("Agent models") });
        const models = Needle.models || [];
        if (models.length === 0) {
            rows.push({
                kind: "note", disabled: true,
                label: I18n.tr("Models load with the agent"),
                sub: I18n.tr("Send an Agent message to fetch them")
            });
        }
        const chosen = Needle.pick || Needle.currentModel;
        for (let i = 0; i < models.length; i++) {
            const model = models[i];
            rows.push({
                kind: "model", value: String(model.id || ""),
                label: String(model.name || model.id || ""),
                sub: String(model.description || ""),
                active: String(model.id || "") === chosen
            });
        }
        if (Needle.pickUnavailable) {
            rows.push({
                kind: "note", disabled: true,
                label: I18n.tr("Saved model unavailable"),
                sub: I18n.tr("The agent is using its current model")
            });
        }
        return rows;
    }

    function cycleMode() {
        const modes = ["ask", "chat", "tools", "web"];
        const index = Math.max(0, modes.indexOf(mode));
        requestMode(modes[(index + 1) % modes.length]);
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
        if (mode === "chat") {
            if (text.length > 0 && !Needle.busy) {
                Needle.send(text);
                query = "";
            }
        } else if (mode === "tools") {
            tools.activate(text);
        } else if (mode === "web") {
            web.activate();
        } else if (session.phase === "done" && text.length > 0) {
            answer.selectedChip = 0;
            session.ask(text);
        } else if (answer.chips.length > 0 && session.busy) {
            answer.activate();
        } else {
            answer.selectedChip = 0;
            session.ask(text);
        }
    }

    function newChat() {
        if (mode !== "chat")
            return;
        Needle.newChat();
        query = "";
        Qt.callLater(field.focusField);
    }

    function move(delta) {
        if (historyOpen) historyDrawer.move(delta);
        else if (modelOpen) modelDrawer.move(delta);
        else if (mode === "tools") tools.move(delta);
        else if (mode === "web") web.move(delta);
        else if (mode === "ask") {
            if (query.trim().length === 0 && delta < 0 && !session.recentOpen)
                session.requestRecent();
            else
                answer.move(delta);
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

            // The seal is the installed app icon (rashin-app in the hicolor
            // theme), so the bar, the window and the console share one mark.
            Image {
                id: seal
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                width: 16 * root.s
                height: width
                source: "image://icon/rashin-app"
                sourceSize: Qt.size(width, height)
                smooth: true
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

            // The two header buttons ride the right edge: the conversation
            // history drawer and the model drawer (fast-lane route + chat
            // model). They sit where the eye lands after reading the field.
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter

                spacing: Tokens.s1 * root.s

                Rectangle {
                    id: historyBtn
                    visible: root.mode === "ask" || root.mode === "chat"
                    width: 30 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    scale: historyTap.pressed && root.motionAllowed ? 0.9
                        : historyHover.hovered && root.motionAllowed ? 1.04 : 1
                    color: root.historyOpen ? Tokens.bone
                        : historyTap.pressed ? Tokens.tint16
                        : historyHover.hovered ? Tokens.tint10 : "transparent"
                    border.width: Tokens.border
                    border.color: root.historyOpen ? Tokens.bone : Tokens.line
                    Text {
                        anchors.centerIn: parent
                        text: "history"
                        color: root.historyOpen ? Tokens.inkOnBone : Tokens.inkDim
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: Tokens.fBody * root.s
                    }
                    HoverHandler { id: historyHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { id: historyTap; onTapped: root.toggleHistory() }
                    Behavior on color {
                        enabled: root.motionAllowed
                        ColorAnimation { duration: Tokens.snap }
                    }
                    Behavior on scale {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
                    }
                }

                Rectangle {
                    id: modelBtn
                    visible: root.mode === "ask" || root.mode === "chat"
                    width: 30 * root.s
                    height: width
                    radius: Tokens.radius * root.s
                    scale: modelTap.pressed && root.motionAllowed ? 0.9
                        : modelHover.hovered && root.motionAllowed ? 1.04 : 1
                    color: root.modelOpen ? Tokens.bone
                        : modelTap.pressed ? Tokens.tint16
                        : modelHover.hovered ? Tokens.tint10 : "transparent"
                    Row {
                        anchors.centerIn: parent
                        spacing: 1
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "tune"
                            color: root.modelOpen ? Tokens.inkOnBone : Tokens.inkDim
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: Tokens.fBody * root.s
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "expand_more"
                            rotation: root.modelOpen ? 180 : 0
                            color: root.modelOpen ? Tokens.inkOnBone : Tokens.inkMuted
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: Tokens.fSmall * root.s
                            Behavior on rotation {
                                enabled: root.motionAllowed
                                NumberAnimation { duration: Tokens.move; easing.type: Tokens.ease }
                            }
                        }
                    }
                    border.width: Tokens.border
                    border.color: root.modelOpen ? Tokens.bone : Tokens.line
                    HoverHandler { id: modelHover; cursorShape: Qt.PointingHandCursor }
                    TapHandler { id: modelTap; onTapped: root.toggleModel() }
                    Behavior on color {
                        enabled: root.motionAllowed
                        ColorAnimation { duration: Tokens.snap }
                    }
                    Behavior on scale {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
                    }
                }
            }
        }

        AskField {
            id: field
            width: parent.width
            s: root.s
            active: root.active
            mode: root.mode
            phase: root.session.phase
            working: root.session.working
            chatBusy: Needle.busy
            chatActivity: Needle.activity
            pendingHandoff: Needle.pendingHandoff
            horizontalNavigation: root.mode === "ask" && answer.chips.length > 0
            text: root.query
            onTextChanged: if (root.query !== text) root.query = text
            onSubmitted: value => root.submit(value)
            onPrefixRequested: (next, value) => {
                root.query = value;
                root.requestMode(next);
            }
            onTabRequested: root.cycleMode()
            onUpRequested: root.move(-1)
            onDownRequested: root.move(1)
            onLeftRequested: answer.move(-1)
            onRightRequested: answer.move(1)
            onNewChatRequested: root.newChat()
            onEscapeRequested: root.handleEscape()
            onHistoryRequested: root.toggleHistory()
            onModelRequested: root.toggleModel()
        }

        Rectangle {
            id: routePill
            width: parent.width
            height: 28 * root.s
            visible: root.mode === "ask"
            radius: Tokens.radius * root.s
            color: routeTap.pressed ? Tokens.tint16
                : routeHover.hovered ? Tokens.tint10 : Tokens.tint5
            border.width: Tokens.border
            border.color: root.modelOpen ? Tokens.bone : Tokens.lineSoft
            scale: routeTap.pressed && root.motionAllowed ? 0.94
                : routeHover.hovered && root.motionAllowed ? 1.025 : 1
            Row {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: Tokens.s3 * root.s
                anchors.rightMargin: Tokens.s3 * root.s
                spacing: Tokens.s2 * root.s
                Text {
                    text: "route"
                    color: Tokens.inkMuted
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: Tokens.fSmall * root.s
                }
                Text {
                    width: parent.width - Tokens.s6 * root.s
                    text: root.routeLabel
                    color: Tokens.inkDim
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny * root.s
                    elide: Text.ElideRight
                }
            }
            HoverHandler { id: routeHover; cursorShape: Qt.PointingHandCursor }
            TapHandler { id: routeTap; onTapped: root.toggleModel() }
            Behavior on color {
                enabled: root.motionAllowed
                ColorAnimation { duration: Tokens.snap }
            }
            Behavior on scale {
                enabled: root.motionAllowed
                NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap }
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
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded; visible: modeViewport.contentHeight > modeViewport.height + 1; motionEnabled: root.motionAllowed }

            AskAnswer {
                id: answer
                property bool shown: root.mode === "ask"
                width: modeViewport.width
                s: root.s
                session: root.session
                active: root.active && shown
                visible: opacity > 0.001
                opacity: shown ? 1 : 0
                z: shown ? 1 : 0
                transform: Translate {
                    y: answer.shown ? 0 : Tokens.s2 * root.s
                    Behavior on y {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                    }
                }
                Behavior on opacity {
                    enabled: root.motionAllowed
                    NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                }
                onSwitchToChat: root.requestMode("chat")
                onOpenApp: {
                    // The chip's promise: leave the quick bar and land in the
                    // full app. The same summon the keybind drives: raise the
                    // open window, or flock-guard a cold start.
                    Spawn.runApp(["sh", "-c",
                        "ryoku-summon Rashin flock -n -o /tmp/rashin-app.lock rashin-app"], { label: "rashin-app" });
                    root.requestClose();
                }
                onRecentLoaded: question => root.query = question
                onPinBubbleRequested: {
                    Config.patchAskBubble("screen", String(root.screen ? root.screen.name : ""));
                    Config.patchAskBubble("enabled", true);
                }
            }

            AskChat {
                id: chat
                property bool shown: root.mode === "chat"
                width: modeViewport.width
                s: root.s
                showComposer: false
                active: root.active && shown
                visible: opacity > 0.001
                opacity: shown ? 1 : 0
                z: shown ? 1 : 0
                transform: Translate {
                    y: chat.shown ? 0 : Tokens.s2 * root.s
                    Behavior on y {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                    }
                }
                Behavior on opacity {
                    enabled: root.motionAllowed
                    NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                }
                onModelMenuOpenChanged: {
                    if (modelMenuOpen) {
                        root.historyOpen = false;
                        root.modelOpen = false;
                    }
                }
                onInputFocusRequested: root.focusField()
                maximumHeight: root.screenHeight * 0.46
            }

            AskTools {
                id: tools
                property bool shown: root.mode === "tools"
                width: modeViewport.width
                s: root.s
                active: root.active && shown
                visible: opacity > 0.001
                opacity: shown ? 1 : 0
                z: shown ? 1 : 0
                transform: Translate {
                    y: tools.shown ? 0 : Tokens.s2 * root.s
                    Behavior on y {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                    }
                }
                Behavior on opacity {
                    enabled: root.motionAllowed
                    NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                }
                query: root.query
                tool: root.tool
                maximumHeight: root.bodyBudget
                onAccepted: root.query = ""
            }

            AskWeb {
                id: web
                property bool shown: root.mode === "web"
                width: modeViewport.width
                s: root.s
                active: root.active && shown
                visible: opacity > 0.001
                opacity: shown ? 1 : 0
                z: shown ? 1 : 0
                transform: Translate {
                    y: web.shown ? 0 : Tokens.s2 * root.s
                    Behavior on y {
                        enabled: root.motionAllowed
                        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                    }
                }
                Behavior on opacity {
                    enabled: root.motionAllowed
                    NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
                }
                query: root.query
            }
        }

        AskModeStrip {
            width: parent.width
            s: root.s
            mode: root.mode
            onModeRequested: next => root.requestMode(next)
        }
    }

    // Drawer overlay. The blocker sits under the drawers and over everything
    // else: a press anywhere (field, body, the other button) dismisses the
    // open drawer first, and Escape closes the drawer before the bar.
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
        emptyText: I18n.tr("Nothing yet. Ask something, or start a chat.")
        maxHeight: Math.max(120 * root.s, root.drawerReserve)
        onActivated: row => root.pickHistory(row)
        onRenamed: (row, title) => Needle.renameSession(row.value, title)
        onEditFinished: root.focusField()
    }

    AskDrawer {
        id: modelDrawer
        anchors.left: content.left
        anchors.right: content.right
        y: content.y + header.y + header.height + Tokens.s1 * root.s
        s: root.s
        open: root.modelOpen
        rows: root.modelRows
        emptyText: I18n.tr("No models yet.")
        maxHeight: Math.max(120 * root.s, root.drawerReserve)
        onActivated: row => root.pickModel(row)
    }
}

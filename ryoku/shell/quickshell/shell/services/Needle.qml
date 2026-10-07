pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/chatstate.js" as ChatState

// Needle: the Super+S sidebar's live window onto the one shared agent session.
// A single resident `ryoku-rashin chat --follow` process proxies /ws/chat both
// ways; every server frame feeds the pure v3 reducer (lib/chatstate.js) and the
// projected state drives a ListModel the view binds to. Because the session is
// shared, a turn started from the CLI or a Fix with AI terminal shows here too,
// and a shell reload just reconnects and replays. No per-turn process, no reset.
Singleton {
    id: root

    readonly property alias convo: messages

    // Projected reducer state the view reads.
    property bool busy: false
    property string activity: ""          // working-strip label ("" = idle)
    property string approvalsMode: "ask"  // "read-only" auto-approves reads
    property string bannerState: "starting"
    property var models: []
    property string currentModel: ""
    property string currentAgent: ""
    property var commands: []
    property var sessions: []
    property var usage: null
    // The live session's id, so the history drawer can mark where you are.
    property string currentSession: ""
    // Approvals with no tool row yet (a permission that arrived before its
    // tool_call), rendered as trailing cards.
    property var standalonePerms: []

    // Recent quick asks (the launcher's \resume history), newest first. The
    // Ask bar's history drawer reads this; it reloads when an ask completes.
    property var recentAsks: []
    // Resolved fast-lane state (`backend --json`): the active Prowl route,
    // available routes, gateway state, and readiness.
    property var quickLane: null

    // All chat-capable agents (agent --json) for the "what's answering" picker.
    property var backends: []
    // Whether an agent is configured at all; starts true so a working box never
    // flashes the first-run setup prompt while status loads.
    property bool ready: true

    signal touched()

    // The reducer state; replaced (never mutated) on every frame.
    property var _state: ChatState.initialState()
    // Outbound messages queued until the follow process has started its stdin.
    property var _outbox: []
    property bool _started: false

    // ---- outbound (all client messages are one stdin line) -------------------

    function _send(obj) {
        var line = JSON.stringify(obj) + "\n";
        if (root._started)
            follow.write(line);
        else
            root._outbox.push(line);
    }

    function send(text, imagePaths) {
        var q = String(text).trim();
        var imgs = imagePaths || [];
        if ((q.length === 0 && imgs.length === 0) || root.busy)
            return;
        if (imgs.length === 0) {
            root._send({ type: "user", text: q });
            return;
        }
        // QML cannot base64 a file, so the daemon encodes the attachments and we
        // fold the result into the user message it hands back.
        encodeProc.pendingText = q;
        var cmd = ["ryoku-rashin", "chat"];
        for (var i = 0; i < imgs.length; i++) {
            cmd.push("--encode-image");
            cmd.push(String(imgs[i]));
        }
        encodeProc.command = cmd;
        encodeProc.running = true;
    }

    function cancel() { root._send({ type: "cancel" }); }
    function newChat() { root._send({ type: "new" }); }
    function switchSession(id) { if (id) root._send({ type: "load", sessionId: String(id) }); }
    function loadSessions() { root._send({ type: "history" }); }

    function setModel(id) {
        if (!id || id === root.currentModel)
            return;
        root.currentModel = String(id);
        root._send({ type: "set_model", modelId: String(id) });
    }

    function setApprovals(mode) {
        if (mode !== "read-only" && mode !== "ask" && mode !== "auto")
            return;
        root.approvalsMode = mode;
        root._send({ type: "approvals", mode: mode });
    }

    // Answer the approval the agent is waiting on; an empty option declines.
    function answerPermission(requestId, optionId) {
        if (!requestId)
            return;
        root._send({ type: "permission", requestId: String(requestId), optionId: String(optionId || "") });
    }

    // Switch the chat backend live: the daemon drops its session, then the
    // follow stream delivers fresh state and models frames.
    function setBackend(id) {
        if (!id)
            return;
        for (var i = 0; i < root.backends.length; i++)
            if (root.backends[i].id === id)
                root.currentAgent = String(root.backends[i].name || id);
        root.currentModel = "";
        root.models = [];
        Quickshell.execDetached(["ryoku-rashin", "agent", "use", String(id)]);
        backendsReload.restart();
    }

    function copyText(t) {
        Quickshell.execDetached(["sh", "-c", "printf '%s' \"$1\" | wl-copy", "_", String(t)]);
    }

    function noteOpened() {
        root.loadReady();
        root.loadBackends();
        root.loadRecentAsks();
        root.loadQuickLane();
        if (!follow.running && !followRestart.running)
            follow.running = true;
    }

    function noteClosed() {}

    function loadRecentAsks() { recentProc.running = true; }

    // Point the fast ask lane at a Prowl route, then reload the resolved state.
    function setQuickLane(route) {
        if (!route)
            return;
        Quickshell.execDetached(["ryoku-rashin", "backend", String(route)]);
        backendReload.restart();
    }

    function loadQuickLane() { backendProc.running = true; }

    // ---- inbound (frame -> reducer -> projection) ----------------------------

    function _frame(line) {
        var f;
        try {
            f = JSON.parse(String(line));
        } catch (e) {
            return;
        }
        if (!f || !f.type)
            return;
        root._state = ChatState.applyEvent(root._state, f);
        root._project();
    }

    function _project() {
        var st = root._state;
        root.busy = st.busy === true;
        // An open approval is what the turn waits on, whatever ran last.
        root.activity = (st.permissions || []).length > 0 && st.busy ? I18n.tr("waiting for your approval") : (st.activity || "");
        root.approvalsMode = st.approvals || "read-only";
        root.models = st.models || [];
        root.currentModel = st.currentModel || "";
        if (st.agent && st.agent.length > 0)
            root.currentAgent = st.agent;
        root.commands = st.commands || [];
        root.sessions = st.history || [];
        root.usage = st.usage;
        root.currentSession = st.session && st.session.id ? String(st.session.id) : "";
        root.bannerState = st.banner ? String(st.banner.state || "") : "";
        var items = st.items || [];
        var perms = st.permissions || [];
        root._sync(items, perms);
        root.standalonePerms = root._standalone(items, perms);
        root.touched();
    }

    function _permForTool(perms, id) {
        for (var i = 0; i < perms.length; i++)
            if (perms[i].toolId && perms[i].toolId === id)
                return perms[i];
        return null;
    }

    // A ListModel row for one reducer item; every row carries the full role set
    // so the model's schema stays consistent across msg and tool rows.
    function _rowFull(it, perms, last) {
        var row = {
            kind: it.kind, key: String(it.id),
            role: "", body: "", thought: "", open: false, live: false, cont: false, failed: false, imagesJson: "[]",
            toolId: "", title: "", tkind: "", status: "", input: "", output: "",
            diffsJson: "[]", auto: false, permJson: ""
        };
        if (it.kind === "tool") {
            row.toolId = String(it.id);
            row.title = it.title || "";
            row.tkind = it.kind2 || "";
            row.status = it.status || "pending";
            row.input = it.input || "";
            row.output = it.output || "";
            row.diffsJson = JSON.stringify(it.diffs || []);
            row.auto = it.auto === true;
            var perm = root._permForTool(perms, it.id);
            row.permJson = perm ? JSON.stringify(perm) : "";
        } else {
            row.role = it.role || "agent";
            row.body = it.text || "";
            row.thought = it.thought || "";
            row.open = it.open === true;
            // Only the newest open segment is being written right now.
            row.live = row.open && last;
            row.cont = it.cont === true;
            row.failed = it.failed === true;
            row.imagesJson = JSON.stringify(it.images || []);
        }
        return row;
    }

    // Reconcile the ListModel to the reducer's items: v3 events only ever append
    // an item, mutate the open agent message, mutate a tool by id, or clear on
    // replay, so this is a cheap tail-diff that keeps delegates (and their scroll
    // and selection) stable while a chunk streams in.
    function _sync(items, perms) {
        var n = items.length;
        if (n < messages.count)
            messages.clear();
        for (var i = 0; i < n; i++) {
            var row = root._rowFull(items[i], perms, i === n - 1);
            if (i < messages.count) {
                var cur = messages.get(i);
                if (cur.key !== row.key) {
                    messages.set(i, row);
                } else {
                    for (var k in row)
                        if (cur[k] !== row[k])
                            messages.setProperty(i, k, row[k]);
                }
            } else {
                messages.append(row);
            }
        }
    }

    function _standalone(items, perms) {
        var toolIds = ({});
        for (var i = 0; i < items.length; i++)
            if (items[i].kind === "tool")
                toolIds[items[i].id] = true;
        var out = [];
        for (var j = 0; j < perms.length; j++)
            if (!perms[j].toolId || !toolIds[perms[j].toolId])
                out.push(perms[j]);
        return out;
    }

    ListModel { id: messages }

    // The one resident bridge. It stays connected across turns and reconnects on
    // its own; it exits only if it crashes, and then we bring it straight back.
    Process {
        id: follow
        command: ["ryoku-rashin", "chat", "--follow"]
        running: true
        stdinEnabled: true
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => root._frame(line)
        }
        onStarted: {
            root._started = true;
            for (var i = 0; i < root._outbox.length; i++)
                follow.write(root._outbox[i]);
            root._outbox = [];
        }
        onExited: () => {
            root._started = false;
            followRestart.restart();
        }
    }
    Timer {
        id: followRestart
        interval: 400
        onTriggered: if (!follow.running) follow.running = true
    }

    // Attachment encoding: the daemon returns {type:"images", images:[...]} and
    // we send the user message that carries them.
    Process {
        id: encodeProc
        property string pendingText: ""
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => {
                var f;
                try { f = JSON.parse(String(line)); } catch (e) { return; }
                if (f && f.type === "images")
                    root._send({ type: "user", text: encodeProc.pendingText, images: f.images || [] });
            }
        }
    }

    // First-run readiness: whether any agent is configured. Independent of the
    // live stream so a box with no agent still shows the setup prompt.
    Process {
        id: readyProc
        command: ["ryoku-rashin", "status", "--json"]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => {
                var f;
                try { f = JSON.parse(String(line)); } catch (e) { return; }
                if (f && typeof f.ready === "boolean")
                    root.ready = f.ready;
            }
        }
    }
    function loadReady() { readyProc.running = true; }

    Process {
        id: recentProc
        command: ["ryoku-rashin", "ask", "--recent"]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => {
                var arr;
                try { arr = JSON.parse(String(line)); } catch (e) { return; }
                if (Array.isArray(arr))
                    root.recentAsks = arr;
            }
        }
    }

    Process {
        id: backendProc
        command: ["ryoku-rashin", "backend", "--json"]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => {
                var f;
                try { f = JSON.parse(String(line)); } catch (e) { return; }
                if (f && f.routes)
                    root.quickLane = f;
                root.loadRecentAsks();
            }
        }
    }
    Timer {
        id: backendReload
        interval: 400
        onTriggered: root.loadQuickLane()
    }

    Process {
        id: backendsProc
        command: ["ryoku-rashin", "agent", "--json"]
        stdout: SplitParser {
            splitMarker: "\n"
            onRead: (line) => {
                var arr;
                try { arr = JSON.parse(String(line)); } catch (e) { return; }
                if (Array.isArray(arr))
                    root.backends = arr;
            }
        }
    }
    function loadBackends() { backendsProc.running = true; }

    // After a live agent switch the config lands a moment later; refresh the
    // active marker in the picker once it settles.
    Timer {
        id: backendsReload
        interval: 300
        onTriggered: root.loadBackends()
    }

    Component.onCompleted: {
        root.loadReady();
        root.loadBackends();
        root.loadRecentAsks();
        root.loadQuickLane();
    }
}

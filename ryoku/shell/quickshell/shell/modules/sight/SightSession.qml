pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../ask"

Scope {
    id: root
    required property RashinClient client
    required property var card
    property string sessionId: ""
    property int seq: 0
    property bool busy: false
    property bool awaitingId: false
    property var pendingEvents: []
    property var lease: ({ closed: false })
    property var queued: null
    property string turnMode: ""
    property var answers: ({})
    property var turns: ({})
    property var lastAction: null
    property bool pendingStop: false
    property string errorCode: ""
    property string errorText: ""
    property string model: ""
    property string engine: ""
    property bool localModel: false
    property string keptId: ""
    signal answerArrived()

    ShellSetting { id: engineSetting; path: "sight.engine"; fallback: "ask" }
    ShellSetting { id: modelSetting; path: "sight.model"; fallback: "" }
    ShellSetting { id: localSetting; path: "sight.localOnly"; fallback: false }

    readonly property string languageName: {
        for (var i = 0; i < I18n.langs.length; ++i)
            if (String(I18n.langs[i].code) === I18n.lang) return String(I18n.langs[i].name);
        return "English";
    }

    function fail(error) {
        busy = false;
        awaitingId = false;
        errorCode = String(error && error.code || "error");
        errorText = String(error && error.message || I18n.tr("Rashin could not answer."));
        queued = null;
    }

    function request(mode, question, text, target, source) {
        const action = { mode: mode, question: question || "", text: text || "", target: target || "", source: source || "" };
        lastAction = action;
        if (busy || !client.ready) {
            queued = action;
            return;
        }
        queued = null;
        turnMode = mode;
        const key = mode === "read" ? "text" : mode;
        const thread = (turns[key] || []).slice();
        const questionLine = thread.length === 0 ? "" : action.question;
        if (thread.length > 0 && !thread[thread.length - 1].answer && thread[thread.length - 1].question === questionLine)
            thread[thread.length - 1] = { question: questionLine, answer: "" };
        else
            thread.push({ question: questionLine, answer: "" });
        const nextTurns = Object.assign({}, turns);
        nextTurns[key] = thread;
        turns = nextTurns;
        busy = true;
        errorCode = "";
        errorText = "";
        keptId = "";
        const next = Object.assign({}, answers);
        next[mode] = "";
        answers = next;
        awaitingId = sessionId.length === 0;
        pendingEvents = [];
        client.glance({
            session: sessionId, mode: action.question ? "ask" : mode, question: action.question, text: action.text,
            images: card.kind === "text" ? [] : [{ path: String(card.image || "") }],
            language: root.languageName, target: action.target, source: action.source, engine: String(engineSetting.value || "ask"),
            model: String(modelSetting.value || ""), local: localSetting.value === true
        }, lease, function(result, error, sessionId) {
            if (error) {
                root.fail(error);
                return;
            }
            root.sessionId = sessionId;
            root.awaitingId = false;
            if (root.pendingStop) {
                root.pendingStop = false;
                root.client.call("session.cancel", { id: root.sessionId });
            }
            const waiting = root.pendingEvents;
            root.pendingEvents = [];
            for (var i = 0; i < waiting.length; ++i)
                if (String(waiting[i].session || "") === root.sessionId)
                    root.apply(waiting[i]);
        });
    }

    function flush() {
        if (!queued || busy || !client.ready || lease.closed)
            return;
        const action = queued;
        queued = null;
        request(action.mode, action.question, action.text, action.target, action.source);
    }

    function apply(event) {
        const number = Number(event.seq || 0);
        if (number > 0 && number <= seq)
            return;
        seq = Math.max(seq, number);
        const data = event.data || ({});
        const kind = String(event.kind || "");
        if (kind === "text" || (kind === "answer" && typeof data.text === "string")) {
            const next = Object.assign({}, answers);
            next[turnMode] = kind === "text" ? String(next[turnMode] || "") + String(data.delta || "") : data.text;
            answers = next;
            const key = turnMode === "read" ? "text" : turnMode;
            const thread = (turns[key] || []).slice();
            if (thread.length > 0) {
                thread[thread.length - 1] = { question: thread[thread.length - 1].question, answer: next[turnMode] };
                const nextTurns = Object.assign({}, turns);
                nextTurns[key] = thread;
                turns = nextTurns;
            }
            answerArrived();
        } else if (kind === "model") {
            model = String(data.model || data.current || "");
            localModel = data.local === true;
            engine = String(data.engine || "");
        } else if (kind === "error") {
            fail(data);
        } else if (kind === "turn_end") {
            busy = false;
            Qt.callLater(flush);
        }
    }

    function cancel() {
        queued = null;
        if (sessionId) client.call("session.cancel", { id: sessionId });
        else if (awaitingId) pendingStop = true;
        busy = false;
    }
    function retry() {
        if (lastAction && !busy)
            request(lastAction.mode, lastAction.question, lastAction.text, lastAction.target, lastAction.source);
    }

    function keep(callback) {
        if (!sessionId || busy)
            return;
        if (keptId) {
            if (callback) callback(keptId);
            return;
        }
        const token = lease;
        client.call("session.fork", { id: sessionId, seq: seq }, function(result, error) {
            if (token.closed) return;
            if (error) { root.fail(error); return; }
            root.keptId = String(result.id || result.session && result.session.id || "");
            if (callback && root.keptId) callback(root.keptId);
        });
    }

    function openSession(sessionId) {
        client.call("ui.open", { session: sessionId }, function(result, error) {
            if (error || result.delivered === false)
                Spawn.run(["rashin", "open", "--session", sessionId]);
        });
    }

    function continueInRashin() { keep(function(sessionId) { root.openSession(sessionId); }); }
    function handoff() {
        if (!sessionId || busy) return;
        const token = lease;
        client.call("session.handoff", { id: sessionId, run: true }, function(result, error) {
            if (token.closed) return;
            if (error) { root.fail(error); return; }
            const sessionId = String(result.id || result.session && result.session.id || "");
            if (sessionId) root.openSession(sessionId);
        });
    }
    function dispose() {
        if (lease.closed) return;
        lease.closed = true;
        queued = null;
        if (sessionId) client.discardGlance(sessionId);
    }
    Component.onDestruction: dispose()

    Connections {
        target: root.client
        function onBecameReady() { root.flush(); }
        function onStateChanged() {
            if (!root.client.ready && root.busy)
                root.fail({ code: root.client.state, message: root.client.errorText });
        }
        function onEventReceived(name, data) {
            if (name !== "session.event" || root.lease.closed) return;
            if (root.awaitingId) {
                root.pendingEvents = root.pendingEvents.concat([data]);
                return;
            }
            if (root.sessionId && String(data.session || "") === root.sessionId)
                root.apply(data);
        }
    }
}

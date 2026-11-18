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
    property int requestGeneration: 0
    property int activeGeneration: 0
    property var pendingEvents: []
    property var lease: ({ closed: false, writing: ({}) })
    property int writingSequence: 0
    readonly property string writingPrefix: "w" + Date.now().toString(36) + "-" + Math.random().toString(36).slice(2) + "-"
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
    ShellSetting { id: proofreadEngineSetting; path: "sight.proofread.engine"; fallback: "auto" }
    ShellSetting { id: dialectSetting; path: "sight.proofread.dialect"; fallback: "us" }
    ShellSetting { id: categoriesSetting; path: "sight.proofread.off"; fallback: [] }

    readonly property string languageName: {
        for (var i = 0; i < I18n.langs.length; ++i)
            if (String(I18n.langs[i].code) === I18n.lang) return String(I18n.langs[i].name);
        return "English";
    }

    function fail(error) {
        requestGeneration += 1;
        stopGlance();
        errorCode = String(error && error.code || "error");
        errorText = String(error && error.message || I18n.tr("Rashin could not answer."));
        queued = null;
    }

    function request(mode, question, text, target, source, options) {
        if (lease.closed) return;
        const action = { mode: mode, question: question || "", text: text || "", target: target || "", source: source || "" };
        action.generation = ++requestGeneration;
        if (options !== undefined && options !== null)
            action.options = Object.assign({}, options);
        lastAction = action;
        if (busy || awaitingId)
            stopGlance();
        if (awaitingId || !client.ready) {
            queued = action;
            return;
        }
        runRequest(action);
    }

    function runRequest(action) {
        const mode = action.mode;
        const generation = action.generation;
        activeGeneration = generation;
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
        awaitingId = true;
        pendingStop = false;
        pendingEvents = [];
        const params = {
            session: sessionId, mode: action.question ? "ask" : mode, question: action.question, text: action.text,
            images: card.kind === "text" ? [] : [{ path: String(card.image || "") }],
            language: root.languageName, target: action.target, source: action.source, engine: String(engineSetting.value || "ask"),
            model: String(modelSetting.value || ""), local: localSetting.value === true
        };
        if (action.options !== undefined)
            params.options = action.options;
        client.glance(params, lease, function(result, error, sessionId) {
            root.awaitingId = false;
            if (generation !== root.requestGeneration || root.pendingStop) {
                if (sessionId)
                    root.client.discardGlance(sessionId);
                root.pendingStop = false;
                root.pendingEvents = [];
                root.busy = false;
                Qt.callLater(root.flush);
                return;
            }
            if (error) {
                root.fail(error);
                return;
            }
            root.sessionId = sessionId;
            const waiting = root.pendingEvents;
            root.pendingEvents = [];
            for (var i = 0; i < waiting.length; ++i)
                if (String(waiting[i].session || "") === root.sessionId)
                    root.apply(waiting[i]);
        });
    }

    function flush() {
        if (!queued || busy || awaitingId || !client.ready || lease.closed)
            return;
        const action = queued;
        queued = null;
        runRequest(action);
    }

    function check(text, kinds, callback) {
        return startWriting("writing.check", {
            text: String(text || ""),
            kinds: kinds || [],
            engine: String(proofreadEngineSetting.value || "auto"),
            dialect: String(dialectSetting.value || "us"),
            off: categoriesSetting.value || [],
            language: root.languageName,
            model: String(modelSetting.value || ""),
            local: localSetting.value === true
        }, callback);
    }

    function addWord(word, callback) {
        const token = lease;
        if (token.closed) return;
        if (!client.ready) {
            callback(null, { message: I18n.tr("Rashin is not connected.") });
            return;
        }
        client.call("writing.words.add", { word: String(word) }, function(result, error) {
            if (!token.closed) callback(result, error);
        });
    }

    function transform(params, callback) {
        const request = Object.assign({
            model: String(modelSetting.value || ""),
            local: localSetting.value === true
        }, params || ({}));
        return startWriting("writing.transform", request, callback);
    }

    function startWriting(method, params, callback) {
        const token = lease;
        if (token.closed) return "";
        const request = writingPrefix + (++writingSequence) + "-" + String(card.id || "");
        const payload = Object.assign({}, params, { request: request });
        token.writing[request] = { method: method, payload: payload, callback: callback, sent: false };
        flushWriting();
        return request;
    }

    function flushWriting() {
        const token = lease;
        if (token.closed || !client.ready) return;
        const ids = Object.keys(token.writing);
        for (let i = 0; i < ids.length; ++i) {
            if (!client.ready) break;
            const request = ids[i];
            const entry = token.writing[request];
            if (!entry || entry.sent) continue;
            entry.sent = true;
            client.call(entry.method, entry.payload, function(result, error) {
                if (token.closed || !token.writing[request]) return;
                delete token.writing[request];
                if (entry.callback) entry.callback(result, error);
            });
        }
    }

    function cancelWriting(request) {
        const pending = lease.writing;
        const ids = request ? [String(request)] : Object.keys(pending);
        for (let i = 0; i < ids.length; ++i) {
            if (!pending[ids[i]]) continue;
            if (pending[ids[i]].sent)
                client.call("writing.cancel", { request: ids[i] });
            delete pending[ids[i]];
        }
    }

    function parseJson(text) {
        const value = String(text || "").trim().replace(/^```(?:json)?[ \t]*(?:\r?\n)?/i, "").replace(/\s*```$/, "").trim();
        try {
            const parsed = JSON.parse(value);
            return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : null;
        } catch (error) {
            return null;
        }
    }

    function apply(event) {
        if (!busy || activeGeneration !== requestGeneration) return;
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

    function stopGlance() {
        pendingStop = awaitingId;
        pendingEvents = [];
        if (sessionId)
            client.discardGlance(sessionId);
        sessionId = "";
        seq = 0;
        keptId = "";
        busy = false;
    }

    function cancel() {
        cancelWriting();
        queued = null;
        requestGeneration += 1;
        stopGlance();
    }
    function retry() {
        if (lastAction && !busy)
            request(lastAction.mode, lastAction.question, lastAction.text, lastAction.target, lastAction.source, lastAction.options);
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
        cancelWriting();
        lease.closed = true;
        queued = null;
        if (sessionId) client.discardGlance(sessionId);
    }
    Component.onDestruction: dispose()

    Connections {
        target: root.client
        function onBecameReady() { root.flush(); root.flushWriting(); }
        function onStateChanged() {
            if (!root.client.ready && root.busy)
                root.fail({ code: root.client.state, message: root.client.errorText });
        }
        function onEventReceived(name, data) {
            if (name !== "session.event" || root.lease.closed) return;
            if (!root.busy || root.activeGeneration !== root.requestGeneration) return;
            if (root.awaitingId) {
                root.pendingEvents = root.pendingEvents.concat([data]);
                return;
            }
            if (root.sessionId && String(data.session || "") === root.sessionId)
                root.apply(data);
        }
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "lib/sessionstate.js" as SessionState

Scope {
    id: root

    required property RashinClient client
    property string kind: "ask"
    property var state: SessionState.initialState(kind)
    property var pendingEvents: []
    property var sessions: []
    property var pendingHandoff: null
    property bool pendingCancel: false
    property bool attaching: false
    property int attachSerial: 0
    property bool awaitingId: false
    property bool creating: false
    signal changed()
    signal failed(string message)

    readonly property string id: String(state.id || "")
    readonly property bool busy: state.busy === true || creating

    function copyState(source) {
        var result = {};
        for (var key in source)
            result[key] = source[key];
        return result;
    }

    function fail(message) {
        var failedState = copyState(state);
        failedState.busy = false;
        failedState.phase = "failed";
        failedState.error = String(message || "");
        state = failedState;
        changed();
        failed(failedState.error);
    }

    function reset() {
        if (id.length > 0 && client.ready)
            client.call("session.detach", { id: id });
        state = SessionState.initialState(kind);
        pendingEvents = [];
        pendingHandoff = null;
        pendingCancel = false;
        attaching = false;
        attachSerial++;
        awaitingId = false;
        creating = false;
        changed();
    }

    function apply(event) {
        const seq = Number(event && event.seq || 0);
        if (seq > 0 && seq <= Number(state.seq || 0))
            return;
        state = SessionState.applyEvent(state, event);
        changed();
    }

    function setSessionId(id) {
        awaitingId = false;
        var next = copyState(state);
        next.id = String(id || "");
        state = next;
        const waiting = pendingEvents;
        pendingEvents = [];
        for (var i = 0; i < waiting.length; i++) {
            if (String(waiting[i].session || "") === state.id)
                apply(waiting[i]);
        }
        if (pendingCancel) {
            pendingCancel = false;
            cancel();
        }
        if (pendingHandoff) {
            const callback = pendingHandoff;
            pendingHandoff = null;
            handoff(callback);
        }
    }

    function list(limit, callback) {
        client.call("session.list", { kind: kind, limit: limit }, function(result, error) {
            if (error) {
                root.fail(String(error.message || ""));
                return;
            }
            root.sessions = result.sessions || [];
            if (callback)
                callback(root.sessions);
        });
    }

    function attachPage(nextId, since, serial, info, callback) {
        client.call("session.attach", { id: nextId, since: since, limit: 0 }, function(result, error) {
            if (serial !== root.attachSerial)
                return;
            if (error) {
                root.attaching = false;
                root.fail(String(error.message || ""));
                return;
            }
            const pageInfo = result.info || info || ({});
            const events = result.events || [];
            var nextSince = since;
            for (var i = 0; i < events.length; i++) {
                root.apply(events[i]);
                const seq = Number(events[i].seq || 0);
                if (seq > nextSince)
                    nextSince = seq;
            }
            if (result.more === true) {
                if (nextSince <= since) {
                    root.attaching = false;
                    root.fail(I18n.tr("Rashin could not finish loading the session. Reopen it and try again."));
                    return;
                }
                root.attachPage(nextId, nextSince, serial, pageInfo, callback);
                return;
            }
            const waiting = root.pendingEvents;
            root.pendingEvents = [];
            root.attaching = false;
            for (var j = 0; j < waiting.length; j++) {
                if (String(waiting[j].session || "") === nextId)
                    root.apply(waiting[j]);
            }
            if (callback)
                callback(pageInfo);
        });
    }

    function attach(id, callback) {
        const nextId = String(id || "");
        if (nextId.length === 0)
            return;
        if (root.id.length > 0 && root.id !== nextId)
            client.call("session.detach", { id: root.id });
        const serial = ++attachSerial;
        var fresh = SessionState.initialState(kind);
        fresh.id = nextId;
        state = fresh;
        pendingEvents = [];
        attaching = true;
        attachPage(nextId, 0, serial, ({}), callback);
    }

    function loadLatest() {
        list(1, function(rows) {
            if (!root.creating && root.id.length === 0 && rows.length > 0)
                root.attach(rows[0].id);
        });
    }

    function ask(question, imagePaths) {
        const text = String(question || "").trim();
        if (text.length === 0 || busy)
            return;
        if (id.length > 0)
            client.call("session.detach", { id: id });
        pendingEvents = [];
        var fresh = SessionState.initialState("ask");
        fresh.phase = "working";
        fresh.busy = true;
        fresh.question = text;
        state = fresh;
        awaitingId = true;
        const images = [];
        const paths = imagePaths || [];
        for (var i = 0; i < paths.length; i++)
            images.push({ path: String(paths[i]) });
        client.call("ask", { question: text, origin: "bar", images: images }, function(result, error) {
            if (error) {
                root.awaitingId = false;
                root.fail(String(error.message || ""));
                return;
            }
            root.setSessionId(result.session ? result.session.id : "");
        });
    }

    function prompt(text) {
        const value = String(text || "").trim();
        if (value.length === 0 || busy)
            return;
        if (id.length === 0) {
            creating = true;
            client.call("session.new", { kind: "agent", origin: "bar" }, function(info, error) {
                if (error) {
                    root.creating = false;
                    root.fail(String(error.message || ""));
                    return;
                }
                root.creating = false;
                root.attach(info.id, function() { root.prompt(value); });
            });
            return;
        }
        client.call("session.prompt", { id: id, text: value }, function(result, error) {
            if (error)
                root.fail(String(error.message || ""));
        });
    }

    function cancel() {
        pendingHandoff = null;
        if (id.length === 0) {
            pendingCancel = true;
            return;
        }
        client.call("session.cancel", { id: id });
    }

    function answerPermission(request, option) {
        if (id.length > 0)
            client.call("session.permission", { id: id, request: request, option: option });
    }

    function setModel(model) {
        if (id.length > 0)
            client.call("session.set_model", { id: id, model: model });
    }

    function handoff(callback) {
        pendingCancel = false;
        if (id.length === 0) {
            pendingHandoff = callback;
            return;
        }
        client.call("session.handoff", { id: id, run: true }, function(info, error) {
            if (error) {
                root.fail(String(error.message || ""));
                return;
            }
            if (callback)
                callback(info);
        });
    }

    Connections {
        target: root.client
        function onEventReceived(name, data) {
            if (name !== "session.event")
                return;
            if (root.attaching) {
                if (String(data.session || "") === root.id)
                    root.pendingEvents = root.pendingEvents.concat([data]);
                return;
            }
            if (root.id.length === 0) {
                if (root.awaitingId)
                    root.pendingEvents = root.pendingEvents.concat([data]);
                return;
            }
            if (String(data.session || "") === root.id)
                root.apply(data);
        }
    }
}

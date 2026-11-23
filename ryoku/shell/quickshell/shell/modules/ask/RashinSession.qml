pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "lib/sessionstate.js" as SessionState

Scope {
    id: root

    required property RashinClient client
    property string kind: "ask"
    property var state: SessionState.initialState()
    property var pendingEvents: []
    property var sessions: []
    property bool pendingCancel: false
    property bool attaching: false
    property int attachSerial: 0
    property bool awaitingId: false
    // a deleted ask waits here for its undo; lists leave it out meanwhile
    property string forgetting: ""
    // deletes the daemon has not answered yet; a list read before them must not bring them back
    property var deleting: []
    signal changed()
    signal failed(string message)

    readonly property string id: String(state.id || "")
    readonly property bool busy: state.busy === true

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
        state = SessionState.initialState();
        pendingEvents = [];
        pendingCancel = false;
        attaching = false;
        attachSerial++;
        awaitingId = false;
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
    }

    function list(limit, callback) {
        client.call("session.list", { kind: kind, limit: limit }, function(result, error) {
            if (error) {
                root.fail(String(error.message || ""));
                return;
            }
            root.sessions = (result.sessions || []).filter(info => root.deleting.indexOf(String(info.id)) < 0);
            if (callback)
                callback(root.sessions);
        });
    }

    function forget(id) {
        commitForget();
        forgetting = String(id || "");
        forgetWindow.restart();
    }

    function undoForget() {
        forgetWindow.stop();
        forgetting = "";
    }

    function commitForget() {
        if (forgetting.length === 0)
            return;
        forgetWindow.stop();
        const gone = forgetting;
        // filtered first, so the list never shows the ask again on its way out
        sessions = sessions.filter(info => String(info.id) !== gone);
        forgetting = "";
        if (root.id === gone)
            reset();
        deleting = deleting.concat([gone]);
        client.call("session.delete", { id: gone }, function(result, error) {
            root.deleting = root.deleting.filter(id => id !== gone);
            // the daemon kept it, so its list is the truth again, once it answers at all
            if (error && root.client.ready)
                root.list(20);
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
        var fresh = SessionState.initialState();
        fresh.id = nextId;
        state = fresh;
        pendingEvents = [];
        attaching = true;
        attachPage(nextId, 0, serial, ({}), callback);
    }

    function ask(question, imagePaths) {
        const text = String(question || "").trim();
        if (text.length === 0 || busy)
            return;
        if (id.length > 0)
            client.call("session.detach", { id: id });
        pendingEvents = [];
        var fresh = SessionState.initialState();
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

    function cancel() {
        if (id.length === 0) {
            pendingCancel = true;
            return;
        }
        client.call("session.cancel", { id: id });
    }

    Timer {
        id: forgetWindow
        interval: 5000
        onTriggered: root.commitForget()
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

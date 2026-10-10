pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// QML view of the daemon `settings` topic (ryoku/shell/ipc/settings.go). The
// settings schema, its defaults, validation, clamping and migration for
// shell.json live in the daemon, so the Hub never parses or writes that file:
// `subscribe settings` streams the full settings tree as one JSON frame on
// connect and again on every change, and edits ride back over a second
// connection as `settings.patch` / `settings.reset` calls. Read a value with
// get("a.b.c"); stage a write with patch("a.b.c", value). Mirrors the pill's
// Clipboard/Tray topic singletons (two sockets: a subscription and a control
// line, since a second write to the subscription would half-close the stream).
Singleton {
    id: root

    // The full settings tree the daemon last pushed. Values are read by dotted
    // path through get(); revision ticks on every accepted frame so a binding
    // that walks the tree in a helper re-runs when a new frame lands.
    property var data: ({})
    property bool ready: false
    property int revision: 0
    property var pendingCalls: ({})
    property int pendingCallCount: 0
    property int nextCallId: 0


    // Compositor gating, carried on the `settings` frame with the values it
    // gates. caps is behavioural (supports() -> a gated row is hidden, not
    // disabled); deadKeys are provider store leaves the active compositor does
    // not model, so modelsKey() drops a row nothing would write. provider and
    // its config files still ride the `wm` topic below.
    property var caps: ({})
    property var deadKeys: []
    // The active provider's window-rule action ids, in display order, carried on
    // the same frame beside caps/deadKeys. Empty until the frame lands or when a
    // probe fails, so a consumer falls back to its own static list.
    property var windowRuleActions: []
    property var configFiles: []
    property string provider: ""
    property var windows: []
    function supports(cap) { return !cap || root.caps[cap] === true; }
    // A schema row's key names a provider store leaf. When some installed
    // provider models it but the active one does not, nothing writes it, so the
    // row is dropped; a keyless row, or a leaf no provider models (Hub-owned),
    // is kept.
    function modelsKey(key) { return !key || root.deadKeys.indexOf(key) < 0; }

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    // Nested read by dotted path; undefined for any missing segment.
    function get(path) {
        var cur = root.data;
        var parts = String(path).split(".");
        for (var i = 0; i < parts.length; i++) {
            if (cur === undefined || cur === null || typeof cur !== "object")
                return undefined;
            cur = cur[parts[i]];
        }
        return cur;
    }

    // Intent -> daemon. Existing callers remain fire-and-forget; a caller that
    // supplies `completion(ok, error)` gets the daemon acknowledgement or a
    // bounded timeout and can keep its UI honest while the daemon is absent.
    function patch(path, value, completion) {
        return root.send("settings.patch", { path: path, value: value }, completion);
    }
    function reset(path, completion) {
        return root.send("settings.reset", { path: path }, completion);
    }

    function send(method, args, completion) {
        var payload = {};
        var source = args || {};
        for (var key in source)
            payload[key] = source[key];
        var id = "";
        if (typeof completion === "function") {
            id = "hub-settings-" + (++root.nextCallId);
            payload.id = id;
            var pending = Object.assign({}, root.pendingCalls);
            pending[id] = { callback: completion, deadline: Date.now() + 5000 };
            root.pendingCalls = pending;
            root.pendingCallCount++;
        }
        ctl.queued += "call " + method + " " + JSON.stringify(payload) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
        return id;
    }

    function finishCall(id, ok, error) {
        var entry = root.pendingCalls[id];
        if (!entry)
            return;
        var pending = Object.assign({}, root.pendingCalls);
        delete pending[id];
        root.pendingCalls = pending;
        root.pendingCallCount = Math.max(0, root.pendingCallCount - 1);
        entry.callback(ok, error || "");
    }

    function applyReply(line) {
        try {
            var reply = JSON.parse(line);
            if (reply && reply.id)
                root.finishCall(String(reply.id), reply.ok === true, String(reply.error || ""));
        } catch (e) {
        }
    }

    function expireCalls() {
        var now = Date.now();
        var ids = Object.keys(root.pendingCalls);
        for (var i = 0; i < ids.length; ++i) {
            var id = ids[i];
            var entry = root.pendingCalls[id];
            if (entry && entry.deadline <= now) {
                ctl.dropQueued(id);
                root.finishCall(id, false, "The shell daemon did not answer.");
            }
        }
    }

    function apply(line) {
        try {
            var frame = JSON.parse(line);
            if (frame && typeof frame === "object" && !Array.isArray(frame)) {
                root.caps = frame.caps || ({});
                root.deadKeys = frame.deadKeys || [];
                root.windowRuleActions = frame.windowRuleActions || [];
                root.data = frame;
                root.ready = true;
                root.revision++;
            }
        } catch (e) {
            // A malformed frame must never wedge the Hub; keep the last good tree.
        }
    }

    // Subscription: connect, ask once, then stream. A second write to this
    // connection would half-close the stream (daemon rule), so calls use ctl.
    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe settings\n");
                flush();
            } else {
                root.ready = false;
                retry.restart();
            }
        }
    }

    // The daemon may be down when the Hub loads (or restart under it); retry
    // quietly so the settings repopulate once it returns.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }
    Timer {
        interval: 250
        repeat: true
        running: root.pendingCallCount > 0
        onTriggered: root.expireCalls()
    }


    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""
        parser: SplitParser { onRead: line => root.applyReply(line) }

        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        function dropQueued(id) {
            var needle = "\"id\":\"" + id + "\"";
            var lines = queued.split("\n");
            var kept = "";
            for (var i = 0; i < lines.length; ++i) {
                if (lines[i].length > 0 && lines[i].indexOf(needle) < 0)
                    kept += lines[i] + "\n";
            }
            queued = kept;
        }


        onConnectionStateChanged: if (connected) flushQueued()
    }

    function applyWm(line) {
        try {
            var f = JSON.parse(line);
            if (f && typeof f === "object" && !Array.isArray(f)) {
                root.configFiles = f.configFiles || [];
                root.provider = f.provider || "";
                root.windows = f.windows || [];
            }
        } catch (e) {
        }
    }

    Socket {
        id: wmSub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.applyWm(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe wm\n");
                flush();
            } else {
                wmRetry.restart();
            }
        }
    }
    Timer {
        id: wmRetry
        interval: 2000
        onTriggered: if (!wmSub.connected) wmSub.connected = true
    }
}

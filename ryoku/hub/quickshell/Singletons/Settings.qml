pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "SettingsWrites.js" as Writes

// QML view of the daemon `settings` topic (ryoku/shell/ipc/settings.go). The
// settings schema, its defaults, validation, clamping and migration for
// shell.json live in the daemon, so the Hub never parses or writes that file:
// the tree comes from ShellSettings and edits ride the bus as settings calls.
Singleton {
    id: root

    // The full settings tree the daemon last pushed. Values are read by dotted
    // path through get(); revision ticks on every accepted frame so a binding
    // that walks the tree in a helper re-runs when a new frame lands.
    property var data: ({})
    readonly property bool ready: root.loaded && Bus.connected
    property bool loaded: false
    property int revision: 0
    // bump on reconnect so baseline readers re-run even when the tree matches
    onReadyChanged: if (root.ready) root.revision++
    property var pendingCalls: ({})
    property int pendingCallCount: 0
    property int nextCallId: 0
    property var writes: Writes.create()
    property int overlayRevision: 0

    // Compositor gating, carried on the `settings` frame with the values it
    // gates. caps is behavioural (supports() -> a gated row is hidden, not
    // disabled); deadKeys are provider store leaves the active compositor does
    // not model, so modelsKey() drops a row nothing would write. provider and
    // its config files still ride the `wm` topic below.
    property var caps: ({})
    property var deadKeys: []
    // The active provider's window-rule action ids, in display order, and the
    // per-app look fields (desktop.appOverrides) it applies, carried on the
    // same frame beside caps/deadKeys. Empty until the frame lands or when a
    // probe fails, so a consumer falls back to its own static list.
    property var windowRuleActions: []
    property var appOverrideFields: []
    property var configFiles: []
    property string provider: ""
    property var windows: []
    function supports(cap, hostCap) {
        if (cap && root.caps[cap] !== true)
            return false;
        if (!hostCap)
            return true;
        if (typeof hostCap === "string")
            return Host.supports(hostCap);
        for (var i = 0; i < hostCap.length; i++) {
            if (!Host.supports(hostCap[i]))
                return false;
        }
        return true;
    }
    // A schema row's key names a provider store leaf. When some installed
    // provider models it but the active one does not, nothing writes it, so the
    // row is dropped; a keyless row, or a leaf no provider models (Hub-owned),
    // is kept.
    function modelsKey(key) { return !key || root.deadKeys.indexOf(key) < 0; }

    function get(path) {
        root.overlayRevision;
        root.revision;
        return Writes.read(root.writes, root.data, path);
    }

    function patch(path, value, completion) {
        Writes.stage(root.writes, path, value, completion, Date.now());
        root.overlayRevision++;
        var wait = Writes.dueAt(root.writes) - Date.now();
        writeFlush.interval = Math.max(1, wait);
        writeFlush.restart();
    }
    function reset(path, completion) {
        return root.send("settings.reset", { path: path }, completion);
    }
    function flushWrites() {
        writeFlush.stop();
        var ws = Writes.take(root.writes);
        for (var i = 0; i < ws.length; i++)
            root.sendWrite(ws[i]);
        if (ws.length > 0)
            root.overlayRevision++;
        return ws.length;
    }
    readonly property bool writesUnsettled: {
        root.overlayRevision;
        return Writes.unsent(root.writes) + Writes.unacked(root.writes) > 0;
    }
    function sendWrite(w) {
        root.call("settings.patch", { path: w.path, value: w.value }, function(ok, error) {
            var done = Writes.ack(root.writes, w.seq, ok, root.data, Date.now());
            root.overlayRevision++;
            root.scheduleSettle();
            for (var i = 0; i < done.length; i++)
                done[i](ok, error);
            root.drainAfterWrites();
        });
    }
    function scheduleSettle() {
        var r = Writes.expire(root.writes, Date.now());
        if (r.changed)
            root.overlayRevision++;
        if (r.next < 0) {
            writeSettle.stop();
        } else {
            writeSettle.interval = Math.max(1, r.next - Date.now());
            writeSettle.restart();
        }
    }

    // the bus runs each call namespace on its own, so a dependent call waits for the writes
    property var afterWritesQueue: []
    function afterWrites(fn) {
        if (Writes.unsent(root.writes) + Writes.unacked(root.writes) === 0) {
            fn();
            return;
        }
        root.afterWritesQueue = root.afterWritesQueue.concat([fn]);
    }
    function drainAfterWrites() {
        if (root.afterWritesQueue.length === 0 || Writes.unsent(root.writes) + Writes.unacked(root.writes) > 0)
            return;
        var fns = root.afterWritesQueue;
        root.afterWritesQueue = [];
        for (var i = 0; i < fns.length; i++)
            fns[i]();
    }

    function send(method, args, completion) {
        root.flushWrites();
        root.afterWrites(function() { root.call(method, args, completion); });
    }

    // capped and cancelled on expiry so a write reported as failed can't land later
    function call(method, args, completion) {
        if (typeof completion !== "function") {
            Bus.call(method, args || {});
            return "";
        }
        var id = "hub-settings-" + (++root.nextCallId);
        var entry = { callback: completion, deadline: Date.now() + 5000, busId: "" };
        var pending = Object.assign({}, root.pendingCalls);
        pending[id] = entry;
        root.pendingCalls = pending;
        root.pendingCallCount++;
        entry.busId = Bus.call(method, args || {}, function(ok, result, error) {
            root.finishCall(id, ok === true, String(error || ""));
        });
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

    function expireCalls() {
        var now = Date.now();
        var ids = Object.keys(root.pendingCalls);
        for (var i = 0; i < ids.length; ++i) {
            var id = ids[i];
            var entry = root.pendingCalls[id];
            if (entry && entry.deadline <= now) {
                Bus.cancel(entry.busId);
                root.finishCall(id, false, I18n.tr("The desktop service did not answer. Try again once it is running."));
            }
        }
    }

    // shared frame, never mutate it
    function apply(frame) {
        if (!frame || typeof frame !== "object" || Array.isArray(frame))
            return;
        root.caps = frame.caps || ({});
        root.deadKeys = frame.deadKeys || [];
        root.windowRuleActions = frame.windowRuleActions || [];
        root.appOverrideFields = frame.appOverrideFields || [];
        // retire acked overlays before the new tree lands so reads get the daemon's value
        Writes.frame(root.writes);
        root.data = frame;
        root.loaded = true;
        root.revision++;
    }

    function applyWm(f) {
        if (!f || typeof f !== "object" || Array.isArray(f))
            return;
        root.configFiles = f.configFiles || [];
        root.provider = f.provider || "";
        root.windows = f.windows || [];
    }

    Connections {
        target: ShellSettings
        function onChanged(paths) { root.apply(ShellSettings.data); }
        function onReadyChanged() {
            if (ShellSettings.ready)
                root.apply(ShellSettings.data);
        }
    }

    Timer {
        interval: 250
        repeat: true
        running: root.pendingCallCount > 0
        onTriggered: root.expireCalls()
    }
    Timer {
        id: writeFlush
        onTriggered: root.flushWrites()
    }
    Timer {
        id: writeSettle
        onTriggered: root.scheduleSettle()
    }

    Component.onCompleted: {
        if (ShellSettings.ready)
            root.apply(ShellSettings.data);
        Bus.subscribe("wm", f => root.applyWm(f));
    }
}

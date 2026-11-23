pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/format.js" as Format

// The bar's line to the ryoget daemon: newline-delimited JSON on its socket.
// Nothing connects until open() or a call; a missing socket runs `ryoget ensure`
// once, which starts the on-demand service. The daemon owns the queue and the
// settings; this mirrors them (tasks, stats, settings) from hello and events.
Scope {
    id: root

    // idle | connecting | ready | not-installed | error
    property string state: "idle"
    property string errorText: ""
    property var tasks: []
    property var stats: ({ down: 0, up: 0, active: 0, queued: 0, seeding: 0, history: [] })
    property var settings: ({})
    property int nextId: 1
    property var callbacks: ({})
    property var waiting: []
    property bool ensureTried: false
    property bool closing: false
    property bool greeted: false
    property string uid: ""
    property bool passiveConnect: false

    readonly property bool ready: state === "ready"
    readonly property string socketPath: {
        const runtime = Quickshell.env("XDG_RUNTIME_DIR") || "";
        if (runtime.length > 0)
            return runtime + "/ryoget.sock";
        return uid.length > 0 ? "/tmp/ryoget-" + uid + ".sock" : "";
    }

    // one plain sentence per kind of failure; rows, alerts and cards all say these
    readonly property var reasons: ({
        "network": I18n.tr("Couldn't reach the site."),
        "private": I18n.tr("This is private or needs a sign-in."),
        "unavailable": I18n.tr("This isn't available any more."),
        "unsupported": I18n.tr("There's nothing here to download."),
        "rate_limited": I18n.tr("The site is limiting requests. Try again in a minute."),
        "invalid": I18n.tr("That doesn't look like a link."),
        "no_space": I18n.tr("There isn't enough free space for it."),
        "browser_check": I18n.tr("The site wants a browser to check you first."),
        "checksum": I18n.tr("The file didn't match its checksum, so it was deleted."),
        "other": I18n.tr("Something went wrong. Try again.")
    })

    signal eventReceived(string name, var data)

    // reason: the sentence for a task's or a call's error. detail: the engine's
    // own words, for a second line where there is room (never on a row).
    function reason(error) {
        return Format.reason(error, root.reasons);
    }

    function detail(error) {
        return Format.detail(error);
    }

    function openExisting() {
        if (state === "ready" || state === "connecting" || socketPath.length === 0)
            return;
        passiveConnect = true;
        state = "connecting";
        connectSocket();
    }

    function open() {
        passiveConnect = false;
        if (state === "ready" || state === "connecting" || state === "not-installed")
            return;
        ensureTried = false;
        state = "connecting";
        errorText = "";
        if (socketPath.length === 0) {
            whoami.running = true;
            return;
        }
        connectSocket();
    }

    function connectSocket() {
        greeted = false;
        closing = true;
        socket.connected = false;
        Qt.callLater(function() {
            root.closing = false;
            if (root.state === "connecting")
                socket.connected = true;
        });
    }

    function send(method, params, callback) {
        const id = nextId++;
        if (callback)
            callbacks[id] = callback;
        socket.write(JSON.stringify({ id: id, method: method, params: params || ({}) }) + "\n");
        socket.flush();
    }

    // call answers callback(result, error); it connects first when it has to.
    function call(method, params, callback) {
        if (ready) {
            send(method, params, callback);
            return;
        }
        if (state === "not-installed") {
            if (callback)
                callback(null, { code: "engine", message: errorText });
            return;
        }
        waiting = waiting.concat([{ method: method, params: params, callback: callback }]);
        open();
    }

    function failAll(code, message) {
        const pending = callbacks;
        const held = waiting;
        callbacks = ({});
        waiting = [];
        for (var id in pending) {
            try {
                pending[id](null, { code: code, message: message });
            } catch (error) {
                // one broken consumer must not strand the rest
            }
        }
        for (var i = 0; i < held.length; i++) {
            if (held[i].callback)
                held[i].callback(null, { code: code, message: message });
        }
    }

    function lost() {
        if (closing)
            return;
        const wasReady = greeted;
        greeted = false;
        if (wasReady) {
            // the daemon left (idle exit or a restart): the next call wakes it
            ensureTried = false;
            state = "idle";
            failAll("engine", I18n.tr("The download engine closed the connection."));
            return;
        }
        if (passiveConnect) {
            state = "idle";
            return;
        }
        if (state !== "connecting" || ensure.running)
            return;
        if (ensureTried) {
            state = "error";
            errorText = I18n.tr("The download engine did not start.");
            failAll("engine", errorText);
            return;
        }
        ensureTried = true;
        ensure.running = true;
    }

    function upsert(task) {
        const id = String(task.id || "");
        const next = tasks.slice();
        for (var i = 0; i < next.length; i++) {
            if (String(next[i].id) === id) {
                next[i] = task;
                tasks = next;
                return;
            }
        }
        next.push(task);
        tasks = next;
    }

    function frame(line) {
        var message;
        try {
            message = JSON.parse(String(line));
        } catch (error) {
            return;
        }
        if (message.event) {
            const data = message.data || ({});
            if (message.event === "task")
                upsert(data);
            else if (message.event === "removed")
                tasks = tasks.filter(task => String(task.id) !== String(data.id));
            else if (message.event === "stats")
                stats = data;
            else if (message.event === "settings")
                settings = data;
            eventReceived(String(message.event), data);
            return;
        }
        if (message.id === undefined || message.id === null)
            return;
        const callback = callbacks[message.id];
        if (!callback)
            return;
        delete callbacks[message.id];
        callback(message.result || ({}), message.error || null);
    }

    function task(id) {
        for (var i = 0; i < tasks.length; i++) {
            if (String(tasks[i].id) === String(id))
                return tasks[i];
        }
        return null;
    }

    function resolve(url, callback) { call("resolve", { url: url }, callback); }
    function add(params, callback) { call("add", params, callback); }

    // links: the engine reads pasted text into cleaned links (no network) and
    // answers {links:[{raw, url, kind, host, notes, duplicate}], total, cleaned, duplicates}.
    function links(text, callback) { call("links", { text: String(text || "") }, callback); }

    // addMany: items [{url, mode?, format?, files?}], opts {dir?, paused?};
    // answers {tasks:[Task], errors:[{index, code, message}]}.
    function addMany(items, opts, callback) {
        const params = { items: items || [] };
        if (opts && opts.dir)
            params.dir = String(opts.dir);
        if (opts && opts.paused === true)
            params.paused = true;
        call("addMany", params, callback);
    }

    function pause(id) { call("pause", id === undefined ? { all: true } : { id: id }); }
    function resume(id) { call("resume", id === undefined ? { all: true } : { id: id }); }
    function cancel(id) { call("cancel", { id: id }); }
    function remove(id, deleteFile) { call("remove", { id: id, deleteFile: deleteFile === true }); }
    function retry(id, callback) { call("retry", { id: id }, callback); }
    function move(id, index) { call("move", { id: id, index: index }); }
    function files(id, selected, callback) { call("files", { id: id, selected: selected }, callback); }
    function openFile(id) { call("open", { id: id }); }
    function reveal(id) { call("reveal", { id: id }); }
    function clear(states) { call("clear", { states: states || ["done", "error", "stopped"] }); }
    // ids a home list showed settled; at (a Date) backdates the look so the list drops them at once
    function seen(ids, at) {
        const params = { ids: ids || [] };
        if (at)
            params.at = at.toISOString().replace(/\.\d{3}Z$/, "Z");
        call("seen", params);
    }
    // KiB/s for one download alone, 0 for none
    function limit(id, kib) { call("limit", { id: id, kib: Math.max(0, Math.round(Number(kib) || 0)) }); }
    // gives a failed or paused download a new link; its partial data stays when the file is the same
    function refresh(id, url, callback) { call("refresh", { id: id, url: String(url || "") }, callback); }
    function setSettings(patch, callback) { call("settings.set", { patch: patch }, callback); }

    Socket {
        id: socket
        path: root.socketPath
        parser: SplitParser { onRead: line => root.frame(line) }
        onConnectionStateChanged: {
            if (!connected) {
                root.lost();
                return;
            }
            root.send("hello", { client: "bar", version: 1 }, function(result, error) {
                if (error) {
                    root.state = "error";
                    root.errorText = String(error.message || "");
                    root.failAll("engine", root.errorText);
                    return;
                }
                root.tasks = result.tasks || [];
                root.stats = result.stats || root.stats;
                root.settings = result.settings || ({});
                root.greeted = true;
                root.ensureTried = false;
                root.state = "ready";
                const held = root.waiting;
                root.waiting = [];
                for (var i = 0; i < held.length; i++)
                    root.send(held[i].method, held[i].params, held[i].callback);
            });
        }
        onError: root.lost()
    }

    Process {
        id: ensure
        command: ["/bin/sh", "-c", "exec ryoget ensure"]
        environment: Spawn.env
        onExited: code => {
            if (code === 0) {
                root.connectSocket();
            } else if (code === 127) {
                root.state = "not-installed";
                root.errorText = I18n.tr("The download engine is not installed.");
                root.failAll("engine", root.errorText);
            } else {
                root.state = "error";
                root.errorText = I18n.tr("The download engine did not start.");
                root.failAll("engine", root.errorText);
            }
        }
    }

    // only when the session has no runtime directory to hold the socket
    Process {
        id: whoami
        command: ["id", "-u"]
        stdout: SplitParser { onRead: line => root.uid = String(line).trim() }
        onExited: {
            if (root.socketPath.length > 0)
                root.connectSocket();
            else
                root.lost();
        }
    }
}

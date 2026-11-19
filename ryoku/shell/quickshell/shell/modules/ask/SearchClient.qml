pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "lib/web.js" as Web

// The bar's line to ryoseek, the search daemon: newline-delimited JSON on its
// socket. Nothing connects until start() or a search; a missing socket runs
// `ryoseek ensure` once, which starts the on-demand service. The daemon owns
// every engine's budget and block timer; this holds what each source has
// answered for the current query. Grouping is lib/web.js.
Scope {
    id: root

    // idle | connecting | ready | unavailable
    property string state: "idle"
    property var sources: []
    // [{id, name, sources:[sourceId], state: "ok"|"paused", until}]
    property var engines: []
    // per source id: {state: "loading"|"waiting"|"done"|"error"|"paused", items, more, rank, until, error}
    property var states: ({})
    property string query: ""
    property string scope: "all"
    // suggest (while typing, no web engine runs) | full
    property string mode: "suggest"
    property int queryId: 0
    property bool settled: true
    // the sources that have spoken for queryId
    property var touched: ({})
    property var detail: null
    property int detailId: 0
    property string detailFor: ""
    property int nextId: 1
    property var queued: null
    property bool ensureTried: false
    property bool missing: false
    property bool closing: false
    property bool greeted: false
    property string uid: ""

    readonly property bool ready: state === "ready"
    readonly property string lang: Web.langTag(Qt.locale().name, I18n.lang)
    readonly property string socketPath: {
        const runtime = Quickshell.env("XDG_RUNTIME_DIR") || "";
        if (runtime.length > 0)
            return runtime + "/ryoseek.sock";
        return uid.length > 0 ? "/tmp/ryoseek-" + uid + ".sock" : "";
    }

    function start() {
        if (state === "ready" || state === "connecting" || missing)
            return;
        ensureTried = false;
        state = "connecting";
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

    function send(message) {
        socket.write(JSON.stringify(message) + "\n");
        socket.flush();
    }

    function fail() {
        state = "unavailable";
        settled = true;
        queued = null;
    }

    function lost() {
        if (closing)
            return;
        if (greeted) {
            // the daemon left (a restart): the next search wakes it
            greeted = false;
            state = "idle";
            settled = true;
            return;
        }
        if (state !== "connecting" || ensure.running)
            return;
        if (ensureTried) {
            fail();
            return;
        }
        ensureTried = true;
        ensure.running = true;
    }

    // What a source last answered stays in sight until it answers again. While
    // typing, only the sources that answer as you type keep theirs: the rest
    // belong to an older search.
    function search(text, scope, mode) {
        const value = String(text || "").trim();
        const where = String(scope || "all");
        const how = mode === "full" ? "full" : "suggest";
        query = value;
        root.scope = where;
        root.mode = how;
        if (!ready) {
            queued = value.length > 0 ? { text: value, scope: where, mode: how } : null;
            if (value.length === 0)
                states = ({});
            else if (state === "idle")
                start();
            return;
        }
        if (!settled)
            send({ cancel: queryId });
        queryId = nextId++;
        detail = null;
        detailFor = "";
        touched = ({});
        if (value.length === 0) {
            states = ({});
            settled = true;
            return;
        }
        if (how === "suggest") {
            const kept = {};
            for (var i = 0; i < sources.length; i++) {
                const id = sources[i].id;
                if (sources[i].typing === true && states[id])
                    kept[id] = states[id];
            }
            states = kept;
        }
        settled = false;
        send({ id: queryId, q: value, scope: where, mode: how });
    }

    function requestDetail(source, key) {
        const mark = String(source) + ":" + String(key);
        if (!ready || mark === detailFor)
            return;
        detailFor = mark;
        detail = null;
        detailId = nextId++;
        send({ id: detailId, detail: { source: String(source), key: String(key) } });
    }

    function frame(line) {
        var message;
        try {
            message = JSON.parse(String(line));
        } catch (error) {
            return;
        }
        if (message.ready === true) {
            sources = message.sources || [];
            engines = message.engines || [];
            greeted = true;
            ensureTried = false;
            state = "ready";
            if (queued) {
                const pending = queued;
                queued = null;
                search(pending.text, pending.scope, pending.mode);
            }
            return;
        }
        if (message.engines !== undefined) {
            engines = message.engines || [];
            return;
        }
        if (message.detail !== undefined) {
            if (Number(message.id) === detailId)
                detail = message.detail;
            return;
        }
        if (Number(message.id) !== queryId)
            return;
        const next = {};
        if (message.source === undefined) {
            if (message.state !== "done")
                return;
            // a source this query never reached has nothing to say about it
            for (var id in states) {
                const entry = states[id];
                if (touched[id] !== true)
                    continue;
                next[id] = entry.state === "loading" || entry.state === "waiting"
                    ? { state: "done", items: [], more: false, rank: 0, until: "", error: "" } : entry;
            }
            states = next;
            settled = true;
            return;
        }
        const from = String(message.source);
        const before = states[from] || ({});
        for (var key in states)
            next[key] = states[key];
        if (message.state === "loading" || message.state === "waiting")
            next[from] = { state: String(message.state), items: before.items || [], more: before.more === true, rank: before.rank || 0, until: "", error: "" };
        else if (message.state === "error" || message.state === "paused")
            next[from] = { state: String(message.state), items: [], more: false, rank: 0, until: String(message.until || ""), error: String(message.error || "") };
        else
            next[from] = { state: "done", items: message.items || [], more: message.more === true, rank: Number(message.rank || 0), until: "", error: "" };
        touched[from] = true;
        states = next;
    }

    Socket {
        id: socket
        path: root.socketPath
        parser: SplitParser { onRead: line => root.frame(line) }
        onConnectionStateChanged: {
            if (!connected) {
                root.lost();
                return;
            }
            root.send({ hello: { client: "bar", version: 2, lang: root.lang } });
        }
        onError: root.lost()
    }

    Process {
        id: ensure
        command: ["/bin/sh", "-c", "exec ryoseek ensure"]
        environment: Spawn.env
        onExited: code => {
            if (code === 0) {
                root.connectSocket();
                return;
            }
            root.missing = code === 127;
            root.fail();
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

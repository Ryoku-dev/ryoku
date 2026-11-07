pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import "../lib/busclient.js" as BusClient
import "../lib/buspatch.js" as BusPatch

// The one connection this process keeps to the ryoku-shell daemon. Every topic
// subscription, daemon call and verb, cross-process IpcHandler call and served
// target shares it, so a frame is parsed once however many surfaces read it.
// The protocol state lives in lib/busclient.js; this owns the socket and the
// reconnect timer.
Singleton {
    id: root

    readonly property bool connected: root._connected
    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    readonly property string clientName: BusClient.clientName(Quickshell.env("RYOKU_BUS_CLIENT"), Quickshell.shellDir)

    property bool _connected: false
    property var _client: null

    // Opened on first use: a process that never touches the bus never dials.
    function _c() {
        if (root._client === null) {
            root._client = BusClient.create({
                name: root.clientName,
                pid: Quickshell.processId,
                write: text => {
                    sock.write(text);
                    sock.flush();
                },
                applyPatch: BusPatch.apply,
                onConnected: v => root._connected = v,
                log: text => console.warn(text)
            });
            sock.connected = true;
        }
        return root._client;
    }

    // fn(data, info): info is {rev, paths} for state (paths null on a full
    // frame) or {seq, dropped, event: true} for an ordered event. The data is
    // shared by every subscriber in the process; never mutate it.
    function subscribe(topic, fn) { return root._c().subscribe(topic, fn); }
    function unsubscribe(token) { if (root._client !== null) root._client.unsubscribe(token); }

    // Each returns a request id; cancel(id) drops it while it is still queued
    // (the callback is then never called) and reports whether it did.
    function call(method, args, cb) { return root._c().call(method, args, cb); }
    function run(line, cb, stdin) { return root._c().run(line, cb, stdin); }
    function ipc(target, fn, args, cb, client) { return root._c().ipc(target, fn, args, cb, client); }
    function ipcGet(target, prop, cb, client) { return root._c().ipcGet(target, prop, cb, client); }
    function cancel(id) { return root._client !== null && root._client.cancel(id); }

    function serve(target, handler) { root._c().serve(target, handler); }
    function unserve(target, handler) { if (root._client !== null) root._client.unserve(target, handler); }
    // BusIpcHandler's hook: handler serves exactly target ("" = nothing).
    function _serveAs(handler, target) {
        if (target === "" && root._client === null)
            return;
        root._c().serveAs(handler, target);
    }

    function _down() {
        if (root._client === null || retry.running)
            return;
        retry.interval = root._client.socketClosed();
        retry.start();
    }

    // A failed dial reports only error, with connected still false; a dropped
    // peer reports error and then the state change, so the state change owns
    // that case.
    Socket {
        id: sock
        path: root.sockPath
        parser: SplitParser {
            onRead: line => root._client.receive(line)
        }
        onConnectionStateChanged: {
            if (root._client === null)
                return;
            if (connected) {
                retry.stop();
                root._client.socketOpened();
            } else {
                root._down();
            }
        }
        onError: {
            if (!connected)
                root._down();
        }
    }

    Timer {
        id: retry
        onTriggered: sock.connected = true
    }
}

pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Bus

// The one connection this process keeps to the ryoku-shell daemon. Every topic
// subscription, daemon call and verb, cross-process IpcHandler call and served
// target shares it, so a frame is parsed once however many surfaces read it.
// The client is native (Ryoku.Bus BusClient: socket and parsing on a worker
// thread); this keeps the name every Ryoku surface already uses.
Singleton {
    id: root

    readonly property bool connected: BusClient.connected
    readonly property string sockPath: BusClient.socketPath
    readonly property string clientName: BusClient.clientName

    // fn(data, info): info is {rev, paths} for state (paths null on a full
    // frame) or {seq, dropped, event: true} for an ordered event. The data is
    // shared by every subscriber in the process; never mutate it.
    function subscribe(topic, fn) { return BusClient.subscribe(topic, fn); }
    function unsubscribe(token) { BusClient.unsubscribe(token); }

    // Each returns a request id; cancel(id) drops it while it is still queued
    // (the callback is then never called) and reports whether it did.
    function call(method, args, cb) { return BusClient.call(method, args, cb); }
    function run(line, cb, stdin) { return BusClient.run(line, cb, stdin); }
    function ipc(target, fn, args, cb, client) { return BusClient.ipc(target, fn, args, cb, client); }
    function ipcGet(target, prop, cb, client) { return BusClient.ipcGet(target, prop, cb, client); }
    function cancel(id) { return BusClient.cancel(id); }

    function serve(target, handler) { BusClient.serve(target, handler); }
    function unserve(target, handler) { BusClient.unserve(target, handler); }
    // BusIpcHandler's hook: handler serves exactly target ("" = nothing).
    function _serveAs(handler, target) { BusClient.serveAs(handler, target); }
}

pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Bus

Singleton {
    id: root

    readonly property bool connected: BusClient.connected
    readonly property string sockPath: BusClient.socketPath
    readonly property string clientName: BusClient.clientName

    // data is shared by every subscriber in the process, never mutate it
    function subscribe(topic, fn) { return BusClient.subscribe(topic, fn); }
    function unsubscribe(token) { BusClient.unsubscribe(token); }

    function call(method, args, cb) { return BusClient.call(method, args, cb); }
    function run(line, cb, stdin) { return BusClient.run(line, cb, stdin); }
    function ipc(target, fn, args, cb, client) { return BusClient.ipc(target, fn, args, cb, client); }
    function ipcGet(target, prop, cb, client) { return BusClient.ipcGet(target, prop, cb, client); }
    function cancel(id) { return BusClient.cancel(id); }

    function serve(target, handler) { BusClient.serve(target, handler); }
    function unserve(target, handler) { BusClient.unserve(target, handler); }
    function _serveAs(handler, target) { BusClient.serveAs(handler, target); }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Scope {
    id: root

    property string state: "connecting"
    property string errorText: ""
    property string logPath: (Quickshell.env("XDG_STATE_HOME") || ((Quickshell.env("HOME") || "") + "/.local/state")) + "/ryoku/rashin/rashind.log"
    property int nextId: 1
    property var callbacks: ({})
    property bool ensureTried: false
    property bool helloDone: false
    property bool closingSocket: false
    property bool socketWasConnected: false

    readonly property bool ready: state === "ready"

    signal eventReceived(string name, var data)
    signal becameReady()

    function send(method, params, callback) {
        const id = nextId++;
        if (callback)
            callbacks[id] = callback;
        socket.write(JSON.stringify({ id: id, method: method, params: params || ({}) }) + "\n");
        socket.flush();
        return id;
    }

    function call(method, params, callback) {
        if (!ready) {
            if (callback)
                callback(null, { code: state, message: errorText });
            return 0;
        }
        return send(method, params, callback);
    }

    function failPending(code, message) {
        const pending = callbacks;
        callbacks = ({});
        for (var id in pending) {
            try {
                pending[id](null, { code: code, message: message });
            } catch (error) {
                // One broken consumer must not leave later requests pending.
            }
        }
    }

    function closeSocket() {
        closingSocket = true;
        helloDone = false;
        socketWasConnected = false;
        socket.connected = false;
        Qt.callLater(function() { root.closingSocket = false; });
    }

    function connectSocket() {
        state = "connecting";
        errorText = "";
        helloDone = false;
        socketWasConnected = false;
        closingSocket = true;
        socket.connected = false;
        Qt.callLater(function() {
            root.closingSocket = false;
            if (root.state === "connecting")
                socket.connected = true;
        });
    }

    function tryEnsure() {
        if (state === "off" || state === "not-installed" || ensureProc.running)
            return;
        if (ensureTried) {
            state = "error";
            errorText = I18n.tr("Rashin could not reconnect. See %1").arg(logPath);
            return;
        }
        ensureTried = true;
        state = "connecting";
        errorText = "";
        ensureProc.running = true;
    }

    function connectionFailed() {
        if (closingSocket || state === "off" || state === "not-installed")
            return;
        const wasReady = helloDone || state === "ready";
        helloDone = false;
        socketWasConnected = false;
        failPending("disconnected", I18n.tr("Rashin disconnected. See %1").arg(logPath));
        if (wasReady)
            ensureTried = false;
        if (ensureProc.running)
            return;
        tryEnsure();
    }

    function frame(line) {
        var frame;
        try {
            frame = JSON.parse(String(line));
        } catch (error) {
            return;
        }
        if (frame.event) {
            eventReceived(String(frame.event), frame.data || ({}));
            return;
        }
        if (frame.id === undefined || frame.id === null)
            return;
        const callback = callbacks[frame.id];
        if (!callback)
            return;
        delete callbacks[frame.id];
        callback(frame.result || ({}), frame.error || null);
    }

    Socket {
        id: socket
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/rashin/rashin.sock"
        parser: SplitParser { onRead: line => root.frame(line) }
        Component.onCompleted: root.connectSocket()
        onConnectionStateChanged: {
            if (!connected) {
                if (!root.closingSocket && (root.helloDone || root.socketWasConnected))
                    root.connectionFailed();
                return;
            }
            root.socketWasConnected = true;
            root.send("hello", { client: "bar", version: 1 }, function(result, error) {
                if (error) {
                    if (error.code === "disconnected")
                        return;
                    root.state = error.code === "disabled" ? "off" : "error";
                    root.errorText = String(error.message || "");
                    root.closeSocket();
                    return;
                }
                if (result.enabled === false) {
                    root.state = "off";
                    root.closeSocket();
                    return;
                }
                root.ensureTried = false;
                root.helloDone = true;
                root.state = "ready";
                root.becameReady();
            });
        }
        onError: root.connectionFailed()
    }

    Process {
        id: ensureProc
        command: ["/bin/sh", "-c", "exec rashin ensure"]
        environment: Spawn.env
        onExited: code => {
            if (code === 0) {
                root.connectSocket();
            } else if (code === 3) {
                root.state = "off";
                root.closeSocket();
            } else if (code === 127) {
                root.state = "not-installed";
                root.closeSocket();
            } else {
                root.state = "error";
                root.errorText = I18n.tr("Rashin could not start. See %1").arg(root.logPath);
                root.closeSocket();
            }
        }
    }
}

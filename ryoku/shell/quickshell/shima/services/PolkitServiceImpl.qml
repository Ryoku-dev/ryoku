pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io

Scope {
    id: root
    property var agent: null
    property var state: ({})
    property bool busy: false
    readonly property bool active: state.active === true
    property bool interactionAvailable: active && !busy && String(state.prompt || "").length > 0
    property QtObject flow: QtObject {
        readonly property string message: root.state.message || ""
        readonly property string inputPrompt: root.state.prompt || ""
        readonly property bool responseVisible: root.state.echo === true
    }
    readonly property string socketPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    function apply(line: string): void {
        try {
            const next = JSON.parse(line)
            if (!next.active || !state.active || next.error !== state.error || next.prompt !== state.prompt) busy = false
            state = next
        } catch (e) { state = {}; busy = false }
    }
    function send(method: string, args: var): void {
        // The response travels only over the local control socket, never argv.
        ctl.pending = "call " + method + " " + JSON.stringify(args) + "\n"
        if (ctl.connected) ctl.sendPending()
        else ctl.connected = true
    }
    function cancel(): void { if (active) send("polkit.cancel", {}) }
    function submit(text: string): void {
        if (!interactionAvailable) return
        busy = true
        send("polkit.submit", { password: text })
    }
    Loader {
        id: subscriber
        sourceComponent: Socket {
            path: root.socketPath
            parser: SplitParser { onRead: line => root.apply(line) }
            Component.onCompleted: connected = true
            onConnectionStateChanged: {
                if (connected) { write("subscribe polkit\n"); flush() }
                else { root.state = {}; subscriber.active = false }
            }
            onError: { root.state = {}; subscriber.active = false }
        }
    }
    Timer {
        interval: 500
        repeat: true
        running: !subscriber.active
        onTriggered: subscriber.active = true
    }
    Socket {
        id: ctl
        path: root.socketPath
        property string pending: ""
        function sendPending(): void {
            write(pending)
            flush()
            pending = ""
        }
        onConnectionStateChanged: if (connected) sendPending()
        onError: { pending = ""; root.busy = false }
    }
}

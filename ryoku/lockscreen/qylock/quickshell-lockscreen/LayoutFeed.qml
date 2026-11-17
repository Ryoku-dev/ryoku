import QtQuick
import Quickshell
import Quickshell.Io

// The active keyboard layout on the lock, read from the shell daemon's window
// manager topic and changed through the compositor provider, so it works on
// every compositor and never depends on a desktop keybind.
Scope {
    id: feed

    property string active: ""
    property bool switchable: true
    property int seen: -1

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    function cycle() {
        if (!cycleProc.running)
            cycleProc.running = true
    }

    function apply(line) {
        let frame
        try { frame = JSON.parse(line) } catch (e) { return }
        if (!frame || typeof frame !== "object" || Array.isArray(frame))
            return
        if (frame.caps && typeof frame.caps.keyboardLayoutSwitch === "boolean")
            switchable = frame.caps.keyboardLayoutSwitch
        const stamp = frame.versions ? frame.versions.keyboard : undefined
        if (typeof stamp === "number") {
            if (stamp <= seen)
                return
            seen = stamp
        }
        if (typeof frame.keyboardLayout === "string")
            active = frame.keyboardLayout
    }

    Process {
        id: cycleProc
        command: ["ryoku", "wm", "act", "keyboard.cycleLayout"]
    }

    Socket {
        id: sub
        path: feed.sockPath
        parser: SplitParser { onRead: line => feed.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                feed.seen = -1
                write("subscribe wm\n")
                flush()
            } else {
                retry.restart()
            }
        }
    }

    Timer {
        id: retry
        interval: 2000
        onTriggered: sub.connected = true
    }
}

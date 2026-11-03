pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root
    property var frame: ({})
    readonly property bool ready: frame.default !== undefined
    function entry(monitor: string): var {
        return frame.outputs?.[monitor] ?? frame.default ?? {}
    }
    function pathFor(monitor: string): string {
        const value = entry(monitor)
        return value.live ? (value.path ?? "") : (value.videoPath || value.path || "")
    }
    function liveFor(monitor: string): bool { return entry(monitor).live === true }
    function apply(line: string): void {
        try {
            const value = JSON.parse(line)
            if (value.default && value.outputs) frame = value
        } catch (e) { console.warn("Shima wallpaper frame:", e) }
    }
    Loader {
        id: subscriber
        active: true
        sourceComponent: Socket {
            path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryogami.sock"
            parser: SplitParser { onRead: line => root.apply(line) }
            Component.onCompleted: connected = true
            onConnectionStateChanged: {
                if (connected) { write("subscribe wallpaper\n"); flush() }
                else subscriber.active = false
            }
            onError: subscriber.active = false
        }
    }
    Timer {
        interval: 500
        repeat: true
        running: !subscriber.active
        onTriggered: subscriber.active = true
    }
}

import QtQuick
import Quickshell
import Quickshell.Io

// a Wallpaper Engine scene renders beneath the desktop surface and never sees the pointer itself
Item {
    id: root

    property bool active: false
    property string output: ""

    property real nx: 0
    property real ny: 0
    property bool linkUp: true
    property string sent: ""

    function send(): void {
        const socket = link.item;
        if (!socket || !socket.connected)
            return;
        const line = "pointer " + root.output + " " + root.nx.toFixed(5) + " " + root.ny.toFixed(5) + "\n";
        if (line === root.sent)
            return;
        root.sent = line;
        socket.write(line);
        socket.flush();
    }

    function dropLink(): void {
        root.linkUp = false;
    }

    HoverHandler {
        id: hover
        enabled: root.active && root.output !== ""
        onPointChanged: {
            if (!enabled || root.width <= 0 || root.height <= 0)
                return;
            root.nx = point.position.x / root.width;
            root.ny = point.position.y / root.height;
            if (!flush.running)
                flush.start();
        }
    }

    Timer {
        id: flush
        interval: 16
        onTriggered: root.send()
    }

    Loader {
        id: link
        active: hover.enabled && root.linkUp
        sourceComponent: Socket {
            path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryogami.sock"
            parser: SplitParser {
                onRead: line => {}
            }
            Component.onCompleted: {
                root.sent = "";
                connected = true;
            }
            // a refused connect fails while the Loader is still activating
            onConnectionStateChanged: if (!connected) Qt.callLater(root.dropLink)
            onError: Qt.callLater(root.dropLink)
        }
    }

    // A fresh socket per attempt, as the wallpaper bridge does after a daemon restart.
    Timer {
        interval: 1000
        running: hover.enabled && !root.linkUp
        onTriggered: root.linkUp = true
    }
}

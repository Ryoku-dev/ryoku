import QtQuick
import Quickshell
import shell.services as Services

ShellRoot {
    id: root
    property int tick: 0
    property var names: []
    property int failed: 0
    property bool started: false
    FloatingWindow {
        visible: true
        implicitWidth: 640
        implicitHeight: 480
        Grid {
            columns: 20
            Repeater {
                id: images
                model: root.names
                Image {
                    required property string modelData
                    width: 28
                    height: 28
                    asynchronous: true
                    source: Services.Icons.path(modelData, true)
                    sourceSize: Qt.size(24 + (Math.min(root.tick, 30) % 8) * 4, 24 + (Math.min(root.tick, 30) % 8) * 4)
                    onStatusChanged: {
                        if (status === Image.Error) root.failed++;
                    }
                }
            }
        }
    }
    Timer {
        interval: 50
        running: true
        repeat: true
        onTriggered: {
            if (Services.Icons.rev === 0) return;
            if (!root.started) {
                const names = Object.keys(Services.Icons.index).filter(n => /\.(svg|png)$/.test(Services.Icons.index[n])).slice(0, 300);
                if (names.length < 50) throw new Error("Insufficient indexed icons");
                for (const name of names) {
                    const path = Services.Icons.path(name, true);
                    if (!path.startsWith("file://")) throw new Error("Provider URL leaked: " + path);
                    if (Services.Icons.path("image://icon/" + name, true) !== path) throw new Error("Provider input failed");
                    if (Services.Icons.path("missing-test-icon-2381", name) !== path) throw new Error("Fallback failed");
                }
                if (Services.Icons.path("missing-test-icon-2381", true) !== "") throw new Error("Missing icon check failed");
                root.names = names;
                root.started = true;
            }
            root.tick++;
            if (root.tick === 50) {
                for (let i = 0; i < images.count; i++) {
                    if (images.itemAt(i).status !== Image.Ready) throw new Error("Icon not decoded: " + i);
                }
                console.log("ICONS_SMOKE_PASS icons=" + root.names.length + " resizeCycles=" + Math.min(root.tick, 30) + " decodeErrors=" + root.failed);
                Qt.quit();
            }
        }
    }
    Timer { interval: 15000; running: true; onTriggered: { console.error("ICONS_SMOKE_TIMEOUT"); Qt.quit(); } }
}

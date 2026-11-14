import QtQuick
import Quickshell
import shell.services as Services

ShellRoot {
    id: root
    property int tick: 0
    property var names: []
    property int failed: 0
    property bool finished: false
    property bool started: false
    function fail(message) {
        if (root.finished) return;
        root.finished = true;
        checks.stop();
        deadline.stop();
        console.error("ICONS_SMOKE_FAIL " + message);
        Qt.quit();
    }

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
                        if (status === Image.Error) {
                            root.failed++;
                            root.fail("Icon decode failed: " + modelData);
                        }
                    }
                }
            }
        }
    }
    Timer {
        id: checks
        interval: 50
        running: true
        repeat: true
        onTriggered: {
            if (root.finished) return;
            try {
                if (Services.Icons.rev === 0) return;
                if (!root.started) {
                    const names = Object.keys(Services.Icons.index).filter(n => /\.(svg|png)$/.test(Services.Icons.index[n])).slice(0, 300);
                    if (names.length < 50) throw new Error("Insufficient indexed icons");
                    for (const name of names) {
                        const path = Services.Icons.path(name, true);
                        if (!path.startsWith("file://")) throw new Error("Provider URL leaked: " + path);
                        if (Services.Icons.path("image://icon/" + name, true) !== path) throw new Error("Provider input failed");
                        if (Services.Icons.path("missing-test-icon-2381", name) !== path) throw new Error("Fallback failed");
                        const withExtension = name + ".svg";
                        if (!Services.Icons.has(withExtension)) {
                            const baseName = withExtension.replace(/\.(png|svg|xpm|ico)$/i, "");
                            if (Services.Icons.path(withExtension, "") !== "") throw new Error("Unexpected first-lookup fallback");
                            const resolved = Services.Icons.path(withExtension, "") || Services.Icons.path(baseName, true);
                            if (resolved !== path) throw new Error("Extension-stripped lookup failed");
                        }
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
                    root.finished = true;
                    checks.stop();
                    deadline.stop();
                    console.log("ICONS_SMOKE_PASS icons=" + root.names.length + " resizeCycles=" + Math.min(root.tick, 30) + " decodeErrors=" + root.failed);
                    Qt.quit();
                }
            } catch (error) {
                root.fail(String(error));
            }
        }
    }
    Timer {
        id: deadline
        interval: 15000
        running: true
        onTriggered: root.fail("Timeout waiting for icon checks")
    }
}

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
                        if (Services.Icons.appIcon(name) === "") throw new Error("App icon lost an indexed name: " + name);
                    }
                    if (Services.Icons.appIcon("") !== "") throw new Error("Empty app id resolved");
                    if (Services.Icons.appIcon("missing-test-icon-2381") !== "") throw new Error("Unknown app id resolved");
                    // a theme name with no desktop entry and a case of its own, so only
                    // the suffix-stripped stem answers it
                    let bare = "";
                    for (const name of names) {
                        if (DesktopEntries.heuristicLookup(name) || name === name.toLowerCase()) continue;
                        bare = name;
                        break;
                    }
                    if (bare !== "" && Services.Icons.appIcon(bare + ".desktop") !== Services.Icons.path(bare, true))
                        throw new Error("Suffix-stripped stem lost its icon: " + bare);
                    const entry = DesktopEntries.heuristicLookup("ryoku-icon-probe");
                    const declared = (entry && entry.icon) ? Services.Icons.path(entry.icon, true) : "";
                    if (declared === "") throw new Error("Controlled desktop entry unreadable");
                    if (Services.Icons.appIcon("ryoku-icon-probe") !== declared) throw new Error("Desktop entry icon ignored");
                    if (Services.Icons.path("ryoku-icon-probe", true) !== "") throw new Error("Test id collides with an icon name");
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

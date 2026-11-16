pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// the system card's code: the tick rulers that carry its design in its pixels
// (`ryoku-hub profile code`, backend/cardcode.go), read again a moment after the design
// changes. `code` is null until the first read lands
Singleton {
    id: root

    property var code: null
    readonly property bool ready: root.code !== null && !reader.running && !settle.running

    function refresh() {
        settle.restart();
    }

    Connections {
        target: ProfileStore
        function onDataChanged() { settle.restart(); }
    }
    // the store writes profile.json a beat after its data changes
    Timer {
        id: settle
        interval: 400
        onTriggered: reader.running = true
    }
    Process {
        id: reader
        running: true
        command: ["ryoku-hub", "profile", "code"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    root.code = JSON.parse(text);
                } catch (e) {
                    root.code = null;
                }
            }
        }
    }
}

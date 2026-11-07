pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import shell.services as Ryoku

Singleton {
    id: root

    property bool isInitialized: false
    readonly property var syncedLines: (Ryoku.Music.lines ?? []).map(line => ({
        time: Number(line.t ?? line.time ?? 0),
        text: String(line.text ?? "")
    }))
    readonly property int currentIndex: Ryoku.Music.index
    readonly property bool hasSyncedLines: root.syncedLines.length > 0
    readonly property string statusText: root.currentIndex >= 0 && root.currentIndex < root.syncedLines.length
        ? root.syncedLines[root.currentIndex].text : ""
    readonly property string status: Ryoku.Music.status
    readonly property bool searching: root.status === "loading"
    readonly property bool hasAnyLyrics: root.hasSyncedLines

    function initiliazeLyrics(): void {
        if (root.isInitialized)
            return;
        root.isInitialized = true;
        Ryoku.Music.hold(root, true);
    }

    function initializeLyrics(): void { root.initiliazeLyrics(); }

    function getLineDuration(index): real {
        if (index < 0 || index >= root.syncedLines.length)
            return 0;
        if (index === root.syncedLines.length - 1)
            return 5;
        return Math.max(0, Number(root.syncedLines[index + 1].time ?? 0)
            - Number(root.syncedLines[index].time ?? 0));
    }

    function changeDurationToIndex(index): void {
        if (index < 0 || index >= root.syncedLines.length)
            return;
        Ryoku.Music.seek(root.syncedLines[index].time);
    }

    Component.onDestruction: {
        if (root.isInitialized)
            Ryoku.Music.hold(root, false);
    }
}

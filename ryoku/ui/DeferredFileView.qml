pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io

FileView {
    id: root

    property int readRetryLimit: 3
    property string readLabel: path
    property int _readAttempt: 0

    signal textReady(string content)

    function _readText(): void {
        try {
            root.textReady(root.text());
        } catch (error) {
            if (root._readAttempt < root.readRetryLimit) {
                root._readAttempt++;
                Qt.callLater(() => root._readText());
                return;
            }
            console.warn("FileView text read failed for " + root.readLabel + ":", error);
        }
    }

    onLoaded: {
        root._readAttempt = 0;
        Qt.callLater(() => root._readText());
    }

}

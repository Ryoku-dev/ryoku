pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property bool open: false
    property var screen: null

    function show(target) {
        if (root.open)
            return;
        root.screen = target || (Quickshell.screens.length > 0 ? Quickshell.screens[0] : null);
        root.open = root.screen !== null;
    }

    function close() {
        root.open = false;
        root.screen = null;
    }

    IpcHandler {
        target: "kairos"
        function settings(): void {
            root.show(null);
        }
    }
}

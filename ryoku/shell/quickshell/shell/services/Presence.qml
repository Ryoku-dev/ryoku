pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Whether anyone can see the desktop: the session is locked, or the idle policy
// powered the displays off. Samplers that only feed on-screen readouts pause
// while it is dark, so a desktop nobody can see changes nothing and draws
// nothing. Any fault reads as not dark, so a readout can only keep running.
Singleton {
    id: root

    property bool locked: false
    property bool displaysOff: false
    readonly property bool dark: root.locked || root.displaysOff

    function clear() {
        root.locked = false;
        root.displaysOff = false;
    }

    function apply(frame) {
        if (!frame || typeof frame.locked !== "boolean" || typeof frame.displaysOff !== "boolean") {
            root.clear();
            return;
        }
        root.locked = frame.locked;
        root.displaysOff = frame.displaysOff;
    }

    BusTopic {
        id: topicFeed
        topic: "presence"
        onUpdated: root.apply(topicFeed.data)
    }

    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected)
                root.clear();
        }
    }
}

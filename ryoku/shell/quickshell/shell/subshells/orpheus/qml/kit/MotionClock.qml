pragma Singleton

import QtQuick
import Quickshell
import shell.services

// One shared step drives the spinning cover and seek squiggle. The analyser's
// level updates already provide that cadence while it is active, so the timer
// sleeps then. Its 40 ms gap check bridges the handoff after analysis stops.
Singleton {
    id: root

    property int users: 0
    property double _last: 0

    signal step(double now)

    function _fire(): void {
        root._last = Date.now();
        root.step(root._last);
    }

    Connections {
        target: root.users > 0 ? AudioBars : null
        function onLevelsChanged(): void { root._fire(); }
    }

    Timer {
        interval: 33
        repeat: true
        running: root.users > 0 && !AudioBars.analysing
        onTriggered: {
            if (Date.now() - root._last > 40)
                root._fire();
        }
    }
}

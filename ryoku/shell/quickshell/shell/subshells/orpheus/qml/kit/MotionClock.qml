pragma Singleton

import QtQuick
import Quickshell
import shell.services

// One 30 Hz step for the bar's decorative motion: the spinning cover and the
// seek squiggle. On timers of their own they each landed out of phase with the
// level bars, and the bar presented at the union of every clock. While the
// analyser runs the step rides its level updates, which redraw the bar anyway;
// the timer only covers silence and a stopped analyser.
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
        running: root.users > 0
        onTriggered: {
            if (Date.now() - root._last > 40)
                root._fire();
        }
    }
}

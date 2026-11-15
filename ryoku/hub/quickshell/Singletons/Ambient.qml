pragma Singleton

import QtQuick
import Quickshell
import Lithic

// the sheet's small life at rest: a clock that ticks and a scan that crosses the plates now
// and then. It runs only while Settings is the focused window and motion is allowed (the
// frame sets `on`); between scans nothing moves, so an idle window draws no frames
Singleton {
    id: root

    property bool on: false
    property date now: new Date()
    // the row a scan is crossing (0 top, 1 bottom), or -1 between scans
    property real scan: -1

    Timer {
        interval: 1000
        repeat: true
        triggeredOnStart: true
        running: root.on
        onTriggered: root.now = new Date()
    }

    // a scan every few seconds. The wait is a Timer, not a paused animation, and the scan
    // steps at 30 frames a second rather than the display's rate: a dither reads as stepped
    // anyway, and the window then redraws a few times a second on average, not continuously
    readonly property int scanEvery: 11000
    readonly property int scanLength: 2200
    property real scanStart: 0
    Timer {
        interval: root.scanEvery
        repeat: true
        running: root.on
        onTriggered: {
            root.scanStart = Date.now();
            step.restart();
        }
    }
    Timer {
        id: step
        interval: 33
        repeat: true
        onTriggered: {
            var t = (Date.now() - root.scanStart) / root.scanLength;
            if (t >= 1) {
                step.stop();
                root.scan = -1;
                return;
            }
            root.scan = -0.1 + 1.2 * (0.5 - 0.5 * Math.cos(Math.PI * t));
        }
    }
    onOnChanged: {
        if (!root.on) {
            step.stop();
            root.scan = -1;
        }
    }
}

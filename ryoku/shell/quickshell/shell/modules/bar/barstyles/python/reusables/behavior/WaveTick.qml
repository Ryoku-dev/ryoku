pragma Singleton

import QtQuick
import Quickshell

// The serpantinum wave: a slow sine that slides the liquid surface of a battery
// or progress pill through one crest and trough per `duration`. The ported
// upstream drew each wave with `NumberAnimation on phase { loops: Infinite }`,
// which advances on every display frame. On a 165 Hz panel that repaints the
// surface (and, for a desktop tile, the whole wallpaper layer) 165 times a
// second to move a 2.5 px crest a fraction of a pixel between ticks — the
// single largest idle CPU cost on the shell.
//
// One tick for every wave, like the shell's OrganicIdleClock: separate timers
// land on separate frames, so two drifting waves repainted the bar twice per
// tick instead of once. The ticker runs at the fastest interval any registered
// wave needs (a tick every ~32 cycles of its wave, which keeps the crest's
// per-frame move under half a pixel), and every wave reads its phase from the
// same tick, so all waves on screen advance in one repaint.
Singleton {
    id: root

    // Registered WaveClock facades; the tick reads each one's duration live.
    property var clients: []
    property real _t: 0

    readonly property int interval: {
        let min = 0;
        for (const c of root.clients) {
            if (min === 0 || c.duration < min)
                min = c.duration;
        }
        if (min === 0)
            return 100;
        return Math.max(33, Math.min(100, Math.round(min / 32)));
    }

    Timer {
        interval: root.interval
        repeat: true
        running: root.clients.length > 0
        onTriggered: root._t += root.interval
    }

    // One full turn per `duration` ms of accumulated real time, so a wave's
    // phase stays continuous when the shared interval changes.
    function phase(duration: int): real {
        const d = Math.max(1, duration);
        return (root._t % d) / d * 2 * Math.PI;
    }
}

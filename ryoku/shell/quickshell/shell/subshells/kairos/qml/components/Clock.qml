// The Kairos clock, shared by the bar island and the launcher so both surfaces
// read the same time in the same regional-formats locale.

import QtQuick
import Ryoku.Ui.Singletons
import Quickshell
import shell.services

// An Item, not a QtObject: QtObject has no default property, so the SystemClock
// below could not be declared inside it. It draws nothing and stays 0x0.
Item {
    id: clock

    readonly property var cfg: (Config.kairos && typeof Config.kairos === "object")
        ? Config.kairos : ({})
    readonly property bool twelve: !TimeFormat.is24h
    readonly property bool seconds: cfg.clockSeconds === true

    readonly property date now: systemClock.date
    readonly property string time: {
        var h = now.getHours();
        var m = now.getMinutes();
        var text;
        if (clock.twelve) {
            var h12 = h % 12;
            if (h12 === 0) h12 = 12;
            text = h12 + ":" + clock.pad(m);
        } else {
            text = clock.pad(h) + ":" + clock.pad(m);
        }
        if (clock.seconds)
            text += ":" + clock.pad(now.getSeconds());
        return text;
    }
    readonly property string date: now.toLocaleDateString(Config.formatLoc, "ddd d MMM").toUpperCase()

    function pad(n) { return (n < 10 ? "0" : "") + n }

    SystemClock {
        id: systemClock
        precision: clock.seconds ? SystemClock.Seconds : SystemClock.Minutes
    }
}

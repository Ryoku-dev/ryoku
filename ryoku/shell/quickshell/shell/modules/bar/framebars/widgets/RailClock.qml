pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import Quickshell

// The one timer-driven widget: a SystemClock ticks the readout. Horizontal
// bars show HH:mm on one line, vertical bars stack the hour and minute.
// The clock is intentionally inert: no click, no hover affordance, plain cursor.
// Contract 04 sec 3.2.
Item {
    id: root

    required property string edge
    required property real scale

    readonly property bool horizontal: edge === "top" || edge === "bottom"
    readonly property bool format24h: TimeFormat.is24h

    implicitWidth: btn.implicitWidth
    implicitHeight: btn.implicitHeight

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
    }

    function timeText() {
        const d = clock.date;
        const h = d.getHours();
        const shown = format24h ? h : ((h % 12) || 12);
        const hh = ("0" + shown).slice(-2);
        const mm = ("0" + d.getMinutes()).slice(-2);
        return horizontal ? TimeFormat.format(d) : (hh + "\n" + mm);
    }

    RailButton {
        id: btn
        anchors.centerIn: parent
        edge: root.edge
        scale: root.scale
        label: root.timeText()
        labelSize: 13
        labelLineHeight: 17.2
        interactive: false
    }
}

pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../../../ask/look"
import "../../../parts"
import "../../../../ask/lib/format.js" as Format

// The foot's context in Downloads: the total rate over the last minute, the
// rates now, and how many transfers are moving. note replaces it when the
// engine has something else to say.
Item {
    id: root

    property real s: 1
    property var stats: ({})
    property string note: ""
    property string counts: ""
    readonly property bool fetching: Number(root.stats.active || 0) > 0 || Number(root.stats.down || 0) > 0
    readonly property bool sending: Number(root.stats.up || 0) > 0

    Row {
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(10 * root.s)

        SpeedTrace {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.note.length === 0 && root.fetching
            width: Math.round(84 * root.s)
            height: Math.round(18 * root.s)
            s: root.s * 0.8
            samples: root.stats.history || []
            scaled: false
        }
        Row {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.note.length === 0 && root.fetching
            spacing: Math.round(2 * root.s)

            Symbol { anchors.verticalCenter: parent.verticalCenter; s: root.s; name: "arrow_downward"; size: 14; ink: Lithic.fg2 }
            Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12; figures: true; text: Format.speed(root.stats.down || 0); ink: Lithic.fg }
        }
        Row {
            anchors.verticalCenter: parent.verticalCenter
            visible: root.note.length === 0 && root.sending
            spacing: Math.round(2 * root.s)

            Symbol { anchors.verticalCenter: parent.verticalCenter; s: root.s; name: "arrow_upward"; size: 14; ink: Lithic.fg3 }
            Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12; figures: true; text: Format.speed(root.stats.up || 0); ink: Lithic.fg2 }
        }
        Line {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            size: 12
            text: root.note.length > 0 ? root.note : root.counts
            ink: Lithic.fg2
        }
    }
}

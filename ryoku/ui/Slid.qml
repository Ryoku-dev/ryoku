import QtQuick
import Ryoku.Ui.Singletons

// ratio. for quantities where the felt effect outranks the number. the track
// is 42% of the cell; the cell's numeral is the readout, so this stays mute.
Item {
    id: slid
    property real value: 0
    property real from: 0
    property real to: 100
    signal modified(real v)

    readonly property real frac: to > from ? (value - from) / (to - from) : 0
    implicitHeight: 24

    Rectangle {
        anchors.verticalCenter: parent.verticalCenter
        width: parent.width
        height: 4
        color: "transparent"
        border.width: Tokens.border
        border.color: hh.hovered ? Tokens.lineStrong : Tokens.line
        antialiasing: false
    }
    Rectangle {
        anchors.verticalCenter: parent.verticalCenter
        width: parent.width * slid.frac
        height: 4
        color: Tokens.ink
        antialiasing: false
    }
    Rectangle {
        x: Math.min(parent.width - 6, Math.max(0, parent.width * slid.frac - 3))
        anchors.verticalCenter: parent.verticalCenter
        width: 6
        height: 17
        color: Tokens.ink
        antialiasing: false
    }
    HoverHandler { id: hh; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: (p) => slid.seat(p.position.x) }
    DragHandler {
        target: null
        onCentroidChanged: if (active) slid.seat(centroid.position.x)
    }
    function seat(x) {
        var f = Math.max(0, Math.min(1, x / width));
        modified(Math.round((from + f * (to - from)) * 100) / 100);
    }
}

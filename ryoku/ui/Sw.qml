import QtQuick
import Ryoku.Ui.Singletons

// boolean. two named modes are a Seg, not a switch.
Rectangle {
    id: sw
    property bool on: false
    signal toggled(bool v)

    activeFocusOnTab: true
    function activate() { toggled(!on) }
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            activate();
            event.accepted = true;
        }
    }

    implicitWidth: 54
    implicitHeight: 24
    radius: Tokens.radius
    // off sits on a faint tint with a dim knob, on brightens the track and fills
    // the knob: two signals for one state, so it reads at a glance.
    color: tap.pressed ? Tokens.tint16 : (sw.on ? Tokens.tint16 : Tokens.tint5)
    border.width: Tokens.border
    border.color: activeFocus ? Tokens.bone : (hh.hovered ? Tokens.lineStrong : Tokens.line)
    antialiasing: false
    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
    Behavior on color { ColorAnimation { duration: Tokens.snap } }

    Rectangle {
        width: 25
        height: 17
        y: 3
        x: sw.on ? parent.width - width - 3 : 3
        radius: Tokens.radius
        antialiasing: false
        // off is a dim knob on a hairline track, not an empty outline: the state
        // has to be readable at a glance, which an outlined box is not.
        color: sw.on ? Tokens.ink : Tokens.inkDim
        border.width: 0
        Behavior on x { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
    }
    HoverHandler { id: hh; cursorShape: Qt.PointingHandCursor }
    TapHandler { id: tap; onTapped: sw.toggled(!sw.on) }
}

import QtQuick
import Ryoku.Ui.Singletons

Rectangle {
    id: btn

    property alias text: lab.text
    property bool primary: false
    property bool armed: true
    property bool compact: false
    property bool motionEnabled: !Tokens.reduceMotion
    // A TapHandler is passive: it never takes focus from a TextInput, so a
    // field's editingFinished (and its commit into the draft) does not fire
    // when the user types and then clicks the button directly. The commit-on-
    // save loss it causes (#294) is invisible and silent, so buttons that act
    // on the whole page opt in to stealing focus on press, the way a real
    // toolkit button does. Off by default: a button inside a list row would
    // have the commit's rebuild destroy its own delegate mid-tap.
    property bool stealFocus: false
    signal act()
    Keys.onPressed: event => {
        if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            if (btn.armed && !event.isAutoRepeat)
                btn.act();
            event.accepted = true;
        }
    }

    implicitWidth: lab.implicitWidth + (compact ? 20 : 30)
    implicitHeight: compact ? 24 : 32
    radius: Tokens.radius
    // 0.5, not 0.3: a control that cannot act must still be legible as itself
    // (the reader needs to see what is there and why it is inert), while the
    // dimming keeps it from reading as available.
    opacity: armed ? 1 : 0.5
    color: primary && armed ? Tokens.bone : (tap.pressed && armed ? Tokens.tint16 : (bh.hovered && armed ? Tokens.tint10 : "transparent"))
    border.width: Tokens.border
    border.color: activeFocus ? Tokens.bone : (primary && armed ? Tokens.bone : (bh.hovered && armed ? Tokens.lineStrong : Tokens.line))
    Behavior on color { enabled: btn.motionEnabled; ColorAnimation { duration: Tokens.snap } }
    Behavior on opacity { enabled: btn.motionEnabled; NumberAnimation { duration: Tokens.snap } }

    Text {
        id: lab
        anchors.centerIn: parent
        color: btn.primary && btn.armed ? Tokens.inkOnBone : Tokens.ink
        font.family: Tokens.ui
        font.pixelSize: btn.compact ? 10 : 11
        font.weight: Font.Medium
        font.letterSpacing: Tokens.trackLabel
        // Longer translations (Portuguese, German) must not be clipped when a
        // caller fixes the button narrower than the label wants: the text
        // elides instead. Unconstrained buttons still size to their text.
        width: Math.max(0, parent.width - (btn.compact ? 20 : 30))
        elide: Text.ElideRight
    }
    HoverHandler { id: bh; enabled: btn.armed; cursorShape: Qt.PointingHandCursor }
    TapHandler { id: tap; enabled: btn.armed
        onPressedChanged: if (tap.pressed && btn.stealFocus) btn.forceActiveFocus()
        onTapped: btn.act() }
}

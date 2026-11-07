import QtQuick
import Ryoku.Ui.Singletons

// The square utility button. It was an inline component in both apps, and an
// inline component is private, so having one in two apps means having two.
Rectangle {
    id: btn
    property string glyph: ""
    property bool armed: true
    // See Btn.stealFocus: page-level buttons grab focus on press so a focused
    // text field commits its edit first; row buttons leave it off.
    property bool stealFocus: false
    signal act()

    implicitWidth: 26
    implicitHeight: 26
    radius: Tokens.radius
    opacity: armed ? 1 : 0.3
    color: tap.pressed && armed ? Tokens.tint16 : (hh.hovered && armed ? Tokens.tint10 : "transparent")
    border.width: Tokens.border
    border.color: hh.hovered && armed ? Tokens.lineStrong : Tokens.line
    Behavior on color { ColorAnimation { duration: Tokens.snap } }

    // Close is not danger. A red X on a sheet with no other red reads as an
    // error, not an exit.
    Text {
        anchors.centerIn: parent
        text: btn.glyph
        color: Tokens.inkDim
        font.family: Tokens.ui
        font.pixelSize: 12
    }
    HoverHandler { id: hh; enabled: btn.armed; cursorShape: Qt.PointingHandCursor }
    TapHandler { id: tap; enabled: btn.armed
        onPressedChanged: if (tap.pressed && btn.stealFocus) btn.forceActiveFocus()
        onTapped: btn.act() }
}

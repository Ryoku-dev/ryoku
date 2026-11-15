import QtQuick
import Lithic
import "../../Singletons"

// the page's folio: its number set huge and faint in the sheet's lower corner, behind the
// page, so a short page's empty foot still reads as part of the sheet. A new page's number
// rolls in over the old one
Item {
    id: root

    property string number: ""

    clip: true

    property string shown: root.number
    property string leaving: ""
    onNumberChanged: {
        if (Lithic.reduceMotion || root.shown === "") {
            root.shown = root.number;
            return;
        }
        root.leaving = root.shown;
        root.shown = root.number;
        roll.restart();
    }

    property real travel: 0
    readonly property real base: root.height - incoming.height * 0.8
    NumberAnimation {
        id: roll
        target: root
        property: "travel"
        from: 1
        to: 0
        duration: Tone.move * 2
        easing.type: Easing.OutCubic
    }

    Label {
        id: incoming
        anchors.right: parent.right
        y: root.base + root.travel * height * 0.5
        opacity: 1 - root.travel
        text: root.shown
        typeRole: "mono"
        font.pixelSize: 168
        font.weight: Font.Light
        font.letterSpacing: -6
        ink: Lithic.alpha(Lithic.fg, Lithic.dark ? 0.05 : 0.07)
    }
    Label {
        anchors.right: parent.right
        y: root.base - (1 - root.travel) * incoming.height * 0.5
        visible: roll.running
        opacity: root.travel
        text: root.leaving
        typeRole: "mono"
        font.pixelSize: 168
        font.weight: Font.Light
        font.letterSpacing: -6
        ink: incoming.ink
    }
}

import QtQuick
import Ryoku.Ui.Singletons

// The bar every settings page needs before it needs anything else. A page that
// previews live and cannot save does not look broken, it looks fine and then
// eats the edit on the way out, so this exists as one component rather than as
// something each page remembers to build.
//
// Revert and Reset are not the same verb and the old pages conflated them:
//   revert  discards unsaved edits, back to what is on disk
//   reset   sets every key to its factory value, which is itself an edit
// Reset is destructive and sits behind a divider, away from Save.
Item {
    id: bar

    property int dirty: 0
    property string cleanText: I18n.tr("SAVED · LIVE ON YOUR DESKTOP")
    property string savingText: ""

    signal saved()
    signal reverted()
    signal reset()
    signal diffRequested()

    implicitHeight: 60

    Rectangle { anchors.top: parent.top; width: parent.width; height: 1; color: Tokens.line }

    Row {
        anchors.verticalCenter: parent.verticalCenter
        anchors.left: parent.left
        anchors.leftMargin: Tokens.s5
        spacing: Tokens.s3

        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "///"
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: 10
        }
        Rectangle {
            width: 6; height: 6
            anchors.verticalCenter: parent.verticalCenter
            // no colour in the chrome: the unsaved heartbeat is ink, and the
            // /// cluster beside it carries the sheet's mark.
            color: Tokens.ink
            SequentialAnimation on opacity {
                running: bar.dirty > 0
                loops: Animation.Infinite
                NumberAnimation { to: 0.3; duration: 600 }
                NumberAnimation { to: 1.0; duration: 600 }
            }
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: bar.dirty > 0
                  ? (bar.dirty === 1 ? I18n.tr("%1 CHANGE · PREVIEWING · NOT SAVED") : I18n.tr("%1 CHANGES · PREVIEWING · NOT SAVED")).arg(bar.dirty)
                  : I18n.tr(bar.cleanText)
            color: bar.dirty > 0 ? Tokens.ink : Tokens.inkDim
            font.family: Tokens.ui
            font.pixelSize: 11
            font.weight: Font.Medium
            font.letterSpacing: 1.6
        }
        Text {
            visible: bar.dirty > 0
            anchors.verticalCenter: parent.verticalCenter
            text: sdh.hovered ? I18n.tr("\u25b8 VIEW DIFF") : I18n.tr("\u00b7 VIEW DIFF")
            color: sdh.hovered ? Tokens.ink : Tokens.inkFaint
            font.family: Tokens.mono; font.pixelSize: 9
            font.letterSpacing: 1.2
        }
        HoverHandler { id: sdh; enabled: bar.dirty > 0; cursorShape: Qt.PointingHandCursor }
        TapHandler { enabled: bar.dirty > 0
            onPressedChanged: if (pressed) parent.forceActiveFocus()
            onTapped: bar.diffRequested() }
    }

    Row {
        anchors.verticalCenter: parent.verticalCenter
        anchors.right: parent.right
        anchors.rightMargin: Tokens.s6
        spacing: Tokens.s3

        Btn { text: I18n.tr("RESET TO DEFAULTS"); stealFocus: true; onAct: bar.reset() }
        Rectangle {
            width: 1; height: 22
            color: Tokens.line
            anchors.verticalCenter: parent.verticalCenter
        }
        Btn { text: I18n.tr("REVERT"); armed: bar.dirty > 0; stealFocus: true; onAct: bar.reverted() }
        Btn { text: I18n.tr("SAVE"); primary: true; armed: bar.dirty > 0; stealFocus: true; onAct: bar.saved() }
    }
}

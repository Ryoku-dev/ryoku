import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// the search as it stands on Home: it looks like the field at rest and opens the
// navigation pane, where the field itself takes the keys
T.AbstractButton {
    id: root

    implicitWidth: LithicTokens.size.searchWidth
    implicitHeight: LithicTokens.size.searchHeight
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    text: I18n.tr("Search settings")
    Accessible.role: Accessible.Button
    Accessible.name: root.text
    Keys.onReturnPressed: root.click()
    Keys.onEnterPressed: root.click()

    // the field's own surface: its fill, its edge hairline, the keyboard's hairline
    background: CutRect {
        fill: root.hovered ? Lithic.mix(Lithic.field, Lithic.fg, LithicTokens.alpha.hoverPlane) : Lithic.field
        strokeColor: root.visualFocus ? Lithic.fg2 : Lithic.edge
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.mid + (root.down ? Lithic.pressCutGrow : 0)
        ChiselColorBehavior on fill {
            duration: root.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
        }
        ChiselNumberBehavior on cut {
            duration: LithicTokens.motion.press
        }
    }

    contentItem: Item {
        Glyph {
            x: LithicTokens.size.searchIconInset
            anchors.verticalCenter: parent.verticalCenter
            width: LithicTokens.size.searchIcon
            height: width
            name: "search"
            ink: Lithic.fg2
            size: LithicTokens.size.searchIcon
        }
        Label {
            x: LithicTokens.size.searchLeading
            width: Math.max(0, hint.x - x - LithicTokens.space.s2)
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            typeRole: "body"
            inkRole: "fg3"
        }
        Label {
            id: hint
            anchors.right: parent.right
            anchors.rightMargin: LithicTokens.size.textFieldInset
            anchors.verticalCenter: parent.verticalCenter
            text: "/"
            typeRole: "monoSm"
            inkRole: "fg3"
        }
    }
}

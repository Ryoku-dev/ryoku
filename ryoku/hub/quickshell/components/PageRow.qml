import QtQuick
import QtQuick.Templates as T
import Lithic

// one page in the list pane: its title over a live summary
T.AbstractButton {
    id: root

    property string summary: ""
    property string glyph: ""
    property string trailingGlyph: ""
    property bool selected: false
    property bool cursor: false

    readonly property bool _hover: root.hovered || root.cursor
    readonly property color _ink: root.selected ? Lithic.onSel : Lithic.fg
    readonly property color _ink2: root.selected ? Lithic.onSel2 : Lithic.fg2

    implicitWidth: LithicTokens.size.listMedium
    implicitHeight: root.summary.length > 0 ? 48 : 36
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    Accessible.role: Accessible.Button
    Accessible.name: root.text
    Accessible.description: root.summary

    background: CutRect {
        cut: LithicTokens.cut.small
        fill: root.selected ? Lithic.sel
            : root.down ? Lithic.pressRow(Lithic.stone1)
            : root._hover ? Lithic.hoverPlane(Lithic.stone1) : "transparent"
        ChiselColorBehavior on fill {
            duration: root._hover ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
        }
        FocusRing {
            x: -outset
            y: -outset
            width: parent.width + outset * 2
            height: parent.height + outset * 2
            keyboardFocusVisible: root.visualFocus
            cut: LithicTokens.cut.small
        }
    }

    contentItem: Item {
        Glyph {
            id: lead
            visible: root.glyph.length > 0
            x: LithicTokens.space.s3
            anchors.verticalCenter: parent.verticalCenter
            name: root.glyph
            size: LithicTokens.size.menuIcon
            ink: root._ink2
        }

        Column {
            anchors.left: lead.visible ? lead.right : parent.left
            anchors.leftMargin: LithicTokens.space.s3
            anchors.right: trail.visible ? trail.left : parent.right
            anchors.rightMargin: LithicTokens.space.s3
            anchors.verticalCenter: parent.verticalCenter
            spacing: LithicTokens.size.settingsDetailGap

            Label {
                width: parent.width
                text: root.text
                typeRole: "row"
                ink: root._ink
            }
            Label {
                width: parent.width
                visible: root.summary.length > 0
                text: root.summary
                typeRole: "detail"
                ink: root._ink2
            }
        }

        Glyph {
            id: trail
            visible: root.trailingGlyph.length > 0
            anchors.right: parent.right
            anchors.rightMargin: LithicTokens.space.s3
            anchors.verticalCenter: parent.verticalCenter
            name: root.trailingGlyph
            size: LithicTokens.size.plaqueIconSmall
            ink: root._ink2
        }
    }
}

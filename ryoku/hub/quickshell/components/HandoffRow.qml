import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

// a line that opens another app or surface; command runs detached, or the
// host handles triggered() when there is no command
T.AbstractButton {
    id: root

    property string detail: ""
    property var command: []
    property string glyph: "open_in_new"
    property color plane: Lithic.slab
    property bool first: true

    signal triggered()

    readonly property real _gap: first ? 0 : LithicTokens.size.settingsRowGap

    implicitWidth: LithicTokens.size.settingsMediumMax
    implicitHeight: _gap + Math.max(LithicTokens.size.factsRowHeight, copy.implicitHeight)
    topPadding: _gap
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.Link
    Accessible.name: root.text
    Accessible.description: root.detail

    onClicked: {
        if (root.command && root.command.length > 0)
            Spawn.run(root.command);
        root.triggered();
    }
    Keys.onReturnPressed: clicked()
    Keys.onEnterPressed: clicked()

    background: Item {
        Rectangle {
            visible: !root.first
            y: Math.floor((root._gap - LithicTokens.size.hairline) / 2)
            width: root.width
            height: LithicTokens.size.hairline
            color: Lithic.vein
        }
        CutRect {
            id: plate
            x: -LithicTokens.space.s2
            y: root._gap - LithicTokens.space.s1
            width: root.width + LithicTokens.space.s2 * 2
            height: root.height - root._gap + LithicTokens.space.s1 * 2
            cut: LithicTokens.cut.small
            fill: root.down ? Lithic.pressRow(Lithic.mix(root.plane, Lithic.fg, LithicTokens.alpha.hoverPlane))
                : root.hovered ? Lithic.mix(root.plane, Lithic.fg, LithicTokens.alpha.hoverPlane)
                : Lithic.alpha(root.plane, 0)
            ChiselColorBehavior on fill {
                duration: root.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }
            FocusRing {
                x: -outset
                y: -outset
                width: plate.width + outset * 2
                height: plate.height + outset * 2
                keyboardFocusVisible: root.visualFocus
                cut: plate.cut
            }
        }
    }

    contentItem: Item {
        implicitHeight: Math.max(LithicTokens.size.factsRowHeight, copy.implicitHeight)

        Column {
            id: copy
            anchors.left: parent.left
            anchors.right: mark.left
            anchors.rightMargin: LithicTokens.space.s4
            anchors.verticalCenter: parent.verticalCenter
            spacing: LithicTokens.size.settingsDetailGap
            Label {
                width: parent.width
                text: root.text
                typeRole: "row"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Label {
                width: parent.width
                visible: text.length > 0
                text: root.detail
                typeRole: "detail"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }

        Glyph {
            id: mark
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            name: root.glyph
            size: LithicTokens.size.plaqueIcon
            ink: Lithic.fg2
        }
    }
}

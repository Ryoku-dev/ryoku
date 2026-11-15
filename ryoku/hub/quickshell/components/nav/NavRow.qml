import QtQuick
import QtQuick.Templates as T
import Lithic
import "../../Singletons"

// one line of the navigation tree: a folder (a category) or a leaf (a page). Its place in
// the tree is drawn by the guide the parent folder lays down; the row draws its own branch
// tick, mark and name, and answers the pointer with a quiet wash and a small lean
T.AbstractButton {
    id: root

    property int depth: 0
    property string glyph: ""
    property bool folder: false
    property bool expanded: false
    property bool selected: false
    // a folder holding the open page while it is shut
    property bool holdsSelection: false
    property string trailing: ""
    property string trailingGlyph: ""
    // a row outside the tree's travelling mark draws its own while it is the open page
    property bool wash: false
    property bool notch: false

    readonly property real indent: root.depth * 22
    readonly property bool _lit: root.hovered || root.visualFocus

    signal stepped(int direction)
    signal folded(bool open)

    implicitHeight: root.folder ? 34 : 30
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    Accessible.role: root.folder ? Accessible.ButtonDropDown : Accessible.Button
    Accessible.name: root.text

    Keys.onUpPressed: root.stepped(-1)
    Keys.onDownPressed: root.stepped(1)
    Keys.onRightPressed: if (root.folder && !root.expanded) root.folded(true)
    Keys.onLeftPressed: if (root.folder && root.expanded) root.folded(false)
    Keys.onReturnPressed: root.clicked()
    Keys.onEnterPressed: root.clicked()

    background: Item {
        Rectangle {
            anchors.fill: parent
            anchors.leftMargin: root.indent + 4
            radius: Tone.radiusSmall
            color: Tone.accentSoft
            opacity: root.wash ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Tone.quick } }
        }
        Rectangle {
            anchors.fill: parent
            anchors.leftMargin: root.indent + 4
            radius: Tone.radiusSmall
            color: root.down ? Tone.press : Tone.hover
            opacity: root._lit || root.down ? 1 : 0
            Behavior on opacity {
                NumberAnimation {
                    duration: root._lit ? Tone.quick : Tone.quick * 2
                    easing.type: Easing.OutCubic
                }
            }
        }
        Rectangle {
            visible: root.visualFocus
            anchors.fill: parent
            anchors.leftMargin: root.indent + 4
            radius: Tone.radiusSmall
            color: "transparent"
            border.width: 1
            border.color: Lithic.fg2
        }
        // the branch from the folder's guide to this leaf
        Rectangle {
            visible: root.depth > 0
            x: root.indent - 6
            anchors.verticalCenter: parent.verticalCenter
            width: 8
            height: 1
            color: root.selected ? Tone.accent : Tone.guide
        }
    }

    contentItem: Item {
        id: body
        // the row leans in under the pointer
        property real lean: root._lit && !root.selected ? 2 : 0
        Behavior on lean {
            NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
        }

        Glyph {
            id: chevron
            visible: root.folder
            x: root.indent + 8
            anchors.verticalCenter: parent.verticalCenter
            name: "chevron_right"
            size: 16
            ink: root._lit ? Lithic.fg : Lithic.fg3
            rotation: root.expanded ? 90 : 0
            Behavior on rotation {
                NumberAnimation { duration: Tone.move; easing.type: Easing.OutBack; easing.overshoot: 1.6 }
            }
        }

        Glyph {
            id: mark
            x: (root.folder ? chevron.x + chevron.width + 6 : root.indent + 6) + body.lean
            anchors.verticalCenter: parent.verticalCenter
            name: root.glyph
            size: root.folder ? 18 : 17
            ink: root.selected || root.holdsSelection ? Tone.accent : root._lit ? Lithic.fg : Lithic.fg2
            font.variableAxes: ({ "FILL": root.selected || (root.folder && root.expanded) ? 1 : 0 })
            Behavior on ink {
                ColorAnimation { duration: Tone.quick }
            }
            scale: root.down ? 0.86 : 1
            Behavior on scale {
                NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic }
            }
        }

        Label {
            id: name
            anchors.left: mark.right
            anchors.leftMargin: 10
            anchors.right: tail.left
            anchors.rightMargin: 6
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            typeRole: root.folder ? "label" : "detail"
            ink: root.selected ? Lithic.fg : root._lit || root.holdsSelection ? Lithic.fg : root.folder ? Lithic.fg : Lithic.fg2
            font.weight: root.selected || root.folder ? Font.DemiBold : Font.Normal
        }

        Item {
            id: tail
            anchors.right: parent.right
            anchors.rightMargin: 10
            anchors.verticalCenter: parent.verticalCenter
            width: Math.max(count.visible ? count.implicitWidth : 0, out.visible ? out.width : 0, dot.visible ? dot.width : 0)
            height: parent.height

            Label {
                id: count
                visible: root.trailing.length > 0 && !root.notch
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                text: root.trailing
                typeRole: "monoSm"
                inkRole: "fg3"
                // a shut folder says how much it holds; an open one shows it instead
                opacity: !root.expanded ? 1 : root._lit ? 0.6 : 0
                Behavior on opacity { NumberAnimation { duration: Tone.quick } }
            }
            Glyph {
                id: out
                visible: root.trailingGlyph.length > 0
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                name: root.trailingGlyph
                size: 14
                ink: Lithic.fg3
                opacity: root._lit ? 1 : 0.6
            }
            Rectangle {
                id: dot
                visible: root.notch
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                width: 8
                height: 8
                radius: 4
                color: Tone.accent
                SequentialAnimation on scale {
                    running: dot.visible && !Lithic.reduceMotion
                    loops: 3
                    NumberAnimation { from: 1; to: 1.5; duration: 420; easing.type: Easing.OutCubic }
                    NumberAnimation { from: 1.5; to: 1; duration: 520; easing.type: Easing.InOutCubic }
                }
            }
        }
    }
}

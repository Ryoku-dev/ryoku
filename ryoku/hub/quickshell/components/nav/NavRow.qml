import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// one line of the navigation tree: a folder (a category) or a leaf (a page). Its place in
// the tree is drawn by the guide the parent folder lays down; the row draws its own branch
// tick, its number (a sheet's index, as ryoku.dev numbers its sections) or mark, its name
// and its kanji gloss, and answers the pointer with a quiet wash and a small lean
T.AbstractButton {
    id: root

    property int depth: 0
    property string glyph: ""
    // the line's place in the index ("01"), drawn in mono instead of a mark
    property string numeral: ""
    // its Japanese gloss, shown while the desktop's glosses are on
    property string gloss: ""
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
    // Left on a page line goes back up to its folder, shutting it
    Keys.onLeftPressed: if (!root.folder || root.expanded) root.folded(false)
    Keys.onReturnPressed: root.clicked()
    Keys.onEnterPressed: root.clicked()

    // each plane is a CutRect, so a row's corners follow the design: cut for Lithic, a pill
    // for Material, a soft corner for the rest
    background: Item {
        CutRect {
            anchors.fill: parent
            anchors.leftMargin: root.indent + 4
            cut: Tone.rowCut
            fill: Tone.select
            opacity: root.wash ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: Tone.quick } }
        }
        CutRect {
            anchors.fill: parent
            anchors.leftMargin: root.indent + 4
            cut: Tone.rowCut
            fill: root.down ? Tone.press : Tone.hover
            opacity: root._lit || root.down ? 1 : 0
            Behavior on opacity {
                NumberAnimation {
                    duration: root._lit ? Tone.quick : Tone.quick * 2
                    easing.type: Easing.OutCubic
                }
            }
        }
        CutRect {
            visible: root.visualFocus
            anchors.fill: parent
            anchors.leftMargin: root.indent + 4
            cut: Tone.rowCut
            strokeWidth: 1
            strokeColor: Lithic.fg2
        }
        // the branch from the folder's guide to this leaf
        Rectangle {
            visible: root.depth > 0
            anchors.left: parent.left
            anchors.leftMargin: root.indent - 6
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

        // placed by anchors, so a right-to-left language mirrors the whole line
        Glyph {
            id: chevron
            visible: root.folder
            anchors.left: parent.left
            anchors.leftMargin: root.indent + 8
            anchors.verticalCenter: parent.verticalCenter
            name: root.LayoutMirroring.enabled ? "chevron_left" : "chevron_right"
            size: 16
            ink: root._lit ? Lithic.fg : Lithic.fg3
            rotation: root.expanded ? (root.LayoutMirroring.enabled ? -90 : 90) : 0
            Behavior on rotation {
                NumberAnimation { duration: Tone.move; easing.type: Easing.OutBack; easing.overshoot: 1.6 }
            }
        }

        Glyph {
            id: mark
            visible: root.numeral === "" && root.glyph !== ""
            anchors.left: parent.left
            anchors.leftMargin: (root.folder ? root.indent + 8 + 16 + 6 : root.indent + 6) + body.lean
            anchors.verticalCenter: parent.verticalCenter
            name: root.glyph
            size: root.folder ? 18 : 17
            // Lithic marks the open page in its sun; a round design inks it as its selection
            ink: root.selected ? (Tone.flat ? Tone.accent : Tone.selectInk) : root.holdsSelection ? Tone.accent : root._lit ? Lithic.fg : Lithic.fg2
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
            id: number
            visible: root.numeral !== ""
            anchors.left: parent.left
            anchors.leftMargin: (root.folder ? root.indent + 8 + 16 + 4 : root.indent + 6) + body.lean
            anchors.baseline: name.baseline
            text: root.numeral
            typeRole: "monoSm"
            ink: root.selected || root.holdsSelection || (root.folder && root.expanded) ? Tone.accent : root._lit ? Lithic.fg : Lithic.fg3
            Behavior on ink {
                ColorAnimation { duration: Tone.quick }
            }
        }

        Label {
            id: name
            anchors.left: number.visible ? number.right : mark.visible ? mark.right : parent.left
            anchors.leftMargin: number.visible || mark.visible ? 10 : root.indent + 8 + body.lean
            anchors.right: glossLabel.visible ? glossLabel.left : tail.left
            anchors.rightMargin: 6
            anchors.verticalCenter: parent.verticalCenter
            text: root.text
            typeRole: root.numeral !== "" ? "group" : root.folder ? "label" : "detail"
            ink: root.selected ? Tone.selectInk : root._lit || root.holdsSelection ? Lithic.fg : root.folder ? Lithic.fg : Lithic.fg2
            font.weight: root.selected || root.folder ? Font.DemiBold : Font.Normal
            font.capitalization: root.numeral !== "" ? Font.AllUppercase : Font.MixedCase
            font.letterSpacing: root.numeral !== "" ? 1.4 : 0
        }

        // an index line's leader, from its name to its gloss or count
        Rectangle {
            visible: root.numeral !== "" && width > 6
            anchors.verticalCenter: parent.verticalCenter
            x: name.x + name.contentWidth + 10
            width: (glossLabel.visible ? glossLabel.x : tail.x) - 8 - x
            height: 1
            color: root._lit || root.expanded ? Tone.guideLit : Tone.guide
            Behavior on color {
                ColorAnimation { duration: Tone.quick }
            }
        }

        // the line's Japanese gloss, its seal beside the Latin name
        Label {
            id: glossLabel
            visible: root.gloss !== "" && UiPreferences.showJapaneseGloss
            anchors.right: tail.left
            anchors.rightMargin: tail.width > 0 ? 8 : 0
            anchors.verticalCenter: parent.verticalCenter
            text: root.gloss
            typeRole: "caption"
            font.pixelSize: root.folder || root.numeral !== "" ? 12 : 11
            font.letterSpacing: 1
            ink: root.selected || root.holdsSelection || (root.folder && root.expanded) ? Tone.accent : root._lit ? Lithic.fg2 : Lithic.fg3
            Behavior on ink {
                ColorAnimation { duration: Tone.quick }
            }
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

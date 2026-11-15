import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// one design to choose: its name, what sets it apart, and its shapes in small (a button, a
// switch, a card corner) drawn in it. Resting on it previews the design below at full size;
// the one in use carries a ring and a mark
T.AbstractButton {
    id: root

    property string design: "lithic"
    property string detail: ""
    property bool chosen: false
    readonly property bool _lit: root.hovered || root.visualFocus

    implicitHeight: shapes.height + words.implicitHeight + 34
    hoverEnabled: true
    focusPolicy: Qt.TabFocus
    Accessible.role: Accessible.RadioButton
    Accessible.name: root.text
    Accessible.description: root.detail
    Accessible.checked: root.chosen
    Keys.onReturnPressed: root.clicked()
    Keys.onEnterPressed: root.clicked()

    background: CutRect {
        cut: Tone.cardCut
        fill: root.chosen ? Tone.select : root._lit ? Tone.hover : "transparent"
        strokeWidth: root.chosen ? 2 : 1
        strokeColor: root.chosen ? Tone.accent : root.visualFocus ? Lithic.fg2 : Tone.line
        Behavior on fill { ColorAnimation { duration: Tone.quick } }
    }

    contentItem: Item {
        DesignScope {
            id: shapes
            lithicDesign: root.design
            x: 14
            y: 14
            width: parent.width - 28
            height: 40

            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 8
                CutRect {
                    width: 54
                    height: 32
                    cut: LithicTokens.cut.small
                    fill: Tone.accent
                    Glyph {
                        anchors.centerIn: parent
                        name: "check"
                        size: 16
                        ink: Tone.accentInk
                    }
                }
                CutRect {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 40
                    height: 22
                    cut: LithicTokens.cut.toggleTrack
                    fill: Tone.accent
                    CutRect {
                        anchors.right: parent.right
                        anchors.rightMargin: 4
                        anchors.verticalCenter: parent.verticalCenter
                        width: 14
                        height: 14
                        cut: LithicTokens.cut.toggleKnob
                        fill: Tone.accentInk
                    }
                }
                CutRect {
                    width: 40
                    height: 32
                    cut: Tone.cardCut
                    fill: "transparent"
                    strokeWidth: 1.5
                    strokeColor: Lithic.fg2
                }
            }
        }

        Column {
            id: words
            x: 14
            anchors.top: shapes.bottom
            anchors.topMargin: 10
            width: parent.width - 28
            spacing: 2

            Row {
                spacing: 6
                Label {
                    text: root.text
                    typeRole: "label"
                    font.weight: Font.DemiBold
                    ink: root.chosen ? Tone.selectInk : Lithic.fg
                }
                Glyph {
                    visible: root.chosen
                    anchors.verticalCenter: parent.verticalCenter
                    name: "check_circle"
                    size: 15
                    ink: Tone.accent
                    font.variableAxes: ({ "FILL": 1 })
                }
            }
            Label {
                width: parent.width
                text: root.detail
                typeRole: "caption"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
                maximumLineCount: 3
            }
        }
    }
}

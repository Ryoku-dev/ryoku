import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// one design to choose: a small Settings window drawn in that design and in the colours it
// would wear now, its name and what sets it apart. The one in use carries a ring and a mark
T.AbstractButton {
    id: root

    property string design: "lithic"
    property string detail: ""
    property bool chosen: false

    // the roles and planes this design would wear, so every card shows itself, not the one in use
    readonly property var r: {
        Lithic.source;
        Lithic.mode;
        return Lithic.rolesFor(root.design);
    }
    readonly property var p: Tone.planes(root.design, root.r)
    readonly property bool _lit: root.hovered || root.visualFocus

    implicitHeight: shot.height + words.implicitHeight + 28
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
        // the miniature: a sidebar with the open page washed, and a card of rows with a switch
        // and a button, all drawn in this card's design
        CutRect {
            id: shot
            x: 10
            y: 10
            width: parent.width - 20
            height: Math.round(width * 0.6)
            design: root.design
            cut: Tone.cardCut
            fill: root.p.side
            strokeWidth: 1
            strokeColor: Lithic.alpha(root.r.fg, 0.12)
            scale: root._lit && !Lithic.reduceMotion ? 1.02 : 1
            Behavior on scale { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }

            Column {
                x: 8
                y: 10
                width: parent.width * 0.3
                spacing: 5
                Repeater {
                    model: 4
                    CutRect {
                        required property int index
                        width: parent.width
                        height: 9
                        design: root.design
                        cut: Tone.rowCut
                        fill: index === 1 ? root.r.container : "transparent"
                        Rectangle {
                            x: 5
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width * (index === 1 ? 0.6 : 0.5)
                            height: 3
                            radius: 1.5
                            color: Lithic.alpha(root.r.fg, index === 1 ? 0.8 : 0.4)
                        }
                    }
                }
            }

            CutRect {
                id: page
                x: parent.width * 0.34
                y: 6
                width: parent.width - x - 6
                height: parent.height - 12
                design: root.design
                cut: Tone.cardCut
                fill: root.p.pane
                strokeWidth: 1
                strokeColor: root.p.paneStroke

                Rectangle {
                    x: 8
                    y: 8
                    width: parent.width * 0.45
                    height: 5
                    radius: 2
                    color: Lithic.alpha(root.r.fg, 0.85)
                }
                CutRect {
                    x: 8
                    y: 20
                    width: parent.width - 16
                    height: parent.height - 28
                    design: root.design
                    cut: Tone.cardCut
                    fill: root.p.card
                    strokeWidth: 1
                    strokeColor: root.p.cardStroke

                    Rectangle {
                        x: 7
                        y: 8
                        width: parent.width * 0.4
                        height: 3
                        radius: 1.5
                        color: Lithic.alpha(root.r.fg, 0.6)
                    }
                    // a switch, on
                    CutRect {
                        anchors.right: parent.right
                        anchors.rightMargin: 7
                        y: 5
                        width: 18
                        height: 10
                        design: root.design
                        cut: LithicTokens.cut.toggleTrack
                        fill: root.r.sel
                        CutRect {
                            anchors.right: parent.right
                            anchors.rightMargin: 2
                            anchors.verticalCenter: parent.verticalCenter
                            width: 6
                            height: 6
                            design: root.design
                            cut: LithicTokens.cut.toggleKnob
                            fill: root.r.onSel
                        }
                    }
                    Rectangle {
                        x: 7
                        y: 24
                        width: parent.width * 0.55
                        height: 3
                        radius: 1.5
                        color: Lithic.alpha(root.r.fg, 0.35)
                    }
                    // the one action
                    CutRect {
                        anchors.right: parent.right
                        anchors.rightMargin: 7
                        anchors.bottom: parent.bottom
                        anchors.bottomMargin: 6
                        width: 30
                        height: 11
                        design: root.design
                        cut: LithicTokens.cut.small
                        fill: root.r.sel
                    }
                }
            }
        }

        Column {
            id: words
            x: 12
            anchors.top: shot.bottom
            anchors.topMargin: 10
            width: parent.width - 24
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

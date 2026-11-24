pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// One card a bar style declares in its studio-card.json (StyleCard.js parses
// it), drawn the way the hand-built style cards are: its blocks top to bottom
// inside the card's margins, text as plain text and buttons in a row that
// wraps inside the card. A card that is one editor and nothing else gives that
// editor the whole body, the way the settings rows of Sumi's frame or Obi's
// widgets run edge to edge. A button only reports itself; the page decides
// what running it means.
SettingCard {
    id: card

    property var spec: null
    // editor name -> Component, from the page that owns the editors' state
    property var editors: ({})
    signal act(var button)

    readonly property var blocks: card.spec ? card.spec.blocks : []
    readonly property bool wholeBody: card.blocks.length === 1 && card.blocks[0].kind === "editor"

    // an editor may name its card itself, when the title follows its state
    // (Sumi's widgets card names the rail it edits).
    title: wholeEditor.item && wholeEditor.item.cardTitle ? wholeEditor.item.cardTitle
        : card.spec ? I18n.tr(card.spec.title) : ""
    kana: card.spec ? card.spec.glyph : ""

    Loader {
        id: wholeEditor
        width: parent.width
        active: card.wholeBody
        visible: card.wholeBody
        sourceComponent: card.wholeBody ? card.editors[card.blocks[0].editor] : null
    }

    Item {
        width: parent.width
        visible: !card.wholeBody
        height: card.wholeBody ? 0 : body.height + Tokens.s3 + Tokens.s4
        Column {
            id: body
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
            spacing: Tokens.s3

            Repeater {
                model: card.wholeBody ? [] : card.blocks
                delegate: Loader {
                    id: slot
                    required property var modelData
                    required property int index
                    width: body.width
                    // an inline editor with nothing to show (QS Bar's empty
                    // layout summary) takes no row and no spacing.
                    visible: !slot.item || slot.item.present !== false
                    sourceComponent: slot.modelData.kind === "text" ? textBlock
                        : slot.modelData.kind === "buttons" ? buttonsBlock
                        : card.editors[slot.modelData.editor]

                    Component {
                        id: textBlock
                        Text {
                            width: slot.width
                            textFormat: Text.PlainText
                            text: I18n.tr(slot.modelData.text)
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fBody
                            // a word wider than the card (a long URL) breaks too
                            wrapMode: Text.Wrap
                        }
                    }
                    Component {
                        id: buttonsBlock
                        Flow {
                            width: slot.width
                            spacing: Tokens.s2
                            Repeater {
                                model: slot.modelData.buttons
                                delegate: Btn {
                                    required property var modelData
                                    required property int index
                                    objectName: "studio-card-button-" + slot.index + "-" + index
                                    // a label wider than the card elides
                                    width: Math.min(implicitWidth, slot.width)
                                    text: I18n.tr(modelData.label)
                                    onAct: card.act(modelData)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

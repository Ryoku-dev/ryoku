pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

// One card a bar style declares in its studio-card.json (StyleCard.js parses
// it), drawn the way the hand-built style cards are: its blocks top to bottom,
// text as plain text and buttons in a row that wraps inside the card. A button
// only reports itself; the page decides what running it means.
SettingCard {
    id: card

    property var spec: null
    signal act(var button)

    title: card.spec ? I18n.tr(card.spec.title) : ""
    kana: card.spec ? card.spec.glyph : ""

    Item {
        width: parent.width
        height: body.height + Tokens.s3 + Tokens.s4
        Column {
            id: body
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s3
            spacing: Tokens.s3

            Repeater {
                model: card.spec ? card.spec.blocks : []
                delegate: Loader {
                    id: slot
                    required property var modelData
                    required property int index
                    width: body.width
                    sourceComponent: slot.modelData.kind === "text" ? textBlock : buttonsBlock

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

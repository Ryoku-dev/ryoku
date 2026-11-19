pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import "../../look"

// The dictionary's answer as a row of the list: the word, what kind of word it
// is, the sense, an example when the sense leaves room, and where it comes from.
// row: {title, part, body, example, credit, glyph}. The row opens the entry, so
// the credit reads as the link it is.
Item {
    id: root

    property real s: 1
    property var row: ({})
    property bool selected: false

    readonly property real pad: Math.round(12 * root.s)
    readonly property real textX: root.pad * 2 + Math.round(20 * root.s)
    readonly property bool onTone: root.selected && Look.material
    readonly property color titleInk: root.selected ? Look.plateInk : Lithic.fg
    readonly property color subInk: root.onTone ? Lithic.alpha(Look.plateInk, 0.78) : Lithic.fg2
    readonly property color faintInk: root.onTone ? Lithic.alpha(Look.plateInk, 0.7) : Lithic.fg3
    readonly property color linkInk: root.onTone ? Look.plateInk : Lithic.goldInk
    readonly property string example: String(root.row.example || "")

    // the selection plate takes the card's place; Adwaita's list is boxed already
    Shape {
        anchors.fill: parent
        visible: !Look.boxed
        opacity: root.selected ? 0 : 1
        s: root.s
        cutSize: Look.cutRow
        fill: Look.card
        strokeColor: Look.cardStroke
        strokeWidth: Qt.colorEqual(Look.cardStroke, "transparent") ? 0 : 1
        Behavior on opacity {
            NumberAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    Symbol {
        x: root.pad
        y: column.y + Math.round((head.height - height) / 2)
        width: Math.round(20 * root.s)
        height: width
        s: root.s
        name: String(root.row.glyph || "dictionary")
        size: 18
        ink: root.selected ? Look.plateMark : Lithic.fg2
        chosen: root.selected
    }

    Column {
        id: column
        x: root.textX
        width: root.width - root.textX - root.pad
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(3 * root.s)

        Item {
            id: head
            width: parent.width
            height: word.implicitHeight

            Text {
                id: word
                width: Math.min(implicitWidth, Math.max(0, parent.width - credit.width - root.pad))
                text: String(root.row.title || "")
                color: root.titleInk
                font.family: Look.sans
                font.pixelSize: Math.round(18 * root.s)
                font.weight: Font.Medium
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
            Micro {
                anchors.left: word.right
                anchors.leftMargin: Math.round(10 * root.s)
                anchors.baseline: word.baseline
                width: Math.max(0, credit.x - x - root.pad)
                visible: width > 40 * root.s && text.length > 0
                s: root.s
                text: String(root.row.part || "")
                color: root.faintInk
            }
            Row {
                id: credit
                anchors.right: parent.right
                anchors.verticalCenter: word.verticalCenter
                visible: creditText.text.length > 0
                width: visible ? implicitWidth : 0
                spacing: Math.round(3 * root.s)

                Text {
                    id: creditText
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(root.row.credit || "")
                    color: root.linkInk
                    font.family: Look.sans
                    font.pixelSize: Math.round(11 * root.s)
                    font.underline: true
                    renderType: Text.NativeRendering
                }
                Symbol {
                    anchors.verticalCenter: parent.verticalCenter
                    s: root.s
                    name: "open_in_new"
                    size: 12
                    ink: root.linkInk
                }
            }
        }

        Text {
            id: sense
            width: parent.width
            text: String(root.row.body || "")
            textFormat: Text.PlainText
            color: root.subInk
            font.family: Look.sans
            font.pixelSize: Math.round(13 * root.s)
            lineHeight: Math.round(18 * root.s)
            lineHeightMode: Text.FixedHeight
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            visible: root.example.length > 0 && sense.lineCount < 2
            text: root.example
            textFormat: Text.PlainText
            color: root.faintInk
            font.family: Look.sans
            font.pixelSize: Math.round(12 * root.s)
            lineHeight: Math.round(18 * root.s)
            lineHeightMode: Text.FixedHeight
            elide: Text.ElideRight
            renderType: Text.NativeRendering
        }
    }
}

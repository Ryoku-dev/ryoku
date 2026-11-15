import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "sheet"
import "../registry/registry.js" as Registry
import "../registry/plates.js" as Plates

// the head of a page under the masthead (which says where it lives): its name, what it is
// for, and what it says about the machine now. The page's own actions (Reset page) stand on
// the name's line. At the head's right, where it would otherwise leave the pane's width
// empty, a page with a preview shows it; any other page shows its category's dithered plate
Item {
    id: root

    required property var hub
    required property string pageId
    property string title: ""
    property string lead: ""
    property string stateLine: ""
    property string note: ""
    // the page's figures in a spec strip under the words: [{ label, value, lit }]
    property var facts: []
    default property alias actions: actionRow.data
    // a live illustration of the page's settings (components/previews/), or null
    property Component preview: null

    readonly property var category: {
        var p = Registry.page(root.pageId);
        return p ? Registry.category(p.category) : null;
    }
    readonly property bool previewed: root.preview !== null && root.width >= 660
    readonly property bool plated: !root.previewed && root.width >= 720 && !!root.plate
    readonly property var plate: root.category ? Plates.forCategory(root.category.id) : null
    readonly property real previewWidth: 264
    readonly property real plateWidth: Math.min(288, root.width * 0.32)
    // the page's kanji set vertically beside the plate or preview, while glosses are on
    readonly property bool glossed: UiPreferences.showJapaneseGloss && Registry.gloss(root.pageId) !== ""
        && (root.previewed || root.plated)
    readonly property real glossRoom: root.glossed ? 36 : 0
    readonly property real sideWidth: (root.previewed ? root.previewWidth : root.plated ? root.plateWidth : 0) + root.glossRoom

    height: Math.max(words.height, root.previewed ? previewSlot.y + previewSlot.height : 0,
        root.plated ? plateSlot.height : 0, root.glossed ? tategaki.y + tategaki.height : 0) + 26

    Column {
        id: words
        anchors.left: parent.left
        width: root.sideWidth > 0 ? root.width - root.sideWidth - 28 : root.width

        Item {
            width: words.width
            height: Math.max(titleLabel.height, actionRow.height)

            Label {
                id: titleLabel
                width: Math.min(implicitWidth, parent.width - actionRow.width - 12)
                anchors.verticalCenter: parent.verticalCenter
                text: root.title
                typeRole: "sentence"
                font.pixelSize: 32
                lineHeight: 40
            }
            // the window's close button stands in the masthead, so the actions keep to the
            // line's end
            Row {
                id: actionRow
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6
            }
        }

        Label {
            width: root.sideWidth > 0 ? parent.width : Math.min(parent.width - 44, 720)
            visible: root.lead.length > 0
            topPadding: 4
            text: root.lead
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Label {
            width: root.sideWidth > 0 ? parent.width : Math.min(parent.width - 44, 720)
            visible: root.stateLine.length > 0
            topPadding: 6
            text: root.stateLine
            typeRole: "detail"
            inkRole: "fg"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Row {
            visible: root.note.length > 0
            topPadding: 8
            spacing: 6

            Glyph {
                name: "info"
                size: 14
                ink: Lithic.fg3
                y: 2
            }
            Label {
                width: Math.min(implicitWidth, words.width - 20)
                text: root.note
                typeRole: "caption"
                inkRole: "fg3"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
        }

        Row {
            visible: root.facts.length > 0
            topPadding: 16
            spacing: 18

            Repeater {
                model: root.facts
                delegate: Row {
                    required property var modelData
                    spacing: 6
                    Label {
                        text: modelData.label
                        typeRole: "monoSm"
                        inkRole: "fg3"
                        font.capitalization: Font.AllUppercase
                        font.letterSpacing: 1.2
                    }
                    Label {
                        text: modelData.value
                        typeRole: "monoSm"
                        ink: modelData.lit ? Tone.accent : Lithic.fg
                    }
                }
            }
        }
    }

    Loader {
        id: previewSlot
        active: root.previewed
        anchors.right: parent.right
        y: 44
        width: root.previewWidth
        height: 128
        sourceComponent: root.preview
    }

    Tategaki {
        id: tategaki
        visible: root.glossed
        x: root.width - root.sideWidth + 2
        y: 4
        gloss: Registry.gloss(root.pageId)
        category: root.category ? Registry.gloss(root.category.id) : ""
    }

    Plate {
        id: plateSlot
        visible: root.plated
        anchors.right: parent.right
        y: 4
        width: root.plateWidth
        height: implicitHeight
        plate: root.plated ? root.plate : null
    }
}

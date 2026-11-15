import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
import "../../registry/summaries.js" as Summaries
import "../../registry/numbering.js" as Numbering
import "../../registry/registry.js" as Registry
import "../sheet"

// Settings' sidebar, laid out like an editor's project tree: the app's own title line with the
// tree's one action, the search, then the whole of Settings as a tree (or the search's results
// while there is a query), and Updates at the foot
Rectangle {
    id: root

    required property var hub
    readonly property alias search: searchField
    readonly property alias results: resultsList
    readonly property alias tree: tree

    color: Tone.side

    // the title line: the mark and the app's name, quiet, and collapse-all at its end
    Item {
        id: head
        x: 12
        y: 8
        width: root.width - 20
        height: 40

        RyokuMark {
            id: brand
            anchors.left: parent.left
            anchors.leftMargin: 2
            anchors.verticalCenter: parent.verticalCenter
            width: 20
            height: 18
            ink: Lithic.fg
            accent: Tone.accent
        }
        Label {
            anchors.left: brand.right
            anchors.leftMargin: 10
            anchors.right: collapse.left
            anchors.rightMargin: 6
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Settings") + (UiPreferences.showJapaneseGloss ? "  設定" : "")
            typeRole: "group"
            inkRole: "fg2"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.6
        }
        IconButton {
            id: collapse
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: 30
            height: 30
            glyph: "unfold_less"
            accessibleName: I18n.tr("Close every folder")
            enabled: tree.anyOpen && !root.hub.searching
            opacity: enabled ? 1 : 0.35
            onClicked: tree.collapseAll()
        }
    }

    Search {
        id: searchField
        x: 10
        y: head.y + head.height + 2
        width: root.width - 20
        height: 36
        placeholderText: I18n.tr("Search settings")
        shortcutHint: "/"
        accessibleName: I18n.tr("Search settings")
        Keys.onUpPressed: root.hub.moveCursor(-1)
        Keys.onDownPressed: root.hub.moveCursor(1)
        Keys.onReturnPressed: root.hub.openCursor()
        Keys.onEnterPressed: root.hub.openCursor()
        onTextChanged: root.hub.query = text
        onActiveFocusChanged: {
            // build the index as the field is reached, before the first key
            if (activeFocus)
                root.hub.armSearch();
            else
                root.hub.searchLeftField();
        }
    }

    Item {
        id: body
        anchors.top: searchField.bottom
        anchors.topMargin: 10
        anchors.bottom: foot.top
        anchors.bottomMargin: 4
        x: 4
        width: root.width - 8

        NavTree {
            id: tree
            anchors.fill: parent
            hub: root.hub
            opacity: root.hub.searching ? 0 : 1
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic } }
        }

        ScrollRail {
            flickable: tree
            visible: tree.visible && overflowing
            anchors.top: parent.top
            anchors.bottom: parent.bottom
            anchors.right: parent.right
            anchors.rightMargin: -3
        }

        // the room the tree leaves under itself holds the machine now and its activity map
        Activity {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.bottomMargin: 6
            visible: !root.hub.searching && parent.height - tree.contentHeight >= implicitHeight + 28
        }

        SearchResults {
            id: resultsList
            anchors.fill: parent
            anchors.leftMargin: 6
            anchors.rightMargin: 6
            hub: root.hub
            query: root.hub.searchText
            results: root.hub.results
            nearby: root.hub.nearby
            opacity: root.hub.searching ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Tone.quick; easing.type: Easing.OutCubic } }
            onChosen: entry => root.hub.choose(entry)
        }
    }

    // a hairline between the tree and the foot, once the tree runs under it
    Rectangle {
        anchors.bottom: foot.top
        x: 12
        width: root.width - 24
        height: 1
        color: Tone.line
        opacity: tree.contentHeight > tree.height && !tree.atYEnd ? 1 : 0
        Behavior on opacity { NumberAnimation { duration: Tone.quick } }
    }

    Item {
        id: foot
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 8
        x: 4
        width: root.width - 8
        height: updatesRow.height

        NavRow {
            id: updatesRow
            width: parent.width
            folder: false
            glyph: "system_update"
            numeral: Numbering.categoryNumber("updates")
            gloss: Registry.gloss("updates")
            text: I18n.tr("Updates")
            selected: root.hub.pageId === "updates"
            notch: Updates.available
            trailing: Updates.available ? "" : (Updates.currentName || Updates.currentVersion || "")
            implicitHeight: 34
            wash: selected
            onClicked: root.hub.navigate("updates", "")
        }
    }

    // a peek at the page under the pointer: what it holds now, read live, beside its line
    Item {
        id: peek
        readonly property bool on: tree.peekShown && !!tree.peekRow && !!tree.peekPage && !root.hub.searching
        readonly property real rowY: {
            tree.contentY;
            tree.peekRow;
            return tree.peekRow ? tree.peekRow.mapToItem(root, 0, 0).y + tree.peekRow.height / 2 : 0;
        }
        x: root.width + (peek.on ? 4 : -4)
        y: Math.max(8, Math.min(root.height - card.height - 8, peek.rowY - card.height / 2))
        // as wide as its words, up to a comfortable line
        width: Math.min(300, Math.max(heading.implicitWidth, summaryLabel.implicitWidth) + 28)
        height: card.height
        opacity: peek.on ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: Tone.quick + 40; easing.type: Easing.OutCubic } }
        Behavior on x { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
        Behavior on y {
            enabled: peek.opacity > 0.5
            NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic }
        }

        CutRect {
            x: 2
            y: 4
            width: card.width
            height: card.height
            cut: Tone.cardCut
            fill: Lithic.alpha("#000000", Lithic.dark ? 0.5 : 0.12)
        }
        // a popover's plane: the paper itself in Lithic, drawn by its hairline
        CutRect {
            id: card
            width: parent.width
            height: words.implicitHeight + 24
            cut: Tone.cardCut
            fill: Tone.flat ? Lithic.ground : Lithic.stone4
            strokeWidth: 1
            strokeColor: Lithic.vein

            Column {
                id: words
                x: 14
                y: 12
                width: parent.width - 28
                spacing: 4

                Row {
                    id: heading
                    spacing: 8
                    Glyph {
                        anchors.verticalCenter: parent.verticalCenter
                        name: tree.peekPage ? tree.peekPage.glyph : ""
                        size: 16
                        ink: Tone.accent
                    }
                    Label {
                        anchors.verticalCenter: parent.verticalCenter
                        text: tree.peekPage ? I18n.tr(tree.peekPage.title) : ""
                        typeRole: "label"
                        font.weight: Font.DemiBold
                    }
                }
                Label {
                    id: summaryLabel
                    width: Math.min(implicitWidth, parent.width)
                    text: !tree.peekPage ? ""
                        : tree.peekPage.folder ? Summaries.categorySentence(tree.peekPage.id, root.hub.summaries)
                        : Summaries.pageSummary(tree.peekPage.id, root.hub.summaries)
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
            }
        }
    }
}

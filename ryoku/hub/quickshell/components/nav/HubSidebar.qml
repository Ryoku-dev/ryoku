import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
import "../../registry/summaries.js" as Summaries

// Settings' sidebar: Home at the head, the search under it, then the whole of Settings as a
// tree (or the search's results while there is a query), and Updates at the foot
Rectangle {
    id: root

    required property var hub
    readonly property alias search: searchField
    readonly property alias results: resultsList
    readonly property alias tree: tree

    color: Tone.side

    T.AbstractButton {
        id: home
        x: 8
        y: 10
        width: root.width - 16
        height: 44
        hoverEnabled: true
        focusPolicy: Qt.TabFocus
        Accessible.role: Accessible.Button
        Accessible.name: I18n.tr("Home")
        readonly property bool here: root.hub.pageId === ""
        onClicked: root.hub.goHome()

        background: Rectangle {
            radius: Tone.radiusSmall
            color: home.here ? Tone.accentSoft : home.down ? Tone.press : Tone.hover
            opacity: home.here || home.hovered || home.visualFocus ? 1 : 0
            border.width: home.visualFocus ? 1 : 0
            border.color: Lithic.fg2
            Behavior on opacity { NumberAnimation { duration: Tone.quick } }
        }

        contentItem: Item {
            RyokuMark {
                id: brand
                x: 8
                anchors.verticalCenter: parent.verticalCenter
                width: 26
                height: 23
                ink: Lithic.fg
                accent: Tone.accent
                rotation: home.hovered && !Lithic.reduceMotion ? -8 : 0
                Behavior on rotation {
                    NumberAnimation { duration: Tone.move; easing.type: Easing.OutBack; easing.overshoot: 2.2 }
                }
            }
            Column {
                anchors.left: brand.right
                anchors.leftMargin: 12
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                Label {
                    width: parent.width
                    text: I18n.tr("Settings")
                    typeRole: "row"
                    font.weight: Font.Bold
                }
                Label {
                    width: parent.width
                    text: home.here ? I18n.tr("Home") : I18n.tr("Back to Home")
                    typeRole: "caption"
                    inkRole: "fg3"
                }
            }
        }
    }

    Search {
        id: searchField
        x: 12
        y: home.y + home.height + 10
        width: root.width - 24
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

        Rectangle {
            x: 2
            y: 4
            width: card.width
            height: card.height
            radius: Tone.radius
            color: Lithic.alpha("#000000", Lithic.dark ? 0.45 : 0.12)
        }
        // the point toward the line it tells about
        Rectangle {
            x: -4
            y: Math.max(10, Math.min(card.height - 20, peek.rowY - peek.y - 5))
            width: 10
            height: 10
            rotation: 45
            color: Tone.card
            border.width: 1
            border.color: Tone.line
        }
        Rectangle {
            id: card
            width: parent.width
            height: words.implicitHeight + 24
            radius: Tone.radius
            color: Tone.card
            border.width: 1
            border.color: Tone.line

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

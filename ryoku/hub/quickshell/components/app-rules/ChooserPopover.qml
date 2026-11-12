import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a searchable list of apps or windows under the plaque that opened it
Popover {
    id: pop

    // [{ label, id }]; id shows at the right in mono
    property var entries: []
    property string emptyText: ""
    property string query: ""
    readonly property var shown: {
        var q = pop.query.trim().toLowerCase();
        if (q === "")
            return pop.entries;
        return pop.entries.filter(e => (e.label + " " + e.id).toLowerCase().indexOf(q) >= 0);
    }

    signal chosen(var entry)

    function pick(index) {
        var e = pop.shown[index];
        if (!e)
            return;
        pop.close();
        pop.chosen(e);
    }

    onAboutToShow: {
        search.clear();
        pop.query = "";
        menuFlick.contentY = 0;
    }
    onOpened: search.forceActiveFocus()

    CutRect {
        width: LithicTokens.size.menuWidthMax
        implicitWidth: width
        implicitHeight: body.implicitHeight + LithicTokens.space.s2 * 2
        height: implicitHeight
        fill: Lithic.stone4
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
        cut: LithicTokens.cut.menu

        Column {
            id: body
            x: LithicTokens.space.s2
            y: LithicTokens.space.s2
            width: parent.width - LithicTokens.space.s2 * 2
            spacing: LithicTokens.space.s2

            Search {
                id: search
                width: parent.width
                placeholderText: I18n.tr("Search")
                onTextChanged: pop.query = text
                onAccepted: pop.pick(0)
                Keys.onPressed: event => {
                    if (event.key === Qt.Key_Escape && search.text === "") {
                        pop.close();
                        event.accepted = true;
                    } else if (event.key === Qt.Key_Down && pop.shown.length > 0) {
                        menu.forceActiveFocus();
                        event.accepted = true;
                    }
                }
            }

            Label {
                width: parent.width
                visible: pop.shown.length === 0
                topPadding: LithicTokens.space.s2
                bottomPadding: LithicTokens.space.s2
                text: pop.entries.length === 0 ? pop.emptyText : I18n.tr("Nothing matches")
                typeRole: "body"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }

            Flickable {
                id: menuFlick
                visible: pop.shown.length > 0
                width: parent.width
                height: Math.min(menu.implicitHeight, LithicTokens.size.menuItem * 8)
                contentWidth: width
                contentHeight: menu.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                clip: true

                Menu {
                    id: menu
                    width: menuFlick.width
                    menuWidth: menuFlick.width
                    background: null
                    items: pop.shown.map(e => ({ label: e.label, shortcut: e.id }))
                    onTriggered: index => pop.pick(index)
                    onFocusedIndexChanged: {
                        var top = menu.topPadding + menu.focusedIndex * LithicTokens.size.menuItem;
                        if (top < menuFlick.contentY)
                            menuFlick.contentY = top;
                        else if (top + LithicTokens.size.menuItem > menuFlick.contentY + menuFlick.height)
                            menuFlick.contentY = top + LithicTokens.size.menuItem - menuFlick.height;
                    }
                    Keys.onEscapePressed: pop.close()
                }
            }
        }
    }
}

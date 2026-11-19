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
        menu.focusedIndex = -1;
    }
    onOpened: search.forceActiveFocus()

    CutRect {
        width: pop.availableWidth
        implicitWidth: LithicTokens.size.menuWidthMax
        implicitHeight: search.implicitHeight + LithicTokens.space.s2
                        + (menu.visible ? menu.implicitHeight : empty.implicitHeight)
                        + LithicTokens.space.s2 * 2
        height: pop.availableHeight
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
            height: parent.height - LithicTokens.space.s2 * 2

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
                id: empty
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

            Menu {
                id: menu
                visible: pop.shown.length > 0
                width: parent.width
                height: Math.min(implicitHeight, Math.max(0,
                    body.height - search.height - body.spacing))
                menuWidth: parent.width
                background: null
                items: pop.shown.map(e => ({ label: e.label, shortcut: e.id }))
                onTriggered: index => pop.pick(index)
                Keys.onEscapePressed: pop.close()
            }
        }
    }
}

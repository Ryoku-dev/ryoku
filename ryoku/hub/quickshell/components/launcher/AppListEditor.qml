import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// a list of installed apps kept by id, edited through Changes; adding picks an app by name
Column {
    id: root

    required property var bind
    required property string changeLabel
    property string addText: ""
    property string emptyText: ""
    property bool reorderable: false
    property string accessibleName: ""
    // one step back to an empty list, offered while it holds more than one app
    property string clearText: ""
    // apps another list holds, which this one does not offer
    property var excluded: []

    readonly property var ids: {
        var v = Changes.value(root.bind);
        return v && v.length !== undefined ? Array.prototype.slice.call(v).map(String) : [];
    }

    function entry(id) {
        return DesktopEntries.byId(id);
    }

    function nameOf(id) {
        var e = root.entry(id);
        return e && e.name ? e.name : id;
    }

    function commit(next) {
        Changes.set(root.bind, next, root.changeLabel, "launcher");
    }

    // every app the launcher would list; one already in this list stays reachable even when hidden from menus
    function catalog(exclude) {
        var src = DesktopEntries.applications ? DesktopEntries.applications.values : [];
        var out = [];
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || !e.id || exclude.indexOf(e.id) >= 0 || e.noDisplay)
                continue;
            out.push({ id: e.id, name: e.name || e.id });
        }
        out.sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
        return out;
    }

    spacing: LithicTokens.space.s2

    ListEditor {
        id: list
        width: parent.width
        model: root.ids.map(id => ({ id: id }))
        titleOf: item => root.nameOf(item.id)
        detailOf: item => root.entry(item.id) ? item.id : I18n.tr("%1 is no longer installed").arg(item.id)
        addText: root.addText
        doneText: I18n.tr("Add")
        emptyText: root.emptyText
        reorderable: root.reorderable
        accessibleName: root.accessibleName
        onAdded: item => root.commit(root.ids.concat([item.id]))
        onEdited: (index, item) => {
            var next = root.ids.slice();
            next[index] = item.id;
            root.commit(next.filter((id, i) => next.indexOf(id) === i));
        }
        onRemoved: index => {
            var next = root.ids.slice();
            next.splice(index, 1);
            root.commit(next);
        }
        onMoved: (from, to) => {
            var next = root.ids.slice();
            var moved = next.splice(from, 1)[0];
            next.splice(to, 0, moved);
            root.commit(next);
        }

        editor: Component {
            Column {
                id: chooser
                property var item: ({})
                property string picked: chooser.item && chooser.item.id ? chooser.item.id : ""
                // the pick counts while the search still shows it; a search that leaves one app picks it, so Enter adds it
                readonly property string chosen: chooser.apps.some(a => a.id === chooser.picked) ? chooser.picked
                    : chooser.apps.length === 1 ? chooser.apps[0].id : ""
                readonly property bool valid: chooser.chosen.length > 0
                readonly property var apps: {
                    var q = search.text.trim().toLowerCase();
                    var all = root.catalog(root.ids.filter(id => id !== chooser.item.id).concat(root.excluded));
                    return q.length === 0 ? all : all.filter(a => (a.name + " " + a.id).toLowerCase().indexOf(q) >= 0);
                }
                function value() {
                    return { id: chooser.chosen };
                }
                // arrows move the pick from the search or the list, so Enter adds what is marked
                function step(delta) {
                    if (chooser.apps.length === 0)
                        return;
                    var at = chooser.apps.findIndex(a => a.id === chooser.chosen);
                    at = at < 0 ? (delta > 0 ? 0 : chooser.apps.length - 1) : Math.max(0, Math.min(chooser.apps.length - 1, at + delta));
                    chooser.picked = chooser.apps[at].id;
                    appList.listView.positionViewAtIndex(at, ListView.Contain);
                }
                spacing: LithicTokens.space.s2

                Search {
                    id: search
                    width: parent.width
                    placeholderText: I18n.tr("Search apps")
                    accessibleName: I18n.tr("Search apps")
                    Keys.onDownPressed: chooser.step(1)
                    Keys.onUpPressed: chooser.step(-1)
                }

                RowList {
                    id: appList
                    width: parent.width
                    visible: chooser.apps.length > 0
                    height: Math.min(264, chooser.apps.length * 52)
                    model: chooser.apps
                    currentIndex: chooser.apps.findIndex(a => a.id === chooser.chosen)
                    Accessible.name: I18n.tr("Apps")
                    Keys.onDownPressed: chooser.step(1)
                    Keys.onUpPressed: chooser.step(-1)
                    delegate: Tablet {
                        required property var modelData
                        required property int index
                        width: ListView.view.width
                        compact: true
                        title: modelData.name
                        facts: modelData.id
                        claim: ""
                        selected: modelData.id === chooser.chosen
                        onClicked: chooser.picked = modelData.id
                        onPrimaryAction: chooser.picked = modelData.id
                    }
                }

                Label {
                    width: parent.width
                    visible: chooser.apps.length === 0
                    text: I18n.tr("No app matches")
                    typeRole: "detail"
                    inkRole: "fg2"
                }
            }
        }
    }

    Plaque {
        id: clear
        visible: root.clearText.length > 0 && root.ids.length > 1
        kind: "quiet"
        small: true
        glyph: "clear_all"
        text: root.clearText
        // the plaque goes with the list it cleared, so focus moves back to the add plaque before it
        onClicked: {
            var back = clear.activeFocus ? clear.nextItemInFocusChain(false) : null;
            var reason = clear.visualFocus ? Qt.TabFocusReason : Qt.OtherFocusReason;
            root.commit([]);
            if (back)
                back.forceActiveFocus(reason);
        }
    }
}

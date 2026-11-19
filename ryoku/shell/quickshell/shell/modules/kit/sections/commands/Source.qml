pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "commands.js" as Logic

Item {
    id: root
    property var memory: null
    property var downloads: null
    property bool active: false
    property bool current: false
    property bool searching: false
    property bool started: false
    readonly property var catalogue: loader.item ? loader.item.catalogue : null
    readonly property var store: loader.item ? loader.item.store : null
    readonly property var entries: root.catalogue ? root.catalogue.entries.concat(root.store.commands) : []
    readonly property int revision: (root.catalogue ? root.catalogue.revision : 0) + (root.store ? root.store.revision : 0)
    readonly property int saved: root.store ? root.store.commands.length + root.store.favouriteIds.filter(id => !root.store.commands.some(entry => entry.id === id)).length : root.memory ? Number(root.memory.get("commands.saved", 0)) : 0
    readonly property var figure: ({ text: root.saved > 0 ? I18n.tr("%1 saved").arg(root.saved) : "", value: -1, tone: "" })
    readonly property var lit: null
    function seen() {}
    function searchAll(query) {
        const observed = root.revision;
        if (!root.catalogue || !root.catalogue.ready) return [];
        const groups = ({});
        for (const library of root.catalogue.libraries)
            for (const group of library.groups) groups[group.id] = group.label;
        return root.entries.filter(entry => Logic.matches(entry, query, function(text) { return I18n.tr(text); }, groups[entry.group] || "")).map(entry => ({
            id: entry.id, glyph: "terminal", title: entry.source === "user" ? entry.title : I18n.tr(entry.title),
            subtitle: entry.source === "user" ? entry.description : I18n.tr(entry.description),
            trail: entry.command, trailMono: true,
            open: { page: entry.library, query: "", args: { command: entry.id } }
        }));
    }
    onCurrentChanged: { if (current) started = true; }
    onActiveChanged: if (!active) started = false
    onSearchingChanged: { if (searching) started = true; }
    onSavedChanged: { if (root.memory && root.store) root.memory.set("commands.saved", root.saved); }
    Loader {
        id: loader
        active: root.active && (root.started || root.current || root.searching)
        sourceComponent: Item {
            property alias catalogue: catalogue
            property alias store: store
            CommandCatalog { id: catalogue; Component.onCompleted: start() }
            CommandStore { id: store; catalogue: catalogue }
        }
    }
}

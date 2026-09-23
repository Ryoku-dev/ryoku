import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../../../../utils/fuzzy.js" as Fuzzy
import "catalog.js" as Catalog
import ".."

// System-action provider: the "/" prefix lists fire-and-forget commands (lock,
// wallpaper, screenshot, night light, media keys, settings...) from the catalog,
// fuzzy-filtered. Each action runs its catalog `exec` argv detached. The active
// category tab (set by the view) narrows the list.
Provider {
    id: actions

    providerId: "actions"
    prefix: "/"
    defaultProvider: false

    property string activeCategory: "All"
    onActiveCategoryChanged: Dispatcher.notifyAsync()


    function rowFor(entry) {
        return {
            id: "action:" + entry.id,
            title: I18n.tr(entry.name),
            subtitle: "",
            icon: "",
            type: entry.category,
            score: 0,
            category: entry.category,
            actions: [{
                id: "run",
                name: I18n.tr("Run"),
                icon: "",
                execute: function () { Spawn.run(entry.exec); }
            }]
        };
    }

    function query(text) {
        var pool = Catalog.CATALOG.filter(function (a) {
            // an action gated on a window-manager capability is dropped on a
            // compositor that lacks it (its helper is not even installed there).
            if (a.caps && Wm.caps[a.caps] !== true)
                return false;
            return actions.activeCategory === "All" || a.category === actions.activeCategory;
        });
        var q = (text || "").trim().toLowerCase();
        var rows = [];
        for (var i = 0; i < pool.length; i++) {
            if (q.length === 0 || Fuzzy.score({ name: pool[i].name, keywords: [pool[i].category] }, q) < 99)
                rows.push(rowFor(pool[i]));
        }
        return rows;
    }

    Component.onCompleted: Dispatcher.register(actions);
}

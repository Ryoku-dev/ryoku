pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../../lib/memory.js" as Memory
import "../../lib/search.js" as Search

// What Typing test knows while Kit is open: the best on record, read from
// Kit's memory (the history lives there).
Item {
    id: root

    property var memory: null
    property var downloads: null
    property bool current: false
    property bool searching: false

    readonly property int revision: 0
    readonly property int best: root.memory ? Memory.typingBest(root.memory.values) : 0
    readonly property var figure: ({ text: root.best > 0 ? I18n.tr("Best %1 wpm").arg(root.best) : "", value: -1, tone: "" })
    readonly property var lit: null

    function seen() {}

    function searchAll(query) {
        const pages = [
            { id: "test", title: I18n.tr("Typing test") },
            { id: "history", title: I18n.tr("History"), texts: [I18n.tr("Typing test")] },
            { id: "stats", title: I18n.tr("Stats"), texts: [I18n.tr("Typing test")] },
            { id: "settings", title: I18n.tr("Settings"), texts: [I18n.tr("Typing test")] }
        ];
        return Search.listRows(pages, query, function(page) {
            return { id: page.id, glyph: "speed", title: page.title, subtitle: page.id === "test" ? "" : I18n.tr("Typing test"), texts: page.texts || [], open: { page: page.id } };
        });
    }
}

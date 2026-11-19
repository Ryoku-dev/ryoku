pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../../lib/search.js" as Search

Item {
    id: root

    property var memory: null
    property var downloads: null
    property bool current: false
    property bool searching: false

    readonly property int revision: 0
    readonly property var figure: ({ text: I18n.tr("%1 tools").arg(registry.tools.length), value: -1, tone: "" })
    readonly property var lit: null

    function seen() {}

    function searchAll(query) {
        return Search.listRows(registry.tools, query, function(tool) {
            return { id: tool.id, glyph: "build", title: tool.name, subtitle: tool.description, open: { page: tool.id } };
        });
    }

    DevToolsRegistry { id: registry }
}

pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../lib/memory.js" as Memory
import "shortcuts.js" as Shortcuts

Item {
    id: root

    property var memory: null
    property var downloads: null
    property bool active: false
    property bool current: false
    property bool searching: false
    property var categories: []
    property bool loaded: false
    property int revision: 0
    readonly property int counted: root.categories.reduce((n, category) => n + category.binds.length, 0)
    readonly property int shown: root.loaded ? root.counted : (root.memory ? Memory.count(root.memory.values, "keys") : 0)
    readonly property var figure: ({ text: root.shown > 0 ? I18n.tr("%1 shortcuts").arg(root.shown) : "", value: -1, tone: "" })
    readonly property var lit: null
    readonly property bool wanted: root.current || root.searching

    function seen() {}
    function searchAll(query) {
        return Shortcuts.groups(root.categories, query, text => I18n.tr(text)).reduce(function(out, group) {
            return out.concat(group.rows.map(function(row) {
                return {
                    id: row.id, size: "one", glyph: "keyboard",
                    title: I18n.tr(row.bind.desc || ""), subtitle: I18n.tr(row.bind.unhonored || group.name),
                    trail: (row.bind.keys || []).join(" + "), trailMono: true,
                    open: { page: "category-" + row.category, query: I18n.tr(row.bind.desc || ""), args: { shortcut: row.id } }
                };
            }));
        }, []);
    }
    function load() {
        if (root.active && root.wanted && !root.loaded && !legend.running)
            legend.running = true;
    }
    onWantedChanged: root.load()
    Component.onCompleted: root.load()
    onActiveChanged: if (!root.active) legend.running = false; else root.load()

    Process {
        id: legend
        command: ["ryoku-hub", "keybinds"]
        environment: Spawn.env
        stdout: StdioCollector {
            onStreamFinished: {
                if (!root.active)
                    return;
                var cats = [];
                try {
                    var parsed = JSON.parse(this.text);
                    cats = (parsed.categories || []).map(function(category) {
                        return { name: category.name, binds: (category.binds || []).filter(bind => !bind.disabled) };
                    }).filter(category => category.binds.length > 0);
                } catch (error) {
                    cats = [];
                }
                root.categories = cats;
                root.loaded = true;
                root.revision++;
                if (root.memory)
                    root.memory.set(Memory.key("keys", "count"), root.counted);
            }
        }
    }
}

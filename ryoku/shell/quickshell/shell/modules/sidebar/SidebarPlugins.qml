pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Item {
    id: root

    property var plugins: []

    readonly property string shellDir: Quickshell.env("RYOKU_SHELL_DIR")
    readonly property string discoverScript: shellDir !== ""
        ? shellDir + "/quickshell/plugins/discover.sh"
        : (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
            + "/quickshell/plugins/discover.sh"
    readonly property string stateHome: Quickshell.env("XDG_STATE_HOME")
        || (Quickshell.env("HOME") + "/.local/state")

    width: 0
    height: 0

    function reload() {
        discover.running = false;
        discover.running = true;
    }

    function syncPlugins(all) {
        var next = [];
        for (var i = 0; i < all.length; ++i) {
            var plugin = all[i];
            var placement = plugin && plugin.placement ? plugin.placement : null;
            if (placement && placement.enabled === true && placement.host === "sidebarCard")
                next.push(plugin);
        }
        root.plugins = next;
    }

    function cards(side) {
        var result = [];
        for (var i = 0; i < root.plugins.length; ++i) {
            var plugin = root.plugins[i];
            var placement = plugin.placement || {};
            var card = placement.sidebarCard || {};
            var cardSide = card.side === "right" ? "right" : "left";
            if (cardSide !== side)
                continue;
            var tab = typeof card.tab === "string" && card.tab.trim() !== ""
                ? card.tab.trim() : "Plugins";
            var manifest = plugin.manifest || {};
            var defaults = manifest.defaults || {};
            result.push({
                id: plugin.id,
                label: card.label || manifest.name || plugin.id,
                glyph: card.glyph || defaults.icon || "extension",
                order: isFinite(Number(card.order)) ? Number(card.order) : 0,
                tab: tab,
                dir: plugin.dir,
                entry: plugin
            });
        }
        result.sort(function(a, b) {
            return a.order === b.order ? a.id.localeCompare(b.id) : a.order - b.order;
        });
        return result;
    }

    Process {
        id: discover
        command: ["bash", root.discoverScript, "--all"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var parsed = [];
                try { parsed = JSON.parse(text || "[]"); } catch (error) { parsed = []; }
                root.syncPlugins(Array.isArray(parsed) ? parsed : []);
            }
        }
    }

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config"))
            + "/ryoku/plugins.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: root.reload()
    }

    FileView {
        path: root.stateHome + "/ryoku/store/revision.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: root.reload()
    }
}

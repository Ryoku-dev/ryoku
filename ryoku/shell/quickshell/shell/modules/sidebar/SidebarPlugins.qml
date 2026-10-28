pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Item {
    id: root

    property bool active: false
    property bool discovered: false
    property var plugins: []
    property var availablePlugins: []

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
        root.discovered = false;
    }

    function normalizedEntry(plugin) {
        if (!plugin)
            return plugin;
        const result = {};
        for (const key in plugin)
            result[key] = plugin[key];
        const placement = {};
        const sourcePlacement = plugin.placement || {};
        for (const placementKey in sourcePlacement)
            placement[placementKey] = sourcePlacement[placementKey];
        const sidebarCard = {};
        const sourceCard = sourcePlacement.sidebarCard || {};
        for (const cardKey in sourceCard)
            sidebarCard[cardKey] = sourceCard[cardKey];
        sidebarCard.side = "left";
        placement.sidebarCard = sidebarCard;
        result.placement = placement;
        return result;
    }

    function syncPlugins(all) {
        root.availablePlugins = all.filter(plugin => plugin && plugin.placement
            && plugin.placement.host === "sidebarCard").map(root.normalizedEntry);
        const next = [];
        for (let i = 0; i < all.length; ++i) {
            const plugin = all[i];
            const placement = plugin && plugin.placement ? plugin.placement : null;
            if (placement && placement.enabled === true && placement.host === "sidebarCard")
                next.push(root.normalizedEntry(plugin));
        }
        root.plugins = next;
    }

    function cards() {
        const result = [];
        for (let i = 0; i < root.plugins.length; ++i) {
            const plugin = root.plugins[i];
            const placement = plugin.placement || {};
            const card = placement.sidebarCard || {};
            const tab = typeof card.tab === "string" && card.tab.trim() !== ""
                ? card.tab.trim() : "Plugins";
            const manifest = plugin.manifest || {};
            const defaults = manifest.defaults || {};
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
        running: root.active && !root.discovered
        stdout: StdioCollector {
            onStreamFinished: {
                var parsed = [];
                try { parsed = JSON.parse(text || "[]"); } catch (error) { parsed = []; }
                root.syncPlugins(Array.isArray(parsed) ? parsed : []);
                root.discovered = true;
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

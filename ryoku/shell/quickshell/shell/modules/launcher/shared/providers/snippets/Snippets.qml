import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../../../../utils/fuzzy.js" as Fuzzy
import "../../lib/providerids.js" as ProviderIds
import "placeholders.js" as Placeholders
import ".."

// Snippets + quicklinks provider. Snippets expand dynamic placeholders ({date},
// {clipboard}, {selection}, {cursor}) and copy to the clipboard; quicklinks open
// a URL with {query} substituted. Both are read from JSON under ~/.config/ryoku,
// fuzzy-matched by keyword/name. Default-ranked below apps.
Provider {
    id: snippets

    providerId: "snippets"

    readonly property string dir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    property var snippetList: []
    property var quicklinkList: []
    onSnippetListChanged: Dispatcher.notifyAsync()
    onQuicklinkListChanged: Dispatcher.notifyAsync()

    FileView {
        id: snippetFile
        path: snippets.dir + "/launcher-snippets.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: snippets.snippetList = snippets.parseList(snippetFile.text())
        onLoadFailed: snippets.snippetList = []
    }
    FileView {
        id: quicklinkFile
        path: snippets.dir + "/launcher-quicklinks.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: snippets.quicklinkList = snippets.parseList(quicklinkFile.text())
        onLoadFailed: snippets.quicklinkList = []
    }

    function parseList(raw) {
        try {
            var v = raw && raw.length ? JSON.parse(raw) : [];
            return Array.isArray(v) ? v : [];
        } catch (e) {
            return [];
        }
    }

    function context() {
        return { now: new Date(), clipboard: Quickshell.clipboardText, selection: "" };
    }

    function snippetRow(entry) {
        return {
            id: ProviderIds.snippetRowId(entry),
            title: entry.name || entry.keyword || I18n.tr("Snippet"),
            subtitle: I18n.tr("Snippet"),
            icon: "",
            type: "Snippet",
            score: 40,
            actions: [{
                id: "copy",
                name: I18n.tr("Copy"),
                icon: "",
                execute: function () {
                    Quickshell.clipboardText = Placeholders.expand(entry.body || "", snippets.context()).text;
                }
            }]
        };
    }

    function quicklinkRow(entry, text) {
        return {
            id: ProviderIds.quicklinkRowId(entry),
            title: entry.name || entry.url,
            subtitle: I18n.tr("Quicklink"),
            icon: "",
            type: "Quicklink",
            score: 40,
            actions: [{
                id: "open",
                name: I18n.tr("Open"),
                icon: "",
                execute: function () {
                    var url = String(entry.url || "").replace(/\{query\}/g, encodeURIComponent(text));
                    Qt.openUrlExternally(url);
                }
            }]
        };
    }

    function matchable(entry) {
        return { name: entry.name || entry.keyword || "", keywords: entry.keywords || [] };
    }

    function query(text) {
        if (!text || text.length === 0)
            return [];
        var rows = [];
        var q = text.toLowerCase();
        for (var i = 0; i < snippets.snippetList.length; i++)
            if (Fuzzy.score(snippets.matchable(snippets.snippetList[i]), q) < 99)
                rows.push(snippets.snippetRow(snippets.snippetList[i]));
        for (var j = 0; j < snippets.quicklinkList.length; j++) {
            var link = snippets.quicklinkList[j];
            // "name rest..." routes the rest into the {query} placeholder, so
            // a templated link can carry arbitrary search text; the plain
            // fuzzy path alone could never match name + extra words.
            var kw = String(link.keyword || link.name || "").toLowerCase();
            if (kw.length > 0 && q.indexOf(kw + " ") === 0) {
                rows.push(snippets.quicklinkRow(link, text.slice(kw.length + 1).trim()));
                continue;
            }
            if (Fuzzy.score(snippets.matchable(link), q) < 99)
                rows.push(snippets.quicklinkRow(link, text));
        }
        return ProviderIds.dedupeRows(rows);
    }

    Component.onCompleted: Dispatcher.register(snippets);
}

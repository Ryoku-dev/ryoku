pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// Cross-surface presentation preferences from the daemon-owned shell store.
// Readers watch the file; the daemon remains its only writer.
Singleton {
    id: root

    property bool showJapaneseGloss: true

    function containsJapanese(value) {
        return /[\u3040-\u30ff\u3400-\u9fff]/.test(String(value || ""));
    }

    function text(value, fallback) {
        const source = String(value || "");
        return root.showJapaneseGloss || !root.containsJapanese(source)
            ? source : String(fallback || "");
    }

    function shippedText(value, shippedValue, fallback) {
        const source = String(value || "");
        return !root.showJapaneseGloss && source === String(shippedValue || "")
            ? String(fallback || "") : source;
    }

    function brandMark(value) {
        const source = String(value || "") || "力";
        return !root.showJapaneseGloss && source === "力" ? "R" : source;
    }

    function load() {
        var data = {};
        try { data = JSON.parse(store.text()) || {}; } catch (e) {}
        root.showJapaneseGloss = data.showJapaneseGloss !== false;
    }

    FileView {
        id: store
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onLoaded: root.load()
        onFileChanged: reload()
        onLoadFailed: root.showJapaneseGloss = true
    }
}

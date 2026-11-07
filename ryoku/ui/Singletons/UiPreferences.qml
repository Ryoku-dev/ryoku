pragma Singleton
import QtQuick
import Quickshell
import "buspath.js" as BusPath

// Cross-surface presentation preferences from the daemon-owned shell store,
// read off the shared settings frame.
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
        root.showJapaneseGloss = ShellSettings.get("showJapaneseGloss", true) !== false;
    }

    Connections {
        target: ShellSettings
        function onChanged(paths) {
            if (BusPath.touches("showJapaneseGloss", paths))
                root.load();
        }
    }

    Component.onCompleted: root.load()
}

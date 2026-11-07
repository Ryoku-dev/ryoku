import QtQuick
import "Singletons"
import "lib/buspath.js" as BusPath

// One shell setting as a bindable value. `value` moves only when a change
// reaches its path (the key itself, inside it, or an ancestor), so a binding on
// bar.style sleeps through every unrelated settings patch.
QtObject {
    id: setting

    property var path: ""
    property var fallback: undefined
    readonly property var value: setting._value

    // Bound only until completion, so a reader during construction already
    // sees the value; then assigned, since a binding would re-run on every
    // settings frame.
    property var _value: ShellSettings.get(setting.path, setting.fallback)

    function _refresh() {
        const next = ShellSettings.get(setting.path, setting.fallback);
        if (next === setting._value && (next === null || typeof next !== "object"))
            return;
        setting._value = next;
    }

    onPathChanged: setting._refresh()
    onFallbackChanged: setting._refresh()
    Component.onCompleted: setting._value = ShellSettings.get(setting.path, setting.fallback)

    property Connections _watch: Connections {
        target: ShellSettings
        function onChanged(paths) {
            if (BusPath.touches(setting.path, paths))
                setting._refresh();
        }
    }
}

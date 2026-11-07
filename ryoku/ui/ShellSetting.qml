import QtQuick
import Ryoku.Bus
import Ryoku.Ui.Singletons

// One shell setting as a bindable value. `value` moves only when a change
// reaches its path (the key itself, inside it, or an ancestor) and the value
// there differs, so a binding on bar.style sleeps through every unrelated
// settings patch. The path check and the comparison run natively (BusValue).
QtObject {
    id: setting

    property var path: ""
    property var fallback: undefined
    readonly property var value: setting._value.value

    property BusValue _value: BusValue {
        topic: "settings"
        path: setting.path
        fallback: setting.fallback
    }
    // ShellSettings seeds the settings topic from shell.json before the daemon
    // answers; reading it makes sure it exists.
    readonly property bool _seeded: ShellSettings.ready
}

import QtQuick
import Ryoku.Bus
import Ryoku.Ui.Singletons

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
    // reading ShellSettings makes it seed the topic from shell.json before the daemon answers
    readonly property bool _seeded: ShellSettings.ready
}

import QtQuick
import Lithic

// a setting row the page draws itself; it registers with its SettingsPage so
// search can land on it and spotlight it like a schema row
SettingRow {
    id: own

    required property Item host
    property string rowId: ""
    property string registered: ""
    readonly property Item settingRow: own

    width: parent ? parent.width : 0
    hoverEnabled: !!own.host && !own.host.pointerQuiet
    highlighted: !!own.host && own.rowId !== "" && own.host.highlightId === own.rowId

    function register() {
        if (!own.host)
            return;
        if (own.registered !== "" && own.host.rowItems[own.registered] === own)
            delete own.host.rowItems[own.registered];
        own.registered = own.rowId;
        if (own.rowId !== "")
            own.host.rowItems[own.rowId] = own;
    }

    onRowIdChanged: own.register()
    onHostChanged: own.register()
    Component.onCompleted: own.register()
    Component.onDestruction: {
        if (own.host && own.registered !== "" && own.host.rowItems[own.registered] === own)
            delete own.host.rowItems[own.registered];
    }
}

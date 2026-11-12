import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a row the page draws itself: a SettingRow search can land on, with its error and Retry below
Column {
    id: item

    // the SettingsPage hosting it; a row with an id registers there so search lands on it
    property Item host: null
    property string rowId: ""
    property alias title: settingRow.title
    property alias detail: settingRow.detail
    property alias first: settingRow.first
    property alias activatable: settingRow.activatable
    property alias changed: settingRow.changed
    property alias unavailable: settingRow.unavailable
    property string error: ""
    property bool canRetry: true
    readonly property alias settingRow: settingRow
    default property alias control: settingRow.control

    signal activated()
    signal resetRequested()
    signal retry()

    width: parent ? parent.width : 0
    spacing: LithicTokens.space.s1
    property string registered: ""

    function unregister() {
        if (item.host && item.registered !== "" && item.host.rowItems[item.registered] === item)
            delete item.host.rowItems[item.registered];
        item.registered = "";
    }

    function register() {
        item.unregister();
        if (item.host && item.rowId !== "") {
            item.host.rowItems[item.rowId] = item;
            item.registered = item.rowId;
        }
    }

    Component.onCompleted: item.register()
    onRowIdChanged: item.register()
    Component.onDestruction: item.unregister()

    SettingRow {
        id: settingRow
        width: parent.width
        hoverEnabled: !item.host || !item.host.pointerQuiet
        highlighted: !!item.host && item.rowId !== "" && item.host.highlightId === item.rowId
        onActivated: item.activated()
        onResetRequested: item.resetRequested()
    }

    Row {
        visible: item.error.length > 0
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - (retry.visible ? retry.width + LithicTokens.space.s3 : 0))
            text: item.error
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            visible: item.canRetry
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: item.retry()
        }
    }
}

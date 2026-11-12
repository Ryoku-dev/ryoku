import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a one-off action the page runs itself: host[row.run](), with the page's busy state
Plaque {
    id: control

    property var row: ({})
    readonly property var host: control.row.host || null

    function activate() {
        if (control.enabled && control.host && typeof control.host[control.row.run] === "function")
            control.host[control.row.run]();
    }

    kind: control.row.actionKind || "quiet"
    small: true
    glyph: control.row.actionGlyph || ""
    busy: !!control.host && control.host.busyId === control.row.id
    enabled: !control.host || control.host.busyId.length === 0
    text: I18n.tr(control.row.actionText || "Open")
    accessibleName: I18n.tr(control.row.label || "")
    onClicked: control.activate()
}

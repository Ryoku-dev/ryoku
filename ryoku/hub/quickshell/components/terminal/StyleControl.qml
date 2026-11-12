import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// choose one installed RyoStore greeting style; the page confirms before acting
Item {
    id: control

    property var row: ({})
    readonly property var host: control.row.host || null
    readonly property var styles: control.host ? control.host.store.styles : []

    implicitWidth: picker.width
    implicitHeight: picker.implicitHeight
    width: implicitWidth

    function activate() {
        picker.open();
    }

    Picker {
        id: picker
        width: Math.min(Math.max(implicitWidth, 220), LithicTokens.size.settingsControlMax)
        enabled: control.styles.length > 0 && (!control.host || control.host.busyId.length === 0)
        options: [{ label: I18n.tr(control.row.placeholder || "Choose a style"), disabled: true }]
            .concat(control.styles.map(s => ({ label: String(s.name || s.id) })))
        currentIndex: 0
        overlayParent: control.host ? control.host.hub.overlayItem : null
        Accessible.name: I18n.tr(control.row.label || "")
        // a choice only opens the page's Confirm, so the picker goes back to its prompt
        onActivated: index => {
            picker.currentIndex = 0;
            if (index > 0 && control.host && typeof control.host[control.row.run] === "function")
                control.host[control.row.run](control.styles[index - 1]);
        }
    }
}

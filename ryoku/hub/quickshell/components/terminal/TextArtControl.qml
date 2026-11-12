import QtQuick
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

// the text art file: typed or chosen, then copied in beside the greeting
Item {
    id: control

    property var row: ({})
    property var value
    readonly property var host: control.row.host || null

    implicitWidth: field.implicitWidth
    implicitHeight: field.implicitHeight
    width: LithicTokens.size.settingsControlMax

    PathField {
        id: field
        width: parent.width
        kind: "file"
        path: control.value ? String(control.value) : ""
        placeholder: I18n.tr("Choose a text file")
        accessibleName: I18n.tr(control.row.label || "")
        errorText: control.host ? control.host.textArtError : ""
        onChooseRequested: dialog.open()
        onEdited: p => {
            if (control.host)
                control.host.importTextArt(p);
        }
    }

    RyokuUi.FileDialogHost {
        id: dialog
        title: I18n.tr("Choose text art")
        nameFilters: [I18n.tr("Text art") + " (*.txt *.ascii *.art)", I18n.tr("All files") + " (*)"]
        // where the file last came from, else the folder of the file in use (not the greeting's
        // own copy beside its settings), else home
        currentFolder: {
            var h = control.host;
            var own = h && h.store ? h.folderOf(h.store.configPath) : "";
            var shown = h && control.value ? h.folderOf(String(control.value)) : "";
            var folder = h && h.textArtFolder.length > 0 ? h.textArtFolder
                : shown.length > 0 && shown !== own ? shown : Paths.home;
            return "file://" + folder;
        }
        onAccepted: {
            if (control.host)
                control.host.importTextArt(decodeURIComponent(String(dialog.selectedFile).replace(/^file:\/\//, "")));
        }
    }
}

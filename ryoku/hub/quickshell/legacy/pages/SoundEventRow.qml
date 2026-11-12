pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// One desktop sound: play it, swap in your own file, go back to the built-in one,
// or switch it off. Every change is written live through the shell daemon, which
// owns playback (`sounds.events.<event>` in shell.json).
SettingRow {
    id: row

    required property string event
    property bool masterOn: true
    property bool busy: false
    signal failed(string message)

    readonly property var setting: {
        Settings.revision;
        var v = Settings.get("sounds.events." + row.event);
        return v && typeof v === "object" ? v : ({});
    }
    readonly property bool on: row.setting.enabled === true
    readonly property string file: typeof row.setting.file === "string" ? row.setting.file : ""
    readonly property string fileName: row.file.length > 0 ? row.file.substring(row.file.lastIndexOf("/") + 1) : ""

    function save(key, value) {
        row.busy = true;
        Settings.patch("sounds.events." + row.event + "." + key, value, function(ok, error) {
            row.busy = false;
            if (!ok)
                row.failed(I18n.tr(error));
        });
    }

    function preview(path) {
        Settings.send("sounds.preview", { event: row.event, file: path || "" }, function(ok, error) {
            if (!ok)
                row.failed(I18n.tr(error));
        });
    }

    value: row.fileName.length > 0 ? row.fileName : I18n.tr("Built-in")
    controlWidth: ctl.implicitWidth
    enabled: Settings.ready && row.masterOn && !row.busy

    Row {
        id: ctl
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.s2

        Btn {
            anchors.verticalCenter: parent.verticalCenter
            compact: true
            text: I18n.tr("Play")
            onAct: row.preview("")
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            compact: true
            text: row.fileName.length > 0 ? I18n.tr("Change") : I18n.tr("Choose")
            onAct: picker.open()
        }
        Btn {
            anchors.verticalCenter: parent.verticalCenter
            compact: true
            visible: row.fileName.length > 0
            text: I18n.tr("Remove")
            onAct: row.save("file", "")
        }
        Sw {
            anchors.verticalCenter: parent.verticalCenter
            on: row.on
            onToggled: value => row.save("enabled", value)
        }
    }

    FileDialogHost {
        id: picker
        title: I18n.tr("Choose a sound for %1").arg(row.label)
        nameFilters: [I18n.tr("Sounds") + " (*.oga *.ogg *.opus *.wav *.flac *.mp3)", I18n.tr("All files") + " (*)"]
        currentFolder: "file://" + (row.file.length > 0
            ? row.file.substring(0, row.file.lastIndexOf("/"))
            : (Quickshell.env("HOME") || "/"))
        onAccepted: {
            var path = decodeURIComponent(("" + picker.selectedFile).replace(/^file:\/\//, ""));
            row.save("file", path);
            row.preview(path);
        }
    }
}

pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons as Ui

Singleton {
    id: root

    property real fontScale: 1.3
    property alias matchWallpaper: themeAdapter.followWallpaper

    function refreshScale() {
        const v = Ui.ShellSettings.get("fontScale", null);
        root.fontScale = v !== null && isFinite(Number(v)) ? Number(v) : 1.3;
    }
    Connections {
        target: Ui.ShellSettings
        function onChanged() {
            if (Ui.ShellSettings.touches("fontScale", Ui.ShellSettings.lastPaths))
                root.refreshScale();
        }
    }
    Component.onCompleted: root.refreshScale()

    FileView {
        id: themeFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/theme.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter { id: themeAdapter; property bool followWallpaper: true }
    }
}

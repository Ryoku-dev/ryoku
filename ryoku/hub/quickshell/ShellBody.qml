import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "Singletons"

// the Settings window and its wiring; shell.qml loads it by file URL
Scope {
    id: root

    readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")
    readonly property string cacheHome: Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")

    Component.onCompleted: {
        Quickshell.watchFiles = !Quickshell.shellDir.startsWith(root.configHome + "/quickshell/");
        Lithic.translator = s => I18n.tr(s);
        // Ctrl+Z inside a dialog undoes what Ctrl+Z on the page would
        Lithic.undo = () => Changes.undo(hub.pageId);
    }

    // reduce motion has one home, the desktop-wide setting
    Binding {
        target: Lithic
        property: "reduceMotion"
        value: Settings.get("theme.motion.reduce") === true
    }

    FloatingWindow {
        id: win
        title: I18n.tr("Ryoku Settings")
        implicitWidth: 1280
        implicitHeight: 800
        // never smaller than the sidebar beside the page; the drawer a narrower window
        // keeps it in is only for a compositor that sizes the window below this anyway
        minimumSize: Qt.size(1280, 800)
        color: Lithic.ground

        // closing through the compositor quits too, so the single-instance lock releases
        onClosed: hub.requestQuit()

        Hub {
            id: hub
            anchors.fill: parent
            focus: true
            LayoutMirroring.enabled: I18n.rtl
            LayoutMirroring.childrenInherit: true
        }
    }

    // the look follows the files Lithic resolves it from: the colours, light or dark, contrast,
    // tint and design are the desktop's (Appearance > Style), so Settings wears what every
    // Ryoku surface does
    FileView {
        path: root.configHome + "/ryoku/shell.json"
        watchChanges: true
        printErrors: false
        onFileChanged: Lithic.reload()
    }
    FileView {
        path: root.configHome + "/ryoku/theme.json"
        watchChanges: true
        printErrors: false
        onFileChanged: Lithic.reload()
    }
    FileView {
        path: root.cacheHome + "/ryoku/colors.json"
        watchChanges: true
        printErrors: false
        onFileChanged: Lithic.reload()
    }
    FileView {
        path: root.configHome + "/ryoku/matugen.json"
        watchChanges: true
        printErrors: false
        onFileChanged: Lithic.reload()
    }

    // the daemon and CLI drive navigation through the bus; a BusIpcHandler is also a
    // Quickshell IpcHandler, so `qs ipc call nav open <page>` drives one instance directly
    RyokuUi.BusIpcHandler {
        target: "nav"
        function open(section: string): void { hub.navigate(section, ""); }
        function section(): string { return hub.pageId; }
    }
    RyokuUi.BusIpcHandler {
        target: "users"
        function fromProfile(id: string): void { hub.openUsersProfile(id); }
    }
    RyokuUi.BusIpcHandler {
        target: "hub"
        function close(): void { hub.requestQuit(); }
    }
}

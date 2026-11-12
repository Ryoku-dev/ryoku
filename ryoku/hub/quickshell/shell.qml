//@ pragma Env QML_XHR_ALLOW_FILE_READ=1
import QtQuick
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

// Ryoku Settings entry point; `qs -c hub` loads this
ShellRoot {
    id: root

    readonly property bool preview: Quickshell.env("RYOKU_HUB_PREVIEW") === "1"
    readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")
    readonly property string cacheHome: Quickshell.env("XDG_CACHE_HOME") || (Quickshell.env("HOME") + "/.cache")

    Component.onCompleted: {
        Quickshell.watchFiles = !Quickshell.shellDir.startsWith(root.configHome + "/quickshell/");
        Lithic.translator = s => I18n.tr(s);
    }

    FloatingWindow {
        id: win
        title: I18n.tr("Ryoku Settings")
        implicitWidth: 1280
        implicitHeight: 800
        minimumSize: Qt.size(640, 520)
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

    // the palette follows the files Lithic resolves it from
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

    // the daemon and CLI drive navigation (`qs -c hub ipc call nav open <id>`);
    // a preview beside the installed Settings answers only its own qs instance
    LazyLoader {
        active: !root.preview
        component: Scope {
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
    }
    LazyLoader {
        active: root.preview
        component: Scope {
            IpcHandler {
                target: "nav"
                function open(section: string): void { hub.navigate(section, ""); }
                function section(): string { return hub.pageId; }
            }
            IpcHandler {
                target: "users"
                function fromProfile(id: string): void { hub.openUsersProfile(id); }
            }
            IpcHandler {
                target: "hub"
                function close(): void { hub.requestQuit(); }
            }
        }
    }
}

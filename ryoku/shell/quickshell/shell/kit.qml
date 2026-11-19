//@ pragma Env QML_XHR_ALLOW_FILE_READ=1
//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded
//@ pragma DefaultEnv QS_DROP_EXPENSIVE_FONTS=1
//@ pragma DefaultEnv QT_QUICK_FLICKABLE_WHEEL_DECELERATION=10000
// Lithic reads the desktop's style and palette files through file:// requests;
// the rest keeps Kit on the shell's render loop, fonts and wheel feel.
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import Lithic
import "modules/kit"

ShellRoot {
    id: app

    property bool initialized: false
    property bool shown: false
    property string monitor: ""
    property string section: ""
    property string page: ""
    property var args: ({})
    property int serial: 0
    readonly property var output: Quickshell.screens.find(screen => screen.name === app.monitor)
        || Quickshell.screens.find(screen => screen.name === Wm.focusedOutput)
        || Quickshell.screens[0] || null
    readonly property int lingerMs: {
        const seconds = Number(Quickshell.env("RYOKU_KIT_LINGER_SECONDS") || "60");
        return Math.round((Number.isFinite(seconds) && seconds > 0 ? seconds : 60) * 1000);
    }

    function show(section, page, args, monitor) {
        linger.stop();
        app.monitor = monitor || Wm.focusedOutput;
        app.section = section || "";
        app.page = page || "";
        try { app.args = JSON.parse(args || "{}"); }
        catch (error) { app.args = ({}); }
        app.serial++;
        app.shown = true;
    }

    function close() {
        if (!app.shown)
            return;
        app.shown = false;
        kit.flush();
        linger.restart();
    }

    function initialize() {
        if (app.initialized)
            return;
        app.initialized = true;
        Quickshell.watchFiles = false;
        app.show(Quickshell.env("RYOKU_KIT_SECTION"), Quickshell.env("RYOKU_KIT_PAGE"),
            Quickshell.env("RYOKU_KIT_ARGS"), Quickshell.env("RYOKU_KIT_MONITOR"));
    }

    Component.onCompleted: app.initialize()

    IpcHandler {
        target: "kit"
        function open(section: string, page: string, args: string, monitor: string): void {
            app.initialize();
            app.show(section, page, args, monitor);
        }
        function toggle(monitor: string): void {
            app.initialize();
            if (app.shown)
                app.close();
            else
                app.show("", "", "{}", monitor);
        }
        function close(): void {
            app.initialize();
            app.close();
        }
    }

    Timer {
        id: linger
        interval: app.lingerMs
        onTriggered: {
            kit.flush();
            Qt.quit();
        }
    }

    Variants {
        model: [
            (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/ryoku/shell.json",
            (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/ryoku/theme.json",
            (Quickshell.env("XDG_CONFIG_HOME") || Quickshell.env("HOME") + "/.config") + "/ryoku/matugen.json",
            (Quickshell.env("XDG_CACHE_HOME") || Quickshell.env("HOME") + "/.cache") + "/ryoku/colors.json"
        ]
        Scope {
            id: watched
            required property string modelData
            FileView {
                path: app.shown ? watched.modelData : ""
                watchChanges: app.shown
                printErrors: false
                onFileChanged: reload()
                onLoaded: Lithic.reload()
            }
        }
    }

    Kit {
        id: kit
        screen: app.output
        active: app.shown && app.output !== null
        requestSection: app.section
        requestPage: app.page
        requestArgs: app.args
        requestSerial: app.serial
        onRequestClose: app.close()
    }
}

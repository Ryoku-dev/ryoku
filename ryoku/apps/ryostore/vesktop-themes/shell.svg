import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

ShellRoot {
    FloatingWindow {
        id: win
        title: I18n.tr("Ryostore")
        readonly property int fitW: win.screen ? Math.min(1180, win.screen.width - Tokens.s5) : 1180
        readonly property int fitH: win.screen ? Math.min(760, win.screen.height - Tokens.s7) : 760
        readonly property bool cramped: win.fitW < 1180 || win.fitH < 760
        minimumSize: Qt.size(Math.min(980, win.fitW), Math.min(640, win.fitH))
        maximumSize: win.cramped ? Qt.size(win.fitW, win.fitH) : Qt.size(16777215, 16777215)
        color: Tokens.paper
        onClosed: app.requestQuit()

        App {
            id: app
            anchors.fill: parent
        }
    }

    IpcHandler {
        target: "nav"
        function open(section: string): void { app.openRoute(section); }
        function section(): string { return app.categoryID !== "" ? app.categoryID : app.view; }
    }
}

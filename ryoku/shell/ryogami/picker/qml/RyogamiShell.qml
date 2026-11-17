import QtQuick
import Quickshell
import Quickshell.Io

// The daemon releases this process after the closed picker has stayed idle for
// its configured grace. A later open cold-starts directly onto the requested view.
Scope {
    id: shell

    readonly property bool _startVisible: Quickshell.env("RYOGAMI_START_VISIBLE") === "1"
    readonly property var _startSettings: Quickshell.env("RYOGAMI_START_SETTINGS")
    readonly property bool _startRices: Quickshell.env("RYOGAMI_START_RICES") === "1"
    property bool _startupPending: false

    Picker { id: picker }

    function show() { picker.show() }
    function hide() { picker.hide() }
    function toggle() { picker.toggle() }
    function settings(tab) { picker.openSettings(tab) }
    function rices() { picker.openRices() }
    function reportState() {
        if (Daemon.connected)
            Daemon.call("picker.state", { visible: picker.active, busy: picker.busy })
    }

    function completeStartup() {
        shell._startupPending = false
        if (shell._startRices)
            shell.rices()
        else if (typeof shell._startSettings === "string")
            shell.settings(shell._startSettings)
        else if (shell._startVisible)
            shell.show()
    }

    Connections {
        target: picker
        function onActiveChanged() { shell.reportState() }
        function onBusyChanged() { shell.reportState() }
    }

    Connections {
        target: Daemon
        function onEvent(name, data) {
            if (name === "ryogami.wall.toggle")
                shell.toggle()
            else if (name === "ryogami.wall.settings")
                shell.settings(data && data.tab ? data.tab : "")
            else if (name === "ryogami.wall.rices")
                shell.rices()
        }
        function onReconnected() {
            if (shell._startupPending)
                shell.completeStartup()
            shell.reportState()
        }
    }

    IpcHandler {
        target: "ryogami"
        function toggle(): void { shell.toggle() }
        function show(): void { shell.show() }
        function hide(): void { shell.hide() }
        function settings(tab: string): void { shell.settings(tab) }
    }

    Component.onCompleted: {
        if (typeof shell._startSettings !== "string" && !shell._startVisible) {
            shell.reportState()
            return
        }
        if (Daemon.connected) {
            shell.completeStartup()
            shell.reportState()
        } else {
            shell._startupPending = true
        }
    }
}

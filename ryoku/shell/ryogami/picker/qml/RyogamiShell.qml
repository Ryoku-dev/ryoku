import QtQuick
import Quickshell
import Quickshell.Io

// The daemon releases this process after the closed picker has stayed idle for
// its configured grace. A later open cold-starts directly onto the requested view.
Scope {
    id: shell

    readonly property bool _startVisible: Quickshell.env("RYOGAMI_START_VISIBLE") === "1"
    readonly property var _startSettings: Quickshell.env("RYOGAMI_START_SETTINGS")
    readonly property var _startSection: Quickshell.env("RYOGAMI_START_SECTION")
    readonly property var _startControl: Quickshell.env("RYOGAMI_START_CONTROL")
    readonly property var _startCollection: Quickshell.env("RYOGAMI_START_COLLECTION")
    property bool _startupPending: false

    Picker { id: picker }

    function show() { picker.show() }
    function hide() { picker.hide() }
    function toggle() { picker.toggle() }
    function settings(tab, section, control) { picker.openSettings(tab, section, control) }
    function collection(name) { picker.openCollection(name) }
    function reportState() {
        if (Daemon.connected)
            Daemon.call("picker.state", { visible: picker.active, busy: picker.busy })
    }

    function _str(v) { return typeof v === "string" ? v : "" }

    function completeStartup() {
        shell._startupPending = false
        if (typeof shell._startSettings === "string")
            shell.settings(shell._startSettings, shell._str(shell._startSection), shell._str(shell._startControl))
        else if (typeof shell._startCollection === "string")
            shell.collection(shell._startCollection)
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
                shell.settings(data && data.tab ? data.tab : "", data && data.section ? data.section : "",
                               data && data.control ? data.control : "")
            else if (name === "ryogami.wall.collection")
                shell.collection(data && data.name ? data.name : "")
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
        function settings(tab: string): void { shell.settings(tab, "", "") }
        function settingsAt(tab: string, section: string, control: string): void { shell.settings(tab, section, control) }
        function collection(name: string): void { shell.collection(name) }
    }

    Component.onCompleted: {
        if (typeof shell._startSettings !== "string" && typeof shell._startCollection !== "string" && !shell._startVisible) {
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

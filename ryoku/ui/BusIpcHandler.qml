import QtQuick
import Quickshell.Io
import Ryoku.Bus

// An IpcHandler that the daemon can also reach over the bus, so a keybind or
// `ryoku-shell ipc call` lands without spawning a `qs ipc` client. `qs ipc`
// keeps working through the IpcHandler itself. Declare functions on it exactly
// as on an IpcHandler; arguments arrive as strings and typed parameters
// coerce them. It declares no properties of its own: IpcHandler warns about
// every change signal on the object, and the bus remembers what it serves.
IpcHandler {
    id: handler

    onTargetChanged: BusClient.serveAs(handler, handler.enabled ? handler.target : "")
    onEnabledChanged: BusClient.serveAs(handler, handler.enabled ? handler.target : "")
    Component.onCompleted: BusClient.serveAs(handler, handler.enabled ? handler.target : "")
    Component.onDestruction: BusClient.serveAs(handler, "")
}

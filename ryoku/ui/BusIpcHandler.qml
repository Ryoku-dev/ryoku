import QtQuick
import Quickshell.Io
import Ryoku.Bus

// no properties of its own, IpcHandler warns about every change signal on the object
IpcHandler {
    id: handler

    onTargetChanged: BusClient.serveAs(handler, handler.enabled ? handler.target : "")
    onEnabledChanged: BusClient.serveAs(handler, handler.enabled ? handler.target : "")
    Component.onCompleted: BusClient.serveAs(handler, handler.enabled ? handler.target : "")
    Component.onDestruction: BusClient.serveAs(handler, "")
}

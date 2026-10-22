import "../" as Services
import "../../utils" as Utils
import QtQuick
import Caelestia as Backend

QtObject {
    readonly property var session: Backend.SessionManager
    readonly property var utils: Backend.CUtils
    readonly property var cpu: Backend.Cpu
    readonly property var memory: Backend.Memory
    readonly property var gpu: Backend.Gpu
    readonly property var storage: Backend.Storage
    readonly property var sysInfo: Utils.SysInfo
    readonly property var time: Services.Time
    readonly property var update: Services.UpdateChecker
}

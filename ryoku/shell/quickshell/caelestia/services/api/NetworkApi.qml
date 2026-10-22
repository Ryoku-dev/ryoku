import "../" as Services
import QtQuick
import Caelestia as Backend
import Caelestia.Services

QtObject {
    readonly property var manager: Services.Nmcli
    readonly property var requests: Backend.Requests
    readonly property var usage: NetworkUsage
    readonly property var vpn: Services.VPN
}

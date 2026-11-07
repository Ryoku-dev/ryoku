pragma Singleton
import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../utils/menupoll.js" as MenuPoll

// QML view of the daemon `network` topic. NetworkManager lives in ryoku-shell
// (network.go), so QML never shells out to nmcli nor speaks D-Bus itself.
// The bus streams a full frame; Wi-Fi, WireGuard, VPN, and DNS changes use
// `network.*` bus calls. The status derivations the
// rail and collapsed row read (kind/level/wifiRadio), the active WireGuard
// indicator, and DNS provider state are computed from that typed frame.
Singleton {
    id: root

    // --- raw topic state (defaults are the pre-daemon "empty" frame) ---
    property var wifi: ({ present: false, enabled: false, connectivity: "Disconnected", ssid: "", strength: 0 })
    property var wired: ({ present: false, connectivity: "Disconnected" })
    property var connection: ({ kind: "", connectivity: "unknown", online: false })
    property var accessPoints: []
    property var wgTunnels: []
    // Saved NetworkManager VPN profiles (OpenVPN, OpenConnect, ...), service
    // order: { uuid, name, service, active, connecting, ip4 }.
    property var vpnProfiles: []
    property var dns: ({ provider: "dhcp", servers: [] })

    // --- derived status the rail + collapsed row bind to (preserved interface) ---
    readonly property string kind: root.connection.online === true && root.connection.kind
        ? root.connection.kind
        : root.wired.connectivity === "Connected" ? "ethernet"
        : root.wifi.connectivity === "Connected" ? "wifi" : ""
    readonly property string connectivity: root.connection.connectivity || "unknown"
    readonly property real level: Math.max(0, Math.min(1, (root.wifi.strength || 0) / 100))
    readonly property bool wifiRadio: root.wifi.enabled === true
    readonly property string activeSsid: root.wifi.ssid || ""
    readonly property string wifiConnectivity: root.wifi.connectivity || "Disconnected"
    readonly property bool wifiPresent: root.wifi.present === true
    readonly property string dnsProvider: root.dns.provider || "dhcp"
    readonly property var dnsServers: Array.isArray(root.dns.servers) ? root.dns.servers : []

    // The VPN indicator is an active WireGuard tunnel (contract 06 sec 3
    // appends " (+WG)"; RailVpn shows only while a tunnel is up).
    readonly property var activeWg: root.wgTunnels.filter(t => t && t.active)
    readonly property bool vpnActive: root.activeWg.length > 0
    readonly property string vpnName: root.activeWg.length > 0 ? (root.activeWg[0].name || "") : ""

    // Every saved tunnel a VPN switch drives, VPN profiles then WireGuard, in
    // one shape: { uuid, name, service, active, connecting, ip4 }.
    readonly property var vpnTunnels: root.vpnProfiles.concat(root.wgTunnels.map(tunnel => ({
        uuid: tunnel.uuid,
        name: tunnel.name || "",
        service: "wireguard",
        active: tunnel.active === true,
        connecting: false,
        ip4: ""
    })))

    // vpnPolling ownership is retained for the rail/menu lifecycle (RailVpn and
    // MenuNetwork claim it on reveal); the topic streams regardless of who
    // watches, so this now only tracks owners for that contract.
    property bool vpnPolling: false
    property var vpnPollOwners: []
    function setVpnPolling(owner, enabled) {
        vpnPollOwners = MenuPoll.setOwnership(vpnPollOwners, owner, enabled);
        vpnPolling = vpnPollOwners.length > 0;
    }

    // Throughput comes from the shared SystemMonitor sampler; a bar that shows
    // it claims Sysinfo, so sampling stops with the last owner or a dark session.
    function setTrafficActive(owner, enabled) {
        Sysinfo.setActive(owner, enabled);
    }
    readonly property real downBps: Sysinfo.monitor.networkRxBytesPerSec
    readonly property real upBps: Sysinfo.monitor.networkTxBytesPerSec

    // --- intents (QML -> daemon) ---
    function refresh() { root.call("network.wifiScan", {}); }
    function setWifiEnabled(on) { root.call("network.wifiSetEnabled", { enabled: on === true }); }
    function connectWifi(ssid, password, bssid, hidden) { return root.call("network.wifiConnect", { ssid: ssid, password: password || "", bssid: bssid || "", hidden: hidden === true }); }
    function disconnectWifi() { root.call("network.wifiDisconnect", {}); }
    function forgetWifi(ssid) { root.call("network.wifiForget", { ssid: ssid }); }
    function wgActivate(uuid) { root.call("network.wgActivate", { uuid: uuid }); }
    function wgDeactivate(uuid) { root.call("network.wgDeactivate", { uuid: uuid }); }
    function wgImport(path) { root.call("network.wgImport", { path: path }); }
    function vpnActivate(uuid) { return root.call("network.vpnActivate", { uuid: uuid }); }
    function vpnDeactivate(uuid) { return root.call("network.vpnDeactivate", { uuid: uuid }); }
    function setVpnTunnel(tunnel, on) {
        if (!tunnel || !tunnel.uuid)
            return 0;
        return on ? root.vpnActivate(tunnel.uuid) : root.vpnDeactivate(tunnel.uuid);
    }
    function setDnsProvider(provider, servers) {
        return root.call("network.dnsSet", {
            provider: provider || "",
            servers: Array.isArray(servers) ? servers : []
        });
    }

    // Reply correlation: connectWifi returns an id, and `replied(id, ok, error)`
    // fires when the daemon answers, so the reveal can show "Error Connecting"
    // on a failed connect (contract 06 sec 6).
    signal replied(int id, bool ok, string error)
    property int nextCallId: 1

    function apply(f) {
        if (!f || typeof f !== "object")
            return;
        if (f.wifi) root.wifi = f.wifi;
        if (f.wired) root.wired = f.wired;
        if (f.connection) root.connection = f.connection;
        root.accessPoints = Array.isArray(f.accessPoints) ? f.accessPoints : [];
        root.wgTunnels = Array.isArray(f.wireguard) ? f.wireguard : [];
        root.vpnProfiles = Array.isArray(f.vpn) ? f.vpn : [];
        if (f.dns) root.dns = f.dns;
    }

    function call(method, args) {
        const id = root.nextCallId++;
        const payload = Object.assign({}, args, { id: id });
        Bus.call(method, payload, (ok, result, error) => root.replied(id, ok === true, error || ""));
        return id;
    }

    BusTopic {
        id: topicFeed
        topic: "network"
        onUpdated: root.apply(topicFeed.data)
    }
}

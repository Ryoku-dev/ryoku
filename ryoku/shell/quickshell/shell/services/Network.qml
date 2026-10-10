pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "../utils/menupoll.js" as MenuPoll

// QML view of the daemon `network` topic. NetworkManager lives in ryoku-shell
// (network.go), so QML never shells out to nmcli nor speaks D-Bus itself.
// `subscribe network` streams a full frame; Wi-Fi, WireGuard, and DNS changes use
// `call network.*` intents on a second connection. The status derivations the
// rail and collapsed row read (kind/level/wifiRadio), the active WireGuard
// indicator, and DNS provider state are computed from that typed frame.
Singleton {
    id: root

    // --- raw topic state (defaults are the pre-daemon "empty" frame) ---
    property var wifi: ({ present: false, enabled: false, connectivity: "Disconnected", ssid: "", strength: 0 })
    property var wired: ({ present: false, connectivity: "Disconnected" })
    property var accessPoints: []
    property var wgTunnels: []
    property var dns: ({ provider: "dhcp", servers: [] })

    // --- derived status the rail + collapsed row bind to (preserved interface) ---
    readonly property string kind: root.wired.connectivity === "Connected" ? "ethernet"
        : root.wifi.connectivity === "Connected" ? "wifi" : ""
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

    // vpnPolling ownership is retained for the rail/menu lifecycle (RailVpn and
    // MenuNetwork claim it on reveal); the topic streams regardless of who
    // watches, so this now only tracks owners for that contract.
    property bool vpnPolling: false
    property var vpnPollOwners: []
    function setVpnPolling(owner, enabled) {
        vpnPollOwners = MenuPoll.setOwnership(vpnPollOwners, owner, enabled);
        vpnPolling = vpnPollOwners.length > 0;
    }

    // Throughput is shared by every bar. A visible owner starts one /proc read
    // for the whole shell; hiding the last owner drops the byte baseline and
    // leaves no timer running.
    property var trafficOwners: []
    readonly property bool trafficActive: root.trafficOwners.length > 0
    function setTrafficActive(owner, enabled) {
        root.trafficOwners = MenuPoll.setOwnership(root.trafficOwners, owner, enabled);
    }
    property real downBps: 0
    property real upBps: 0
    property real _trafficRx: 0
    property real _trafficTx: 0
    property bool _haveTraffic: false

    function _setTraffic(down, up) {
        if (root.downBps !== down)
            root.downBps = down;
        if (root.upBps !== up)
            root.upBps = up;
    }

    function _readTraffic(text) {
        const lines = String(text || "").split("\n");
        let rx = 0;
        let tx = 0;
        for (let i = 0; i < lines.length; i++) {
            const colon = lines[i].indexOf(":");
            if (colon < 0 || lines[i].slice(0, colon).trim() === "lo")
                continue;
            const fields = lines[i].slice(colon + 1).trim().split(/\s+/);
            if (fields.length < 9)
                continue;
            rx += Number(fields[0]) || 0;
            tx += Number(fields[8]) || 0;
        }
        if (root._haveTraffic)
            root._setTraffic(Math.max(0, (rx - root._trafficRx) / 2),
                             Math.max(0, (tx - root._trafficTx) / 2));
        root._trafficRx = rx;
        root._trafficTx = tx;
        root._haveTraffic = true;
    }

    onTrafficActiveChanged: if (!root.trafficActive) {
        root._haveTraffic = false;
        root._setTraffic(0, 0);
    }

    FileView {
        id: trafficFile
        path: root.trafficActive ? "/proc/net/dev" : ""
        blockLoading: true
        printErrors: false
        onLoaded: root._readTraffic(trafficFile.text())
    }

    Timer {
        interval: 2000
        running: root.trafficActive
        repeat: true
        triggeredOnStart: true
        onTriggered: trafficFile.reload()
    }

    // --- intents (QML -> daemon) ---
    function refresh() { root.call("network.wifiScan", {}); }
    function setWifiEnabled(on) { root.call("network.wifiSetEnabled", { enabled: on === true }); }
    function connectWifi(ssid, password, bssid, hidden) { return root.call("network.wifiConnect", { ssid: ssid, password: password || "", bssid: bssid || "", hidden: hidden === true }); }
    function disconnectWifi() { root.call("network.wifiDisconnect", {}); }
    function forgetWifi(ssid) { root.call("network.wifiForget", { ssid: ssid }); }
    function wgActivate(uuid) { root.call("network.wgActivate", { uuid: uuid }); }
    function wgDeactivate(uuid) { root.call("network.wgDeactivate", { uuid: uuid }); }
    function wgImport(path) { root.call("network.wgImport", { path: path }); }
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

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    function apply(line) {
        try {
            const f = JSON.parse(line);
            if (f.wifi) root.wifi = f.wifi;
            if (f.wired) root.wired = f.wired;
            root.accessPoints = Array.isArray(f.accessPoints) ? f.accessPoints : [];
            root.wgTunnels = Array.isArray(f.wireguard) ? f.wireguard : [];
            if (f.dns) root.dns = f.dns;
        } catch (e) {
            // A malformed frame keeps the last good state.
        }
    }

    function call(method, args) {
        const id = root.nextCallId++;
        const payload = Object.assign({}, args, { id: id });
        ctl.queued += "call " + method + " " + JSON.stringify(payload) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
        return id;
    }

    // Subscription: connect, ask once, then stream. A second write to this
    // connection would half-close the stream (daemon rule), so calls use ctl.
    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe network\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }

    // The daemon may be down when the shell loads (or restart under it); retry
    // quietly so the view repopulates once it returns.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""
        parser: SplitParser {
            onRead: line => {
                try {
                    const r = JSON.parse(line);
                    root.replied(r.id !== undefined ? r.id : 0, r.ok === true, r.error || "");
                } catch (e) {
                }
            }
        }

        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}

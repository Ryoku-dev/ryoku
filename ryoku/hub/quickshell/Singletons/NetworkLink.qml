pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons

// the shell daemon's network topic for the Ethernet, VPN and DNS pages and the IP editor, and the
// network.* calls they make. NetworkManager lives in the daemon, so Settings never reads it or runs
// nmcli for these: a frame comes on connect and on every change, an action goes back as a call
Singleton {
    id: root

    property bool ready: false
    property var dns: ({ provider: "dhcp", servers: [] })
    property var wireguard: []
    property var vpn: []
    // wired adapters ({ device, mac, state, speed, ip4, activeUuid, activeName }) and the saved
    // Ethernet profiles ({ uuid, name, device, autoconnect, active })
    property var ethernet: []
    property var ethernetProfiles: []
    // the frame has not come for a few seconds although the desktop service answers
    property bool silent: false
    // connection uuid -> when its IP change reverts (ms since the epoch), kept here so a page left
    // and opened again still offers Keep
    property var ipRevert: ({})

    readonly property string dnsProvider: root.dns.provider || "dhcp"
    readonly property var dnsServers: Array.isArray(root.dns.servers) ? root.dns.servers : []
    readonly property bool usable: root.ready && Bus.connected

    // why the controls are off, in plain words; empty when they work
    readonly property string unavailable: {
        if (!Bus.connected)
            return I18n.tr("The desktop service is not running, so network changes are off here. Run ryoku doctor in a terminal, then look again.");
        if (root.ready)
            return "";
        return root.silent ? I18n.tr("NetworkManager is not answering through the desktop service, so these settings are off until it runs.")
                           : I18n.tr("Waiting for NetworkManager…");
    }

    // a DNS change waits on authorization and a VPN on its plugin, both longer than a settings write
    readonly property int callWait: 30000

    function send(method, args, done) {
        if (!root.usable) {
            var why = root.unavailable;
            Qt.callLater(function () { if (done) done(false, why, undefined); });
            return;
        }
        Changes.call(method, args || {}, done, root.callWait);
    }

    function setDns(provider, servers, done) {
        root.send("network.dnsSet", { provider: provider, servers: servers || [] }, done);
    }
    function wireguardSet(uuid, on, done) {
        root.send(on ? "network.wgActivate" : "network.wgDeactivate", { uuid: uuid }, done);
    }
    function wireguardImport(path, done) {
        root.send("network.wgImport", { path: path }, done);
    }
    function wireguardDelete(uuid, done) {
        root.send("network.wgDelete", { uuid: uuid }, done);
    }
    function vpnSet(uuid, on, done) {
        root.send(on ? "network.vpnActivate" : "network.vpnDeactivate", { uuid: uuid }, done);
    }
    // an empty uuid lets NetworkManager pick the adapter's profile, or make one
    function ethernetConnect(device, uuid, done) {
        root.send("network.ethernetConnect", { device: device, uuid: uuid || "" }, done);
    }
    function ethernetDisconnect(device, done) {
        root.send("network.ethernetDisconnect", { device: device }, done);
    }
    function ethernetSetAutoconnect(uuid, on, done) {
        root.send("network.ethernetSetAutoconnect", { uuid: uuid, autoconnect: on }, done);
    }
    // a tunnel's or profile's details; a tunnel that is up adds rxBytes and txBytes. wgEdit changes a
    // tunnel's name and one peer, picked by publicKey, and says applied: false when a tunnel that is up
    // takes it on its next connect. wgExport writes the tunnel out as a private .conf and answers its
    // path; wgReplace loads that file back in place of the tunnel and answers the new uuid
    function vpnGet(uuid, done) {
        root.send("network.vpnGet", { uuid: uuid }, done);
    }
    function vpnSetAutoconnect(uuid, on, done) {
        root.send("network.vpnSetAutoconnect", { uuid: uuid, autoconnect: on }, done);
    }
    function wgEdit(uuid, edit, done) {
        root.send("network.wgEdit", Object.assign({ uuid: uuid }, edit), done);
    }
    function wgExport(uuid, done) {
        root.send("network.wgExport", { uuid: uuid }, done);
    }
    function wgReplace(uuid, path, done) {
        root.send("network.wgReplace", { uuid: uuid, path: path }, done);
    }
    // a saved connection's IPv4 settings. ipSet on a live connection answers revertSeconds: the old
    // settings come back then unless ipConfirm keeps the new ones; ipRevert brings them back at once
    function ipGet(uuid, done) {
        root.send("network.ipGet", { uuid: uuid }, done);
    }
    function ipSet(uuid, config, done) {
        root.send("network.ipSet", Object.assign({ uuid: uuid }, config), done);
    }
    function ipConfirm(uuid, done) {
        root.send("network.ipConfirm", { uuid: uuid }, done);
    }
    function ipRevertNow(uuid, done) {
        root.send("network.ipRevert", { uuid: uuid }, done);
    }

    function revertUntil(uuid) {
        var at = root.ipRevert[uuid] || 0;
        return at > Date.now() ? at : 0;
    }
    function setRevertUntil(uuid, at) {
        var next = Object.assign({}, root.ipRevert);
        if (at > Date.now())
            next[uuid] = at;
        else
            delete next[uuid];
        root.ipRevert = next;
    }

    function same(a, b) {
        return JSON.stringify(a) === JSON.stringify(b);
    }

    // a frame comes on every NetworkManager change, most of them elsewhere (a Wi-Fi signal); a list
    // changes only when its content does, so the rows built from it are not rebuilt for nothing
    function apply(f) {
        if (!f || typeof f !== "object" || Array.isArray(f))
            return;
        if (f.dns && !root.same(f.dns, root.dns))
            root.dns = f.dns;
        var lists = ["wireguard", "vpn", "ethernet", "ethernetProfiles"];
        for (var i = 0; i < lists.length; ++i) {
            var next = Array.isArray(f[lists[i]]) ? f[lists[i]] : [];
            if (!root.same(next, root[lists[i]]))
                root[lists[i]] = next;
        }
        root.silent = false;
        root.ready = true;
    }

    property QtObject feed: RU.BusTopic {
        topic: "network"
        onUpdated: root.apply(root.feed.data)
    }

    // the daemon's own answer to a subscribe is the first frame; none in a few seconds means
    // NetworkManager is not reachable from it
    Timer {
        interval: 5000
        running: Bus.connected && !root.ready
        onTriggered: root.silent = true
    }
}

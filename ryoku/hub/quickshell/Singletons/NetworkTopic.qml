pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// QML view of the daemon `network` topic (ryoku/shell/ipc/network.go) for the
// Hub's Connections Ethernet, DNS and VPN tabs. NetworkManager lives in the shell
// daemon, so the Hub never reads it or calls nmcli for these: `subscribe
// network` streams the full frame on connect and on every change, and actions
// ride back over a second connection as `network.*` calls. Two sockets, as in
// Settings, since a second write to the subscription would half-close it.
Singleton {
    id: root

    property bool ready: false
    property var dns: ({ provider: "dhcp", servers: [] })
    property var wireguard: []
    property var vpn: []
    // wired adapters ({device, state, speed, ip4, activeUuid, ...}) and the
    // saved Ethernet profiles ({uuid, name, device, autoconnect, active})
    property var ethernet: []
    property var ethernetProfiles: []
    // IP settings edits not applied yet, and revert deadlines, by connection
    // uuid; see ConnectionsIpEditor.stash
    property var ipDrafts: ({})
    property var pendingCalls: ({})
    property int pendingCallCount: 0
    property int nextCallId: 0

    readonly property string dnsProvider: root.dns.provider || "dhcp"
    readonly property var dnsServers: Array.isArray(root.dns.servers) ? root.dns.servers : []

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    // `completion(ok, error, result)` gets the daemon's answer, or a failure
    // once the call has waited 30 s: a DNS change waits on authorization and a
    // VPN on its plugin, both longer than a settings write.
    function setDns(provider, servers, completion) {
        return root.send("network.dnsSet", { provider: provider, servers: servers || [] }, completion);
    }
    function wireguardSet(uuid, on, completion) {
        return root.send(on ? "network.wgActivate" : "network.wgDeactivate", { uuid: uuid }, completion);
    }
    function wireguardImport(path, completion) {
        return root.send("network.wgImport", { path: path }, completion);
    }
    function wireguardDelete(uuid, completion) {
        return root.send("network.wgDelete", { uuid: uuid }, completion);
    }
    function vpnSet(uuid, on, completion) {
        return root.send(on ? "network.vpnActivate" : "network.vpnDeactivate", { uuid: uuid }, completion);
    }
    // An empty uuid lets NetworkManager pick the adapter's profile, or make one.
    function ethernetConnect(device, uuid, completion) {
        return root.send("network.ethernetConnect", { device: device, uuid: uuid || "" }, completion);
    }
    function ethernetDisconnect(device, completion) {
        return root.send("network.ethernetDisconnect", { device: device }, completion);
    }
    function ethernetSetAutoconnect(uuid, on, completion) {
        return root.send("network.ethernetSetAutoconnect", { uuid: uuid, autoconnect: on }, completion);
    }
    // A WireGuard tunnel's or VPN profile's details; a tunnel that is up adds
    // rxBytes and txBytes. wgEdit changes a tunnel's name and one peer, picked
    // by publicKey, and answers applied false when a tunnel that is up only
    // takes it on its next connect. wgExport writes the tunnel out as a
    // private .conf and answers its path; wgReplace loads that file back in
    // place of the tunnel and answers the new uuid.
    function vpnGet(uuid, completion) {
        return root.send("network.vpnGet", { uuid: uuid }, completion);
    }
    function vpnSetAutoconnect(uuid, on, completion) {
        return root.send("network.vpnSetAutoconnect", { uuid: uuid, autoconnect: on }, completion);
    }
    function wgEdit(uuid, edit, completion) {
        return root.send("network.wgEdit", Object.assign({ uuid: uuid }, edit), completion);
    }
    function wgExport(uuid, completion) {
        return root.send("network.wgExport", { uuid: uuid }, completion);
    }
    function wgReplace(uuid, path, completion) {
        return root.send("network.wgReplace", { uuid: uuid, path: path }, completion);
    }
    // A saved connection's IPv4 settings. ipSet on a live connection answers
    // with revertSeconds: the old settings come back then unless ipConfirm
    // keeps the new ones; ipRevert brings them back at once.
    function ipGet(uuid, completion) {
        return root.send("network.ipGet", { uuid: uuid }, completion);
    }
    function ipSet(uuid, config, completion) {
        return root.send("network.ipSet", Object.assign({ uuid: uuid }, config), completion);
    }
    function ipConfirm(uuid, completion) {
        return root.send("network.ipConfirm", { uuid: uuid }, completion);
    }
    function ipRevert(uuid, completion) {
        return root.send("network.ipRevert", { uuid: uuid }, completion);
    }

    function ipStashed(uuid) {
        return root.ipDrafts[uuid] || null;
    }
    function ipStash(uuid, draft) {
        var next = Object.assign({}, root.ipDrafts);
        if (draft)
            next[uuid] = draft;
        else
            delete next[uuid];
        root.ipDrafts = next;
    }
    // Opening the box anew starts from the saved settings: edits left from
    // an earlier time it was open are dropped, a running revert is kept.
    function ipOpened(uuid) {
        var kept = root.ipDrafts[uuid];
        if (kept && !(kept.revertUntil > Date.now()))
            root.ipStash(uuid, null);
    }

    function send(method, args, completion) {
        var payload = Object.assign({}, args || {});
        var id = "";
        if (typeof completion === "function") {
            id = "hub-network-" + (++root.nextCallId);
            payload.id = id;
            var pending = Object.assign({}, root.pendingCalls);
            pending[id] = { callback: completion, deadline: Date.now() + 30000 };
            root.pendingCalls = pending;
            root.pendingCallCount++;
        }
        ctl.queued += "call " + method + " " + JSON.stringify(payload) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
        return id;
    }

    function finishCall(id, ok, error, result) {
        var entry = root.pendingCalls[id];
        if (!entry)
            return;
        var pending = Object.assign({}, root.pendingCalls);
        delete pending[id];
        root.pendingCalls = pending;
        root.pendingCallCount = Math.max(0, root.pendingCallCount - 1);
        entry.callback(ok, error || "", result);
    }

    function applyReply(line) {
        try {
            var reply = JSON.parse(line);
            if (reply && reply.id)
                root.finishCall(String(reply.id), reply.ok === true, String(reply.error || ""), reply.result);
        } catch (e) {
        }
    }

    function expireCalls() {
        var now = Date.now();
        var ids = Object.keys(root.pendingCalls);
        for (var i = 0; i < ids.length; ++i) {
            var entry = root.pendingCalls[ids[i]];
            if (entry && entry.deadline <= now) {
                ctl.dropQueued(ids[i]);
                root.finishCall(ids[i], false, "The shell daemon did not answer.");
            }
        }
    }

    function same(a, b) {
        return JSON.stringify(a) === JSON.stringify(b);
    }
    function apply(line) {
        try {
            var f = JSON.parse(line);
            if (!f || typeof f !== "object" || Array.isArray(f))
                return;
            // A frame comes on every NetworkManager change, most of them
            // elsewhere (a Wi-Fi signal); a list only changes when its content
            // does, so the rows built from it are not rebuilt for nothing.
            if (f.dns && !root.same(f.dns, root.dns))
                root.dns = f.dns;
            var lists = ["wireguard", "vpn", "ethernet", "ethernetProfiles"];
            for (var i = 0; i < lists.length; ++i) {
                var next = Array.isArray(f[lists[i]]) ? f[lists[i]] : [];
                if (!root.same(next, root[lists[i]]))
                    root[lists[i]] = next;
            }
            root.ready = true;
        } catch (e) {
            // A malformed frame keeps the last good state.
        }
    }

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
                root.ready = false;
                retry.restart();
            }
        }
    }

    // The daemon may be down when the Hub loads, or restart under it.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }
    Timer {
        interval: 250
        repeat: true
        running: root.pendingCallCount > 0
        onTriggered: root.expireCalls()
    }

    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""
        parser: SplitParser { onRead: line => root.applyReply(line) }
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        function dropQueued(id) {
            var needle = "\"id\":\"" + id + "\"";
            var lines = queued.split("\n");
            var kept = "";
            for (var i = 0; i < lines.length; ++i) {
                if (lines[i].length > 0 && lines[i].indexOf(needle) < 0)
                    kept += lines[i] + "\n";
            }
            queued = kept;
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }
}

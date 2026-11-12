import QtQuick
import Quickshell.Io
import Quickshell.Networking
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/wifi"
import "../schema/wifi.js" as Schema

// Wi-Fi networks, saved networks with Forget, the hotspot, the Wi-Fi backend and the
// switch that blocks every connection; every change is a command, so nothing here
// writes to NetworkManager over its bus
Item {
    id: root

    required property var hub
    property string pendingRow: ""

    readonly property var store: Changes.stores["wifi"] || null
    readonly property var devices: Networking.devices ? Networking.devices.values : []
    readonly property var wifiDev: root.devices.find(d => d && d.type === DeviceType.Wifi) || null
    readonly property bool wifiOn: Networking.wifiEnabled === true
    readonly property bool radioSwitchOff: Networking.wifiHardwareEnabled === false
    readonly property bool wiredUp: root.devices.some(d => d && d.type === DeviceType.Wired && d.connected)
    readonly property var nets: {
        var list = root.wifiDev && root.wifiDev.networks ? root.wifiDev.networks.values : [];
        var out = {};
        for (var i = 0; i < list.length; i++) {
            var n = list[i];
            if (!n || !n.name)
                continue;
            if (!out[n.name] || n.connected || (!out[n.name].connected && n.signalStrength > out[n.name].signalStrength))
                out[n.name] = n;
        }
        return out;
    }
    readonly property var connectedNet: {
        for (var k in root.nets)
            if (root.nets[k].connected)
                return root.nets[k];
        return null;
    }

    // ssid -> the bands NetworkManager can hold a profile to: "bg", "a"
    property var bandMap: ({})
    // ssid -> one access point per band, the connected one or else the strongest:
    // [{ ghz, mhz, bssid, signal, active }], lowest band first
    property var apMap: ({})
    // Details lists each network's access points, so two networks sharing a name can be
    // told apart; off on every visit
    property bool showDetails: false
    // ssid -> the band chosen in its panel: "", "bg" or "a"
    property var bandPick: ({})
    property string expanded: ""
    property string password: ""
    property bool scanning: false
    property bool moreOpen: false
    property bool showAll: false
    // connected and saved networks in range, then the five strongest of the rest
    readonly property int listLimit: Object.keys(root.nets).filter(k => root.rankOf(k) <= 1).length + 5

    // the one connection attempt at a time: { ssid, uuid, known, proc, cancelled }
    property var attempt: null
    readonly property bool connecting: root.attempt !== null
    property var netErrors: ({})
    // saved networks whose password was refused: ssid -> true
    property var askPassword: ({})
    property var savedErrors: ({})
    property string forgetting: ""
    property string disconnecting: ""

    // the saved rows by uuid: only a change in which networks are saved, or their order,
    // rebuilds rows, so the row the keys or the pointer are on survives a switch and the
    // read that follows it
    property var savedIds: []
    function syncSaved() {
        var ids = root.store ? root.store.profiles.map(p => p.uuid) : [];
        if (ids.join("\n") !== root.savedIds.join("\n"))
            root.savedIds = ids;
    }
    onStoreChanged: root.syncSaved()
    Connections {
        target: root.store
        ignoreUnknownSignals: true
        function onProfilesChanged() { root.syncSaved(); }
    }

    property string killState: "checking"
    // the failed change's message, and what it asked for so Retry asks again
    property string killError: ""
    property bool killWant: false
    property bool killBusy: false
    // the running status read or change, so a long wait can be called off
    property var killProc: null
    property var killReadProc: null
    property bool killStopped: false
    readonly property bool killActive: root.killState === "on" || root.killState === "armed" || root.killState === "blocked"

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: []
    })

    readonly property string stateLine: {
        var name = root.connectedNet ? root.connectedNet.name : "";
        if (name !== "" && Networking.connectivity === NetworkConnectivity.Portal)
            return I18n.tr("Connected to %1. The network wants you to sign in, in a browser.").arg(name);
        if (name !== "" && Networking.connectivity === NetworkConnectivity.Limited)
            return I18n.tr("Connected to %1, but without internet access.").arg(name);
        if (root.wiredUp && name !== "")
            return I18n.tr("Connected by cable and to %1.").arg(name);
        if (root.wiredUp)
            return I18n.tr("Connected by cable.");
        if (name !== "")
            return I18n.tr("Connected to %1.").arg(name);
        if (!root.wifiDev)
            return I18n.tr("Not connected.");
        return root.wifiOn ? I18n.tr("Not connected.") : I18n.tr("Wi-Fi is off.");
    }

    function rankOf(name) {
        var n = root.nets[name];
        return !n ? 3 : n.connected ? 0 : root.isSaved(n) ? 1 : 2;
    }

    // which networks are listed and in which group; signal changes alone never move a row
    readonly property string listKey: Object.keys(root.nets).sort().map(k => k + "\u0001" + root.rankOf(k)).join("\u0002")
    onListKeyChanged: root.syncList(false)

    // while the pointer is over the list, networks coming and going wait, so nothing moves under a click
    property bool pointerOnList: false
    property bool syncHeld: false
    onPointerOnListChanged: {
        if (!root.pointerOnList && root.syncHeld)
            root.syncList(false);
    }

    ListModel { id: netModel }
    // rows held under the pointer can be out of range by now; the count leaves them out
    readonly property int inRangeCount: {
        var n = 0;
        for (var i = 0; i < netModel.count; i++)
            if (root.nets[netModel.get(i).ssid])
                n++;
        return n;
    }

    // connected first, then saved, then the rest; rows already shown keep their place and
    // newcomers join by signal, unless a scan asks for a fresh order. rows move rather than
    // rebuild, so a half-typed password survives
    function syncList(fresh) {
        if (!fresh && root.pointerOnList && netModel.count > 0) {
            root.syncHeld = true;
            return;
        }
        root.syncHeld = false;
        var names = Object.keys(root.nets);
        var shown = [];
        for (var i = 0; i < netModel.count; i++)
            shown.push(netModel.get(i).ssid);
        var bySignal = (a, b) => (root.bars(root.nets[b]) - root.bars(root.nets[a])) || a.localeCompare(b);
        var kept = shown.filter(s => names.indexOf(s) >= 0);
        if (fresh)
            kept.sort(bySignal);
        var all = kept.concat(names.filter(s => shown.indexOf(s) < 0).sort(bySignal));
        var target = all.map((s, at) => ({ s: s, at: at, r: root.rankOf(s) }))
            .sort((x, y) => (x.r - y.r) || (x.at - y.at)).map(x => x.s);
        for (var r = netModel.count - 1; r >= 0; r--)
            if (target.indexOf(netModel.get(r).ssid) < 0)
                netModel.remove(r);
        for (var j = 0; j < target.length; j++) {
            var found = -1;
            for (var k = j; k < netModel.count; k++)
                if (netModel.get(k).ssid === target[j]) {
                    found = k;
                    break;
                }
            if (found < 0)
                netModel.insert(j, { ssid: target[j] });
            else if (found !== j)
                netModel.move(found, j, 1);
        }
    }

    function bars(net) {
        var s = net ? Math.round((net.signalStrength || 0) * 100) : 0;
        return s >= 75 ? 4 : s >= 50 ? 3 : s >= 25 ? 2 : s > 0 ? 1 : 0;
    }

    function signalGlyph(net) {
        switch (root.bars(net)) {
        case 4: return "signal_wifi_4_bar";
        case 3: return "network_wifi_3_bar";
        case 2: return "network_wifi_2_bar";
        case 1: return "network_wifi_1_bar";
        }
        return "signal_wifi_0_bar";
    }

    function secured(net) {
        return !!net && net.security !== WifiSecurityType.Open && net.security !== WifiSecurityType.Owe;
    }

    function netDetail(net) {
        if (!net)
            return I18n.tr("Out of range");
        var parts = [];
        if (root.attempt && root.attempt.ssid === net.name)
            parts.push(I18n.tr("Connecting…"));
        else if (root.disconnecting === net.name)
            parts.push(I18n.tr("Disconnecting…"));
        else if (net.connected)
            parts.push(I18n.tr("Connected"));
        else if (root.isSaved(net))
            parts.push(I18n.tr("Saved"));
        parts.push(root.secured(net) ? I18n.tr("secured") : I18n.tr("open"));
        var b = root.bars(net);
        parts.push(b >= 4 ? I18n.tr("strong signal") : b === 3 ? I18n.tr("good signal") : I18n.tr("weak signal"));
        var text = parts.join(", ");
        return text.charAt(0).toUpperCase() + text.slice(1);
    }

    function setError(map, key, message) {
        var next = Object.assign({}, root[map]);
        if (message)
            next[key] = message;
        else
            delete next[key];
        root[map] = next;
    }

    // what nmcli said, in words with the next step
    function connectError(raw) {
        var r = String(raw || "").toLowerCase();
        if (/secrets were required|password|802-11-wireless-security|psk|authenticat/.test(r))
            return I18n.tr("The password was not accepted. Check it and try again.");
        if (/no network with ssid|not found|not available/.test(r))
            return I18n.tr("The network is out of range. Move closer, or scan again.");
        if (/timeout|timed out|in time/.test(r))
            return I18n.tr("The network did not answer in time. Try again.");
        return raw ? I18n.tr("Could not connect: %1").arg(raw) : I18n.tr("Could not connect. Try again.");
    }

    function chosenBand(ssid) {
        if (root.bandPick[ssid] !== undefined)
            return root.bandPick[ssid];
        var p = root.store ? root.store.profileFor(ssid) : null;
        return p && (p.band === "a" || p.band === "bg") ? p.band : "";
    }

    function pickBand(ssid, value) {
        var next = Object.assign({}, root.bandPick);
        next[ssid] = value;
        root.bandPick = next;
    }

    // the Details line: each band's access point by its hardware address
    function apLine(ssid) {
        return (root.apMap[ssid] || []).filter(ap => ap.bssid !== "").map(function (ap) {
            var band = ap.ghz === "6" ? I18n.tr("6 GHz") : ap.ghz === "5" ? I18n.tr("5 GHz") : I18n.tr("2.4 GHz");
            return band + " " + ap.bssid + (ap.active ? " " + I18n.tr("(connected)") : "");
        }).join("  \u00b7  ");
    }

    // a new secured network, or a saved one whose password stopped working
    function needsPassword(net) {
        return !!net && root.secured(net) && (!root.isSaved(net) || !!root.askPassword[net.name]);
    }

    // the network service or a saved profile with this name
    function isSaved(net) {
        return !!net && (net.known || (!!root.store && root.store.profileFor(net.name) !== null));
    }

    function toggleExpanded(ssid) {
        root.expanded = root.expanded === ssid ? "" : ssid;
        root.password = "";
    }

    // the plaque's action: known and open networks join at once, a new secured one asks for its password
    function primary(net) {
        if (!net)
            return;
        if (net.connected) {
            root.disconnect(net);
            return;
        }
        if (root.needsPassword(net) && root.expanded !== net.name) {
            root.expanded = net.name;
            root.password = "";
            return;
        }
        root.connect(net);
    }

    function connect(net) {
        if (!net || !root.store || root.connecting)
            return;
        var ssid = net.name;
        var prof = root.store.profileFor(ssid);
        var band = root.chosenBand(ssid);
        // a new secured network, or a saved one whose password was refused
        var withPassword = root.secured(net) && (!!root.askPassword[ssid] || (!prof && !net.known));
        if (withPassword && root.password.length === 0)
            return;
        root.setError("netErrors", ssid, "");
        var a = { ssid: ssid, uuid: prof ? prof.uuid : "", known: !!prof, saved: root.isSaved(net),
                  secured: root.secured(net), band: band, autoOff: !!prof && prof.autoconnect === false,
                  viaDaemon: withPassword, password: withPassword ? root.password : "",
                  proc: null, cancelled: false, before: root.store.profiles.map(p => p.uuid) };
        root.attempt = a;
        if (withPassword) {
            root.joinViaDaemon(a);
            return;
        }
        var steps = [];
        if (prof) {
            if (band !== (prof.band || ""))
                steps.push(["nmcli", "connection", "modify", "uuid", prof.uuid, "802-11-wireless.band", band]);
            steps.push(["nmcli", "-w", "45", "connection", "up", "uuid", prof.uuid]);
        } else {
            steps.push(["nmcli", "-w", "45", "dev", "wifi", "connect", ssid]);
        }
        root.runSteps(a, steps, "");
    }

    // a join with a password goes through the shell daemon, which owns Wi-Fi joins: a refused
    // password fails as soon as NetworkManager drops it. ryoku-hub replaces the network's saved
    // profiles with the new password, which goes through stdin, never argv
    function joinViaDaemon(a) {
        a.proc = root.store.run(["ryoku-hub", "wifi", "connect", a.ssid], function (ok, err, out, code) {
            if (a.cancelled)
                return;
            if (ok)
                root.connected(a);
            // 3: the daemon did not run the join; -1: the tool is missing. Either way nmcli joins
            else if (code === 3 || code === -1)
                root.joinWithNmcli(a);
            else
                root.failed(a, err);
        }, a.password);
    }

    // the join without the daemon: nmcli reads the password from stdin, into the saved profile
    // when one is still there
    function joinWithNmcli(a) {
        a.viaDaemon = false;
        a.known = false;
        a.uuid = "";
        a.proc = null;
        root.store.refresh(function () {
            if (a.cancelled)
                return;
            var prof = root.store.profileFor(a.ssid);
            var input = a.password;
            a.password = "";
            a.known = !!prof;
            a.uuid = prof ? prof.uuid : "";
            var steps = [];
            if (prof && a.band !== (prof.band || ""))
                steps.push(["nmcli", "connection", "modify", "uuid", prof.uuid, "802-11-wireless.band", a.band]);
            steps.push(prof ? ["nmcli", "-w", "45", "--ask", "connection", "up", "uuid", prof.uuid]
                            : ["nmcli", "-w", "45", "--ask", "dev", "wifi", "connect", a.ssid]);
            root.runSteps(a, steps, input);
        });
    }

    function runSteps(a, steps, input) {
        if (steps.length === 0) {
            root.connected(a);
            return;
        }
        var last = steps.length === 1;
        a.proc = root.store.run(steps[0], function (ok, err) {
            if (a.cancelled)
                return;
            if (!ok) {
                root.failed(a, err);
                return;
            }
            if (last)
                root.connected(a);
            else
                root.runSteps(a, steps.slice(1), input);
        }, last ? input : "");
    }

    // a profile this join made takes the band picked in its panel, and one that replaced a
    // saved network keeps joining only when chosen, as the old one did
    function connected(a) {
        root.attempt = null;
        if (root.askPassword[a.ssid]) {
            var ask = Object.assign({}, root.askPassword);
            delete ask[a.ssid];
            root.askPassword = ask;
        }
        root.expanded = "";
        root.password = "";
        var mods = [];
        if (a.band !== "")
            mods.push("802-11-wireless.band", a.band);
        if (a.autoOff)
            mods.push("connection.autoconnect", "no");
        root.store.refresh(function () {
            if (mods.length === 0)
                return;
            root.store.profiles.filter(p => p.ssid === a.ssid && a.before.indexOf(p.uuid) < 0).forEach(function (p) {
                root.store.run(["nmcli", "connection", "modify", "uuid", p.uuid].concat(mods), () => root.store.refresh());
            });
        });
    }

    // the password field of a network that asks again takes the keys
    signal askAgain(string ssid)

    function failed(a, err) {
        root.attempt = null;
        root.setError("netErrors", a.ssid, root.connectError(err));
        // a saved network whose password changed asks for the new one, and a refused new one asks again
        if (a.secured && /secrets were required|password|802-11-wireless-security|psk|authenticat/i.test(String(err || ""))) {
            var ask = Object.assign({}, root.askPassword);
            ask[a.ssid] = true;
            root.askPassword = ask;
            root.expanded = a.ssid;
            root.password = "";
            Qt.callLater(root.askAgain, a.ssid);
        }
        if (a.known && !a.viaDaemon)
            root.store.refresh();
        else
            root.dropNew(a);
    }

    // a failed or called-off first join can leave a profile behind that would read as
    // saved and fail forever; only profiles this attempt made are deleted
    function dropNew(a) {
        root.store.refresh(function () {
            root.store.profiles.filter(p => p.ssid === a.ssid && a.before.indexOf(p.uuid) < 0).forEach(function (p) {
                root.store.run(["nmcli", "connection", "delete", "uuid", p.uuid], () => root.store.refresh());
            });
        });
    }

    function cancelConnect() {
        var a = root.attempt;
        if (!a)
            return;
        a.cancelled = true;
        a.password = "";
        if (a.proc)
            a.proc.running = false;
        root.attempt = null;
        // the network manager keeps trying after nmcli or the daemon's join stops waiting, so the
        // attempt is called off there too; the daemon's join goes with the profile it made
        if (a.viaDaemon || (!a.known && !a.saved))
            root.dropNew(a);
        else if (a.known)
            root.store.run(["nmcli", "connection", "down", "uuid", a.uuid], () => root.store.refresh());
        else
            root.store.run(["nmcli", "connection", "down", "id", a.ssid], () => root.store.refresh());
    }

    function disconnect(net) {
        if (!net || !root.store)
            return;
        var p = root.store.profiles.find(x => x.active && x.ssid === net.name);
        var argv = p ? ["nmcli", "connection", "down", "uuid", p.uuid] : ["nmcli", "connection", "down", "id", net.name];
        root.disconnecting = net.name;
        root.setError("netErrors", net.name, "");
        root.store.run(argv, function (ok, err) {
            root.disconnecting = "";
            if (!ok)
                root.setError("netErrors", net.name, I18n.tr("Could not disconnect: %1").arg(err || I18n.tr("no reason given")));
            root.store.refresh();
        });
    }

    function scan() {
        if (!root.wifiOn || root.scanning || !root.store)
            return;
        root.scanning = true;
        scanEnd.restart();
        root.store.run(["nmcli", "dev", "wifi", "rescan"], function () { bandsRead.restart(); });
    }

    function forget(p) {
        root.hub.confirm({
            title: I18n.tr("Forget %1?").arg(p.ssid),
            text: I18n.tr("This computer stops joining %1 and deletes its saved password. To join again you need the password.").arg(p.ssid),
            actionText: I18n.tr("Forget")
        }, function () {
            root.forgetting = p.uuid;
            root.setError("savedErrors", p.uuid, "");
            root.store.run(["nmcli", "connection", "delete", "uuid", p.uuid], function (ok, err) {
                root.forgetting = "";
                if (!ok)
                    root.setError("savedErrors", p.uuid, I18n.tr("Could not forget it: %1").arg(err || I18n.tr("no reason given")));
                root.store.refresh();
            });
        });
    }

    function setAutoconnect(p, on) {
        Changes.set({ store: "wifi", key: "autoconnect." + p.uuid }, on, p.ssid, "wifi",
                    on ? I18n.tr("%1 joins on its own").arg(p.ssid) : I18n.tr("%1 joins only when you choose it").arg(p.ssid));
    }

    function savedDetail(p) {
        var here = p.active || (root.connectedNet && root.connectedNet.name === p.ssid && !root.store.profiles.some(x => x.active))
                 ? I18n.tr("Connected") : root.nets[p.ssid] ? I18n.tr("In range") : I18n.tr("Not in range");
        var parts = [here, p.autoconnect ? I18n.tr("joins on its own") : I18n.tr("joins only when you choose it")];
        // two saved copies of one network read the same, so each says when it was last used
        if (root.store.profiles.filter(x => x.ssid === p.ssid).length > 1)
            parts.push(p.timestamp > 0 ? I18n.tr("last used %1").arg(new Date(p.timestamp * 1000).toLocaleDateString(Qt.locale(), Locale.ShortFormat))
                                       : I18n.tr("never used"));
        return parts.join(", ");
    }

    function readKill() {
        if (root.killReadProc)
            return;
        var over = false;
        var p = Changes.run(["ryoku-host", "network", "kill", "status"], function (ok, out, err, code) {
            over = true;
            root.killReadProc = null;
            var s = String(out || "").trim();
            root.killState = ok && (s === "on" || s === "off" || s === "armed" || s === "blocked") ? s : "unknown";
        });
        if (!over)
            root.killReadProc = p;
    }

    function setKill(on) {
        root.killWant = on;
        root.killBusy = true;
        root.killError = "";
        root.killStopped = false;
        var over = false;
        var p = Changes.run(["ryoku-host", "network", "kill", on ? "on" : "off"], function (ok, out, err, code) {
            over = true;
            root.killProc = null;
            root.killBusy = false;
            if (!ok) {
                var why = String(err || "").trim().split("\n").pop().replace(/^[\w-]+:\s*/, "");
                root.killError = root.killStopped ? I18n.tr("Stopped before it finished. Retry when you are ready.")
                               : code === 126 || code === 127 ? I18n.tr("The change did not apply: the password prompt was closed or refused.")
                               : why ? I18n.tr("The change did not apply: %1").arg(why) : I18n.tr("The change did not apply.");
            }
            root.killStopped = false;
            root.readKill();
        });
        if (!over)
            root.killProc = p;
    }

    // stop waiting for the helper: a read falls back to "could not read", a change to Retry
    function cancelKill() {
        if (root.killProc) {
            root.killStopped = true;
            root.killProc.running = false;
        } else if (root.killReadProc) {
            root.killReadProc.running = false;
        }
    }

    function askKill() {
        if (root.killActive) {
            root.setKill(false);
            return;
        }
        root.hub.confirm({
            title: I18n.tr("Block all connections?"),
            text: I18n.tr("Wi-Fi, Ethernet, VPN and the local network stop at once, and remote sessions such as SSH are cut. Do this from the computer itself. They stay blocked after a restart until you allow them again."),
            actionText: I18n.tr("Block all")
        }, function () { root.setKill(true); });
    }

    function killDetail() {
        switch (root.killState) {
        case "on": return I18n.tr("Every connection is blocked, and stays blocked after a restart.");
        case "armed": return I18n.tr("Blocked at start-up, but the firewall needs repair before this computer is safe. Allow connections, then block them again.");
        case "blocked": return I18n.tr("Traffic is blocked, but it will not stay blocked after a restart.");
        case "checking": return I18n.tr("Checking…");
        }
        return I18n.tr("Cuts Wi-Fi, Ethernet, VPN and the local network, remote sessions too");
    }

    function sectionIndex(id) {
        for (var i = 0; i < page.layout.length; i++)
            if (page.layout[i].id === id)
                return i;
        return -1;
    }

    readonly property var moreRows: ["wifi-backend", "block-all"]

    // a landing waits for its row and for the saved list, which moves everything below it;
    // a deep link may name a section instead ("wifi/hotspot"), which needs no row
    function landable(rowId) {
        return page.layout.length > 0 && !!root.store && root.store.profilesRead
            && (!!page.rowItems[rowId] || root.isSectionLink(rowId));
    }

    function isSectionLink(id) {
        return !Schema.rows.some(r => r.id === id) && Schema.sections.some(s => s.id === id);
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (rowId !== root.pendingRow)
            focusRetry.tries = 0;
        if (!root.landable(rowId) && focusRetry.tries < 30) {
            root.pendingRow = rowId;
            focusRetry.tries++;
            focusRetry.restart();
            return;
        }
        root.pendingRow = "";
        if (root.isSectionLink(rowId)) {
            page.focusRow(rowId);
            return;
        }
        if (root.moreRows.indexOf(rowId) >= 0 && !root.moreOpen) {
            root.moreOpen = true;
            landTimer.rowId = rowId;
            landTimer.restart();
            return;
        }
        var item = page.rowItems[rowId];
        if (item && item.visible) {
            page.land(rowId);
            return;
        }
        var row = Schema.rows.find(r => r.id === rowId);
        var at = root.sectionIndex(row ? row.section : "");
        if (at >= 0)
            page.chooseSection(at);
    }

    Timer {
        id: focusRetry
        property int tries: 0
        interval: 100
        onTriggered: root.focusRow(root.pendingRow)
    }

    Timer {
        id: landTimer
        property string rowId: ""
        interval: LithicTokens.motion.unfold + 40
        onTriggered: page.land(landTimer.rowId)
    }

    Timer {
        id: scanEnd
        interval: 10000
        onTriggered: {
            root.scanning = false;
            root.syncList(root.expanded === "" && !root.connecting);
        }
    }

    // band data comes from nmcli, read again a moment after the list settles
    Timer {
        id: bandsRead
        interval: 1200
        onTriggered: bands.running = true
    }
    Connections {
        target: root
        function onListKeyChanged() { bandsRead.restart(); }
    }

    Process {
        id: bands
        command: ["nmcli", "-t", "-f", "SSID,FREQ,BSSID,SIGNAL,ACTIVE", "dev", "wifi", "list", "--rescan", "no"]
        stdout: StdioCollector {
            onStreamFinished: {
                if (!root.store)
                    return;
                var map = {}, aps = {};
                this.text.split("\n").forEach(function (line) {
                    var f = root.store.terse(line);
                    var mhz = parseInt(f[1]);
                    if (f.length < 5 || f[0] === "" || !(mhz > 0))
                        return;
                    var ssid = f[0];
                    var b = mhz < 2500 ? "bg" : mhz >= 4900 && mhz < 5925 ? "a" : "";
                    if (b !== "") {
                        var list = map[ssid] || [];
                        if (list.indexOf(b) < 0)
                            list.push(b);
                        map[ssid] = list;
                    }
                    var ap = { ghz: mhz < 2500 ? "2.4" : mhz < 5925 ? "5" : "6", mhz: mhz, bssid: f[2],
                               signal: parseInt(f[3]) || 0, active: f[4] === "yes" };
                    var here = aps[ssid] || [];
                    var at = here.findIndex(x => x.ghz === ap.ghz);
                    if (at < 0)
                        here.push(ap);
                    else if (ap.active || (!here[at].active && ap.signal > here[at].signal))
                        here[at] = ap;
                    aps[ssid] = here;
                });
                for (var s in aps)
                    aps[s].sort((x, y) => x.mhz - y.mhz);
                root.bandMap = map;
                root.apMap = aps;
            }
        }
    }

    Pace { id: killReadPace; running: !!root.killReadProc }

    // the scanner runs while the page is open and Wi-Fi is on
    Binding {
        target: root.wifiDev
        property: "scannerEnabled"
        value: root.wifiOn && root.visible
        when: root.wifiDev !== null
    }

    Component.onCompleted: {
        Changes.ensureStore("wifi", Qt.resolvedUrl("../components/wifi/WifiStore.qml"));
        if (root.store)
            root.store.refresh();
        root.syncList(true);
        root.syncSaved();
        bands.running = true;
    }

    // the status runs through the privileged helper, so it is read when More options opens, not on every visit
    onMoreOpenChanged: {
        if (root.moreOpen && root.killState === "checking")
            root.readKill();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "wifi"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ wifi: wifiBody, saved: savedBody, hotspot: hotspotBody })
        onLayoutChanged: {
            if (root.pendingRow !== "") {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: wifiBody
        Slab {
            width: parent ? parent.width : 0

            Column {
                id: wifiColumn
                width: parent.width

                HoverHandler {
                    onHoveredChanged: root.pointerOnList = hovered
                }

                NetRow {
                    id: radioRow
                    readonly property var bind: ({ store: "wifi", key: "enabled" })
                    readonly property bool busy: !!root.store && !!root.store.busy["enabled"]
                    host: page
                    rowId: "wifi-on"
                    first: true
                    title: I18n.tr("Wi-Fi")
                    activatable: !!root.wifiDev && !root.radioSwitchOff
                    unavailable: !root.wifiDev ? I18n.tr("No Wi-Fi adapter was found")
                               : root.radioSwitchOff ? I18n.tr("Turned off by a switch or key on this computer") : ""
                    detail: radioPace.slow ? (root.wifiOn ? I18n.tr("Turning off…") : I18n.tr("Turning on…"))
                          : !root.wifiOn ? I18n.tr("Off")
                          : root.connectedNet ? I18n.tr("On, connected to %1").arg(root.connectedNet.name)
                          : I18n.tr("On, not connected")
                    error: Changes.error(radioRow.bind)
                    onActivated: radioRow.flip(!root.wifiOn)
                    onRetry: Changes.retry(radioRow.bind)

                    function flip(on) {
                        Changes.set(radioRow.bind, on, I18n.tr("Wi-Fi"), "wifi", on ? I18n.tr("Wi-Fi turned on") : I18n.tr("Wi-Fi turned off"));
                    }

                    Pace { id: radioPace; running: radioRow.busy }

                    Working {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: radioPace.slow
                        active: visible
                    }
                    Toggle {
                        id: radioToggle
                        anchors.verticalCenter: parent.verticalCenter
                        checked: root.wifiOn
                        Accessible.name: I18n.tr("Wi-Fi")
                        onToggled: {
                            var want = radioToggle.checked;
                            radioToggle.checked = Qt.binding(() => root.wifiOn);
                            radioRow.flip(want);
                        }
                    }
                }

                LongWait {
                    visible: radioPace.lagging
                    text: root.wifiOn ? I18n.tr("Still turning Wi-Fi off…") : I18n.tr("Still turning Wi-Fi on…")
                    onCancelled: root.store.cancel("enabled")
                }

                NetRow {
                    host: page
                    rowId: "networks"
                    visible: root.wifiOn && !!root.wifiDev && !root.radioSwitchOff
                    title: I18n.tr("Networks nearby")
                    detail: root.scanning ? I18n.tr("Looking for networks…")
                          : root.inRangeCount === 0 ? I18n.tr("None found yet. Scan to look again.")
                          : root.inRangeCount === 1 ? I18n.tr("One network in range")
                          : I18n.tr("%1 networks in range").arg(root.inRangeCount)

                    Working {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: root.scanning
                        active: visible
                    }
                    Plaque {
                        anchors.verticalCenter: parent.verticalCenter
                        kind: root.showDetails ? "tonal" : "quiet"
                        small: true
                        glyph: "info"
                        text: I18n.tr("Details")
                        accessibleName: I18n.tr("Show each network's access points")
                        Accessible.checkable: true
                        Accessible.checked: root.showDetails
                        onClicked: {
                            root.showDetails = !root.showDetails;
                            if (root.showDetails && !bands.running)
                                bands.running = true;
                        }
                    }
                    Plaque {
                        anchors.verticalCenter: parent.verticalCenter
                        kind: "quiet"
                        small: true
                        glyph: "refresh"
                        text: I18n.tr("Scan")
                        enabled: !root.scanning
                        onClicked: root.scan()
                    }
                }

                Repeater {
                    model: root.wifiOn && !!root.wifiDev && !root.radioSwitchOff ? netModel : null
                    delegate: NetworkItem {}
                }

                Item {
                    readonly property int extra: netModel.count - root.listLimit
                    visible: root.wifiOn && !!root.wifiDev && !root.radioSwitchOff && extra > 0
                    width: parent.width
                    height: visible ? showAll.implicitHeight + LithicTokens.space.s2 : 0

                    Plaque {
                        id: showAll
                        anchors.bottom: parent.bottom
                        kind: "quiet"
                        small: true
                        glyph: root.showAll ? "expand_less" : "expand_more"
                        text: root.showAll ? I18n.tr("Show fewer networks")
                            : parent.extra === 1 ? I18n.tr("Show one more network")
                            : I18n.tr("Show %1 more networks").arg(parent.extra)
                        // folding the list keeps this button where it was, rather than leaving the reader far below it
                        onClicked: {
                            var before = showAll.mapToItem(page.flickable, 0, 0).y;
                            root.showAll = !root.showAll;
                            if (root.showAll)
                                return;
                            wifiColumn.forceLayout();
                            var f = page.flickable;
                            f.contentY = Math.max(0, Math.min(f.contentY + showAll.mapToItem(f, 0, 0).y - before, f.contentHeight - f.height));
                        }
                    }
                }

                Item { width: 1; height: LithicTokens.size.settingsRowGap }

                Disclosure {
                    id: moreFold
                    width: parent.width
                    text: I18n.tr("More options")
                    onToggled: open => root.moreOpen = open

                    Binding {
                        target: moreFold
                        property: "open"
                        value: root.moreOpen
                    }

                    NetRow {
                        id: backendRow
                        readonly property var bind: ({ store: "wifi", key: "backend" })
                        readonly property var row: Schema.rows.find(r => r.id === "wifi-backend")
                        readonly property bool busy: !!root.store && !!root.store.busy["backend"]
                        host: page
                        rowId: "wifi-backend"
                        first: true
                        title: I18n.tr("Wi-Fi backend")
                        detail: backendPace.slow ? I18n.tr("Switching and restarting networking…")
                              : I18n.tr("Try the other one when a WPA3 network or a phone's hotspot will not connect. Switching restarts networking for a moment.")
                        changed: Changes.isChanged(backendRow.row)
                        error: Changes.error(backendRow.bind)
                        onResetRequested: Changes.set(backendRow.bind, "iwd", I18n.tr("Wi-Fi backend"), "wifi")
                        onRetry: Changes.retry(backendRow.bind)

                        Pace { id: backendPace; running: backendRow.busy }

                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: backendPace.slow
                            active: visible
                        }
                        Segmented {
                            id: backendSeg
                            anchors.verticalCenter: parent.verticalCenter
                            enabled: !backendRow.busy && Changes.value(backendRow.bind) !== undefined
                            model: backendRow.row.options.map(o => I18n.tr(o.label))
                            accessibleName: I18n.tr("Wi-Fi backend")
                            onActivated: index => Changes.set(backendRow.bind, backendRow.row.options[index].value, I18n.tr("Wi-Fi backend"), "wifi")
                            // the control assigns its own index on click, so the applied backend keeps driving it
                            Binding {
                                target: backendSeg
                                property: "currentIndex"
                                value: Math.max(0, backendRow.row.options.findIndex(o => o.value === Changes.value(backendRow.bind)))
                            }
                        }
                    }

                    LongWait {
                        visible: backendPace.lagging
                        text: I18n.tr("Still switching. If a password prompt is open, answer it there.")
                        onCancelled: root.store.cancel("backend")
                    }

                    NetRow {
                        id: killRow
                        host: page
                        rowId: "block-all"
                        title: I18n.tr("Block all connections")
                        detail: root.killDetail()
                        error: root.killError !== "" ? root.killError
                             : root.killState === "unknown" ? I18n.tr("Could not read whether connections are blocked.") : ""
                        onRetry: {
                            if (root.killError !== "") {
                                root.setKill(root.killWant);
                            } else {
                                root.killState = "checking";
                                root.readKill();
                            }
                        }

                        Pace { id: killPace; running: root.killBusy }

                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: killPace.slow || killReadPace.slow
                            active: visible
                        }
                        Plaque {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: root.killState !== "unknown"
                            enabled: !root.killBusy && root.killState !== "checking"
                            kind: root.killActive ? "line" : "dangerLine"
                            small: true
                            glyph: root.killActive ? "wifi" : "block"
                            text: root.killActive ? I18n.tr("Allow connections") : I18n.tr("Block all")
                            onClicked: root.askKill()
                        }
                    }

                    LongWait {
                        visible: killPace.lagging || killReadPace.lagging
                        text: root.killBusy ? I18n.tr("Still waiting for the change. If a password prompt is open, answer it there.")
                                            : I18n.tr("Still checking. If a password prompt is open, answer it there.")
                        onCancelled: root.cancelKill()
                    }
                }
            }
        }
    }

    component NetworkItem: Column {
        id: netItem

        required property string ssid
        required property int index
        readonly property var net: root.nets[netItem.ssid] || null
        readonly property bool open: root.expanded === netItem.ssid
        readonly property bool mine: !!root.attempt && root.attempt.ssid === netItem.ssid
        readonly property var bandsHere: root.bandMap[netItem.ssid] || []
        readonly property string error: root.netErrors[netItem.ssid] || ""
        readonly property string apLine: root.showDetails ? root.apLine(netItem.ssid) : ""

        // weak strangers past the first few wait behind Show more, so the page below holds still
        visible: netItem.index < root.listLimit || root.showAll || netItem.open || netItem.mine || netItem.error !== ""
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s2

        Pace { id: netPace; running: netItem.mine }

        NetRow {
            host: page
            title: netItem.ssid
            detail: root.netDetail(netItem.net) + (root.showDetails && netItem.apLine !== "" ? "\n" + netItem.apLine : "")
            activatable: true
            error: netItem.error
            canRetry: !(netItem.net && root.needsPassword(netItem.net))
            onActivated: root.toggleExpanded(netItem.ssid)
            onRetry: root.connect(netItem.net)

            Glyph {
                anchors.verticalCenter: parent.verticalCenter
                visible: root.secured(netItem.net)
                name: "lock"
                ink: Lithic.fg3
                size: LithicTokens.size.iconPlaqueIcon
            }
            Glyph {
                anchors.verticalCenter: parent.verticalCenter
                name: root.signalGlyph(netItem.net)
                ink: Lithic.fg2
                size: LithicTokens.size.iconPlaqueIcon
            }
            Working {
                anchors.verticalCenter: parent.verticalCenter
                visible: netPace.slow || root.disconnecting === netItem.ssid
                active: visible
            }
            Plaque {
                anchors.verticalCenter: parent.verticalCenter
                kind: netItem.net && netItem.net.connected ? "quiet" : "tonal"
                small: true
                enabled: !!netItem.net && !root.connecting && root.disconnecting === ""
                text: netItem.net && netItem.net.connected ? I18n.tr("Disconnect") : I18n.tr("Connect")
                accessibleName: (netItem.net && netItem.net.connected ? I18n.tr("Disconnect from %1") : I18n.tr("Connect to %1")).arg(netItem.ssid)
                onClicked: root.primary(netItem.net)
            }
        }

        // the join panel: band, password, connect
        Column {
            visible: netItem.open
            width: parent.width
            spacing: LithicTokens.space.s3
            bottomPadding: LithicTokens.space.s2

            Item {
                visible: netItem.bandsHere.length > 1
                width: parent.width
                height: Math.max(bandLabel.implicitHeight, bandSeg.implicitHeight)

                Label {
                    id: bandLabel
                    anchors.left: parent.left
                    anchors.right: bandSeg.left
                    anchors.rightMargin: LithicTokens.space.s4
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Band. 5 GHz is faster nearby, 2.4 GHz reaches further.")
                    typeRole: "detail"
                    inkRole: "fg2"
                    wrapMode: Text.Wrap
                    elide: Text.ElideNone
                }
                Segmented {
                    id: bandSeg
                    readonly property var values: ["", "bg", "a"]
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    model: [I18n.tr("Any"), I18n.tr("2.4 GHz"), I18n.tr("5 GHz")]
                    accessibleName: I18n.tr("Band for %1").arg(netItem.ssid)
                    enabled: !root.connecting
                    onActivated: index => root.pickBand(netItem.ssid, bandSeg.values[index])
                    Binding {
                        target: bandSeg
                        property: "currentIndex"
                        value: Math.max(0, bandSeg.values.indexOf(root.chosenBand(netItem.ssid)))
                    }
                }
            }

            TextField {
                id: pwField
                visible: netItem.open && !!netItem.net && root.needsPassword(netItem.net)
                width: parent.width
                label: I18n.tr("Password for %1").arg(netItem.ssid)
                secret: true
                enabled: !root.connecting
                text: root.password
                onTextEdited: root.password = pwField.text
                onAccepted: root.connect(netItem.net)
                onVisibleChanged: if (pwField.visible) Qt.callLater(pwField.forceActiveFocus)

                // typing ends the text binding, so a refused or dropped password is emptied here
                Connections {
                    target: root
                    function onPasswordChanged() {
                        if (pwField.text !== root.password)
                            pwField.text = root.password;
                    }
                    function onAskAgain(ssid) {
                        if (ssid === netItem.ssid && pwField.visible)
                            pwField.forceActiveFocus();
                    }
                }
            }

            Row {
                anchors.right: parent.right
                spacing: LithicTokens.space.s2

                Plaque {
                    kind: "quiet"
                    small: true
                    text: I18n.tr("Close")
                    enabled: !netItem.mine
                    onClicked: root.toggleExpanded(netItem.ssid)
                }
                Plaque {
                    visible: !(netItem.net && netItem.net.connected)
                    kind: "gold"
                    small: true
                    busy: netPace.slow
                    enabled: !root.connecting && (!pwField.visible || root.password.length > 0)
                    text: I18n.tr("Connect")
                    onClicked: root.connect(netItem.net)
                }
            }
        }

        LongWait {
            visible: netPace.lagging
            text: I18n.tr("Still connecting to %1…").arg(netItem.ssid)
            onCancelled: root.cancelConnect()
        }
    }

    Component {
        id: savedBody
        Slab {
            width: parent ? parent.width : 0

            Column {
                width: parent.width

                NetRow {
                    visible: !!root.store && root.store.profiles.length === 0
                    host: page
                    rowId: visible ? "saved" : ""
                    first: true
                    title: root.store && root.store.profilesError !== "" ? I18n.tr("Saved networks could not be read")
                         : !root.store || !root.store.profilesRead ? I18n.tr("Reading saved networks…") : I18n.tr("No saved networks")
                    detail: I18n.tr("A network is saved once you join it")
                    error: root.store ? root.store.profilesError : ""
                    onRetry: root.store.refresh()
                }

                Repeater {
                    model: root.savedIds
                    delegate: NetRow {
                        id: savedRow
                        required property var modelData
                        required property int index
                        readonly property string uuid: String(savedRow.modelData)
                        // the saved network as read last; a row about to go keeps its last values
                        readonly property var p: (root.store && root.store.profile(savedRow.uuid)) || savedRow.last
                        property var last: ({ uuid: savedRow.uuid, name: "", ssid: "", autoconnect: true, timestamp: 0, active: false, band: "" })
                        onPChanged: savedRow.last = savedRow.p
                        readonly property var bind: ({ store: "wifi", key: "autoconnect." + savedRow.uuid })
                        readonly property bool going: root.forgetting === savedRow.uuid
                        readonly property bool joinBusy: !!root.store && !!root.store.busy[savedRow.bind.key]
                        host: page
                        rowId: savedRow.index === 0 ? "saved" : ""
                        first: savedRow.index === 0
                        title: savedRow.p.ssid
                        detail: savedRow.going ? I18n.tr("Forgetting…") : root.savedDetail(savedRow.p)
                        activatable: true
                        changed: savedRow.p.autoconnect === false
                        error: root.savedErrors[savedRow.uuid] || Changes.error(savedRow.bind)
                        onActivated: root.setAutoconnect(savedRow.p, !savedRow.p.autoconnect)
                        onResetRequested: root.setAutoconnect(savedRow.p, true)
                        onRetry: {
                            if (root.savedErrors[savedRow.uuid]) {
                                root.setError("savedErrors", savedRow.uuid, "");
                                root.forget(savedRow.p);
                            } else {
                                Changes.retry(savedRow.bind);
                            }
                        }

                        // Forget and the auto-join switch end in 20 s at most; past 1 s the row says it is working
                        Pace { id: savedPace; running: savedRow.going || savedRow.joinBusy }
                        Working {
                            anchors.verticalCenter: parent.verticalCenter
                            visible: savedPace.slow
                            active: visible
                        }

                        Plaque {
                            anchors.verticalCenter: parent.verticalCenter
                            kind: "quiet"
                            small: true
                            glyph: "delete"
                            text: I18n.tr("Forget")
                            enabled: !savedRow.going
                            accessibleName: I18n.tr("Forget %1").arg(savedRow.p.ssid)
                            onClicked: root.forget(savedRow.p)
                        }
                        Toggle {
                            id: autoToggle
                            anchors.verticalCenter: parent.verticalCenter
                            checked: savedRow.p.autoconnect
                            Accessible.name: I18n.tr("Join %1 on its own").arg(savedRow.p.ssid)
                            onToggled: {
                                var want = autoToggle.checked;
                                autoToggle.checked = Qt.binding(() => savedRow.p.autoconnect);
                                root.setAutoconnect(savedRow.p, want);
                            }
                        }
                    }
                }
            }
        }
    }

    Component {
        id: hotspotBody
        Slab {
            id: hs
            readonly property var onBind: ({ store: "wifi", key: "hotspot.on" })
            readonly property var nameBind: ({ store: "wifi", key: "hotspot.name" })
            readonly property var passBind: ({ store: "wifi", key: "hotspot.password" })
            readonly property bool active: Changes.value(hs.onBind) === true
            readonly property bool busy: !!root.store && (!!root.store.busy["hotspot.on"] || !!root.store.busy["hotspot.name"] || !!root.store.busy["hotspot.password"])
            width: parent ? parent.width : 0

            Pace { id: hsPace; running: hs.busy }

            Column {
                width: parent.width

                NetRow {
                    id: hsRow
                    host: page
                    rowId: "hotspot-on"
                    first: true
                    title: I18n.tr("Hotspot")
                    activatable: !hs.busy
                    unavailable: !root.wifiDev ? I18n.tr("Needs a Wi-Fi adapter")
                               : !root.wifiOn ? I18n.tr("Turn Wi-Fi on to share a hotspot") : ""
                    detail: hsPace.slow ? I18n.tr("Working…")
                          : hs.active ? I18n.tr("On. Other devices can join %1.").arg(Changes.value(hs.nameBind) || "")
                          : I18n.tr("Share this computer's connection over Wi-Fi")
                    error: Changes.error(hs.onBind)
                    onActivated: hsRow.flip(!hs.active)
                    onRetry: Changes.retry(hs.onBind)

                    function flip(on) {
                        Changes.set(hs.onBind, on, I18n.tr("Hotspot"), "wifi", on ? I18n.tr("Hotspot on") : I18n.tr("Hotspot off"));
                    }

                    Working {
                        anchors.verticalCenter: parent.verticalCenter
                        visible: hsPace.slow
                        active: visible
                    }
                    Toggle {
                        id: hsToggle
                        anchors.verticalCenter: parent.verticalCenter
                        enabled: !hs.busy
                        checked: hs.active
                        Accessible.name: I18n.tr("Hotspot")
                        onToggled: {
                            var want = hsToggle.checked;
                            hsToggle.checked = Qt.binding(() => hs.active);
                            hsRow.flip(want);
                        }
                    }
                }

                NetRow {
                    id: nameRow
                    host: page
                    rowId: "hotspot-name"
                    title: I18n.tr("Hotspot name")
                    detail: I18n.tr("The network name other devices see")
                    error: Changes.error(hs.nameBind)
                    onRetry: Changes.retry(hs.nameBind)

                    TextField {
                        id: nameField
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(LithicTokens.size.settingsControlMax, page.columnWidth * 0.45)
                        enabled: !hs.busy
                        placeholderText: "Ryoku"
                        text: Changes.value(hs.nameBind) || ""
                        errorText: nameField.text.trim().length === 0 && nameField.activeFocus ? I18n.tr("Give the hotspot a name") : ""
                        Accessible.name: I18n.tr("Hotspot name")
                        onEditingFinished: {
                            var v = nameField.text.trim();
                            nameField.text = Qt.binding(() => Changes.value(hs.nameBind) || "");
                            if (v.length > 0 && v !== Changes.value(hs.nameBind))
                                Changes.set(hs.nameBind, v, I18n.tr("Hotspot name"), "wifi");
                        }
                    }
                }

                NetRow {
                    id: passRow
                    host: page
                    rowId: "hotspot-password"
                    title: I18n.tr("Hotspot password")
                    detail: I18n.tr("At least 8 characters. One is made for you the first time.")
                    error: Changes.error(hs.passBind)
                    onRetry: Changes.retry(hs.passBind)

                    TextField {
                        id: passField
                        property bool tooShort: false
                        anchors.verticalCenter: parent.verticalCenter
                        width: Math.min(LithicTokens.size.settingsControlMax, page.columnWidth * 0.45)
                        enabled: !hs.busy
                        secret: true
                        mono: true
                        text: Changes.value(hs.passBind) || ""
                        errorText: passField.tooShort ? I18n.tr("Use at least 8 characters") : ""
                        Accessible.name: I18n.tr("Hotspot password")
                        onTextEdited: passField.tooShort = false
                        onEditingFinished: {
                            var v = passField.text;
                            if (v === Changes.value(hs.passBind))
                                return;
                            if (v.length < 8) {
                                passField.tooShort = true;
                                return;
                            }
                            passField.text = Qt.binding(() => Changes.value(hs.passBind) || "");
                            Changes.set(hs.passBind, v, I18n.tr("Hotspot password"), "wifi");
                        }
                    }
                }

                LongWait {
                    visible: hsPace.lagging
                    text: hs.active ? I18n.tr("Still changing the hotspot…") : I18n.tr("Still starting the hotspot…")
                    onCancelled: ["hotspot.on", "hotspot.name", "hotspot.password"].forEach(k => root.store.cancel(k))
                }
            }
        }
    }
}

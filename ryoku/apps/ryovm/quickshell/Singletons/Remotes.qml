pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// The remote fleet: SSH hosts and VPS drawn from ~/.ssh/config (plus ryoport's
// own include file), their live reachability, and on-demand health probes. All
// driven through the `ryossh` engine, which speaks JSON. The list is the facts;
// reach and health are kept in maps beside it so a poll never rebuilds the list.
Singleton {
    id: root

    property bool active: false          // a shown page flips this; polling gates on it
    property var hosts: []
    property bool loading
    property bool probing
    property bool engineOk: true         // ryossh present and answering
    property string selectedAlias: ""

    // alias -> { up, rttMs, sshUp } and alias -> full probe object.
    property var reach: ({})
    property var health: ({})
    property var metricHistory: ({})
    property var metricPrevious: ({})
    property int reachRev: 0
    property int healthRev: 0
    property int metricsRevision: 0
    function reachOf(a) { void reachRev; return reach[a] || null; }
    function healthOf(a) { void healthRev; return health[a] || null; }

    // alias|name -> { state, ms, code } for each host's web apps (glance-style).
    property var appStatus: ({})
    property int appRev: 0
    function appStatusOf(alias, name) { void appRev; return appStatus[alias + "|" + name] || null; }

    // alias -> [ {vmid,name,status,node,type,cpu,mem,maxmem,uptime} ] Proxmox guests.
    property var guests: ({})
    property int guestsRev: 0
    function guestsOf(a) { void guestsRev; return guests[a] || []; }
    function isProxmox(h) { return !!(h && h.pve && h.pve.url && h.pve.url.length > 0); }

    // guests mid-transition after start/stop, so a row shows pending until the
    // cluster reports the new status. key alias|vmid -> { want, since }.
    property var guestBusy: ({})
    property int guestBusyRev: 0
    function guestBusyOf(a, vmid) { void guestBusyRev; return !!guestBusy[a + "|" + vmid]; }

    property var keysData: ({ agent: [], files: [] })

    // A short session log of fleet actions and observed health transitions,
    // newest last. Dashboard and detail views share it with Vm.events.
    property var events: []
    function logEvent(kind, alias, text) {
        var d = new Date();
        function p(n) { return (n < 10 ? "0" : "") + n; }
        var e = events.slice();
        e.push({ time: p(d.getHours()) + ":" + p(d.getMinutes()) + ":" + p(d.getSeconds()),
                 at: d.getTime(), alias: alias, kind: kind, text: text });
        if (e.length > 100) e = e.slice(e.length - 100);
        events = e;
    }

    function stateReason(a, state) {
        state = state || stateOf(a);
        var h = healthOf(a);
        var r = reachOf(a);
        if (state === "down")
            return I18n.tr("connection unavailable");
        if (state === "up" && r && r.rttMs >= 0)
            return I18n.tr("reachable in %1 ms").arg(r.rttMs);
        if (state !== "warn" || !h)
            return "";
        var reasons = [];
        var memPct = h.memTotalKb > 0 ? Math.round(100 * (h.memTotalKb - h.memAvailKb) / h.memTotalKb) : 0;
        if (h.diskPct >= 90)
            reasons.push(I18n.tr("%1% disk used").arg(h.diskPct));
        if (memPct >= 90)
            reasons.push(I18n.tr("%1% memory used").arg(memPct));
        if (h.cpus > 0 && h.load1 > h.cpus)
            reasons.push(I18n.tr("load %1 on %2 cores").arg(Number(h.load1).toFixed(1)).arg(h.cpus));
        if (h.failedUnits > 0)
            reasons.push(I18n.tr("%1 failed services").arg(h.failedUnits));
        return reasons.join(", ");
    }

    function _recordStateChange(alias, before) {
        var after = stateOf(alias);
        if (after === before || after === "unknown")
            return;
        var labels = { up: I18n.tr("UP"), warn: I18n.tr("DEGRADED"), down: I18n.tr("DOWN") };
        var text = before === "unknown"
            ? I18n.tr("Status is %1").arg(labels[after])
            : I18n.tr("Status changed from %1 to %2").arg(labels[before] || before.toUpperCase()).arg(labels[after]);
        var reason = stateReason(alias, after);
        logEvent("status", alias, reason.length > 0 ? text + ": " + reason : text);
    }

    readonly property var selected: {
        for (var i = 0; i < hosts.length; i++)
            if (hosts[i].alias === selectedAlias)
                return hosts[i];
        return null;
    }
    readonly property int hostCount: hosts.length
    readonly property int upCount: {
        void reachRev;
        var n = 0;
        for (var i = 0; i < hosts.length; i++) {
            var r = reach[hosts[i].alias];
            if (r && r.up === true) n++;
        }
        return n;
    }

    // a host's rolled-up state, by word (never colour): the design's language.
    //   up | warn | down | unknown | probing
    function stateOf(a) {
        var h = healthOf(a);
        var r = reachOf(a);
        if (r && r.up === false) return "down";
        if (h && h.ok === true) {
            var memPct = h.memTotalKb > 0 ? 100 * (h.memTotalKb - h.memAvailKb) / h.memTotalKb : 0;
            var loadHot = h.cpus > 0 && h.load1 > h.cpus;
            if (h.diskPct >= 90 || memPct >= 90 || loadHot || h.failedUnits > 0) return "warn";
            return "up";
        }
        if (r && r.up === true) return "up";
        return "unknown";
    }

    function human(b) {
        b = +b || 0;
        if (b <= 0) return "0";
        var u = ["B", "KB", "MB", "GB", "TB"], i = 0;
        while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
        return (b < 10 && i > 0 ? b.toFixed(1) : Math.round(b)) + " " + u[i];
    }
    function uptimeShort(s) {
        s = +s || 0;
        var d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
        if (d > 0) return d + "d " + h + "h";
        if (h > 0) return h + "h " + m + "m";
        return m + "m";
    }

    Component.onCompleted: root.refresh()

    function refresh() { listProc.running = true; loadTunnels(); }

    // ---- tunnels (ssh -L / -R / -D), tracked by the engine ------------------
    property var tunnels: []
    function tunnelsFor(a) {
        var out = [];
        for (var i = 0; i < tunnels.length; i++)
            if (tunnels[i].alias === a) out.push(tunnels[i]);
        return out;
    }
    function loadTunnels() { tunnelListProc.running = true; }
    function openTunnel(alias, spec) {
        tunnelOpenProc.forAlias = alias;
        tunnelOpenProc.spec = spec;
        tunnelOpenProc.command = ["ryossh", "tunnel", "open", alias, spec];
        tunnelOpenProc.running = true;
    }
    function closeTunnel(id) {
        tunnelCloseProc.forAlias = "";
        for (var i = 0; i < tunnels.length; i++)
            if (tunnels[i].id === id) { tunnelCloseProc.forAlias = tunnels[i].alias || ""; break; }
        tunnelCloseProc.tunnelId = id;
        tunnelCloseProc.command = ["ryossh", "tunnel", "close", id];
        tunnelCloseProc.running = true;
    }
    function select(a) {
        selectedAlias = a;
        if (a.length > 0) { root.probe(a); root.appCheck(a); root.loadGuests(a); }
    }
    function pingAll() { if (hosts.length > 0) pingProc.running = true; }
    function probeAll() { if (hosts.length > 0 && !probeProc.running) { probing = true; probeProc.running = true; } }
    function probe(a) {
        if (!a || a.length === 0) return;
        oneProbe.command = ["ryossh", "probe", a];
        oneProbe.running = true;
    }
    function appCheckAll() { if (hosts.length > 0) appCheckProc.running = true; }
    function appCheck(a) { if (a && a.length > 0) { appCheckOne.command = ["ryossh", "appcheck", a]; appCheckOne.running = true; } }

    function _rate(now, previous, sample, key) {
        if (!previous || !previous.sample || previous.sample[key] === undefined || sample[key] === undefined)
            return 0;
        var seconds = Math.max(0.001, (now - previous.at) / 1000);
        return Math.max(0, (+sample[key] - +previous.sample[key]) / seconds);
    }
    function _recordMetrics(alias, sample) {
        if (!sample || sample.ok !== true)
            return;
        var now = Date.now();
        var previous = metricPrevious[alias] || null;
        var memPct = sample.memTotalKb > 0
            ? 100 * (sample.memTotalKb - sample.memAvailKb) / sample.memTotalKb : 0;
        var point = {
            at: now,
            cpu: Math.max(0, +sample.cpuPercent || 0),
            ram: Math.max(0, Math.min(100, memPct)),
            disk: _rate(now, previous, sample, "diskReadBytes") + _rate(now, previous, sample, "diskWriteBytes"),
            net: _rate(now, previous, sample, "netRxBytes") + _rate(now, previous, sample, "netTxBytes")
        };
        var histories = metricHistory;
        var history = (histories[alias] || []).slice();
        history.push(point);
        if (history.length > 60)
            history = history.slice(history.length - 60);
        histories[alias] = history;
        metricHistory = histories;
        var prior = metricPrevious;
        prior[alias] = { at: now, sample: sample };
        metricPrevious = prior;
        metricsRevision++;
    }
    function historyFor(alias) {
        void metricsRevision;
        return metricHistory[alias] || [];
    }
    function series(alias, key) {
        var history = historyFor(alias), values = [];
        for (var i = 0; i < history.length; i++)
            values.push(+history[i][key] || 0);
        return values;
    }
    function loadGuests(a) {
        if (!a || a.length === 0) return;
        var h = null;
        for (var i = 0; i < hosts.length; i++) if (hosts[i].alias === a) { h = hosts[i]; break; }
        if (!root.isProxmox(h)) return;
        guestsProc.forAlias = a;
        guestsProc.command = ["ryossh", "pveguests", a];
        guestsProc.running = true;
    }
    function pveAct(a, node, type, vmid, action) {
        var want = action === "start" ? "running" : ((action === "shutdown" || action === "stop") ? "stopped" : "");
        if (want.length > 0) {
            var b = guestBusy; b[a + "|" + vmid] = { want: want, since: Date.now() }; guestBusy = b; guestBusyRev++;
        }
        pveActProc.forAlias = a;
        pveActProc.command = ["ryossh", "pveaction", a, node, type, String(vmid), action];
        pveActProc.running = true;
        logEvent(action, a, action + " " + type + "/" + vmid);
    }
    function connect(a) {
        connectProc.forAlias = a;
        connectProc.command = ["ryossh", "connect", a];
        connectProc.running = true;
    }
    function loadKeys() { keysProc.running = true; }
    // ssh-copy-id is interactive (it may prompt for a password), so it runs in a
    // real terminal; hold on the result so the key-added line or an auth error
    // stays readable instead of vanishing when the window closes.
    function copyId(a) {
        Quickshell.execDetached(["sh", "-c",
            "exec \"${TERMINAL:-kitty}\" --class=dev.ryoku.ryoport_ssh -e sh -c 'ryossh copyid \"$1\"; printf \"\\n── press enter to close ──\\n\"; read _' _ \"$1\"", "--", a]);
    }
    // run one command on the host in a TTY, then hold on its output with a local
    // read so a fast command like df stays readable instead of being buried under
    // a fresh shell; the remote command stays unwrapped so non-POSIX shells can't mangle it.
    function runOn(alias, cmd) {
        Quickshell.execDetached(["sh", "-c",
            "exec \"${TERMINAL:-kitty}\" --class=dev.ryoku.ryoport_ssh -e sh -c 'ssh -t \"$1\" \"$2\"; printf \"\\n── press enter to close ──\\n\"; read _' _ \"$1\" \"$2\"", "--", alias, cmd]);
        logEvent("run", alias, I18n.tr("%1 on %2").arg(cmd.split(" ")[0]).arg(alias));
    }
    // browse and transfer files over SFTP in the file manager. nautilus (the
    // shipped GUI file manager) auto-mounts the gvfs sftp location and opens it;
    // `gio open` needs it mounted first and the xdg sftp handler is unset here.
    function openFiles(host) {
        if (!host) return;
        var u = host.user && host.user.length > 0 ? host.user + "@" : "";
        var h = host.hostName && host.hostName.length > 0 ? host.hostName : host.alias;
        var p = host.port && host.port !== 22 ? ":" + host.port : "";
        Quickshell.execDetached(["nautilus", "sftp://" + u + h + p + "/"]);
    }
    // open a host's web app in the default browser.
    function openApp(url) { if (url && url.length > 0) Quickshell.execDetached(["xdg-open", url]); }
    function addHost(obj, pw, clearPw) {
        addProc.alias = obj.alias;
        addProc.pw = pw || "";
        addProc.clearPw = !!clearPw;
        addProc.command = ["ryossh", "add", JSON.stringify(obj)];
        addProc.running = true;
        logEvent("add", obj.alias, I18n.tr("saved %1").arg(obj.alias));
    }
    function setPass(alias, pw) {
        setPassProc.pw = pw;
        setPassProc.command = ["ryossh", "setpass", alias];
        setPassProc.running = true;
    }
    function clearPass(alias) {
        clearPassProc.command = ["ryossh", "clearpass", alias];
        clearPassProc.running = true;
    }
    function removeHost(a) {
        rmProc.command = ["ryossh", "remove", a];
        rmProc.running = true;
        logEvent("remove", a, I18n.tr("forgot %1").arg(a));
    }

    function _mergeReach(arr) {
        var before = {};
        for (var i = 0; i < arr.length; i++)
            before[arr[i].alias] = stateOf(arr[i].alias);
        var m = {};
        for (i = 0; i < arr.length; i++)
            m[arr[i].alias] = arr[i];
        reach = m;
        reachRev++;
        for (i = 0; i < arr.length; i++)
            _recordStateChange(arr[i].alias, before[arr[i].alias]);
    }
    function _mergeHealth(arr) {
        var before = {};
        for (var i = 0; i < arr.length; i++)
            before[arr[i].alias] = stateOf(arr[i].alias);
        var m = health;
        for (i = 0; i < arr.length; i++) {
            m[arr[i].alias] = arr[i];
            root._recordMetrics(arr[i].alias, arr[i]);
        }
        health = m;
        healthRev++;
        for (i = 0; i < arr.length; i++)
            _recordStateChange(arr[i].alias, before[arr[i].alias]);
    }
    function _mergeApps(arr) {
        var m = root.appStatus;
        for (var i = 0; i < arr.length; i++) m[arr[i].alias + "|" + arr[i].name] = arr[i];
        root.appStatus = m; root.appRev++;
    }
    function _mergeGuests(a, arr) {
        var m = root.guests;
        m[a] = arr;
        root.guests = m; root.guestsRev++;
        var b = root.guestBusy; var changed = false; var now = Date.now();
        for (var i = 0; i < arr.length; i++) {
            var k = a + "|" + arr[i].vmid;
            if (b[k] && (arr[i].status === b[k].want || now - b[k].since > 45000)) { delete b[k]; changed = true; }
        }
        if (changed) { root.guestBusy = b; root.guestBusyRev++; }
    }

    Process {
        id: listProc
        command: ["ryossh", "list"]
        property string last: ""
        onStarted: root.loading = true
        stdout: StdioCollector {
            onStreamFinished: {
                root.loading = false;
                root.engineOk = true;
                if (this.text === listProc.last) return;
                listProc.last = this.text;
                try {
                    var arr = JSON.parse(this.text);
                    root.hosts = Array.isArray(arr) ? arr : [];
                    if (root.selectedAlias.length === 0 && root.hosts.length > 0)
                        root.select(root.hosts[0].alias);
                    root.pingAll();
                    root.probeAll();
                    root.appCheckAll();
                } catch (e) { root.hosts = []; }
            }
        }
        onExited: (code) => { if (code !== 0) { root.loading = false; root.engineOk = false; root.hosts = []; } }
    }

    Process {
        id: pingProc
        command: ["ryossh", "pingall"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { root._mergeReach(JSON.parse(this.text) || []); } catch (e) {}
            }
        }
    }

    Process {
        id: probeProc
        command: ["ryossh", "probeall"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.probing = false;
                try { root._mergeHealth(JSON.parse(this.text) || []); } catch (e) {}
            }
        }
        onExited: root.probing = false
    }

    Process {
        id: oneProbe
        stdout: StdioCollector {
            onStreamFinished: {
                try { root._mergeHealth([JSON.parse(this.text)]); } catch (e) {}
            }
        }
    }
    Process {
        id: appCheckProc
        command: ["ryossh", "appcheckall"]
        stdout: StdioCollector {
            onStreamFinished: { try { root._mergeApps(JSON.parse(this.text) || []); } catch (e) {} }
        }
    }
    Process {
        id: appCheckOne
        stdout: StdioCollector {
            onStreamFinished: { try { root._mergeApps(JSON.parse(this.text) || []); } catch (e) {} }
        }
    }
    Process {
        id: guestsProc
        property string forAlias: ""
        stdout: StdioCollector {
            onStreamFinished: { try { root._mergeGuests(guestsProc.forAlias, JSON.parse(this.text) || []); } catch (e) {} }
        }
    }
    Process {
        id: pveActProc
        property string forAlias: ""
        onExited: (code) => { if (code === 0) { root.loadGuests(pveActProc.forAlias); guestReload.restart(); } }
    }
    Timer { id: guestReload; interval: 1800; onTriggered: root.loadGuests(root.selectedAlias) }
    Timer {
        id: guestSettle
        interval: 2500
        repeat: true
        running: root.active && Object.keys(root.guestBusy).length > 0
        onTriggered: root.loadGuests(root.selectedAlias)
    }

    Process {
        id: connectProc
        property string forAlias: ""
        onExited: (code) => {
            if (code === 0)
                root.logEvent("connect", forAlias, I18n.tr("opened a session to %1").arg(forAlias));
            else
                root.logEvent("connect", forAlias, I18n.tr("could not open a session to %1").arg(forAlias));
        }
    }
    Process {
        id: addProc
        property string alias: ""
        property string pw: ""
        property bool clearPw: false
        onExited: (code) => {
            if (code === 0 && addProc.pw.length > 0) root.setPass(addProc.alias, addProc.pw);
            else if (code === 0 && addProc.clearPw) root.clearPass(addProc.alias);
            else root.refresh();
            addProc.pw = "";
        }
    }
    Process {
        id: setPassProc
        stdinEnabled: true
        property string pw: ""
        stdout: StdioCollector {}
        stderr: StdioCollector {}
        onStarted: { write(setPassProc.pw + "\n"); setPassProc.pw = ""; }
        onExited: root.refresh()
    }
    Process { id: clearPassProc; onExited: root.refresh() }
    Process { id: rmProc; onExited: (code) => { if (code === 0) { root.selectedAlias = ""; root.refresh(); } } }

    Process {
        id: keysProc
        command: ["ryossh", "keys"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { root.keysData = JSON.parse(this.text); } catch (e) {}
            }
        }
    }

    Process {
        id: tunnelListProc
        command: ["ryossh", "tunnel", "list"]
        stdout: StdioCollector {
            onStreamFinished: { try { root.tunnels = JSON.parse(this.text) || []; } catch (e) { root.tunnels = []; } }
        }
    }
    Process {
        id: tunnelOpenProc
        property string forAlias: ""
        property string spec: ""
        onExited: (code) => {
            root.loadTunnels();
            if (code === 0)
                root.logEvent("tunnel", forAlias, I18n.tr("opened tunnel %1").arg(spec));
        }
    }
    Process {
        id: tunnelCloseProc
        property string forAlias: ""
        property string tunnelId: ""
        onExited: (code) => {
            root.loadTunnels();
            if (code === 0)
                root.logEvent("tunnel", forAlias, I18n.tr("closed tunnel %1").arg(tunnelId));
        }
    }

    // reachability on a short cadence; the fuller health probe less often. Both
    // gate on a page being on screen, so a hidden hub costs nothing.
    Timer {
        interval: 15000
        repeat: true
        running: root.active
        onTriggered: { root.pingAll(); root.loadTunnels(); root.appCheckAll(); root.loadGuests(root.selectedAlias); }
    }
    Timer {
        interval: 15000
        repeat: true
        running: root.active
        triggeredOnStart: true
        onTriggered: root.probeAll()
    }
}

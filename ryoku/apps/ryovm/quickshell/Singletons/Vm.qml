pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// The app's state: host readiness, the local VM library, the quickget OS
// catalogue, and the lifecycle pipeline, all driven through the `ryovm` engine.
// One quickemu .conf per VM, so the library is naturally multi-VM.
Singleton {
    id: root

    // ---- host readiness -----------------------------------------------------
    property var caps: ({})
    readonly property bool capsLoaded: caps.ready !== undefined
    readonly property bool ready: caps.ready === true
    readonly property bool kvmOff: capsLoaded && caps.kvm === false

    // ---- library ------------------------------------------------------------
    property var vms: []
    property bool vmsLoading
    property string selectedName: ""
    property var detail: null            // full `get` of the selected VM
    property string pendingSelect: ""    // select this VM after the next reload (post-rename)
    readonly property var selected: {
        for (var i = 0; i < vms.length; i++)
            if (vms[i].name === selectedName)
                return vms[i];
        return null;
    }

    // ---- catalogue (browse + create) ----------------------------------------
    property var osList: []              // grouped by OS, each with releases/editions
    property bool catalogLoading
    property bool catalogReady           // the on-disk catalogue cache exists (icons resolvable)
    property string catalogError
    property var selectedOs: null        // an osList entry, in Catalog mode
    property var paths: ({})             // engine paths + provider, for Settings
    // OSes that actually have brand art upstream (the rest get a monogram). The
    // prefetch verb warms every logo in parallel and returns this set, so the
    // Catalog can float logo-bearing systems to a "Popular" section.
    property var iconSet: ({})
    function hasArt(slug) { return slug && iconSet[slug] === true; }

    // ---- OS logo cache (real SVG brand marks, cached to disk by the engine) --
    // memoised slug -> local file path (or "" when an OS has no logo), so a card
    // resolves each logo once instead of per paint/recycle. iconRev forces the
    // bindings that read iconCache to re-evaluate when an entry lands.
    property var iconCache: ({})
    property var iconPending: ({})
    property int iconRev: 0
    function iconFor(slug) { void iconRev; return (slug && iconCache[slug] !== undefined) ? iconCache[slug] : ""; }
    function beginIcon(slug) {
        if (!slug || !catalogReady || iconCache[slug] !== undefined || iconPending[slug])
            return false;
        iconPending[slug] = true;
        return true;
    }
    function setIcon(slug, path) { var c = iconCache; c[slug] = path; iconCache = c; iconPending[slug] = false; iconRev++; }

    // ---- pipeline -----------------------------------------------------------
    property bool busy
    property string status
    // the sticky fault surface: errors stay up until dismissed or the next
    // verb succeeds: a fault that clears itself after 4.5s never happened.
    property string fault: ""            // first line, for the fault row
    property string faultDetail: ""      // full engine stderr, un-truncated

    // ---- the yard log (the flight recorder) ---------------------------------
    // Every receipt and fault already funnels through info()/raiseFault(); the
    // log is those two functions growing memory instead of evaporating after
    // 4.5s. Session-scoped, capped at 200, tagged with the machine in focus so
    // the detail sheet can show one machine's history.
    property var events: []
    function _log(kind, text, detail, focus) {
        var s = ("" + text).trim();
        if (s.length === 0)
            return;
        var d = new Date();
        function p(n) { return (n < 10 ? "0" : "") + n; }
        var stamp = p(d.getHours()) + ":" + p(d.getMinutes()) + ":" + p(d.getSeconds());
        var e = events.slice();
        e.push({ time: stamp, at: d.getTime(), vm: (focus && focus.length > 0) ? focus : selectedName, kind: kind, text: s, detail: detail || "" });
        if (e.length > 200)
            e = e.slice(e.length - 200);
        events = e;
    }

    function raiseFault(text, focus) {
        var t = ("" + text).trim();
        if (t.length === 0)
            return;
        fault = t.split("\n")[0];
        faultDetail = t;
        status = "";
        _log("fault", fault, faultDetail, focus);
    }
    function clearFault() { fault = ""; faultDetail = ""; }
    function info(msg, focus) { status = msg; _log("info", msg, "", focus); }

    // ---- in-app downloads (parallel; the fast Go fetcher, streamed live) -----
    // each build is a row in dlJobs run by its own process (the Instantiator
    // below), so several machines download at once; downloading/dlName stay as
    // rolled-up reads for callers that still want a single glance.
    property int maxParallel: 4
    readonly property bool downloading: dlJobs.count > 0
    readonly property int dlCount: dlJobs.count
    readonly property string dlName: dlJobs.count > 0 ? dlJobs.get(0).name : ""
    property var dlJobs: dlJobsModel
    ListModel { id: dlJobsModel }

    // ---- settings (persisted to ~/.config/ryoku/ryovm.json) -----------------
    readonly property var settings: cfg

    function refresh() {
        capsProc.running = true;
        listProc.running = true;
    }

    Component.onCompleted: {
        root.refresh();
        // warm the catalogue cache so Library logos resolve even before the user
        // opens Catalog (cached to disk by the engine, so it's a no-op after the
        // first run), and read the engine paths for the Settings panel.
        root.loadCatalog(false);
        pathsProc.running = true;
    }

    // info lines clear themselves, but not while an operation runs, so a long
    // seal/restore keeps "Sealing alpine" pinned instead of decaying to mystery.
    onStatusChanged: if (status.length > 0 && !busy) statusClear.restart()
    onBusyChanged: if (!busy && status.length > 0) statusClear.restart()
    Timer { id: statusClear; interval: 4500; onTriggered: root.status = "" }
    // bytes to a short human size ("1.9 GB", "880 MB"), for disk footprints.
    function human(b) {
        b = +b || 0;
        if (b <= 0)
            return "0";
        var u = ["B", "KB", "MB", "GB", "TB"], i = 0;
        while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
        return (b < 10 && i > 0 ? b.toFixed(1) : Math.round(b)) + " " + u[i];
    }

    // ---- library ------------------------------------------------------------
    function select(name) {
        // re-selecting the same machine (the 5s poll) keeps the stale detail
        // on screen until the fresh one lands: nulling it blinked every
        // det-gated section once a poll.
        if (name !== selectedName || detail === null)
            detail = null;
        selectedName = name;
        if (name.length > 0) {
            getProc.command = ["ryovm", "get", name];
            getProc.running = true;
            root.loadUsb(name);
            root.loadPortfwd(name);
        }
    }
    function reselect() { if (selectedName.length > 0) select(selectedName); }

    function launch(name, mode, disposable) {
        if (busy)
            return;
        busy = true;
        status = disposable ? I18n.tr("Starting %1 (disposable)").arg(name) : I18n.tr("Starting %1").arg(name);
        var cmd = ["ryovm", "launch", name, mode || "window"];
        if (disposable === true)
            cmd.push("--disposable");
        runProc.exec(cmd);
    }
    // seal = the golden-state anchor: one reserved snapshot + the conf stamp.
    function seal(name) {
        if (busy)
            return;
        busy = true;
        status = I18n.tr("Sealing %1").arg(name);
        runProc.exec(["ryovm", "seal", name]);
    }
    function restoreSeal(name) {
        if (busy)
            return;
        busy = true;
        status = I18n.tr("Restoring the seal on %1").arg(name);
        runProc.exec(["ryovm", "restore-seal", name]);
    }
    // template = freeze this machine (tools + all) into a reusable golden base;
    // spawn = a thin clone off it that boots in seconds with everything baked.
    function template(name) {
        if (busy) return;
        busy = true;
        status = I18n.tr("Saving %1 as a template").arg(name);
        runProc.exec(["ryovm", "template", name, name]);
    }
    function stop(name) {
        if (busy)
            return;
        busy = true;
        status = I18n.tr("Stopping %1").arg(name);
        runProc.exec(["ryovm", "stop", name]);
    }
    function openConsole(name) { runProc.exec(["ryovm", "console", name]); }
    function deleteVm(name) {
        if (busy)
            return;
        busy = true;
        status = I18n.tr("Deleting %1").arg(name);
        runProc.exec(["ryovm", "delete", name]);
    }
    function setConfig(name, key, value) { runProc.exec(["ryovm", "config", name, key, "" + value]); }
    function snapshot(name, sub, tag) {
        if (busy)
            return;
        busy = true;
        status = ({ "create": I18n.tr("Saving snapshot"), "restore": I18n.tr("Restoring snapshot"), "delete": I18n.tr("Deleting snapshot") })[sub] || I18n.tr("Working");
        runProc.exec(["ryovm", "snapshot", name, sub].concat(tag ? [tag] : []));
    }
    function openFolder(name) {
        folderProc.command = ["ryovm", "folder"].concat(name ? [name] : []);
        folderProc.running = true;
    }
    function openSsh(name) {
        sshProc.vmName = name;
        sshProc.command = ["ryovm", "ssh", name];
        sshProc.running = true;
    }
    // copy the ready-to-paste ssh command for a running VM.
    function copySsh(name) {
        copyProc.command = ["sh", "-c", "ryovm ssh \"$1\" | tr -d '\\n' | wl-copy", "--", name];
        copyProc.running = true;
    }
    // host USB devices + this VM's assignments (usb_devices in its conf).
    property var usb: []
    function loadUsb(name) {
        if (!name || name.length === 0) { usb = []; return; }
        usbProc.command = ["ryovm", "usb", "list", name];
        usbProc.running = true;
    }
    function setUsb(name, id, on) {
        usbSetProc.command = ["ryovm", "usb", "set", name, id, on ? "on" : "off"];
        usbSetProc.running = true;
    }

    // host->guest port forwards (port_forwards in its conf), for local VMs.
    property var portfwds: []
    function loadPortfwd(name) {
        if (!name || name.length === 0) { portfwds = []; return; }
        portfwdProc.command = ["ryovm", "portfwd", "list", name];
        portfwdProc.running = true;
    }
    function addPortfwd(name, spec) {
        portfwdProc2.command = ["ryovm", "portfwd", "add", name, spec];
        portfwdProc2.running = true;
    }
    function removePortfwd(name, spec) {
        portfwdProc2.command = ["ryovm", "portfwd", "remove", name, spec];
        portfwdProc2.running = true;
    }
    // rename repoints the conf, dir and relative paths, then reselects the new
    // name once the reloaded list carries it (via pendingSelect).
    function renameVm(name, next) {
        if (busy || !next || next === name)
            return;
        busy = true;
        status = I18n.tr("Renaming to %1").arg(next);
        pendingSelect = next;
        runProc.exec(["ryovm", "rename", name, next]);
    }
    function resizeDisk(name, size) {
        if (busy)
            return;
        busy = true;
        status = I18n.tr("Resizing %1 disk").arg(name);
        runProc.exec(["ryovm", "resize", name, "" + size]);
    }
    // reclaim frees the disk image (and re-installable media) but keeps the
    // machine's config, so it can be reinstalled later.
    function reclaimDisk(name) {
        if (busy)
            return;
        busy = true;
        status = I18n.tr("Reclaiming %1 disk").arg(name);
        runProc.exec(["ryovm", "delete", name, "--disk-only"]);
    }

    // ---- live control of a running VM (via the `ryovm mon` verb) -----------
    // Pause, resume, reset, reballoon and pin a running machine live, and read
    // its host-side cost and guest IP, through the sockets quickemu already
    // opens. monStats is the last reading of the selected machine.
    property var monStats: ({})
    property bool monWatch: false        // set true only while the machine stage is on screen
    readonly property bool monRunning: selected ? selected.running === true : false
    onSelectedNameChanged: monStats = ({})
    function monRefresh() {
        if (!monRunning || selectedName.length === 0) { monStats = ({}); return; }
        monProc.command = ["ryovm", "mon", selectedName, "stats"];
        monProc.running = true;
    }
    function power(name, action) { monActProc.command = ["ryovm", "mon", name, "power", action]; monActProc.running = true; }
    function balloon(name, mb) { monActProc.command = ["ryovm", "mon", name, "balloon", "" + Math.round(mb)]; monActProc.running = true; }
    function pin(name, mode) { monActProc.command = ["ryovm", "mon", name, "pin", mode || "auto"]; monActProc.running = true; }

    // ---- catalogue ----------------------------------------------------------
    function loadCatalog(force) {
        if (osList.length > 0 && !force)
            return;
        catalogLoading = true;
        catalogError = "";
        catProc.command = force ? ["ryovm", "catalog", "--refresh"] : ["ryovm", "catalog"];
        catProc.running = true;
    }
    function selectOs(entry) { selectedOs = entry; }

    // ---- instant (prebuilt cloud image + ryoku burn account) ----------------
    property var cloudList: []
    property bool cloudLoading
    function loadCloud() {
        if (cloudList.length > 0) return;
        cloudLoading = true;
        cloudProc.running = true;
    }
    // instant reuses the same streaming pipeline as create (download/config/done
    // JSON); it just queues as one more parallel build with a disposable flag.
    function instant(os, name, disposable, tools, pkgs) {
        var nm = name && name.length > 0 ? name : os + "-instant";
        if (_building(nm)) return;
        if (dlJobs.count >= maxParallel) { info(I18n.tr("Up to %1 downloads at once").arg(maxParallel)); return; }
        dlJobs.append({ key: nm + "#" + Date.now(), name: nm, cmdName: nm, kind: "instant",
            os: os, release: "", edition: "", disposable: disposable === true,
            tools: tools || "", pkgs: pkgs || "", phase: "resolve", progress: 0, bps: 0,
            indet: true, log: "", cancel: false });
        status = I18n.tr("Building %1").arg(nm);
    }

    // create downloads the image in-app with a live bar (the Go fetcher streams
    // progress), so several can run at once and none needs a detached terminal.
    function createVm(os, release, edition) {
        var nm = os + "-" + release + (edition ? "-" + edition : "");
        if (_building(nm)) return;
        if (dlJobs.count >= maxParallel) { info(I18n.tr("Up to %1 downloads at once").arg(maxParallel)); return; }
        dlJobs.append({ key: nm + "#" + Date.now(), name: nm, cmdName: nm, kind: "create",
            os: os, release: release, edition: edition || "", disposable: false,
            tools: "", pkgs: "", phase: "resolve", progress: 0, bps: 0,
            indet: false, log: "", cancel: false });
        status = I18n.tr("Downloading %1").arg(os);
    }
    function _building(name) {
        for (var i = 0; i < dlJobs.count; i++) if (dlJobs.get(i).name === name) return true;
        return false;
    }
    function _jobIdx(key) {
        for (var i = 0; i < dlJobs.count; i++) if (dlJobs.get(i).key === key) return i;
        return -1;
    }
    // cancelling a build flips its row's cancel flag; the row's process watches
    // it and SIGTERMs, which the engine traps to wipe the half-image. cancelCreate
    // stops every build (the quit guard's blunt lever).
    function cancelJob(key) { var i = _jobIdx(key); if (i >= 0) dlJobs.setProperty(i, "cancel", true); }
    function cancelCreate() { for (var i = 0; i < dlJobs.count; i++) dlJobs.setProperty(i, "cancel", true); }
    function _onJobLine(key, line) {
        var i = _jobIdx(key);
        if (i < 0) return;
        var s = ("" + line).trim();
        if (s.length === 0) return;
        var o;
        try { o = JSON.parse(s); } catch (e) { return; }
        if (o.event !== undefined) {
            if (o.total > 0) { dlJobs.setProperty(i, "progress", Math.max(0, Math.min(1, o.recv / o.total))); dlJobs.setProperty(i, "indet", false); }
            if (o.bps !== undefined) dlJobs.setProperty(i, "bps", o.bps);
            return;
        }
        switch (o.phase) {
        case "resolve": dlJobs.setProperty(i, "phase", "resolve"); break;
        case "config": dlJobs.setProperty(i, "phase", "config"); dlJobs.setProperty(i, "progress", 1); break;
        case "download":
            dlJobs.setProperty(i, "phase", "download");
            if (o.name) dlJobs.setProperty(i, "name", o.name);
            if (o.fallback === true) dlJobs.setProperty(i, "indet", true);
            break;
        case "log": dlJobs.setProperty(i, "log", o.line || ""); break;
        case "done":
            status = I18n.tr("%1 is ready").arg(o.name || dlJobs.get(i).name);
            if (o.name) { selectedName = o.name; root.created(o.name); }
            break;
        case "cancelled": status = I18n.tr("Download cancelled"); break;
        case "error": root.raiseFault(o.message || I18n.tr("Download failed")); break;
        }
    }
    function _onJobExit(key, code, err, sawTerminal) {
        if (code !== 0 && !sawTerminal)
            root.raiseFault(I18n.tr("create failed (exit %1)").arg(code) + (err.trim().length > 0 ? "\n" + err.trim() : ""));
        Qt.callLater(function () {
            var i = root._jobIdx(key);
            if (i >= 0) dlJobs.remove(i);
            root.refresh();
        });
    }
    // a finished create announces itself so the app can bring the new machine
    // on screen instead of stranding the user in the catalogue.
    signal created(string name)
    // a VM from any local ISO, off-catalogue (full QEMU reach). os is the logo
    // slug (defaults to the guest type, so windows/macos/android still get marks).
    function importVm(name, iso, guest, os) {
        busy = true;
        status = I18n.tr("Creating %1").arg(name);
        runProc.exec(["ryovm", "import", name, iso, guest || "linux", os || guest || "linux"]);
        // Windows has no in-box virtio driver: pull the shared driver CD now (a
        // visible download) so it's cached before the machine first boots.
        if (guest === "windows" || guest === "windows-server")
            fetchDrivers();
    }

    // fetch the shared VirtIO driver CD into the image cache, shown as one more
    // row in the build stack. Skipped when a fetch is already queued.
    function fetchDrivers() {
        if (dlJobs.count >= maxParallel) return;
        for (var i = 0; i < dlJobs.count; i++)
            if (dlJobs.get(i).kind === "virtio") return;
        dlJobs.append({ key: "virtio#" + Date.now(), name: I18n.tr("VirtIO drivers"), cmdName: "virtio", kind: "virtio",
            os: "", release: "", edition: "", disposable: false, tools: "", pkgs: "",
            phase: "download", progress: 0, bps: 0, indet: true, log: "", cancel: false });
    }

    function _group(rows) {
        var map = {};
        var order = [];
        for (var i = 0; i < rows.length; i++) {
            var r = rows[i];
            var key = r.OS;
            if (!map[key]) {
                map[key] = { os: key, name: r["Display Name"] || key, png: r.PNG || "", svg: r.SVG || "", rels: {}, relOrder: [] };
                order.push(key);
            }
            var g = map[key];
            if (!g.rels[r.Release]) { g.rels[r.Release] = []; g.relOrder.push(r.Release); }
            if (r.Option && r.Option.length > 0 && g.rels[r.Release].indexOf(r.Option) < 0)
                g.rels[r.Release].push(r.Option);
        }
        var out = [];
        for (var j = 0; j < order.length; j++) {
            var e = map[order[j]];
            out.push({ os: e.os, name: e.name, png: e.png, svg: e.svg, releases: e.relOrder, editions: e.rels });
        }
        out.sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
        return out;
    }

    // ---- processes ----------------------------------------------------------
    Process {
        id: capsProc
        command: ["ryovm", "caps"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { root.caps = JSON.parse(this.text); }
                catch (e) { console.log("ryovm: caps parse failed: " + e); }
            }
        }
    }
    Process {
        id: listProc
        command: ["ryovm", "list"]
        property string last: ""
        stdout: StdioCollector {
            onStreamFinished: {
                root.vmsLoading = false;
                // identical payload = nothing happened: keep the same model
                // object so the library never rebuilds (a fresh array every
                // 5s poll tore down every card and replayed their entrance).
                if (this.text === listProc.last)
                    return;
                listProc.last = this.text;
                try {
                    var arr = JSON.parse(this.text);
                    root.vms = arr;
                    if (root.pendingSelect.length > 0) {
                        var want = root.pendingSelect;
                        root.pendingSelect = "";
                        if (arr.some(v => v.name === want)) { root.select(want); return; }
                    }
                    // a deleted machine must not leave a dead selection behind
                    // (the pane would empty out and reselect() would fail 5s
                    // after 5s forever): advance to the next machine.
                    if (root.selectedName.length > 0 && !arr.some(v => v.name === root.selectedName)) {
                        root.select(arr.length > 0 ? arr[0].name : "");
                        return;
                    }
                    if (root.selectedName.length === 0 && arr.length > 0)
                        root.select(arr[0].name);
                    else if (root.selectedName.length > 0)
                        root.reselect();
                } catch (e) { root.vms = []; }
            }
        }
        onStarted: root.vmsLoading = true
    }
    Process {
        id: getProc
        property string last: ""
        stdout: StdioCollector {
            onStreamFinished: {
                if (this.text === getProc.last && root.detail !== null)
                    return;
                getProc.last = this.text;
                try { root.detail = JSON.parse(this.text); }
                catch (e) { root.detail = null; }
            }
        }
    }
    Process {
        id: catProc
        stdout: StdioCollector { id: catOut }
        stderr: StdioCollector { id: catErr }
        onExited: (code) => {
            root.catalogLoading = false;
            if (code === 0) {
                // the on-disk cache now exists: Library logos can resolve, and
                // we can warm every catalogue logo in parallel + learn which OSes
                // have art (for the Popular sort).
                root.catalogReady = true;
                prefetchProc.running = true;
                try { root.osList = root._group(JSON.parse(catOut.text)); root.catalogError = ""; return; }
                catch (e) { root.catalogError = I18n.tr("Could not read the OS catalogue"); }
            } else {
                root.catalogError = catErr.text.trim() || I18n.tr("Could not fetch the OS catalogue");
            }
        }
    }
    // warm every logo in parallel and learn which OSes have art. Reassign a fresh
    // object so the iconSet bindings (OsGrid's Popular sort) re-evaluate; bump
    // iconRev so cards already showing a monogram pick up a newly cached logo.
    Process {
        id: prefetchProc
        command: ["ryovm", "prefetch"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var map = JSON.parse(this.text);   // { os: "/abs/path", ... }
                    var set = {};
                    var cache = root.iconCache;
                    for (var os in map) {
                        set[os] = true;
                        cache[os] = map[os];           // seed the path cache directly
                    }
                    root.iconSet = set;
                    root.iconCache = cache;            // reassign so iconFor bindings re-evaluate
                    root.iconRev++;                    // warm tiles render the local file at once
                } catch (e) {}
            }
        }
    }
    Process {
        id: cloudProc
        command: ["ryovm", "cloud-catalog"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.cloudLoading = false;
                try { root.cloudList = JSON.parse(this.text); } catch (e) { root.cloudList = []; }
            }
        }
    }
    Process {
        id: pathsProc
        command: ["ryovm", "paths"]
        stdout: StdioCollector {
            onStreamFinished: { try { root.paths = JSON.parse(this.text); } catch (e) {} }
        }
    }
    // in-app create: streams JSON (resolve/download/config/done) line by line,
    // updating the live bar; on exit, drops the download UI and reloads. A
    // create that dies without ever emitting an error phase (a crashed engine)
    // still surfaces its stderr instead of the pane just vanishing.
    Instantiator {
        model: dlJobs
        delegate: Process {
            id: jobProc
            readonly property string jobKey: model.key
            property bool cancelled: model.cancel
            property bool sawTerminal: false
            property string errText: ""
            onCancelledChanged: if (cancelled) running = false
            Component.onCompleted: {
                var c;
                if (model.kind === "instant") {
                    c = ["ryovm", "instant", model.os];
                    if (model.cmdName.length > 0 && model.cmdName !== model.os + "-instant") c.push(model.cmdName);
                    if (model.disposable) c.push("--disposable");
                    if (model.tools.length > 0) c.push("--tools=" + model.tools);
                    if (model.pkgs.length > 0) c.push("--pkgs=" + model.pkgs);
                } else if (model.kind === "virtio") {
                    c = ["ryovm", "virtio"];
                } else {
                    c = ["ryovm", "create", model.os, model.release];
                    if (model.edition.length > 0) c.push(model.edition);
                }
                jobProc.command = c;
                jobProc.running = true;
            }
            stdout: SplitParser {
                onRead: (line) => {
                    var s = ("" + line);
                    if (s.indexOf('"done"') >= 0 || s.indexOf('"error"') >= 0 || s.indexOf('"cancelled"') >= 0)
                        jobProc.sawTerminal = true;
                    root._onJobLine(jobProc.jobKey, line);
                }
            }
            stderr: StdioCollector { onStreamFinished: jobProc.errText = this.text }
            onExited: (code) => root._onJobExit(jobProc.jobKey, code, jobProc.errText, jobProc.sawTerminal)
        }
    }
    // shared lifecycle runner: any verb that mutates, then reloads the library.
    // Commands issued while one runs are QUEUED: Process.exec mid-run is a
    // silent no-op, which used to eat rapid stepper clicks and even a Launch.
    // Consecutive writes to the same config key coalesce to the final value.
    Process {
        id: runProc
        property string errText: ""
        property string outText: ""
        property var queue: []
        function exec(cmd) {
            if (running) {
                if (cmd[1] === "config") {
                    for (var i = 0; i < queue.length; i++) {
                        if (queue[i][1] === "config" && queue[i][2] === cmd[2] && queue[i][3] === cmd[3]) {
                            queue[i] = cmd;
                            return;
                        }
                    }
                }
                queue.push(cmd);
                return;
            }
            errText = "";
            outText = "";
            command = cmd;
            running = true;
        }
        stderr: StdioCollector { onStreamFinished: runProc.errText = this.text }
        stdout: StdioCollector { onStreamFinished: runProc.outText = this.text }
        onExited: (code) => {
            if (code !== 0 && runProc.errText.trim().length > 0) {
                root.raiseFault(runProc.errText.trim(), runProc.command.length > 2 ? runProc.command[2] : "");
            } else if (code === 0) {
                root.clearFault();
                // the engine speaks in one-line receipts; show them.
                if (runProc.outText.trim().length > 0)
                    root.info(runProc.outText.trim().split("\n")[0], runProc.command.length > 2 ? runProc.command[2] : "");
            }
            if (runProc.queue.length > 0) {
                var next = runProc.queue.shift();
                runProc.errText = "";
                runProc.outText = "";
                runProc.command = next;
                runProc.running = true;
                return;                      // stay busy until the queue drains
            }
            root.busy = false;
            root.refresh();
        }
    }
    Process {
        id: copyProc
        onExited: (code) => {
            if (code === 0)
                root.info(I18n.tr("SSH command copied"));
            else
                root.raiseFault(I18n.tr("Could not copy the SSH command"));
        }
    }
    Process {
        id: usbProc
        property string last: ""
        stdout: StdioCollector {
            onStreamFinished: {
                if (this.text === usbProc.last && root.usb.length > 0)
                    return;
                usbProc.last = this.text;
                try { root.usb = JSON.parse(this.text); } catch (e) { root.usb = []; }
            }
        }
    }
    Process {
        id: usbSetProc
        property string errText: ""
        stderr: StdioCollector { onStreamFinished: usbSetProc.errText = this.text }
        onExited: (code) => {
            if (code !== 0 && usbSetProc.errText.trim().length > 0)
                root.raiseFault(usbSetProc.errText.trim());
            root.loadUsb(root.selectedName);
        }
    }
    Process {
        id: portfwdProc
        property string last: ""
        stdout: StdioCollector {
            onStreamFinished: {
                if (this.text === portfwdProc.last && root.portfwds.length > 0)
                    return;
                portfwdProc.last = this.text;
                try { root.portfwds = JSON.parse(this.text); } catch (e) { root.portfwds = []; }
            }
        }
    }
    Process {
        id: portfwdProc2
        property string errText: ""
        stderr: StdioCollector { onStreamFinished: portfwdProc2.errText = this.text }
        onExited: (code) => {
            if (code !== 0 && portfwdProc2.errText.trim().length > 0)
                root.raiseFault(portfwdProc2.errText.trim());
            root.loadPortfwd(root.selectedName);
        }
    }
    Process {
        id: folderProc
        stdout: StdioCollector {
            onStreamFinished: {
                var p = this.text.trim();
                if (p.length > 0)
                    Quickshell.execDetached(["xdg-open", p]);
            }
        }
    }
    Process {
        id: sshProc
        property string vmName: ""
        stdout: StdioCollector {
            onStreamFinished: {
                var c = this.text.trim();
                if (c.length === 0)
                    return;
                // $TERMINAL wins over the shipped kitty. QEMU's user-net accepts
                // the TCP connect while the guest is still booting, so a bare ssh
                // sits in a pitch-black window with no sign of life (and a plain
                // timeout would kill a session the user IS logged into). Narrate
                // instead: say where and as whom, heartbeat while waiting for the
                // real "SSH-" banner, however long the boot takes, then hand
                // over to a plain interactive ssh. Held open on failure so the
                // fix is readable.
                var pm = c.match(/-p (\d+)/);
                var um = c.match(/(\S+)@localhost/);
                // a burn/cloud machine provisions with cloud-init; its emitted ssh
                // command pins /dev/null known-hosts (the burn host-key policy).
                // That's the reliable "this box installs its tools on boot" tell.
                var burn = c.indexOf("/dev/null") >= 0 ? "1" : "";
                var script = [
                    "cmd=$1; port=$2; vm=$3; user=$4; burn=$5",
                    "printf '  %s, ssh to localhost:%s as \\033[1m%s\\033[0m\\n' \"$vm\" \"$port\" \"$user\"",
                    "printf '  wrong account? set it once:  ryovm config %s ryovm_ssh_user <guest user>\\n\\n' \"$vm\"",
                    "hinted=",
                    "until b=$(timeout 3 bash -c \"exec 3<>/dev/tcp/127.0.0.1/$port && head -c4 <&3\" 2>/dev/null); [ \"$b\" = \"SSH-\" ]; do",
                    "  printf '\\r  waiting for the guest to answer: %ss (first boot can take a minute; Ctrl+C aborts) ' \"$SECONDS\"",
                    "  if [ \"$SECONDS\" -ge 20 ] && [ -z \"$hinted\" ]; then hinted=1; printf '\\n  still booting: a fresh cloud image provisions itself on first boot (Arch and Fedora take about a minute). This is normal.\\n'; fi",
                    "  if [ \"$SECONDS\" -ge 180 ]; then printf '\\n\\n  no answer after 3 minutes: the guest may have no SSH server (a live installer ISO never does), or it did not boot.\\n  press Enter to close\\n'; read _; exit 1; fi",
                    "  sleep 1",
                    "done",
                    "printf '\\n  answered.\\n'",
                    // sshd answers 20-120s BEFORE the tools exist: an instant machine
                    // installs its toolset in cloud-init's FINAL stage. Handing over
                    // the shell at the SSH banner gives a box with no git/go yet:
                    // invisible on Alpine's ~2s apk, glaring on Arch's ~20s pacman
                    // and Fedora's ~2min dnf (the "it didn't deploy the tools" bug).
                    // For a burn machine, wait for cloud-init to finish before the
                    // shell, with a live timer; Ctrl+C drops in early.
                    "if [ -n \"$burn\" ]; then",
                    "  skip=; trap 'skip=1' INT; sec=0",
                    "  while [ -z \"$skip\" ]; do",
                    "    s=$($cmd cloud-init status 2>/dev/null)",
                    "    case \"$s\" in *done*|*error*|*disabled*) break;; esac",
                    "    [ \"$sec\" -ge 420 ] && break",
                    "    printf '\\r  provisioning: installing your tools\\342\\200\\246 %ss (first boot only; Ctrl+C for the shell now)   ' \"$sec\"",
                    "    sec=$((sec+3)); sleep 3",
                    "  done",
                    "  trap - INT",
                    "  if [ -n \"$skip\" ]; then printf '\\n  opening the shell: tools may still be finishing in the background.\\n'; else printf '\\r  tools ready.                                                              \\n'; fi",
                    "fi",
                    "printf '\\n  connecting.\\n\\n'",
                    "$cmd",
                    "ec=$?",
                    "if [ $ec -ne 0 ]; then echo; echo \"ssh exited $ec, a password you never set means the guest has no '$user' account: ryovm config $vm ryovm_ssh_user <guest user>\"; echo 'press Enter to close'; read _; fi"
                ].join("\n");
                Quickshell.execDetached(["sh", "-c",
                    "exec \"${TERMINAL:-kitty}\" --class=ryovm-ssh -e bash -c \"$1\" ryovm-ssh \"$2\" \"$3\" \"$4\" \"$5\" \"$6\"",
                    "--", script, c, pm ? pm[1] : "", sshProc.vmName, um ? um[1] : "", burn]);
            }
        }
    }
    Process {
        id: monProc
        stdout: StdioCollector {
            onStreamFinished: { try { root.monStats = JSON.parse(this.text); } catch (e) { root.monStats = ({}); } }
        }
        onExited: (code) => { if (code !== 0) root.monStats = ({}); }
    }
    Process { id: monActProc; onExited: root.monRefresh() }
    // a running machine's live readout, faster than the 5s library poll but only
    // while a running VM is on the machine stage, so an idle or hidden yard costs
    // nothing.
    Timer {
        interval: 2500
        repeat: true
        running: root.monRunning && root.monWatch
        triggeredOnStart: true
        onTriggered: root.monRefresh()
    }

    // keep Launch/Stop in step while the window is open. caps ride along (five
    // `command -v` checks) so installing the engine lights the app up within 5s
    // instead of after a restart.
    Timer {
        interval: 5000
        repeat: true
        running: true
        onTriggered: {
            listProc.running = true;
            capsProc.running = true;
            if (root.selectedName.length > 0)
                root.reselect();
        }
    }

    FileView {
        id: cfgFile
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/ryovm.json"
        watchChanges: true
        printErrors: false
        onLoadFailed: cfgFile.writeAdapter()
        JsonAdapter {
            id: cfg
            property int defaultCores: 4
            property int defaultRam: 8
            property int defaultDisk: 64
            property string defaultDisplay: "window"
            // instant-machine toolset: chip ids + free-text packages, remembered
            // between sessions. clip (OSC 52) is always baked; spice adds console
            // clipboard. This default gives a light dev box + clipboard.
            property string tools: "git,build,cli,spice"
            property string extraPkgs: ""
        }
    }
    function saveSettings() { cfgFile.writeAdapter(); }
}

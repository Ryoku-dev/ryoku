pragma ComponentBehavior: Bound
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// The single door to the daemon `stage` topic and the `ryostage` engine
// (docs/stage.md): it subscribes to the per-wallpaper frame, turns editor
// gestures into `ryoku-shell stage` intents, and reads the engine directly.
Singleton {
    id: root

    readonly property string bin: {
        const d = Quickshell.env("RYOKU_SHELL_DIR");
        return (d && d.length > 0) ? d + "/scripts/ryostage" : "ryostage";
    }
    property bool available: false
    property bool checked: false
    property bool installing: false
    property string installingModel: ""
    property bool removing: false
    property string removingModel: ""
    // Curated catalogue objects: {id,tier,label,size,installed,licence,upstream}.
    property var models: []
    property string progress: ""

    function recheck() { checkProc.running = false; checkProc.running = true; }
    function install(model) {
        if (root.installing || !model || model.length === 0) return;
        root.installing = true;
        root.installingModel = model;
        root.progress = "";
        installProc.command = [root.bin, "install", model];
        installProc.running = false;
        installProc.running = true;
    }
    function remove(model) {
        if (root.removing || root.installing || !model || model.length === 0) return;
        root.removing = true;
        root.removingModel = model;
        root.progress = "";
        removeProc.command = [root.bin, "remove", model];
        removeProc.running = false;
        removeProc.running = true;
    }
    function modelById(id) {
        const m = root.models || [];
        for (var i = 0; i < m.length; i++) if (m[i].id === id) return m[i];
        return null;
    }
    function defaultModelId(q) {
        return q === "fine" ? "birefnet-general-lite" : "u2netp";
    }
    function selectedModelId(q, choices) {
        const selected = choices && typeof choices[q] === "string" ? choices[q] : "";
        return root.modelById(selected) ? selected : root.defaultModelId(q);
    }
    function modelForQuality(q, choices) {
        return root.modelById(root.selectedModelId(q, choices));
    }
    function qualityInstalled(q, choices) {
        const m = root.modelForQuality(q, choices);
        return !!(m && m.installed === true);
    }

    Process {
        id: checkProc
        command: [root.bin, "check"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.available = ("" + this.text).trim() === "available";
                // Static catalogue: fetch it even before anything is installed,
                // so the Quality control has sizes to offer the first Download.
                modelsProc.running = true;
            }
        }
    }
    Process {
        id: modelsProc
        command: [root.bin, "models", "--json"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const arr = JSON.parse(("" + this.text).trim() || "[]");
                    root.models = Array.isArray(arr) ? arr : [];
                } catch (e) {
                    root.models = [];
                }
                root.checked = true;
            }
        }
    }
    Process {
        id: installProc
        stdout: SplitParser { onRead: line => root.progress = line }
        stderr: SplitParser { onRead: line => root.progress = line }
        onExited: {
            root.installing = false;
            root.installingModel = "";
            root.recheck();
        }
    }
    Process {
        id: removeProc
        stdout: SplitParser { onRead: line => root.progress = line }
        stderr: SplitParser { onRead: line => root.progress = line }
        onExited: {
            root.removing = false;
            root.removingModel = "";
            root.recheck();
        }
    }

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"
    property var frame: ({ current: "", busy: false, stage: "", percent: 0, notice: "", walls: {} })

    readonly property string current: root.frame.current || ""
    readonly property bool busy: root.frame.busy === true
    readonly property string stage: root.frame.stage || ""
    readonly property int percent: (typeof root.frame.percent === "number") ? root.frame.percent : 0
    // Why the last reconcile could not produce a cut ("" when the pipeline is
    // fine): the daemon names the missing model/runtime instead of dying mute.
    readonly property string notice: root.frame.notice || ""
    readonly property string effect: root.effectFor(root.current)

    // Optimistic per-layer overlay so a front/depth flip shows instantly instead
    // of waiting for the intent to round-trip the daemon. Keyed "<path>|<i>".
    property var _opt: ({})

    function apply(line) {
        var f;
        try {
            f = JSON.parse(line);
        } catch (e) {
            return;
        }
        if (!f || typeof f !== "object") return;
        if (!f.walls) f.walls = {};
        root.frame = f;
        const keep = {};
        const cur = f.current || "";
        for (const k in root._opt)
            if (k.indexOf(cur + "|") === 0) keep[k] = root._opt[k];
        root._opt = keep;
    }

    // Transient per-monitor cursor (-1..1), shared so a screen's parallax layers
    // drift in lockstep with its StageBackdrop; not persisted.
    property var _cursor: ({})
    function setCursor(name, nx, ny) {
        const next = {};
        for (const k in root._cursor) next[k] = root._cursor[k];
        next[name] = { nx: nx, ny: ny };
        root._cursor = next;
    }
    function cursorNXFor(name) { const e = root._cursor && root._cursor[name]; return e ? e.nx : 0; }
    function cursorNYFor(name) { const e = root._cursor && root._cursor[name]; return e ? e.ny : 0; }

    // Per-wall lookups (each monitor reads its own wallpaper path).
    function wallOf(path) { return (path && root.frame.walls) ? (root.frame.walls[path] || null) : null; }
    function effectFor(path) { const w = root.wallOf(path); return w && w.effect ? w.effect : "off"; }
    function layersFor(path) { const w = root.wallOf(path); return (w && w.layers) ? w.layers : []; }
    function subjectFor(path) { const w = root.wallOf(path); return (w && w.subject) ? w.subject : ""; }
    function backgroundFor(path) { const w = root.wallOf(path); return (w && w.background) ? w.background : ""; }
    function layerCountFor(path) { return root.layersFor(path).length; }
    function isActiveFor(path) { return root.effectFor(path) !== "off" && root.layerCountFor(path) > 0; }
    function isParallaxFor(path) { return root.effectFor(path) === "parallax" && root.layerCountFor(path) > 0; }

    // The subject (layer 0) draws from the top-level `subject`, set only once the
    // cut lands; every url is busted with the per-wall `rev`.
    function layerUrlFor(path, i) {
        const w = root.wallOf(path);
        const rev = (w && w.rev) ? w.rev : 0;
        if (i === 0) {
            const s = root.subjectFor(path);
            return s === "" ? "" : "file://" + s + "?v=" + rev;
        }
        const ls = root.layersFor(path);
        if (i < 0 || i >= ls.length || !ls[i] || !ls[i].out) return "";
        return "file://" + ls[i].out + "?v=" + rev;
    }
    function backgroundUrlFor(path) {
        const bg = root.backgroundFor(path);
        if (bg === "") return "";
        const w = root.wallOf(path);
        return "file://" + bg + "?v=" + (w && w.rev ? w.rev : 0);
    }

    // Per-layer properties (frame ∪ optimistic overlay): on/off, front, depth.
    function _raw(path, i) {
        const opt = root._opt[path + "|" + i];
        const ls = root.layersFor(path);
        const base = (i >= 0 && i < ls.length && ls[i]) ? ls[i] : {};
        if (!opt) return base;
        const out = {};
        for (const k in base) out[k] = base[k];
        for (const k in opt) out[k] = opt[k];
        return out;
    }
    function _num(v, def) { return (typeof v === "number") ? v : def; }
    function layerEnabled(path, i) { return root._raw(path, i).enabled !== false; }
    function layerFront(path, i) { return root._raw(path, i).front === true; }
    function layerDepth(path, i) { return root._num(root._raw(path, i).depth, 0.5); }
    function layerLabel(path, i) {
        const l = root._raw(path, i);
        const lbl = (typeof l.label === "string" && l.label.length) ? l.label : "";
        if (lbl) return lbl.charAt(0).toUpperCase() + lbl.slice(1);
        return i === 0 ? I18n.tr("Subject") : I18n.tr("Layer %1").arg(i + 1);
    }

    // Verbs (fire-and-forget; the topic republish is the confirmation).
    function call(verb) {
        const cmd = ["ryoku-shell", "stage", verb];
        for (var i = 1; i < arguments.length; i++) cmd.push("" + arguments[i]);
        Quickshell.execDetached(cmd);
    }
    function setEffect(e) { root.call("set-effect", e); }
    function refresh() { root.call("refresh"); }
    function cancel() { root.call("cancel"); }
    function addLayer(png) { root.call("add-layer", png); }
    function cutLayer(picture) { root.call("cut-layer", picture); }
    function removeLayer(index) { root.call("remove-layer", index); }
    function clear() { root.call("clear"); }

    // Per-layer edits coalesce: the overlay shows the value now, one settle flush
    // sends the merged JSON so a drag is one intent.
    property var _pending: ({})
    function setLayer(i, obj) {
        const path = root.current;
        if (!path) return;
        const key = path + "|" + i;
        const cur = root._opt[key] || {};
        const merged = {};
        for (const k in cur) merged[k] = cur[k];
        for (const k in obj) merged[k] = obj[k];
        const opt = {};
        for (const k in root._opt) opt[k] = root._opt[k];
        opt[key] = merged;
        root._opt = opt;
        root._pending[i] = merged;
        layerSettle.restart();
    }
    function setLayerFront(i, front) { root.setLayer(i, { front: front === true }); }
    function setLayerDepth(i, v) { root.setLayer(i, { depth: Math.max(0, Math.min(1, v)) }); }
    Timer {
        id: layerSettle
        interval: 300
        onTriggered: {
            for (const i in root._pending)
                root.call("set-layer", i, JSON.stringify(root._pending[i]));
            root._pending = {};
        }
    }

    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe stage\n");
                flush();
            } else {
                retry.restart();
            }
        }
    }
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Component.onCompleted: root.recheck()
}

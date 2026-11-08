pragma ComponentBehavior: Bound
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

// The single door to the daemon `stage` topic and the `ryostage` engine
// (docs/stage.md): it follows the per-wallpaper frame, turns editor gestures
// into daemon `stage` verbs over the bus, and reads the engine directly.
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
    property var models: []
    property string progress: ""

    property bool spatialInstalled: false
    property bool spatialChecking: false
    property bool spatialInstalling: false
    property bool spatialRemoving: false
    property string spatialProgress: ""
    property var spatialModels: []
    property string spatialInstalledModel: "small"

    function spatialModelById(id) {
        const models = root.spatialModels || [];
        for (var i = 0; i < models.length; i++)
            if (models[i].id === id) return models[i];
        return null;
    }
    function installSpatialModel(model) {
        if (root.spatialInstalling || root.spatialRemoving) return;
        const id = model || "small";
        root.spatialInstalledModel = id;
        root.spatialInstalling = true;
        root.spatialProgress = "";
        spatialInstallProc.command = ["setsid", root.bin, "depth-install", "--model", id];
        spatialInstallProc.running = false;
        spatialInstallProc.running = true;
    }
    function removeSpatialModel(model) {
        if (root.spatialInstalling || root.spatialRemoving) return;
        root.spatialRemoving = true;
        root.spatialProgress = "";
        spatialRemoveProc.command = [root.bin, "depth-remove", "--model", model || "small"];
        spatialRemoveProc.running = false;
        spatialRemoveProc.running = true;
    }

    function recheck() { checkProc.running = false; checkProc.running = true; }
    function recheckSpatial() {
        root.spatialChecking = true;
        spatialCheckProc.running = false;
        const pick = String(Config.spatialModel);
        spatialCheckProc.command = [root.bin, "depth-check", "--json", "--model",
            ["small", "base", "large"].indexOf(pick) >= 0 ? pick : "small"];
        spatialCheckProc.running = true;
    }
    // Downloads use setsid, so signaling the process group also reaches pip, Python and curl.
    function stopGroup(proc) {
        if (proc.running && proc.processId)
            Quickshell.execDetached(["kill", "-TERM", "--", "-" + proc.processId]);
        proc.running = false;
    }
    function cancelSpatialInstall() {
        if (!root.spatialInstalling) return;
        root.stopGroup(spatialInstallProc);
        root.spatialInstalling = false;
        root.spatialProgress = "";
        root.recheckSpatial();
    }
    function install(model) {
        if (root.installing || !model || model.length === 0) return;
        root.installing = true;
        root.installingModel = model;
        root.progress = "";
        installProc.command = ["setsid", root.bin, "install", model];
        installProc.running = false;
        installProc.running = true;
    }
    function cancelInstall() {
        if (!root.installing) return;
        root.stopGroup(installProc);
        root.installing = false;
        root.installingModel = "";
        root.progress = "";
        root.recheck();
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
        const models = root.models || [];
        for (var i = 0; i < models.length; i++)
            if (models[i].id === id) return models[i];
        return null;
    }
    function cutModelInstalled(id) {
        const model = root.modelById(id);
        return !!(model && model.installed === true);
    }

    Process {
        id: checkProc
        command: [root.bin, "check"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.available = ("" + this.text).trim() === "available";
                // Load the catalogue before any model is installed so the first download is available.
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
    Process {
        id: spatialCheckProc
        command: [root.bin, "depth-check", "--json"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const state = JSON.parse(("" + this.text).trim() || "{}");
                    root.spatialInstalled = state.installed === true;
                    if (state.model)
                        root.spatialInstalledModel = String(state.model);
                    if (Array.isArray(state.models))
                        root.spatialModels = state.models;
                } catch (error) {
                    root.spatialInstalled = false;
                }
                root.spatialChecking = false;
            }
        }
    }
    Process {
        id: spatialInstallProc
        command: [root.bin, "depth-install"]
        stdout: SplitParser { onRead: line => root.spatialProgress = line }
        stderr: SplitParser { onRead: line => root.spatialProgress = line }
        onExited: {
            root.spatialInstalling = false;
            root.recheckSpatial();
        }
    }
    Process {
        id: spatialRemoveProc
        command: [root.bin, "depth-remove"]
        stdout: SplitParser { onRead: line => root.spatialProgress = line }
        stderr: SplitParser { onRead: line => root.spatialProgress = line }
        onExited: {
            root.spatialRemoving = false;
            root.recheckSpatial();
        }
    }

    property var frame: ({ current: "", busy: false, job: null, last: null, walls: {} })

    readonly property string current: root.frame.current || ""
    readonly property var job: root.frame.job || null
    readonly property var last: root.frame.last || null
    readonly property bool busy: root.frame.busy === true
    readonly property string effect: root.effectFor(root.current)

    // Key optimistic values by wallpaper so a monitor switch cannot retarget a pending flush.
    property var _opt: ({})

    // the frame is shared with every bus subscriber, so copy it instead of patching
    function apply(f) {
        if (!f || typeof f !== "object") return;
        if (!f.walls) f = Object.assign({}, f, { walls: {} });
        root.frame = f;

        const keep = {};
        for (const key in root._opt) {
            const split = key.lastIndexOf("|");
            if (split < 0) continue;
            const path = key.slice(0, split);
            const index = Number(key.slice(split + 1));
            const wall = f.walls[path];
            const layer = wall && wall.layers && index >= 0
                && index < wall.layers.length ? wall.layers[index] : null;
            const pending = root._opt[key];
            var confirmed = layer !== null;
            for (const field in pending)
                if (!layer || layer[field] !== pending[field]) confirmed = false;
            if (!confirmed && (!wall || layer || root._pending[key])) keep[key] = pending;
        }
        root._opt = keep;
    }

    // Transient per-monitor cursor (-1..1), shared so a screen's parallax
    // layers, its 3D scene and its widget ride drift in lockstep; not
    // persisted. This single place smooths the raw feed, so handing it
    // between hover targets mid-motion never jumps.
    property var _cursor: ({})
    readonly property real cursorSmoothing: 0.35
    function setCursor(name, nx, ny) {
        if (!name)
            return;
        const px = Math.max(-1, Math.min(1, nx));
        const py = Math.max(-1, Math.min(1, ny));
        const prev = root._cursor && root._cursor[name];
        const k = root.cursorSmoothing;
        const next = {};
        for (const key in root._cursor)
            next[key] = root._cursor[key];
        next[name] = prev
            ? { nx: prev.nx + (px - prev.nx) * k, ny: prev.ny + (py - prev.ny) * k }
            : { nx: px, ny: py };
        root._cursor = next;
    }
    function cursorNXFor(name) { const e = root._cursor && root._cursor[name]; return e ? e.nx : 0; }
    function cursorNYFor(name) { const e = root._cursor && root._cursor[name]; return e ? e.ny : 0; }

    // Per-screen spatial pivot published by StageSpatialScene when its mesh
    // loads: the focus distance its camera orbits at, also the radius the
    // desktop widgets ride. Not persisted.
    property var _pivot: ({})
    function setSpatialPivot(name, focusDistance) {
        if (!name || !(focusDistance > 0))
            return;
        const next = {};
        for (const k in root._pivot)
            next[k] = root._pivot[k];
        next[name] = focusDistance;
        root._pivot = next;
    }
    // 2.9: the plugin's pre-publish default (the 0.5 pivot plane).
    function pivotDistanceFor(name) {
        const e = root._pivot && root._pivot[name];
        return e ? e : 2.9;
    }

    // Mirror of DepthMeshGeometry's depth-to-world mapping; it must stay an
    // exact copy or projected widgets leave their wallpaper spot. v is a
    // depth-map value 0..1, larger nearer.
    function spatialDistanceAt(v, R) {
        const pivotDepth = (3.3 - R) / 0.8;
        return R + (pivotDepth - v) * 0.8 * 0.8;
    }

    readonly property real spatialZoom: 1.1

    readonly property real _sceneTan: Math.tan(Math.PI * 42 / 360)
    function widgetSceneAt(v, R, nx, ny, gain, cx, cy, w, h, W, H) {
        const d0 = Math.max(0.31, root.spatialDistanceAt(v, R));
        // Anchored on the widget's own depth, so at rest it renders exactly
        // where the editor placed it; anchoring on the wall would shove
        // off-centre widgets outward and off-screen.
        const k = d0 * 2 * root._sceneTan / Math.max(1, H);
        const g = Math.max(0, Math.min(1.5, gain));
        const yaw = nx * Math.PI * 2.5 / 180 * g;
        const pitch = -ny * Math.PI * 1.8 / 180 * g;
        const cp = Math.cos(pitch), sp = Math.sin(pitch);
        const cyaw = Math.cos(yaw), syaw = Math.sin(yaw);
        const cam = Qt.vector3d(syaw * cp * R, sp * R, -R + cyaw * cp * R);
        const fwd = Qt.vector3d(-syaw * cp, -sp, -cyaw * cp);
        const right = fwd.crossProduct(Qt.vector3d(0, 1, 0)).normalized();
        const up = right.crossProduct(fwd).normalized();
        const p = Qt.vector3d(
            (cx - W / 2) * k, (H / 2 - cy) * k, -d0);
        const rel = Qt.vector3d(p.x - cam.x, p.y - cam.y, p.z - cam.z);
        const z = rel.x * fwd.x + rel.y * fwd.y + rel.z * fwd.z;
        if (!(z > 0.31))
            return { dx: 0, dy: 0, scale: 1 };
        const x = rel.x * right.x + rel.y * right.y + rel.z * right.z;
        const y = rel.x * up.x + rel.y * up.y + rel.z * up.z;
        const focal = H / 2 / root._sceneTan;
        return {
            dx: x / z * focal - (cx - W / 2),
            dy: -(y / z * focal) - (cy - H / 2),
            scale: R / Math.max(0.31, z)
        };
    }

    // Per-wall lookups (each monitor reads its own wallpaper path).
    function wallOf(path) { return (path && root.frame.walls) ? (root.frame.walls[path] || null) : null; }
    function jobFor(path) {
        return root.job && root.job.wall === path ? root.job : null;
    }
    function lastFor(path) {
        return root.last && root.last.wall === path ? root.last : null;
    }
    function stepsFor(path) {
        const wall = root.wallOf(path);
        return wall && Array.isArray(wall.steps) ? wall.steps : [];
    }
    function stepState(path, step) {
        const steps = root.stepsFor(path);
        for (var i = 0; i < steps.length; i++)
            if (steps[i] && steps[i].step === step) return steps[i].state || "";
        return "";
    }
    function sceneUsableFor(path) {
        const steps = root.stepsFor(path);
        if (steps.length === 0) return false;
        for (var i = 0; i < steps.length; i++)
            if (steps[i].state !== "ready" && steps[i].state !== "outdated") return false;
        return true;
    }
    function needsWorkFor(path) {
        const steps = root.stepsFor(path);
        for (var i = 0; i < steps.length; i++)
            if (steps[i].state !== "ready") return true;
        return false;
    }

    function stepLabel(step) {
        if (step === "cut") return I18n.tr("Cut out the subject");
        if (step === "fill") return I18n.tr("Fill in behind the subject");
        if (step === "depth") return I18n.tr("Build the depth map");
        return "";
    }
    function modelLabel(step, id) {
        if (!id) return "";
        const model = step === "depth" ? root.spatialModelById(id) : root.modelById(id);
        return I18n.tr(String(model && (model.label || model.id) || id));
    }
    function errorWords(result) {
        const code = String(result && result.error || "");
        if (code === "timeout")
            return I18n.tr("%1 took longer than %2 minutes and was stopped. Try a lighter model.")
                .arg(result.kind === "layer" ? I18n.tr("Cutting the layer") : root.stepLabel(result.step))
                .arg(Number(result.limit || 0));
        if (code === "model-missing")
            return I18n.tr("The %1 model is not downloaded.")
                .arg(root.modelLabel(result.step, String(result.model || "")) || I18n.tr("selected"));
        if (code === "runtime-missing") return I18n.tr("The cut-out runtime is not installed yet.");
        if (code === "too-large") return I18n.tr("This wallpaper is too large for the depth model.");
        if (code === "input-missing") return I18n.tr("The picture could not be found.");
        return I18n.tr("The model stopped with an error.");
    }
    function effectFor(path) {
        const wall = root.wallOf(path);
        return wall && ["depth", "parallax", "spatial"].indexOf(wall.effect) >= 0
            ? wall.effect : "off";
    }
    function layersFor(path) { const w = root.wallOf(path); return (w && w.layers) ? w.layers : []; }
    function subjectFor(path) { const w = root.wallOf(path); return (w && w.subject) ? w.subject : ""; }
    function backgroundFor(path) { const w = root.wallOf(path); return (w && w.background) ? w.background : ""; }
    function layerCountFor(path) { return root.layersFor(path).length; }
    function isActiveFor(path) { return root.effectFor(path) !== "off"; }
    function isCutModeFor(path) {
        const effect = root.effectFor(path);
        return effect === "depth" || effect === "parallax";
    }
    function isParallaxFor(path) {
        return root.effectFor(path) === "parallax" && root.layerCountFor(path) > 0;
    }

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
    function depthFor(path) { const w = root.wallOf(path); return (w && w.depth) ? w.depth : ""; }
    function depthUrlFor(path) {
        const depth = root.depthFor(path);
        if (depth === "") return "";
        const w = root.wallOf(path);
        return "file://" + depth + "?v=" + (w && w.rev ? w.rev : 0);
    }
    function hasDepthFor(path) { return root.depthFor(path) !== ""; }
    function spatialFor(path) {
        const effect = root.effectFor(path);
        if (!root.hasDepthFor(path))
            return false;
        if (effect === "spatial")
            return true;
        return effect === "parallax" && Config.depthBackdrop
            && root.backgroundFor(path) !== "";
    }
    function backdropUrlFor(path) {
        const effect = root.effectFor(path);
        if (effect === "parallax")
            return root.backgroundUrlFor(path);
        if (effect === "spatial")
            return path === "" ? "" : "file://" + path;
        return "";
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

    // Topic republish, not the verb's reply, confirms fire-and-forget verbs.
    function call(verb, args, wall) {
        const words = ["stage", verb];
        const values = args || [];
        for (var i = 0; i < values.length; i++) words.push("" + values[i]);
        if (wall) words.push("--", "" + wall);
        Bus.run(words.join(" "));
    }
    function setEffect(e, wall) {
        if (["off", "depth", "parallax", "spatial"].indexOf(e) >= 0)
            root.call("set-effect", [e], wall);
    }
    function generate(wall) { root.call("generate", [], wall); }
    function redo(step, wall) {
        root.call("refresh", step ? [step] : [], wall);
    }
    function cancel() { root.call("cancel"); }
    function forceCancel() { root.call("cancel", ["--force"]); }
    function addLayer(png, wall) { root.call("add-layer", [png], wall); }
    function cutLayer(picture, wall) { root.call("cut-layer", [picture], wall); }
    function cutRegion(nx, ny, nw, nh, wall) {
        root.call("cut-region", [nx, ny, nw, nh], wall);
    }
    function removeLayer(index, wall) { root.call("remove-layer", [index], wall); }
    function clear(wall) { root.call("clear", [], wall); }

    property var _pending: ({})
    function setLayer(i, obj, wall) {
        const path = wall;
        if (!path) return;
        const key = path + "|" + i;
        const cur = root._opt[key] || {};
        const merged = {};
        for (const field in cur) merged[field] = cur[field];
        for (const field in obj) merged[field] = obj[field];
        const opt = {};
        for (const field in root._opt) opt[field] = root._opt[field];
        opt[key] = merged;
        root._opt = opt;
        const pending = {};
        for (const field in root._pending) pending[field] = root._pending[field];
        pending[key] = { wall: path, index: i, value: merged };
        root._pending = pending;
        layerSettle.restart();
    }
    function setLayerFront(i, front, wall) {
        root.setLayer(i, { front: front === true }, wall);
    }
    function setLayerDepth(i, v, wall) {
        root.setLayer(i, { depth: Math.max(0, Math.min(1, v)) }, wall);
    }
    Timer {
        id: layerSettle
        interval: 300
        onTriggered: {
            const pending = root._pending;
            root._pending = {};
            for (const key in pending) {
                const edit = pending[key];
                root.call("set-layer", [edit.index, JSON.stringify(edit.value)], edit.wall);
            }
        }
    }

    RyokuUi.BusTopic {
        id: stageTopic
        topic: "stage"
        onUpdated: root.apply(stageTopic.data)
    }

    Component.onCompleted: {
        root.apply(stageTopic.data);
        root.recheck();
        root.recheckSpatial();
    }
}

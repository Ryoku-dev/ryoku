pragma ComponentBehavior: Bound
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property alias editGridSize: adapter.editGridSize
    property alias editGridSnap: adapter.editGridSnap
    readonly property var modes: ["off", "depth", "parallax", "spatial"]

    function cutModel(mode) {
        if (mode !== "depth" && mode !== "parallax")
            return "u2netp";
        const section = adapter[mode] || {};
        const cut = section.cut || {};
        const known = ["u2netp", "silueta", "birefnet-general-lite",
            "birefnet-general", "birefnet-portrait"];
        return known.indexOf(cut.model) >= 0 ? cut.model : "u2netp";
    }
    function cutMatting(mode) {
        if (mode !== "depth" && mode !== "parallax")
            return false;
        const section = adapter[mode] || {};
        return !!(section.cut && section.cut.matting === true);
    }
    function setCutModel(mode, id) {
        const known = ["u2netp", "silueta", "birefnet-general-lite",
            "birefnet-general", "birefnet-portrait"];
        if ((mode !== "depth" && mode !== "parallax") || known.indexOf(id) < 0)
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        const oldCut = src.cut || {};
        const cut = {};
        for (const key in oldCut)
            cut[key] = oldCut[key];
        cut.model = id;
        next.cut = cut;
        adapter[mode] = next;
        file.writeAdapter();
    }
    function setCutMatting(mode, on) {
        if (mode !== "depth" && mode !== "parallax")
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        const oldCut = src.cut || {};
        const cut = {};
        for (const key in oldCut)
            cut[key] = oldCut[key];
        cut.matting = on === true;
        next.cut = cut;
        adapter[mode] = next;
        file.writeAdapter();
    }

    function edge(mode) {
        if (mode !== "depth" && mode !== "parallax")
            return 0.15;
        const section = adapter[mode] || {};
        return typeof section.edge === "number" ? section.edge : 0.15;
    }
    function shadow(mode) {
        if (mode !== "depth" && mode !== "parallax")
            return 0;
        const section = adapter[mode] || {};
        return typeof section.shadow === "number" ? section.shadow : 0;
    }
    function shadowAngle(mode) {
        if (mode !== "depth" && mode !== "parallax" && mode !== "spatial")
            return 90;
        const section = adapter[mode] || {};
        return typeof section.shadowAngle === "number" ? section.shadowAngle : 90;
    }
    function setEdge(mode, v) {
        if ((mode !== "depth" && mode !== "parallax") || !isFinite(v))
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        next.edge = Math.max(0, Math.min(1, v));
        adapter[mode] = next;
        settle.restart();
    }
    function setShadow(mode, v) {
        if ((mode !== "depth" && mode !== "parallax") || !isFinite(v))
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        next.shadow = Math.max(0, Math.min(1, v));
        adapter[mode] = next;
        settle.restart();
    }
    function setShadowAngle(mode, v) {
        if ((mode !== "depth" && mode !== "parallax" && mode !== "spatial")
                || !isFinite(v))
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        next.shadowAngle = Math.round(v);
        adapter[mode] = next;
        settle.restart();
    }
    function resetLook(mode) {
        if (mode !== "depth" && mode !== "parallax" && mode !== "spatial")
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        next.shadowAngle = 90;
        if (mode !== "spatial") {
            next.edge = 0.15;
            next.shadow = 0;
        }
        adapter[mode] = next;
        settle.restart();
    }

    function frontList(mode) {
        if (mode !== "depth" && mode !== "parallax")
            return [];
        const section = adapter[mode] || {};
        const front = section.front;
        if (Array.isArray(front))
            return front;
        // JsonAdapter can expose a saved array as an iterable Qt sequence.
        return front && typeof front === "object" && typeof front[Symbol.iterator] === "function"
            ? Array.from(front) : [];
    }
    function isFront(mode, id) {
        return root.frontList(mode).indexOf(id) >= 0;
    }
    function setFront(mode, id, on) {
        if (mode !== "depth" && mode !== "parallax")
            return;
        const src = adapter[mode] || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        const front = root.frontList(mode).slice();
        const index = front.indexOf(id);
        if (on === true && index < 0)
            front.push(id);
        else if (on !== true && index >= 0)
            front.splice(index, 1);
        else
            return;
        next.front = front;
        adapter[mode] = next;
        settle.restart();
    }

    function motionOf(mode) {
        if (mode !== "parallax" && mode !== "spatial")
            return {};
        const section = adapter[mode] || {};
        const src = section.motion || {};
        const amounts = ["subtle", "normal", "strong"];
        const out = {
            amount: amounts.indexOf(src.amount) >= 0 ? src.amount : "normal",
            mouse: src.mouse !== false,
            sensitivity: typeof src.sensitivity === "number"
                ? Math.max(0, Math.min(2, src.sensitivity)) : 1.0,
            range: typeof src.range === "number"
                ? Math.max(0, Math.min(2, src.range)) : 1.0
        };
        if (mode === "parallax") {
            const idle = ["none", "float", "breathe", "sway"];
            out.idle = idle.indexOf(src.idle) >= 0 ? src.idle : "none";
            out.music = src.music === true;
            out.musicLevel = typeof src.musicLevel === "number"
                ? Math.max(0, Math.min(1, src.musicLevel)) : 0.6;
            out.speed = typeof src.speed === "number"
                ? Math.max(0.25, Math.min(2, src.speed)) : 1.0;
            out.backdrop = typeof src.backdrop === "number"
                ? Math.max(0, Math.min(1, src.backdrop)) : 0.5;
        }
        return out;
    }
    function amountFactor(mode) {
        const amount = root.motionOf(mode).amount;
        return amount === "subtle" ? 0.5 : amount === "strong" ? 1.8 : 1.0;
    }
    function followMouse(mode) {
        return root.motionOf(mode).mouse !== false;
    }
    function sensitivity(mode) {
        const value = root.motionOf(mode).sensitivity;
        return typeof value === "number" ? value : 1.0;
    }
    function range(mode) {
        const value = root.motionOf(mode).range;
        return typeof value === "number" ? value : 1.0;
    }
    function setMotion(mode, key, v) {
        if (mode !== "parallax" && mode !== "spatial")
            return;
        const common = ["amount", "mouse", "sensitivity", "range"];
        const parallax = ["idle", "music", "musicLevel", "speed", "backdrop"];
        if (common.indexOf(key) < 0
                && (mode !== "parallax" || parallax.indexOf(key) < 0))
            return;
        if (key === "amount" && ["subtle", "normal", "strong"].indexOf(v) < 0)
            return;
        if (key === "idle" && ["none", "float", "breathe", "sway"].indexOf(v) < 0)
            return;
        if (["sensitivity", "range", "musicLevel", "speed", "backdrop"].indexOf(key) >= 0
                && !isFinite(v))
            return;
        var value = v;
        if (key === "mouse" || key === "music")
            value = v === true;
        else if (key === "sensitivity" || key === "range")
            value = Math.max(0, Math.min(2, v));
        else if (key === "musicLevel" || key === "backdrop")
            value = Math.max(0, Math.min(1, v));
        else if (key === "speed")
            value = Math.max(0.25, Math.min(2, v));
        const section = adapter[mode] || {};
        const next = {};
        for (const name in section)
            next[name] = section[name];
        const oldMotion = section.motion || {};
        const motion = {};
        for (const name in oldMotion)
            motion[name] = oldMotion[name];
        motion[key] = value;
        next.motion = motion;
        adapter[mode] = next;
        settle.restart();
    }
    function resetMotion(mode) {
        if (mode !== "parallax" && mode !== "spatial")
            return;
        const section = adapter[mode] || {};
        const next = {};
        for (const key in section)
            next[key] = section[key];
        next.motion = mode === "parallax"
            ? ({ amount: "normal", idle: "none", music: false, musicLevel: 0.6,
                speed: 1.0, mouse: true, sensitivity: 1.0, range: 1.0,
                backdrop: 0.5 })
            : ({ amount: "normal", mouse: true, sensitivity: 1.0, range: 1.0 });
        adapter[mode] = next;
        settle.restart();
    }
    function applyPreset(name) {
        const presets = {
            soft: { amount: "subtle", idle: "float", speed: 0.6, music: false },
            cinematic: { amount: "normal", idle: "sway", speed: 0.5, music: false },
            beat: { amount: "strong", idle: "none", speed: 1.0, music: true }
        };
        const preset = presets[name];
        if (!preset)
            return;
        const section = adapter.parallax || {};
        const next = {};
        for (const key in section)
            next[key] = section[key];
        const oldMotion = section.motion || {};
        const motion = {};
        for (const key in oldMotion)
            motion[key] = oldMotion[key];
        for (const key in preset)
            motion[key] = preset[key];
        next.motion = motion;
        adapter.parallax = next;
        settle.restart();
    }

    readonly property string spatialModel: {
        const id = adapter.spatial && adapter.spatial.model;
        return ["small", "base", "large"].indexOf(id) >= 0 ? id : "small";
    }
    function setSpatialModel(id) {
        if (["small", "base", "large"].indexOf(id) < 0)
            return;
        const src = adapter.spatial || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        next.model = id;
        adapter.spatial = next;
        file.writeAdapter();
    }
    readonly property bool depthBackdrop: !adapter.parallax
        || adapter.parallax.depthBackdrop !== false
    function setDepthBackdrop(on) {
        const src = adapter.parallax || {};
        const next = {};
        for (const key in src)
            next[key] = src[key];
        next.depthBackdrop = on === true;
        adapter.parallax = next;
        file.writeAdapter();
    }

    function setEditGridSize(v) {
        const sizes = [16, 32, 48, 64];
        adapter.editGridSize = sizes.indexOf(v) >= 0 ? v : 32;
        file.writeAdapter();
    }
    function cycleEditGridSize() {
        const sizes = [16, 32, 48, 64];
        root.setEditGridSize(sizes[(sizes.indexOf(adapter.editGridSize) + 1) % sizes.length]);
    }
    function toggleEditGridSnap() {
        adapter.editGridSnap = !adapter.editGridSnap;
        file.writeAdapter();
    }

    Timer {
        id: settle
        interval: 400
        onTriggered: file.writeAdapter()
    }

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME")
            || (Quickshell.env("HOME") + "/.config")) + "/ryoku/stage.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()

        JsonAdapter {
            id: adapter
            property int version: 3
            property int editGridSize: 32
            property bool editGridSnap: true
            property var depth: ({
                cut: { model: "u2netp", matting: false },
                edge: 0.15,
                shadow: 0,
                shadowAngle: 90,
                front: []
            })
            property var parallax: ({
                cut: { model: "u2netp", matting: false },
                edge: 0.15,
                shadow: 0,
                shadowAngle: 90,
                front: [],
                depthBackdrop: true,
                motion: {
                    amount: "normal", idle: "none", music: false,
                    musicLevel: 0.6, speed: 1.0, mouse: true,
                    sensitivity: 1.0, range: 1.0, backdrop: 0.5
                }
            })
            property var spatial: ({
                model: "small",
                shadowAngle: 90,
                motion: {
                    amount: "normal", mouse: true, sensitivity: 1.0, range: 1.0
                }
            })
        }
    }

    Component.onCompleted: if (!file.text())
        file.writeAdapter()
}

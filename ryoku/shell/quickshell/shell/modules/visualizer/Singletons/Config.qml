pragma ComponentBehavior: Bound
pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

// Live config for the desktop visualiser. One or more visualisers can share the
// desktop: the PRIMARY is the flat top-level config (the one Ryoku Settings'
// Visualizer tab still edits and reset-to-defaults mirrors), and any EXTRAS ride
// an `extras` list, each a full per-viz object. `activeId` names the stable
// instance the on-desktop editor is tuning. The renderer paints `list` (primary + extras);
// the editor reads/writes the active instance through here.
//
// JSON at ~/.config/ryoku/visualizer.json, watched, so a save in Settings (or a
// Super+M toggle) retunes the running spectrum on the next file event. Fractions
// are of the monitor, matching how the spectrum sizes itself.
Singleton {
    id: root

    // --- globals (shared by every instance) -----------------------------------
    property alias enabled:  adapter.enabled     // master on/off (also Super+M)
    property alias fps:      adapter.fps          // 60 default (30 / 45 / 60)
    property alias adaptive: adapter.adaptive     // auto-throttle under load

    // The adaptive governor of the ACTIVE instance caps fps to 30/24 while it
    // works; the settings popup reads this so it can say so, rather than the
    // user reading the frame-rate control as stuck. 0 while the governor idles.
    property int activeGovTier: 0

    // The looks the renderer knows, canonical here for cycleStyle.
    readonly property var knownStyles: act.knownStyles

    // --- the instances --------------------------------------------------------
    // The primary as a plain object (its flat keys), so it sits in `list` beside
    // the extras and the renderer treats them uniformly.
    // The edge field's knobs ride flat `aura`-prefixed keys, canonical in
    // auraKeys: primaryData folds them into the primary, applyPrimary copies
    // them back on promotion, and VizItem holds the shipped defaults.
    readonly property var auraKeys: [
        "auraEdges", "auraDepth", "auraSpan", "auraTaper", "auraCornerRadius",
        "auraJoin", "auraCornerBlend", "auraFlow", "auraMaterial", "auraShape",
        "auraEffect", "auraEffectStrength", "auraColorMode", "auraColor2",
        "auraColor3", "auraOpacity", "auraColorSpeed", "auraBodyOpacity",
        "auraCrestStrength", "auraGlow", "auraGlowSpread", "auraAudioRange",
        "auraThickness", "auraDetail", "auraBassDrive", "auraTrebleDrive",
        "auraTransient", "auraBeatGlow", "auraCompression", "auraMotionSpeed",
        "auraIdleMotion", "auraAttack", "auraRelease", "auraProfile",
        "auraAccent", "auraSensitivity"
    ]
    function primaryData() {
        var d = {
            "id": adapter.instanceId, "style": adapter.style, "shape": adapter.shape,
            "color": adapter.color, "color2": adapter.color2, "gradient": adapter.gradient,
            "bars": adapter.bars, "thickness": adapter.thickness, "bloom": adapter.bloom,
            "reflection": adapter.reflection, "idleWave": adapter.idleWave,
            "mirror": adapter.mirror, "segments": adapter.segments,
            "gain": adapter.gain, "smoothing": adapter.smoothing, "peaks": adapter.peaks,
            "spin": adapter.spin, "x": adapter.x, "y": adapter.y, "w": adapter.w,
            "h": adapter.h, "grow": adapter.grow, "angle": adapter.angle,
            "tiltX": adapter.tiltX, "tiltY": adapter.tiltY
        };
        for (var i = 0; i < root.auraKeys.length; i++)
            d[root.auraKeys[i]] = adapter[root.auraKeys[i]];
        return d;
    }
    function dataAt(index) {
        const i = Math.round(Number(index));
        return isFinite(i) && i >= 0 && i < root.count ? root.list[i] : ({});
    }
    function indexOfId(instanceId) {
        const wanted = "" + (instanceId || "");
        if (wanted === "")
            return -1;
        const rows = root.list;
        for (let i = 0; i < rows.length; ++i)
            if (("" + (rows[i].id || "")) === wanted)
                return i;
        return -1;
    }
    function idAt(index) {
        const i = Math.round(Number(index));
        const row = isFinite(i) && i >= 0 && i < root.count ? root.list[i] : null;
        return row ? "" + (row.id || "") : "";
    }
    function dataById(instanceId) {
        const index = root.indexOfId(instanceId);
        return index >= 0 ? root.dataAt(index) : ({});
    }
    function labelAt(index) {
        const stableId = root.idAt(index);
        const match = /^viz-(\d+)$/.exec(stableId);
        return match ? match[1] : stableId;
    }
    function allocateId() {
        let serial = Math.max(1, Math.round(adapter.nextId));
        let stableId = "";
        do {
            stableId = "viz-" + serial;
            serial++;
        } while (root.indexOfId(stableId) >= 0);
        adapter.nextId = serial;
        return stableId;
    }
    property var previews: ({})
    function previewAt(instanceId) {
        return root.previews["" + instanceId] || null;
    }
    function setPreview(instanceId, nx, ny) {
        const key = "" + instanceId;
        if (key === "")
            return;
        const next = Object.assign({}, root.previews);
        next[key] = { x: nx, y: ny };
        root.previews = next;
    }
    function clearPreview(instanceId) {
        const key = "" + instanceId;
        if (key === "" || root.previews[key] === undefined)
            return;
        const next = Object.assign({}, root.previews);
        delete next[key];
        root.previews = next;
    }
    readonly property var list: {
        var out = [root.primaryData()];
        var ex = root.cloneData(adapter.extras);
        for (var i = 0; i < ex.length; i++)
            out.push(ex[i]);
        return out;
    }
    readonly property int count: root.list.length
    readonly property int active: {
        const found = root.indexOfId(adapter.activeId);
        return found >= 0 ? found : 0;
    }
    readonly property string activeId: root.idAt(root.active)
    readonly property var activeData: root.list[root.active]

    // Never more than this many at once; each adds a full-screen GPU pass.
    readonly property int maxVisualizers: 4
    // A light estimate for the RAM a visualiser costs (its render buffers; cava is
    // shared), so the editor can warn before the desktop is stacked with passes.
    readonly property int ramPerVizMB: 35
    readonly property int ramEstimateMB: root.count * root.ramPerVizMB

    // The active instance, normalised, for the editor and placer to read.
    VizItem { id: act; data: root.activeData }

    // The active instance, normalised: the editors read their knobs off this
    // rather than re-listing every alias.
    readonly property VizItem instance: act
    readonly property string styleId:       act.styleId
    readonly property bool   isPolar:       act.isPolar
    readonly property bool   isAura:        act.isAura
    readonly property bool   peaksApply:    act.peaksApply
    readonly property bool   mirrorApplies: act.mirrorApplies
    readonly property int    bars:          act.bars
    readonly property real   thickness:     act.thickness
    readonly property real   gain:          act.gain

    // Cava is fed once for every instance: at the largest band count any of them
    // wants (each Motion resamples down), and the scope waveform is captured while
    // any instance is the line look.
    readonly property int maxBars: {
        var m = 16;
        for (var i = 0; i < root.list.length; i++)
            m = Math.max(m, Math.round(root.list[i].bars || 64));
        return Math.max(16, Math.min(128, m));
    }
    readonly property bool anyLine: {
        for (var i = 0; i < root.list.length; i++)
            if (("" + (root.list[i].style || "")) === "line")
                return true;
        return false;
    }
    readonly property real   smoothing:     act.smoothing
    readonly property bool   mirror:        act.mirror
    readonly property bool   peaks:         act.peaks
    readonly property int    segments:      act.segments
    readonly property real   spin:          act.spin
    readonly property real   x:             act.x
    readonly property real   y:             act.y
    readonly property real   w:             act.w
    readonly property real   h:             act.h
    readonly property string grow:          act.grow
    readonly property real   angle:         act.angle
    readonly property real   tiltX:         act.tiltX
    readonly property real   tiltY:         act.tiltY

    // Colour + gradient of the active instance, for the editor's picker.
    readonly property bool   hasCustomColor: act.hasCustomColor
    readonly property color  customColor:    act.customColor
    readonly property string colorHex:       act.hasCustomColor ? act.rawColor : ""
    readonly property bool   gradient: root.active === 0 ? (adapter.gradient === true) : (act.val("gradient", false) === true)
    readonly property bool   hasColor2:      act.hasColor2
    readonly property color  color2Value:    act.color2Value
    readonly property string color2Hex:      act.hasColor2 ? act.rawColor2 : ""

    // --- editing the active instance ------------------------------------------
    // Every per-viz edit lands on the active instance: the primary writes its flat
    // keys, an extra rewrites its slot in the list. Edits settle through one
    // coalescer, since the watched file returns a write as a reload.
    function poke(key, value) {
        if (root.active <= 0) {
            adapter[key] = value;
        } else {
            var arr = root.cloneData(adapter.extras);
            var e = Object.assign({}, arr[root.active - 1]);
            e[key] = value;
            arr[root.active - 1] = e;
            adapter.extras = arr;
        }
        settle.restart();
    }
    // Toggling a screen edge on or off for the field; an empty field is not a
    // valid look, so switching the last one off lights the rails again.
    function toggleAuraEdge(name) {
        var cur = (act.auraEdges || []).slice();
        var i = cur.indexOf(name);
        if (i >= 0)
            cur.splice(i, 1);
        else
            cur.push(name);
        root.poke("auraEdges", cur);
    }

    function setStyle(k) {
        if (root.knownStyles.indexOf(k) < 0)
            return;
        root.poke("style", k);
    }
    // The aura look's vocabularies, canonical so the desktop editor and the Hub
    // cycle the same set rather than each hand-listing it.
    readonly property var auraMaterials: ["silk", "aurora", "contour", "liquid"]
    function cycleAuraMaterial() {
        var m = root.auraMaterials;
        var i = m.indexOf(act.auraMaterial);
        root.poke("auraMaterial", m[(i + 1) % m.length]);
    }
    function setAuraDepth(v) {
        root.poke("auraDepth", Math.max(24, Math.min(600, Math.round(v))));
    }
    function cycleStyle(by) {
        var i = root.knownStyles.indexOf(root.styleId);
        var n = root.knownStyles.length;
        root.setStyle(root.knownStyles[(i + by + n) % n]);
    }
    function setBars(n) { root.poke("bars", Math.max(16, Math.min(128, Math.round(n)))); }
    function toggleMirror() { root.poke("mirror", !root.mirror); }
    function togglePeaks() { root.poke("peaks", !root.peaks); }
    function setGain(v) { root.poke("gain", Math.max(0.5, Math.min(2, v))); }
    function setSmoothing(v) { root.poke("smoothing", Math.max(0, Math.min(1, v))); }

    // Well short of edge-on: past this the bands crowd into a line.
    readonly property real tiltMax: 35
    function setTiltX(v) { root.poke("tiltX", Math.max(-root.tiltMax, Math.min(root.tiltMax, v))); }
    function setTiltY(v) { root.poke("tiltY", Math.max(-root.tiltMax, Math.min(root.tiltMax, v))); }
    function levelTilt() {
        root.poke("tiltX", 0);
        root.poke("tiltY", 0);
    }

    // Colour: a pinned #rrggbb, "" to follow the wallpaper. A gradient adds a
    // second stop; both stops must be valid hex for it to paint.
    function normalizeHex(hex) {
        var t = ("" + hex).trim();
        if (t.length > 0 && t[0] !== "#")
            t = "#" + t;
        return /^#[0-9a-fA-F]{6}$/.test(t) ? t.toLowerCase() : "";
    }
    function setColor(hex) {
        var t = root.normalizeHex(hex);
        if (t === "")
            return false;
        root.poke("color", t);
        return true;
    }
    function clearColor() { root.poke("color", ""); }
    function setColor2(hex) {
        var t = root.normalizeHex(hex);
        if (t === "")
            return false;
        root.poke("color2", t);
        return true;
    }
    function setGradient(on) { root.poke("gradient", on === true); }
    function toggleGradient() { root.setGradient(!root.gradient); }

    // Global, so it always writes the flat key. It shares the settle coalescer
    // with instance edits so enabling a restored instance cannot race its data.
    function setEnabled(on) {
        if (adapter.enabled === (on === true))
            return;
        adapter.enabled = on === true;
        settle.restart();
    }

    // fps and adaptive are global, not per-viz: write the flat keys directly, so
    // tuning while an extra instance is active never buries them in its slot.
    function setFps(v) {
        adapter.fps = Math.max(1, Math.min(240, Math.round(v)));
        settle.restart();
    }
    function setAdaptive(on) {
        adapter.adaptive = on === true;
        settle.restart();
    }

    // How much of the box's own turned footprint is allowed to hang past a
    // screen edge, an aesthetic choice the box can be left sitting in (not just
    // a mid-drag overshoot that snaps back): 0 keeps the whole look on screen,
    // 0.5 would let it hang out to its own centre. Proportional to the box's
    // footprint rather than a flat screen fraction, so a small look doesn't lose
    // a bigger share of itself than a large one does.
    readonly property real overhang: 0.5

    // The box is a fraction of the screen, and a look can be left hanging
    // `overhang` of its own turned footprint past an edge, on purpose - past
    // that, it is clamped back (the placer used to allow a flat quarter-screen
    // past any edge regardless of the box's own size, and a drag or a wheel
    // resize that stopped past the limit left the spectrum clipped further than
    // intended, saved that way on every login). Size is clamped first, then the
    // position to what the size and the overhang allowance leave.
    //
    // A turned box's own footprint is bigger than its unrotated w/h (the same
    // axis-aligned bounding box SpectrumField.qml works out for coverRect), so
    // clamping position against the raw w/h left a turned look free to swing an
    // arc it never actually needed and still short of the edge it visually
    // reached. `angleDeg` and `aspect` (screenWidth / screenHeight, since width
    // and height are fractions of different physical scales on anything but a
    // square screen) fold that footprint into the same clamp. Callers that omit
    // them keep the old unrotated, square-screen behaviour.
    function fitBox(nx, ny, nw, nh, angleDeg, aspect) {
        var w = Math.max(0.04, Math.min(1, nw));
        var h = Math.max(0.03, Math.min(1, nh));
        var a = ((angleDeg || 0) % 180) * Math.PI / 180;
        var ar = (aspect && aspect > 0) ? aspect : 1;
        var c = Math.abs(Math.cos(a)), s = Math.abs(Math.sin(a));
        // The turned box's own bounding box, back in fraction space.
        var bw = Math.min(1, w * c + h * s / ar);
        var bh = Math.min(1, w * s * ar + h * c);
        var oh = Math.max(0, Math.min(0.5, root.overhang));
        var minCx = bw * (0.5 - oh), maxCx = 1 - minCx;
        var minCy = bh * (0.5 - oh), maxCy = 1 - minCy;
        var cx = Math.max(minCx, Math.min(maxCx, nx + w / 2));
        var cy = Math.max(minCy, Math.min(maxCy, ny + h / 2));
        return { x: cx - w / 2, y: cy - h / 2, w: w, h: h };
    }

    // Placement from the desktop: the properties move with the pointer so the
    // look follows the drag frame by frame, written once the gesture settles.
    function moveBox(nx, ny, aspect) {
        var b = root.fitBox(nx, ny, root.w, root.h, root.angle, aspect);
        root.poke("x", b.x);
        root.poke("y", b.y);
    }
    function moveBoxAt(instanceId, nx, ny, aspect) {
        const i = root.indexOfId(instanceId);
        if (i < 0)
            return;
        const data = root.dataAt(i);
        const w = Number(data.w);
        const h = Number(data.h);
        const angle = Number(data.angle);
        const b = root.fitBox(nx, ny,
            isFinite(w) ? w : 1,
            isFinite(h) ? h : 0.42,
            isFinite(angle) ? angle : 0,
            aspect);
        if (i === 0) {
            adapter.x = b.x;
            adapter.y = b.y;
        } else {
            const arr = root.cloneData(adapter.extras);
            const entry = Object.assign({}, arr[i - 1]);
            entry.x = b.x;
            entry.y = b.y;
            arr[i - 1] = entry;
            adapter.extras = arr;
        }
        settle.restart();
    }
    function sizeBox(nw, nh, aspect) {
        root.setBox(root.x, root.y, nw, nh, aspect);
    }
    // Size and position land together, or a turned box swings between two writes.
    function setBox(nx, ny, nw, nh, aspect) {
        var b = root.fitBox(nx, ny, nw, nh, root.angle, aspect);
        if (root.active <= 0) {
            adapter.w = b.w;
            adapter.h = b.h;
            adapter.x = b.x;
            adapter.y = b.y;
        } else {
            var arr = root.cloneData(adapter.extras);
            var e = Object.assign({}, arr[root.active - 1]);
            e.w = b.w;
            e.h = b.h;
            e.x = b.x;
            e.y = b.y;
            arr[root.active - 1] = e;
            adapter.extras = arr;
        }
        settle.restart();
    }
    // Wrapped, so a full circle of dragging never runs into a stop.
    function rotate(deg) {
        var d = deg % 360;
        root.poke("angle", d < 0 ? d + 360 : d);
    }
    // Mirroring what is on screen is a different thing per family.
    function flip() {
        if (root.isAura)
            root.poke("auraFlow", act.auraFlow === "clockwise" ? "counterclockwise" : "clockwise");
        else if (root.isPolar)
            root.poke("spin", -root.spin);
        else if (root.grow === "up")
            root.poke("grow", "down");
        else if (root.grow === "down")
            root.poke("grow", "up");
        else if (root.grow === "left")
            root.poke("grow", "right");
        else if (root.grow === "right")
            root.poke("grow", "left");
        else
            root.poke("mirror", !root.mirror);
    }

    // --- instance management --------------------------------------------------
    function cloneData(o) {
        return JSON.parse(JSON.stringify(o || {}));
    }
    function setActive(instance) {
        const idx = typeof instance === "number"
            ? Math.max(0, Math.min(root.count - 1, Math.round(instance)))
            : root.indexOfId(instance);
        if (idx < 0)
            return false;
        adapter.activeId = root.idAt(idx);
        settle.restart();
        return true;
    }
    function addVisualizer() {
        if (root.count >= root.maxVisualizers)
            return "";
        var arr = root.cloneData(adapter.extras);
        // Seed from the active instance, nudged so the new one is not hidden
        // exactly under it, and given a fresh box so it is easy to grab.
        var seed = root.cloneData(root.activeData);
        seed.id = root.allocateId();
        seed.x = Math.max(-0.2, Math.min(0.8, (seed.x || 0) + 0.06));
        seed.y = Math.max(-0.2, Math.min(0.8, (seed.y || 0.58) - 0.12));
        arr.push(seed);
        adapter.extras = arr;
        adapter.activeId = seed.id;
        settle.restart();
        return seed.id;
    }
    function applyPrimary(o) {
        adapter.instanceId = "" + (o.id || adapter.instanceId); adapter.style = o.style; adapter.shape = o.shape;
        adapter.color = o.color || ""; adapter.color2 = o.color2 || "";
        adapter.gradient = o.gradient === true;
        adapter.bars = o.bars; adapter.thickness = o.thickness; adapter.bloom = o.bloom;
        adapter.reflection = o.reflection; adapter.idleWave = o.idleWave;
        adapter.mirror = o.mirror; adapter.segments = o.segments;
        adapter.gain = o.gain; adapter.smoothing = o.smoothing; adapter.peaks = o.peaks;
        adapter.spin = o.spin; adapter.x = o.x; adapter.y = o.y; adapter.w = o.w;
        adapter.h = o.h; adapter.grow = o.grow; adapter.angle = o.angle;
        adapter.tiltX = o.tiltX; adapter.tiltY = o.tiltY;
        for (var i = 0; i < root.auraKeys.length; i++) {
            var k = root.auraKeys[i];
            if (o[k] !== undefined)
                adapter[k] = o[k];
        }
    }
    function replaceVisualizer(i, data) {
        var idx = Math.max(0, Math.min(root.count - 1, i));
        var copy = root.cloneData(data);
        if (idx === 0) {
            root.applyPrimary(copy);
        } else {
            var arr = root.cloneData(adapter.extras);
            arr[idx - 1] = copy;
            adapter.extras = arr;
        }
        adapter.activeId = copy.id;
        settle.restart();
    }
    function insertVisualizer(i, data) {
        if (root.count >= root.maxVisualizers)
            return -1;
        var idx = Math.max(0, Math.min(root.count, i));
        var copy = root.cloneData(data);
        if (idx === 0) {
            var arr0 = root.cloneData(adapter.extras);
            arr0.unshift(root.cloneData(root.primaryData()));
            root.applyPrimary(copy);
            adapter.extras = arr0;
        } else {
            var arr = root.cloneData(adapter.extras);
            arr.splice(idx - 1, 0, copy);
            adapter.extras = arr;
        }
        adapter.activeId = copy.id;
        settle.restart();
        return copy.id;
    }
    function replaceAll(instances, selected, on) {
        var rows = root.cloneData(instances);
        if (!rows || rows.length === 0)
            rows = [root.primaryData()];
        root.applyPrimary(rows[0]);
        adapter.extras = rows.slice(1, root.maxVisualizers);
        const selectedId = typeof selected === "number"
            ? "" + (rows[Math.max(0, Math.min(rows.length - 1, selected))].id || "")
            : "" + (selected || rows[0].id || "");
        adapter.activeId = selectedId;
        adapter.enabled = on === true;
        settle.restart();
    }
    function removeVisualizer(instance) {
        const idx = instance === undefined ? root.active
            : (typeof instance === "number"
                ? Math.max(0, Math.min(root.count - 1, Math.round(instance)))
                : root.indexOfId(instance));
        if (idx < 0)
            return false;
        if (root.count === 1) {
            root.setEnabled(false);
            return true;
        }
        const selectedId = root.activeId;
        if (idx === 0) {
            var ex = root.cloneData(adapter.extras);
            var promoted = ex.shift();
            root.applyPrimary(promoted);
            adapter.extras = ex;
        } else {
            var arr = root.cloneData(adapter.extras);
            arr.splice(idx - 1, 1);
            adapter.extras = arr;
        }
        adapter.activeId = root.indexOfId(selectedId) >= 0
            ? selectedId : root.idAt(Math.min(idx, root.count - 1));
        settle.restart();
        return true;
    }

    Timer {
        id: settle
        interval: 400
        onTriggered: file.writeAdapter()
    }

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/visualizer.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()

        JsonAdapter {
            id: adapter
            property bool enabled: true
            property int bars: 64
            property real thickness: 0.58
            property real bloom: 0.6
            property real reflection: 0.1
            property bool idleWave: true
            property string style: "wave"
            property string shape: "rounded"
            property string color: ""
            property string color2: ""
            property bool gradient: false
            property bool mirror: false
            property int segments: 10
            property int fps: 60
            property bool adaptive: true
            property real smoothing: 0.5
            property real gain: 1.0
            property bool peaks: false
            property real spin: 0
            property real x: 0
            property real y: 0.58
            property real w: 1
            property real h: 0.42
            property string grow: "up"
            property real angle: 0
            property real tiltX: 0
            property real tiltY: 0
            // The edge field's knobs, flat and aura-prefixed like every other
            // per-viz key. The values mirror VizItem's defaults, so a config
            // that has never touched an aura key keeps them out of the file.
            property var auraEdges: ["left", "right"]
            property real auraDepth: 180
            property real auraSpan: 1.0
            property real auraTaper: 0.14
            property real auraCornerRadius: 24
            property string auraJoin: "auto"
            property real auraCornerBlend: 0.55
            property string auraFlow: "clockwise"
            property string auraMaterial: "silk"
            property string auraShape: "flow"
            property string auraEffect: "clean"
            property real auraEffectStrength: 0.38
            property string auraColorMode: "flow"
            property string auraColor2: ""
            property string auraColor3: ""
            property real auraOpacity: 1.0
            property real auraColorSpeed: 0.35
            property real auraBodyOpacity: 0.32
            property real auraCrestStrength: 0.9
            property real auraGlow: 0.52
            property real auraGlowSpread: 0.48
            property real auraAudioRange: 0.78
            property real auraThickness: 0.22
            property real auraDetail: 0.42
            property real auraBassDrive: 0.88
            property real auraTrebleDrive: 0.68
            property real auraTransient: 0.9
            property real auraBeatGlow: 0.64
            property real auraCompression: 0.12
            property real auraMotionSpeed: 1.0
            property real auraIdleMotion: 0.14
            property real auraAttack: 1.05
            property real auraRelease: 0.82
            property string auraProfile: "flat"
            property real auraAccent: 0.7
            property real auraSensitivity: 0.72
            // Stable identity is stored with each instance. The active pointer is
            // an id, never a list position, so removing one cannot retarget the
            // editor or make another instance inherit its selection.
            property string instanceId: ""
            property int nextId: 1
            property var extras: []
            property string activeId: ""
        }
    }

    // A config written before the box carries an anchored position, height, span
    // and origin instead; fold those into the box once so the spectrum stays where
    // its owner put it, then keep only the box.
    function migrate() {
        var o = {};
        try {
            o = JSON.parse(file.text()) || {};
        } catch (e) {
            return false;
        }
        if (o.x !== undefined)
            return o.style === "circle";
        var pos = o.position || "bottom";
        var depth = typeof o.height === "number" ? o.height : 0.42;
        var span = typeof o.span === "number" ? o.span : 1;
        var align = o.align || "center";
        var polar = ["radial", "orb", "spiral", "circle"].indexOf(o.style) >= 0;
        if (polar) {
            var r = (typeof o.size === "number" ? o.size : 0.3);
            var ox = typeof o.originX === "number" ? o.originX : 0.5;
            var oy = typeof o.originY === "number" ? o.originY : 0.5;
            adapter.w = r * 2 * 0.625;   // the radius was a fraction of the short edge
            adapter.h = r * 2;
            adapter.x = ox - adapter.w / 2;
            adapter.y = oy - adapter.h / 2;
            adapter.grow = "center";
        } else {
            var off = align === "start" ? 0 : (align === "end" ? 1 - span : (1 - span) / 2);
            var vertical = pos === "left" || pos === "right";
            adapter.w = vertical ? depth : span;
            adapter.h = vertical ? span : depth;
            adapter.x = pos === "right" ? 1 - depth : (vertical ? 0 : off);
            adapter.y = pos === "top" ? 0
                : (pos === "center" ? (1 - depth) / 2 : (vertical ? off : 1 - depth));
            adapter.grow = pos === "top" ? "down"
                : (pos === "center" ? "center"
                : (pos === "left" ? "right" : (pos === "right" ? "left" : "up")));
        }
        return true;
    }

    function ensureIdentities(raw) {
        const source = raw || {};
        const rows = root.cloneData(adapter.extras);
        const seen = ({});
        let serial = Math.max(1, Math.round(adapter.nextId));
        const candidates = [adapter.instanceId].concat(rows.map(row => row ? row.id : ""));
        for (let i = 0; i < candidates.length; ++i) {
            const match = /^viz-(\d+)$/.exec("" + (candidates[i] || ""));
            if (match)
                serial = Math.max(serial, parseInt(match[1]) + 1);
        }
        function claim(candidate) {
            let stableId = "" + (candidate || "");
            if (stableId === "" || seen[stableId]) {
                do {
                    stableId = "viz-" + serial;
                    serial++;
                } while (seen[stableId]);
            }
            seen[stableId] = true;
            return stableId;
        }

        let changed = false;
        const primaryId = claim(adapter.instanceId);
        if (primaryId !== adapter.instanceId) {
            adapter.instanceId = primaryId;
            changed = true;
        }
        for (let j = 0; j < rows.length; ++j) {
            const oldId = "" + ((rows[j] || {}).id || "");
            const stableId = claim(oldId);
            if (stableId !== oldId) {
                rows[j] = Object.assign({}, rows[j] || {}, { id: stableId });
                changed = true;
            }
        }
        if (changed)
            adapter.extras = rows;
        if (adapter.nextId !== serial) {
            adapter.nextId = serial;
            changed = true;
        }

        const allIds = [adapter.instanceId].concat(rows.map(row => "" + (row.id || "")));
        let selected = "" + (adapter.activeId || "");
        if (allIds.indexOf(selected) < 0) {
            const legacyIndex = Math.max(0, Math.min(allIds.length - 1,
                typeof source.active === "number" ? Math.round(source.active) : 0));
            selected = allIds[legacyIndex];
        }
        if (adapter.activeId !== selected) {
            adapter.activeId = selected;
            changed = true;
        }
        return changed;
    }

    // A box saved while overhang was still allowed (or by hand) is folded back
    // inside the screen once, primary and extras alike; true when one moved.
    // Best-effort aspect from whatever screen is up first; a stored box off by
    // a turn on a differently-shaped monitor still lands closer than ignoring
    // the turn entirely, and a live drag re-fits against the real screen anyway.
    function fitStored() {
        var moved = false;
        var scr = Quickshell.screens.length > 0 ? Quickshell.screens[0] : null;
        var aspect = (scr && scr.height > 0) ? scr.width / scr.height : 1;
        var b = root.fitBox(adapter.x, adapter.y, adapter.w, adapter.h, adapter.angle, aspect);
        if (b.x !== adapter.x || b.y !== adapter.y || b.w !== adapter.w || b.h !== adapter.h) {
            adapter.x = b.x; adapter.y = b.y; adapter.w = b.w; adapter.h = b.h;
            moved = true;
        }
        var arr = root.cloneData(adapter.extras);
        for (var i = 0; i < arr.length; i++) {
            var e = arr[i] || {};
            var f = root.fitBox(Number(e.x) || 0, Number(e.y) || 0, Number(e.w) || 1, Number(e.h) || 0.42,
                                Number(e.angle) || 0, aspect);
            if (f.x !== e.x || f.y !== e.y || f.w !== e.w || f.h !== e.h) {
                arr[i] = Object.assign({}, e, f);
                moved = true;
            }
        }
        if (moved)
            adapter.extras = arr;
        return moved;
    }

    Component.onCompleted: {
        let raw = {};
        if (file.text()) {
            try {
                raw = JSON.parse(file.text()) || {};
            } catch (e) {
                raw = {};
            }
        }
        var write = root.ensureIdentities(raw);
        if (!file.text()) {
            file.writeAdapter();
            return;
        }
        if (root.migrate())
            write = true;
        if (adapter.style === "circle") {
            adapter.style = "orb";
            write = true;
        }
        if (root.fitStored())
            write = true;
        if (write)
            file.writeAdapter();
    }
}

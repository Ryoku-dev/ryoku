pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import ".."

// Animations, ported to the monochrome instrument. The Hyprland animation tree
// (its inventory comes from the provider defaults, `ryoku-hub desktop defaults`)
// plus a bezier curve editor; per-leaf overrides and user curves persist through
// the shell's hypr store, which previews them live on the desktop and restores
// on revert.
//
// The shell owns the rail, side panel (preview/state/diff), the action bar
// (Save/Revert/Reset read the same hub.dirty/diff this page feeds) and the
// live preview. This page is pure: it owns only the content region (the global
// switch, the focus flash group, the curve editor, the animation table) and
// edits the draft through hub.hyprEdit. It never previews, restores or writes.
Item {
    id: pg
    property var hub

    // ── store shortcuts (reactive: reading hyprDraft/hyprCommitted inside a
    // binding tracks them, so cells refresh on edit/save/revert) ────────────
    function hv(path) { return pg.hub ? pg.hub.hyprVal(path) : undefined }
    function cv(path) { return pg.hub ? pg.hub.hyprCommittedVal(path) : undefined }
    function he(path, v) { if (pg.hub) pg.hub.hyprEdit(path, v) }
    function chg(path) { return JSON.stringify(pg.hv(path)) !== JSON.stringify(pg.cv(path)) }
    function cap(s) { s = String(s); return s.length ? s.charAt(0).toUpperCase() + s.slice(1) : s }

    // the animatable inventory, from the provider defaults (async: guarded, so it
    // rebinds when hyprDefaults lands). The provider probes leaves + curves live.
    readonly property var liveAnims: pg.hub && pg.hub.hyprDefaults && pg.hub.hyprDefaults.wm && pg.hub.hyprDefaults.wm.hyprland && pg.hub.hyprDefaults.wm.hyprland.anim ? pg.hub.hyprDefaults.wm.hyprland.anim.items : []
    readonly property var liveCurves: pg.hub && pg.hub.hyprDefaults && pg.hub.hyprDefaults.wm && pg.hub.hyprDefaults.wm.hyprland && pg.hub.hyprDefaults.wm.hyprland.anim ? pg.hub.hyprDefaults.wm.hyprland.anim.curves : []
    // The Hyprland window-animation editor (preset picker, curve workshop, per-
    // leaf table) is bound to the wm.hyprland.anim subtree, so it stands up only
    // when the running provider is the one that models that subtree. modelsKey is
    // the guard: the moment another provider owns the session, wm.hyprland.anim is
    // a dead key and the whole bespoke editor stands down.
    readonly property bool hasHyprAnims: !!(pg.hub && pg.hub.hyprDefaults && pg.hub.hyprDefaults.wm && pg.hub.hyprDefaults.wm.hyprland && pg.hub.hyprDefaults.wm.hyprland.anim) && Settings.modelsKey("wm.hyprland.anim")
    // A provider without that subtree drives its animations through its own
    // page:"animations" schema rows, drawn below by the shared primitives. The
    // Hyprland provider's own page:"animations" rows are the bespoke editor's
    // territory, so they are never double-drawn: when its editor is up, this list
    // is empty.
    readonly property var providerAnimRows: {
        ProviderSchema.revision;
        if (pg.hasHyprAnims) return [];
        var out = [], rs = ProviderSchema.rowsFor("animations");
        for (var i = 0; i < rs.length; i++) {
            var r = rs[i];
            if (!r || !r.key) continue;
            if (String(r.key).indexOf("[") >= 0) continue;
            if (r.ctl === "action" || r.ctl === "list") continue;
            if (!Settings.supports(r.caps, r.hostCaps)) continue;
            if (!Settings.modelsKey(r.key)) continue;
            out.push(r);
        }
        return out;
    }
    // true when either editor has something to show, so the page offers more than
    // the shell motion and the master switch.
    readonly property bool hasWindowAnims: pg.hasHyprAnims || pg.providerAnimRows.length > 0
    readonly property var providerAnimGroups: {
        var seen = ({}), out = [];
        for (var i = 0; i < pg.providerAnimRows.length; i++) {
            var g = pg.providerAnimRows[i].group || "";
            if (!seen[g]) { seen[g] = true; out.push(g); }
        }
        return out;
    }
    function providerRowsInGroup(g) {
        var out = [];
        for (var i = 0; i < pg.providerAnimRows.length; i++)
            if ((pg.providerAnimRows[i].group || "") === g) out.push(pg.providerAnimRows[i]);
        return out;
    }
    // the provider rows as a flat dotted-key draft, so a `when` gate that names a
    // sibling key (niri gates duration/curve on mode) resolves against live values.
    readonly property var providerAnimDraft: {
        var d = ({});
        for (var i = 0; i < pg.providerAnimRows.length; i++) { var k = pg.providerAnimRows[i].key; if (k) d[k] = pg.hv(k); }
        return d;
    }
    function passesWhen(r) {
        if (!r.when) return true;
        var d = pg.providerAnimDraft;
        if (!d || Object.keys(d).length === 0) return true;
        for (var k in r.when) if (r.when[k].indexOf(d[k]) < 0) return false;
        return true;
    }
    function providerRowVisible(r) {
        if (!pg.passesWhen(r)) return false;
        if (r.adv && !(pg.hub && pg.hub.advanced) && pg.query === "") return false;
        if (pg.query === "") return true;
        return pg.hit(String(r.label) + " " + String(r.desc || "") + " " + String(r.key));
    }
    function providerGroupVisible(g) {
        var rows = pg.providerRowsInGroup(g);
        for (var i = 0; i < rows.length; i++) if (pg.providerRowVisible(rows[i])) return true;
        return false;
    }
    readonly property bool pVisible: {
        if (pg.query === "") return pg.providerAnimRows.length > 0;
        for (var i = 0; i < pg.providerAnimRows.length; i++)
            if (pg.providerRowVisible(pg.providerAnimRows[i])) return true;
        return false;
    }
    // provider-row control glue, mirroring the shared sheet's readouts so a
    // provider row reads the same as every other settings row.
    function provShown(r) {
        var v = pg.hv(r.key);
        if (r.ctl === "slid" && r.pct) return String(Math.round((Number(v) || 0) * 100));
        return v === undefined ? "" : String(v);
    }
    function provShownDef(r) {
        var d = pg.cv(r.key);
        if (d === undefined) return "";
        if (r.ctl === "slid" && r.pct) return String(Math.round((Number(d) || 0) * 100));
        return String(d);
    }
    function provChanged(r) { return JSON.stringify(pg.hv(r.key)) !== JSON.stringify(pg.cv(r.key)); }
    function provBlock(r) { return r.ctl === "chips" || (r.ctl === "seg" && (r.opts ? r.opts.length : 0) >= 3); }
    function provCtlWidth(r, w) {
        if (r.ctl === "sw") return 54;
        if (r.ctl === "step") return 58;
        if (r.ctl === "slid") return Math.min(240, Math.max(160, Math.round(w * 0.34)));
        if (r.ctl === "seg") return Math.max(140, 74 * Math.max(2, (r.opts ? r.opts.length : 2)));
        return 54;
    }
    function provCommitNumber(r, text) {
        var n = Number(String(text).replace(",", "."));
        if (isNaN(n)) return;
        if (r.ctl === "step") {
            var lo = r.lo !== undefined ? Number(r.lo) : 0, hi = r.hi !== undefined ? Number(r.hi) : 100;
            pg.he(r.key, Math.max(lo, Math.min(hi, Math.round(n))));
            return;
        }
        if (r.ctl === "slid") {
            var v = r.pct ? n / 100 : n;
            var l = r.lo !== undefined ? Number(r.lo) : 0, h = r.hi !== undefined ? Number(r.hi) : (r.pct ? 1 : 100);
            pg.he(r.key, Math.max(l, Math.min(h, v)));
        }
    }
    function provReset(r) { var d = pg.cv(r.key); if (d !== undefined) pg.he(r.key, d); }
    property string selectedCurve: ""
    // seed the selection off the first curve once the provider list arrives.
    function seedCurve() { if (pg.selectedCurve === "" && pg.liveCurves.length > 0) pg.selectedCurve = pg.liveCurves[0].name }
    onLiveCurvesChanged: pg.seedCurve()
    Component.onCompleted: pg.seedCurve()

    // theme.motion in shell.json, written instantly through the settings seam
    // (not the staged hypr draft); the shell's ui tokens read the same keys.
    readonly property real motionScale: { Settings.revision; var v = Settings.get("theme.motion.scale"); return (typeof v === "number" && v > 0) ? v : 1.0; }
    readonly property bool reduceMotion: { Settings.revision; return Settings.get("theme.motion.reduce") === true; }
    readonly property string motionScaleLabel: Math.round(pg.motionScale * 100) + "%"
    function setMotion(key, v) { Settings.patch("theme.motion." + key, v); }

    // Hyprland animation personality: the preset the loader picks. Read from the
    // backend at load; setAnimPreset writes ~/.config/ryoku/anim-preset + reloads.
    property string animPreset: "ryoku"
    readonly property var animPresets: [
        { "id": "ryoku", "label": "Ryoku" },
        { "id": "dusky", "label": "Dusky" },
        { "id": "bounce", "label": "Bounce" },
        { "id": "fade", "label": "Fade" },
        { "id": "fast", "label": "Fast" },
        { "id": "mechanical", "label": "Mechanical" },
        { "id": "minimal", "label": "Minimal" },
        { "id": "air", "label": "Air" },
        { "id": "slowmotion", "label": "Slow-mo" },
        { "id": "exaggerated", "label": "Exaggerated" },
        { "id": "hallucination", "label": "Hallucination" },
        { "id": "rage", "label": "Rage" },
        { "id": "disable", "label": "Off" }
    ]
    readonly property var animPresetLabels: pg.animPresets.map(p => p.label)
    function animPresetLabel(id) { for (var i = 0; i < pg.animPresets.length; i++) if (pg.animPresets[i].id === id) return pg.animPresets[i].label; return "Ryoku"; }
    function animPresetId(label) { for (var i = 0; i < pg.animPresets.length; i++) if (pg.animPresets[i].label === label) return pg.animPresets[i].id; return "ryoku"; }
    function setAnimPreset(id) { pg.animPreset = id; animPresetSet.command = ["ryoku-hub", "desktop", "anim-preset", id]; animPresetSet.running = true; }

    Process {
        id: animPresetGet
        command: ["ryoku-hub", "desktop", "anim-preset"]
        running: true
        stdout: StdioCollector { onStreamFinished: { try { var d = JSON.parse(this.text); if (d && d.preset) pg.animPreset = d.preset; } catch (e) {} } }
    }
    Process { id: animPresetSet }

    // ── curves model ────────────────────────────────────────────────────────
    function curvesArr() { var a = pg.hv("wm.hyprland.anim.curves"); return Array.isArray(a) ? a : []; }
    function itemsArr() { var a = pg.hv("wm.hyprland.anim.items"); return Array.isArray(a) ? a : []; }

    function curveNames() {
        var seen = ({}), out = [];
        for (var i = 0; i < pg.liveCurves.length; i++) {
            var n = pg.liveCurves[i].name;
            if (!seen[n]) { seen[n] = true; out.push(n); }
        }
        var cs = pg.curvesArr();
        for (var j = 0; j < cs.length; j++) {
            var m = cs[j].name;
            if (!seen[m]) { seen[m] = true; out.push(m); }
        }
        return out;
    }
    function curveOf(name) {
        var cs = pg.curvesArr();
        for (var i = 0; i < cs.length; i++)
            if (cs[i].name === name)
                return cs[i];
        // provider curves already use lowercase x0/y0/x1/y1, the store shape.
        for (var j = 0; j < pg.liveCurves.length; j++)
            if (pg.liveCurves[j].name === name)
                return { "name": name, "x0": pg.liveCurves[j].x0, "y0": pg.liveCurves[j].y0, "x1": pg.liveCurves[j].x1, "y1": pg.liveCurves[j].y1 };
        return { "name": name, "x0": 0.25, "y0": 0.1, "x1": 0.25, "y1": 1 };
    }
    readonly property bool selectedIsCustom: {
        for (var i = 0; i < pg.liveCurves.length; i++)
            if (pg.liveCurves[i].name === pg.selectedCurve)
                return false;
        return pg.selectedCurve !== "";
    }
    readonly property bool selectedHasOverride: {
        var cs = pg.curvesArr();
        for (var i = 0; i < cs.length; i++)
            if (cs[i].name === pg.selectedCurve)
                return true;
        return false;
    }
    function upsertCurve(name, x0, y0, x1, y1) {
        if (name === "")
            return;
        var arr = pg.curvesArr().slice(), found = false;
        for (var i = 0; i < arr.length; i++)
            if (arr[i].name === name) {
                arr[i] = { "name": name, "x0": x0, "y0": y0, "x1": x1, "y1": y1 };
                found = true;
                break;
            }
        if (!found)
            arr.push({ "name": name, "x0": x0, "y0": y0, "x1": x1, "y1": y1 });
        pg.he("wm.hyprland.anim.curves", arr);
    }
    function resetCurve(name) {
        var arr = [], cs = pg.curvesArr();
        for (var i = 0; i < cs.length; i++)
            if (cs[i].name !== name)
                arr.push(cs[i]);
        pg.he("wm.hyprland.anim.curves", arr);
        if (pg.selectedIsCustom && pg.liveCurves.length > 0)
            pg.selectedCurve = pg.liveCurves[0].name;
    }
    function addCurve() {
        var n = 1, name = "custom", names = pg.curveNames();
        while (names.indexOf(name) >= 0) { name = "custom" + n; n++; }
        pg.upsertCurve(name, 0.25, 0.1, 0.25, 1);
        pg.selectedCurve = name;
    }

    // named feels: one-tap curve shapes, friendlier than dragging handles.
    readonly property var feels: [
        { "name": "Linear", "c": [0.0, 0.0, 1.0, 1.0] },
        { "name": "Gentle", "c": [0.45, 0.0, 0.25, 1.0] },
        { "name": "Smooth", "c": [0.25, 0.1, 0.25, 1.0] },
        { "name": "Snappy", "c": [0.3, 0.0, 0.1, 1.0] },
        { "name": "Bouncy", "c": [0.34, 1.5, 0.64, 1.0] }
    ]
    function feelActive(c) {
        var q = pg.curveOf(pg.selectedCurve);
        return Math.abs(q.x0 - c[0]) < 0.03 && Math.abs(q.y0 - c[1]) < 0.03
            && Math.abs(q.x1 - c[2]) < 0.03 && Math.abs(q.y1 - c[3]) < 0.03;
    }

    // ── per-leaf overrides ──────────────────────────────────────────────────
    function itemOf(leaf) {
        var items = pg.itemsArr();
        for (var i = 0; i < items.length; i++)
            if (items[i].leaf === leaf)
                return items[i];
        for (var j = 0; j < pg.liveAnims.length; j++)
            if (pg.liveAnims[j].leaf === leaf)
                return { "leaf": leaf, "enabled": pg.liveAnims[j].enabled, "speed": pg.liveAnims[j].speed, "bezier": pg.liveAnims[j].bezier, "style": pg.liveAnims[j].style };
        return { "leaf": leaf, "enabled": true, "speed": 1, "bezier": "", "style": "" };
    }
    function upsertItem(leaf, key, val) {
        var cur = pg.itemOf(leaf);
        var next = { "leaf": leaf, "enabled": cur.enabled, "speed": cur.speed, "bezier": cur.bezier, "style": cur.style };
        next[key] = val;
        var arr = pg.itemsArr().slice(), found = false;
        for (var i = 0; i < arr.length; i++)
            if (arr[i].leaf === leaf) { arr[i] = next; found = true; break; }
        if (!found)
            arr.push(next);
        pg.he("wm.hyprland.anim.items", arr);
    }
    // Hyprland style options are grouped by leaf family; keys are the config
    // literals, labels are the human reading.
    function styleOptionsFor(leaf) {
        if (leaf.indexOf("windows") === 0)
            return [{ "key": "", "label": I18n.tr("Default") }, { "key": "slide", "label": I18n.tr("Slide") }, { "key": "popin 80%", "label": I18n.tr("Pop in") }, { "key": "gnomed", "label": I18n.tr("Gnomed") }];
        if (leaf.indexOf("workspaces") === 0 || leaf.indexOf("specialWorkspace") === 0)
            return [{ "key": "", "label": I18n.tr("Default") }, { "key": "slide", "label": I18n.tr("Slide") }, { "key": "slidevert", "label": I18n.tr("Slide vertical") }, { "key": "fade", "label": I18n.tr("Fade") }, { "key": "slidefade", "label": I18n.tr("Slide + fade") }, { "key": "slidefadevert", "label": I18n.tr("Slide + fade vertical") }];
        if (leaf.indexOf("layers") === 0)
            return [{ "key": "", "label": I18n.tr("Default") }, { "key": "slide", "label": I18n.tr("Slide") }, { "key": "popin 90%", "label": I18n.tr("Pop in") }, { "key": "fade", "label": I18n.tr("Fade") }];
        return [];
    }

    // ── search integration ──────────────────────────────────────────────────
    readonly property string query: pg.hub ? (pg.hub.query || "") : ""
    function hit(s) { return pg.query === "" || String(s).toLowerCase().indexOf(pg.query.toLowerCase()) >= 0 }
    readonly property bool gVisible: pg.hit("global animations master switch desktop motion")
    readonly property bool fVisible: pg.hit("focus flash animate focused window style opacity bounce strength slide height")
    readonly property bool cVisible: pg.hit("feel motion smooth easing preset speed curves bezier curve editor control point handles " + pg.curveNames().join(" "))
    readonly property bool aVisible: {
        if (pg.query === "") return true;
        if (pg.hit("animations")) return true;
        for (var i = 0; i < pg.liveAnims.length; i++)
            if (String(pg.liveAnims[i].leaf).toLowerCase().indexOf(pg.query.toLowerCase()) >= 0)
                return true;
        return false;
    }
    readonly property bool anyVisible: pg.gVisible || pg.fVisible || pg.cVisible || pg.aVisible || pg.pVisible

    // ── the page-local catalogue overlay (curve + style + bezier pickers) ────
    // The shell's picker is wired to its JSON store, not the hypr store, so the
    // dropdowns here open this instead. Short lists, so no filter field.
    function openPicker(title, opts, current, cb) {
        var norm = [];
        for (var i = 0; i < opts.length; i++) {
            var o = opts[i];
            norm.push((typeof o === "string") ? { "key": o, "label": o } : o);
        }
        pk.title = title; pk.opts = norm; pk.current = current; pk.cb = cb;
    }
    function closePicker() { pk.opts = null; pk.cb = null; }
    function choosePick(key) { if (pk.cb) pk.cb(key); pg.closePicker(); }

    // ── inline components ────────────────────────────────────────────────────

    // a fractional stepper (speed is 0.1s units; the module Step is integer).
    component NumStep: Row {
        id: ns
        property real value: 1
        property real min: 0.1
        property real max: 10
        property real by: 0.1
        signal changed(real v)
        spacing: 0
        function clamp(v) { return Math.max(ns.min, Math.min(ns.max, Math.round(v * 10) / 10)); }
        Rectangle {
            readonly property bool spent: ns.value <= ns.min + 0.0001
            width: 29; height: Tokens.ctlH; radius: Tokens.radius
            opacity: spent ? 0.3 : 1
            color: mh.hovered && !spent ? Tokens.tint10 : "transparent"
            border.width: Tokens.border
            border.color: mh.hovered && !spent ? Tokens.lineStrong : Tokens.line
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Text { anchors.centerIn: parent; text: "\u2212"; color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            HoverHandler { id: mh; enabled: !parent.spent; cursorShape: Qt.PointingHandCursor }
            TapHandler { enabled: !parent.spent; onTapped: ns.changed(ns.clamp(ns.value - ns.by)) }
        }
        Text {
            width: 46; height: Tokens.ctlH
            horizontalAlignment: Text.AlignHCenter; verticalAlignment: Text.AlignVCenter
            text: ns.value.toFixed(1); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall
        }
        Rectangle {
            readonly property bool spent: ns.value >= ns.max - 0.0001
            width: 29; height: Tokens.ctlH; radius: Tokens.radius
            opacity: spent ? 0.3 : 1
            color: ph.hovered && !spent ? Tokens.tint10 : "transparent"
            border.width: Tokens.border
            border.color: ph.hovered && !spent ? Tokens.lineStrong : Tokens.line
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            Text { anchors.centerIn: parent; text: "+"; color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall }
            HoverHandler { id: ph; enabled: !parent.spent; cursorShape: Qt.PointingHandCursor }
            TapHandler { enabled: !parent.spent; onTapped: ns.changed(ns.clamp(ns.value + ns.by)) }
        }
    }

    // a compact dropdown trigger. Emits activated() to open the overlay and
    // picked() when the overlay reports a choice.
    component MiniPick: Rectangle {
        id: mp
        property var opts: []
        property string current: ""
        property string ph: I18n.tr("select")
        property string heading: I18n.tr("Select")
        signal picked(string key)
        signal activated()
        implicitWidth: 132; height: Tokens.ctlH; radius: Tokens.radius
        color: mph.hovered ? Tokens.tint10 : "transparent"
        border.width: Tokens.border
        border.color: mph.hovered ? Tokens.lineStrong : Tokens.line
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        function labelOf(k) {
            for (var i = 0; i < mp.opts.length; i++) {
                var o = mp.opts[i], kk = (typeof o === "string") ? o : o.key;
                if (kk === k) return (typeof o === "string") ? o : o.label;
            }
            return "";
        }
        readonly property string curLabel: mp.labelOf(mp.current)
        Text {
            anchors { left: parent.left; leftMargin: Tokens.s2; right: car.left; rightMargin: Tokens.s1; verticalCenter: parent.verticalCenter }
            text: mp.curLabel !== "" ? mp.curLabel : mp.ph
            color: mp.curLabel !== "" ? Tokens.inkDim : Tokens.inkMuted
            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; elide: Text.ElideRight
        }
        Text {
            id: car
            anchors { right: parent.right; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
            text: "\u25be"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
        }
        HoverHandler { id: mph; cursorShape: Qt.PointingHandCursor }
        TapHandler { onTapped: mp.activated() }
    }

    // the cubic bezier editor. Endpoints pinned at (0,0) and (1,1); the two
    // control points drag freely. X clamps to [0,1] (monotonic in time), Y to
    // [-1,2] (overshoot for bounce). antialiasing off, ink strokes.
    component BezierEditor: Item {
        id: ed
        property real x0: 0.25
        property real y0: 0.1
        property real x1: 0.25
        property real y1: 1.0
        signal changed(real x0, real y0, real x1, real y1)
        readonly property real pad: 18
        readonly property real yMin: -1.0
        readonly property real yMax: 2.0
        implicitWidth: 300
        implicitHeight: 280
        function px(x) { return ed.pad + x * (ed.width - 2 * ed.pad); }
        function py(y) { return ed.height - ed.pad - (y - ed.yMin) / (ed.yMax - ed.yMin) * (ed.height - 2 * ed.pad); }
        function ux(p) { return Math.max(0, Math.min(1, (p - ed.pad) / (ed.width - 2 * ed.pad))); }
        function uy(p) { return Math.max(ed.yMin, Math.min(ed.yMax, ed.yMin + (ed.height - ed.pad - p) / (ed.height - 2 * ed.pad) * (ed.yMax - ed.yMin))); }
        onX0Changed: cv.requestPaint()
        onY0Changed: cv.requestPaint()
        onX1Changed: cv.requestPaint()
        onY1Changed: cv.requestPaint()
        onWidthChanged: cv.requestPaint()
        onHeightChanged: cv.requestPaint()

        Rectangle { anchors.fill: parent; radius: Tokens.radius; color: "transparent"; border.width: Tokens.border; border.color: Tokens.line }

        Canvas {
            id: cv
            anchors.fill: parent
            antialiasing: false
            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                // baselines at progress 0 and 1
                ctx.lineWidth = 1;
                ctx.strokeStyle = Tokens.lineSoft;
                ctx.beginPath();
                ctx.moveTo(ed.px(0), ed.py(0));
                ctx.lineTo(ed.px(1), ed.py(0));
                ctx.moveTo(ed.px(0), ed.py(1));
                ctx.lineTo(ed.px(1), ed.py(1));
                ctx.stroke();
                // handle arms
                ctx.strokeStyle = Tokens.inkMuted;
                ctx.beginPath();
                ctx.moveTo(ed.px(0), ed.py(0));
                ctx.lineTo(ed.px(ed.x0), ed.py(ed.y0));
                ctx.moveTo(ed.px(1), ed.py(1));
                ctx.lineTo(ed.px(ed.x1), ed.py(ed.y1));
                ctx.stroke();
                // the curve itself (data plot, not chrome)
                ctx.strokeStyle = Tokens.ink;
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(ed.px(0), ed.py(0));
                ctx.bezierCurveTo(ed.px(ed.x0), ed.py(ed.y0), ed.px(ed.x1), ed.py(ed.y1), ed.px(1), ed.py(1));
                ctx.stroke();
            }
        }

        // control-point pucks: hairline square at rest, solid ink while dragging
        Rectangle {
            width: 14; height: 14; radius: Tokens.radius; antialiasing: false
            x: ed.px(ed.x0) - 7; y: ed.py(ed.y0) - 7
            color: hd0.active ? Tokens.ink : "transparent"
            border.width: Tokens.border; border.color: Tokens.ink
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            DragHandler {
                id: hd0
                target: null
                property real lastX: 0
                property real lastY: 0
                onActiveChanged: if (active) { lastX = 0; lastY = 0; }
                onTranslationChanged: {
                    var nx = ed.ux(ed.px(ed.x0) + (translation.x - lastX));
                    var ny = ed.uy(ed.py(ed.y0) + (translation.y - lastY));
                    lastX = translation.x; lastY = translation.y;
                    ed.changed(nx, ny, ed.x1, ed.y1);
                }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }
        Rectangle {
            width: 14; height: 14; radius: Tokens.radius; antialiasing: false
            x: ed.px(ed.x1) - 7; y: ed.py(ed.y1) - 7
            color: hd1.active ? Tokens.ink : "transparent"
            border.width: Tokens.border; border.color: Tokens.ink
            Behavior on color { ColorAnimation { duration: Tokens.snap } }
            DragHandler {
                id: hd1
                target: null
                property real lastX: 0
                property real lastY: 0
                onActiveChanged: if (active) { lastX = 0; lastY = 0; }
                onTranslationChanged: {
                    var nx = ed.ux(ed.px(ed.x1) + (translation.x - lastX));
                    var ny = ed.uy(ed.py(ed.y1) + (translation.y - lastY));
                    lastX = translation.x; lastY = translation.y;
                    ed.changed(ed.x0, ed.y0, nx, ny);
                }
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
        }
    }

    // a live value preview: a window slides with the selected curve, so the
    // easing is felt, not read off control points. Plays on change and on tap
    // -- feedback, never a perpetual loop (that is the Decor's alone).
    component MotionPreview: Item {
        id: mv
        property real x0: 0.25
        property real y0: 0.1
        property real x1: 0.25
        property real y1: 1.0
        property int dur: 640
        property string label: ""
        readonly property real pad: Tokens.s5
        readonly property real winW: 56
        readonly property real leftW: Math.round(mv.width * 0.24)
        readonly property real trackY: Math.round(mv.height * 0.62)
        readonly property real dest: 0.82 * (mv.width - mv.leftW - 2 * mv.pad - mv.winW)
        height: 122

        Rectangle {
            anchors.fill: parent
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: stageH.hovered ? Tokens.lineStrong : Tokens.line
            Behavior on border.color { ColorAnimation { duration: Tokens.snap } }
        }
        Text {
            id: pvLabel
            anchors { left: parent.left; leftMargin: Tokens.s4; top: parent.top; topMargin: Tokens.s3 }
            text: I18n.tr("PREVIEW"); color: Tokens.inkDim
            font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackMark
        }
        Text {
            anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: pvLabel.verticalCenter }
            text: I18n.tr("tap to replay"); color: Tokens.inkFaint
            font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
        }

        // left: what is being previewed
        Column {
            anchors { left: parent.left; leftMargin: Tokens.s4; verticalCenter: parent.verticalCenter; verticalCenterOffset: Tokens.s2 }
            width: mv.leftW - Tokens.s4
            spacing: 2
            Text { text: I18n.tr("CURVE"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackMark }
            Text {
                width: parent.width
                text: mv.label.length ? I18n.tr(mv.label) : I18n.tr("curve")
                color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: Tokens.fBody; font.weight: Font.Medium; elide: Text.ElideRight
            }
            Text { text: mv.dur + " ms"; color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny }
        }

        // right: the runway the window travels, a measured track
        Item {
            id: runway
            anchors { left: parent.left; right: parent.right; top: parent.top; bottom: parent.bottom }
            anchors.leftMargin: mv.leftW + mv.pad; anchors.rightMargin: mv.pad
            clip: true
            Repeater {
                model: 21
                delegate: Rectangle {
                    required property int index
                    readonly property bool major: index % 5 === 0
                    x: index / 20 * runway.width
                    y: mv.trackY; width: 1; height: major ? 7 : 4
                    color: major ? Tokens.line : Tokens.lineSoft
                }
            }
            Rectangle {
                anchors { left: parent.left; right: parent.right }
                y: mv.trackY; height: 1; color: Tokens.line
            }
            Rectangle {
                width: mv.winW; height: 36; radius: Tokens.radius
                color: "transparent"; border.width: Tokens.border; border.color: Tokens.line
                y: mv.trackY - height; x: mv.dest
            }
            Rectangle {
                id: win
                width: mv.winW; height: 36; radius: Tokens.radius
                color: Tokens.bone
                y: mv.trackY - height; x: 0
                Text { anchors.centerIn: parent; text: UiPreferences.brandMark("\u529b"); color: Tokens.inkOnBone; font.family: Tokens.jp; font.pixelSize: Tokens.fSmall }
            }
            NumberAnimation {
                id: anim
                target: win; property: "x"
                from: 0; to: mv.dest
                duration: mv.dur
                easing.type: Easing.Bezier
                easing.bezierCurve: [mv.x0, mv.y0, mv.x1, mv.y1, 1.0, 1.0]
            }
            Text {
                anchors { left: parent.left; top: parent.top; topMargin: mv.trackY + Tokens.s2 }
                text: I18n.tr("START"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackMark
            }
            Text {
                anchors { right: parent.right; top: parent.top; topMargin: mv.trackY + Tokens.s2 }
                text: I18n.tr("SETTLE"); color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackMark
            }
        }

        Timer { id: replay; interval: 150; onTriggered: mv._play() }
        function _play() { anim.stop(); win.x = 0; anim.start(); }
        function play() { replay.restart(); }   // debounced: a drag coalesces into one replay
        onX0Changed: mv.play()
        onY0Changed: mv.play()
        onX1Changed: mv.play()
        onY1Changed: mv.play()
        Component.onCompleted: mv.play()
        TapHandler { onTapped: mv._play() }
        HoverHandler { id: stageH; cursorShape: Qt.PointingHandCursor }
    }

    // ── head ─────────────────────────────────────────────────────────────────
    Column {
        id: head
        anchors.top: parent.top
        // framed page: the pageArea already insets it, so the head starts at the
        // body's own left edge, over the first card column
        width: Math.max(320, pg.width - Tokens.s4)
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle { width: 16; height: 1; color: Tokens.ink; anchors.verticalCenter: parent.verticalCenter }
            Text { text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp; font.pixelSize: Tokens.fMicro; anchors.verticalCenter: parent.verticalCenter }
            Text {
                text: I18n.tr("DESKTOP"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text { text: I18n.tr("Animations"); color: Tokens.ink; font.family: Tokens.display; font.pixelSize: Tokens.fTitle }
        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Pick a feel and watch it live; fine-tune any one animation under Advanced.")
            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
        Item { width: 1; height: Tokens.s1 }
    }

    // ── content ────────────────────────────────────────────────────────────
    Flickable {
        id: flick
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: parent.bottom; topMargin: Tokens.s5 }
        contentWidth: width
        contentHeight: col.height + Tokens.s5
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { }
        WheelScroll { }

        CardColumns {

        id: col
            // a body of cards fills the measure and splits into balanced columns
            width: flick.width - Tokens.s4
            spacing: Tokens.s5
            fillTo: flick.height
            // The shell's own motion (distinct from the Hyprland window editor below).
            SettingCard {
                width: col.colWidth
                title: I18n.tr("SHELL MOTION")
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("How fast the shell's own panels, menus and transitions move. Speed scales every Ryoku animation live; Reduce motion snaps them into place for comfort.") + (pg.hasWindowAnims ? " " + I18n.tr("Separate from the window animations below.") : "")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Speed")
                    value: pg.motionScaleLabel
                    controlWidth: Math.min(240, Math.max(160, Math.round(col.width * 0.34)))
                    Slid {
                        anchors.fill: parent
                        from: 0.25; to: 3.0
                        value: pg.motionScale
                        onModified: (v) => pg.setMotion("scale", Math.round(v * 20) / 20)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    label: I18n.tr("Reduce motion")
                    desc: I18n.tr("Snap animations to their end instead of playing them.")
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        on: pg.reduceMotion
                        onToggled: (v) => pg.setMotion("reduce", v)
                    }
                }
            }

            SettingCard {
                width: col.colWidth
                title: I18n.tr("ANIMATION PRESET")
                visible: Settings.supports("animations") && pg.hasHyprAnims
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("The window animation personality: how windows, workspaces and layers move. Picking one reloads Hyprland; your curve tweaks below still apply on top.")
                    color: Tokens.inkMuted; font.family: Tokens.ui
                    font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    label: I18n.tr("Preset")
                    Chips {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: pg.animPresetLabels
                        current: pg.animPresetLabel(pg.animPreset)
                        onChose: (l) => pg.setAnimPreset(pg.animPresetId(l))
                    }
                }
            }

            // MOTION -- the global motion switch plus the bespoke curve workshop
            SettingCard {
                width: col.colWidth
                title: I18n.tr("MOTION")
                visible: pg.gVisible || pg.cVisible

                // the one genuine setting: the master switch for desktop motion
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.hit("animations master switch desktop motion")
                    label: I18n.tr("Animations")
                    desc: I18n.tr("Master switch for desktop motion")
                    def: pg.cv("desktop.appearance.animations") ? I18n.tr("ON") : I18n.tr("OFF")
                    changed: pg.chg("desktop.appearance.animations")
                    source: "desktop.json"
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        on: !!pg.hv("desktop.appearance.animations")
                        onToggled: (v) => pg.he("desktop.appearance.animations", v)
                    }
                }

                // the curve workshop -- bespoke: curve select + NEW/DELETE, one-tap
                // feels, the live motion preview, and the cubic-bezier editor with
                // its readouts and decor. Kept intact from the original MOTION group.
                Item {
                    width: parent.width
                    height: workshop.height + 2 * Tokens.s4
                    visible: Settings.supports("animations") && pg.hasHyprAnims

                    // hairline off the switch row above (only while it is shown)
                    Rectangle {
                        visible: pg.hit("animations master switch desktop motion")
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                        height: 1; color: Tokens.lineSoft
                    }

                    Column {
                        id: workshop
                        anchors { left: parent.left; right: parent.right; top: parent.top }
                        anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4; anchors.topMargin: Tokens.s4
                        spacing: Tokens.s4

                        // curve selector + NEW / DELETE-RESET
                        Item {
                            width: parent.width
                            height: Tokens.ctlH
                            MiniPick {
                                id: curveSel
                                anchors.left: parent.left
                                anchors.verticalCenter: parent.verticalCenter
                                width: Math.max(180, parent.width - newBtn.width - delBtn.width - 2 * Tokens.s2)
                                heading: I18n.tr("Curve")
                                ph: I18n.tr("no curves")
                                opts: pg.curveNames()
                                current: pg.selectedCurve
                                onActivated: pg.openPicker(I18n.tr("Curve"), curveSel.opts, curveSel.current, function (k) { curveSel.picked(k); })
                                onPicked: (k) => pg.selectedCurve = k
                            }
                            Btn {
                                id: newBtn
                                anchors.right: delBtn.left
                                anchors.rightMargin: Tokens.s2
                                anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("NEW")
                                onAct: pg.addCurve()
                            }
                            Btn {
                                id: delBtn
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                text: pg.selectedIsCustom ? I18n.tr("DELETE") : I18n.tr("RESET")
                                armed: pg.selectedIsCustom || pg.selectedHasOverride
                                onAct: pg.resetCurve(pg.selectedCurve)
                            }
                        }

                        // one-tap feels -- a starting point, friendlier than handles
                        Column {
                            width: parent.width
                            spacing: Tokens.s2

                            Flow {
                                width: parent.width
                                spacing: Tokens.s2

                                Text {
                                    height: Tokens.ctlH
                                    verticalAlignment: Text.AlignVCenter
                                    text: I18n.tr("FEEL"); color: Tokens.inkFaint
                                    font.family: Tokens.mono; font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackMark
                                }
                                Repeater {
                                    model: pg.feels
                                    delegate: Rectangle {
                                        id: chip
                                        required property var modelData
                                        readonly property bool active: pg.feelActive(chip.modelData.c)
                                        width: chipT.implicitWidth + Tokens.s4; height: Tokens.ctlH
                                        radius: Tokens.radius
                                        color: chip.active ? Tokens.bone : (chh.hovered ? Tokens.tint10 : "transparent")
                                        border.width: Tokens.border
                                        border.color: chip.active ? Tokens.bone : (chh.hovered ? Tokens.lineStrong : Tokens.line)
                                        Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                        Text {
                                            id: chipT
                                            anchors.centerIn: parent
                                            text: chip.modelData.name
                                            color: chip.active ? Tokens.inkOnBone : Tokens.inkDim
                                            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                        }
                                        HoverHandler { id: chh; cursorShape: Qt.PointingHandCursor }
                                        TapHandler { onTapped: pg.upsertCurve(pg.selectedCurve, chip.modelData.c[0], chip.modelData.c[1], chip.modelData.c[2], chip.modelData.c[3]) }
                                    }
                                }
                            }
                            Text {
                                width: parent.width
                                horizontalAlignment: Text.AlignRight
                                text: I18n.tr("a one-tap starting point"); color: Tokens.inkFaint
                                font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                                wrapMode: Text.WordWrap
                            }
                        }

                        // the selected curve, felt -- sits between its controls
                        MotionPreview {
                            width: parent.width
                            label: pg.selectedCurve
                            x0: pg.curveOf(pg.selectedCurve).x0
                            y0: pg.curveOf(pg.selectedCurve).y0
                            x1: pg.curveOf(pg.selectedCurve).x1
                            y1: pg.curveOf(pg.selectedCurve).y1
                        }

                        // editor + readouts + decor
                        Row {
                            width: parent.width
                            spacing: Tokens.s5

                            BezierEditor {
                                id: bez
                                width: Math.min(implicitWidth, Math.floor((parent.width - parent.spacing) * 0.66))
                                height: 280
                                x0: pg.curveOf(pg.selectedCurve).x0
                                y0: pg.curveOf(pg.selectedCurve).y0
                                x1: pg.curveOf(pg.selectedCurve).x1
                                y1: pg.curveOf(pg.selectedCurve).y1
                                onChanged: (a, b, c, d) => pg.upsertCurve(pg.selectedCurve, a, b, c, d)
                            }

                            Column {
                                id: readouts
                                width: Math.max(0, parent.width - bez.width - parent.spacing)
                                anchors.verticalCenter: parent.verticalCenter
                                spacing: Tokens.s3

                                Text {
                                    width: parent.width
                                    text: I18n.tr("Fine-tune the handles.")
                                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                    wrapMode: Text.WordWrap
                                }
                                Text { width: parent.width; text: I18n.tr("P1   ") + bez.x0.toFixed(2) + ", " + bez.y0.toFixed(2); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall }
                                Text { width: parent.width; text: I18n.tr("P2   ") + bez.x1.toFixed(2) + ", " + bez.y1.toFixed(2); color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: Tokens.fSmall }
                                Text {
                                    width: parent.width
                                    text: I18n.tr("Presets set the shape; drag to fine-tune. Curves are shared by name, and Advanced animations reference them.")
                                    color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                                }
                            }
                        }
                    }
                }
            }

            // FOCUS FLASH
            SettingCard {
                id: fsec
                width: col.colWidth
                title: I18n.tr("FOCUS FLASH")
                visible: pg.fVisible && Settings.supports("plugins")

                readonly property bool ffOn: !!pg.hv("wm.hyprland.plugins.hyprfocus.enabled")
                readonly property string ffMode: String(pg.hv("wm.hyprland.plugins.hyprfocus.mode"))

                // group note, shown once the effect is on (was the trailing blurb)
                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    visible: fsec.ffOn
                    text: I18n.tr("Briefly flashes, bounces, or slides a window when it gains focus. Applies on Save.")
                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }

                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: fsec.ffOn
                    visible: pg.hit("animate the focused window enabled")
                    label: I18n.tr("Animate the focused window")
                    desc: I18n.tr("A short effect on the focused window")
                    def: pg.cv("wm.hyprland.plugins.hyprfocus.enabled") ? I18n.tr("ON") : I18n.tr("OFF")
                    changed: pg.chg("wm.hyprland.plugins.hyprfocus.enabled")
                    source: "desktop.json"
                    controlWidth: 54
                    Sw {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        on: fsec.ffOn
                        onToggled: (v) => pg.he("wm.hyprland.plugins.hyprfocus.enabled", v)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    block: true
                    visible: fsec.ffOn && pg.hit("style flash bounce slide")
                    label: I18n.tr("Style")
                    desc: I18n.tr("Flash dips, Bounce springs, Slide nudges")
                    def: pg.cap(String(pg.cv("wm.hyprland.plugins.hyprfocus.mode")))
                    changed: pg.chg("wm.hyprland.plugins.hyprfocus.mode")
                    source: "desktop.json"
                    Seg {
                        anchors.left: parent.left; anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        options: ["flash", "bounce", "slide"]
                        current: fsec.ffMode
                        onChose: (k) => pg.he("wm.hyprland.plugins.hyprfocus.mode", k)
                    }
                }
                // opacity/bounce are fractional (0..1); the module Slid is integer,
                // so map to a 0..100 percent domain and store /100.
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    visible: fsec.ffOn && fsec.ffMode === "flash" && pg.hit("flash opacity")
                    label: I18n.tr("Flash opacity")
                    desc: I18n.tr("How far the flash dips; Flash style only")
                    unit: "%"
                    value: String(Math.round((Number(pg.hv("wm.hyprland.plugins.hyprfocus.opacity")) || 0) * 100))
                    def: String(Math.round((Number(pg.cv("wm.hyprland.plugins.hyprfocus.opacity")) || 0) * 100))
                    changed: pg.chg("wm.hyprland.plugins.hyprfocus.opacity")
                    source: "desktop.json"
                    controlWidth: Math.min(240, Math.max(160, Math.round(fsec.width * 0.34)))
                    Slid {
                        anchors.fill: parent
                        from: 0; to: 100
                        value: Math.round((Number(pg.hv("wm.hyprland.plugins.hyprfocus.opacity")) || 0) * 100)
                        onModified: (v) => pg.he("wm.hyprland.plugins.hyprfocus.opacity", v / 100)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    visible: fsec.ffOn && fsec.ffMode === "bounce" && pg.hit("bounce strength")
                    label: I18n.tr("Bounce strength")
                    desc: I18n.tr("How far the window shrinks; Bounce style only")
                    unit: "%"
                    value: String(Math.round((Number(pg.hv("wm.hyprland.plugins.hyprfocus.bounce")) || 0) * 100))
                    def: String(Math.round((Number(pg.cv("wm.hyprland.plugins.hyprfocus.bounce")) || 0) * 100))
                    changed: pg.chg("wm.hyprland.plugins.hyprfocus.bounce")
                    source: "desktop.json"
                    controlWidth: Math.min(240, Math.max(160, Math.round(fsec.width * 0.34)))
                    Slid {
                        anchors.fill: parent
                        from: 50; to: 100
                        value: Math.round((Number(pg.hv("wm.hyprland.plugins.hyprfocus.bounce")) || 0) * 100)
                        onModified: (v) => pg.he("wm.hyprland.plugins.hyprfocus.bounce", v / 100)
                    }
                }
                SettingRow {
                    anchors.left: parent.left; anchors.right: parent.right
                    divider: true
                    visible: fsec.ffOn && fsec.ffMode === "slide" && pg.hit("slide height")
                    label: I18n.tr("Slide height")
                    desc: I18n.tr("How far the window hops, in pixels; Slide style only")
                    unit: "px"
                    value: String(Math.round(Number(pg.hv("wm.hyprland.plugins.hyprfocus.slide")) || 0))
                    def: String(Math.round(Number(pg.cv("wm.hyprland.plugins.hyprfocus.slide")) || 0))
                    changed: pg.chg("wm.hyprland.plugins.hyprfocus.slide")
                    source: "desktop.json"
                    controlWidth: 58
                    Step {
                        anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                        from: 0; to: 150
                        value: Math.round(Number(pg.hv("wm.hyprland.plugins.hyprfocus.slide")) || 0)
                        onModified: (v) => pg.he("wm.hyprland.plugins.hyprfocus.slide", v)
                    }
                }
            }

            // ADVANCED -- per-animation control; the leaf table is a bespoke list
            SettingCard {
                width: col.colWidth
                title: I18n.tr("ADVANCED")
                visible: pg.aVisible && Settings.supports("animations") && pg.hasHyprAnims

                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4
                    topPadding: Tokens.s3; bottomPadding: Tokens.s1
                    text: I18n.tr("Per-animation control, for when a feel isn't enough. Each row is one desktop animation: turn it off, change its speed, its curve, or its style.")
                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; wrapMode: Text.WordWrap
                }

                Text {
                    width: parent.width
                    leftPadding: Tokens.s4; rightPadding: Tokens.s4; bottomPadding: Tokens.s3
                    visible: pg.liveAnims.length === 0
                    text: I18n.tr("No tunable animations reported.")
                    color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }

                // one flat drawer row per tunable leaf: name, then its four controls
                // (enable, speed, style, curve). A bespoke multi-control list, so not
                // a SettingRow, but styled to sit in the drawer.
                Repeater {
                    model: pg.liveAnims
                    delegate: Item {
                        id: ar
                        required property var modelData
                        readonly property string leaf: modelData.leaf
                        readonly property var it: pg.itemOf(ar.leaf)
                        readonly property var styleOpts: pg.styleOptionsFor(ar.leaf)
                        readonly property bool on: !!ar.it.enabled
                        readonly property real innerWidth: Math.max(0, width - 2 * Tokens.s4)
                        readonly property real minimumNameWidth: 6 * Tokens.s4
                        readonly property real minimumPickWidth: 6 * Tokens.s4
                        readonly property int pickerCount: ar.styleOpts.length > 0 ? 2 : 1
                        readonly property real fixedControlsWidth: animSwitch.implicitWidth + speedStep.implicitWidth
                        readonly property real pickWidth: Math.min(bezP.implicitWidth, Math.max(ar.minimumPickWidth,
                            Math.floor((ar.innerWidth - ar.fixedControlsWidth - (ar.pickerCount + 1) * Tokens.s3) / ar.pickerCount)))
                        readonly property bool stackControls: ctl.implicitWidth + ar.minimumNameWidth + Tokens.s3 > ar.innerWidth

                        anchors.left: parent.left; anchors.right: parent.right
                        height: ar.stackControls
                            ? Tokens.s4 + nameLabel.implicitHeight + Tokens.s2 + ctl.implicitHeight + Tokens.s4
                            : Tokens.rowH
                        visible: pg.hit(ar.leaf)

                        Rectangle {
                            anchors.fill: parent
                            anchors.margins: 1
                            radius: Tokens.radius
                            color: arh.hovered ? Tokens.tint5 : "transparent"
                            Behavior on color { ColorAnimation { duration: Tokens.snap } }
                        }
                        Rectangle {
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                            height: 1; color: Tokens.lineSoft
                        }
                        HoverHandler { id: arh }

                        Text {
                            id: nameLabel
                            x: Tokens.s4
                            y: ar.stackControls ? Tokens.s4 : Math.round((ar.height - height) / 2)
                            width: ar.stackControls
                                ? ar.innerWidth
                                : Math.max(ar.minimumNameWidth, ctl.x - Tokens.s3 - x)
                            text: ar.leaf
                            elide: Text.ElideRight
                            color: ar.on ? Tokens.ink : Tokens.inkFaint
                            font.family: Tokens.ui; font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                        }
                        Row {
                            id: ctl
                            x: ar.stackControls ? Tokens.s4 : ar.width - Tokens.s4 - width
                            y: ar.stackControls
                                ? Tokens.s4 + nameLabel.implicitHeight + Tokens.s2
                                : Math.round((ar.height - height) / 2)
                            spacing: Tokens.s3

                            Sw {
                                id: animSwitch
                                anchors.verticalCenter: parent.verticalCenter
                                on: ar.on
                                onToggled: (v) => pg.upsertItem(ar.leaf, "enabled", v)
                            }
                            NumStep {
                                anchors.verticalCenter: parent.verticalCenter
                                id: speedStep
                                value: Number(ar.it.speed) || 0
                                min: 0.1; max: 10
                                onChanged: (v) => pg.upsertItem(ar.leaf, "speed", v)
                            }
                            MiniPick {
                                id: styleP
                                width: ar.pickWidth
                                anchors.verticalCenter: parent.verticalCenter
                                visible: ar.styleOpts.length > 0
                                heading: I18n.tr("Style"); ph: I18n.tr("style")
                                opts: ar.styleOpts
                                current: String(ar.it.style || "")
                                onActivated: pg.openPicker(I18n.tr("Style"), styleP.opts, styleP.current, function (k) { styleP.picked(k); })
                                onPicked: (k) => pg.upsertItem(ar.leaf, "style", k)
                            }
                            MiniPick {
                                id: bezP
                                width: ar.pickWidth
                                anchors.verticalCenter: parent.verticalCenter
                                heading: I18n.tr("Curve"); ph: I18n.tr("curve")
                                opts: pg.curveNames()
                                current: String(ar.it.bezier || "")
                                onActivated: pg.openPicker(I18n.tr("Curve"), bezP.opts, bezP.current, function (k) { bezP.picked(k); })
                                onPicked: (k) => pg.upsertItem(ar.leaf, "bezier", k)
                            }
                        }
                    }
                }
            }

            // Provider animation rows: a compositor that models its own
            // page:"animations" schema (niri's per-kind spring/ease curves) drives
            // them here through the same primitives every settings row uses, one
            // card per declared group, below the Hyprland editor. Empty on Hyprland.
            Repeater {
                model: pg.providerAnimGroups
                delegate: SettingCard {
                    id: pcard
                    required property string modelData
                    width: col.colWidth
                    title: I18n.tr(pcard.modelData === "" ? "ANIMATION" : pcard.modelData)
                    visible: pg.providerGroupVisible(pcard.modelData)

                    Repeater {
                        model: pg.providerRowsInGroup(pcard.modelData)
                        delegate: SettingRow {
                            id: prow
                            required property var modelData
                            required property int index
                            readonly property var r: prow.modelData
                            anchors.left: parent.left; anchors.right: parent.right
                            divider: index > 0
                            visible: pg.providerRowVisible(prow.r)
                            label: I18n.tr(prow.r.label || "")
                            desc: I18n.tr(prow.r.desc || "")
                            eg: I18n.tr(prow.r.eg || "")
                            editableValue: prow.r.ctl === "step" || prow.r.ctl === "slid"
                            onValueCommitted: (text) => pg.provCommitNumber(prow.r, text)
                            value: (prow.r.ctl === "step" || prow.r.ctl === "slid") ? pg.provShown(prow.r) : ""
                            unit: (prow.r.ctl === "step" || prow.r.ctl === "slid") ? (prow.r.pct ? "%" : (prow.r.unit || "")) : ""
                            def: pg.provShownDef(prow.r)
                            changed: pg.provChanged(prow.r)
                            source: "desktop.json"
                            block: pg.provBlock(prow.r)
                            footH: pg.provBlock(prow.r) ? 0 : ((prow.r.ctl === "text" || prow.r.ctl === "color") ? 32 : 0)
                            controlWidth: pg.provCtlWidth(prow.r, pcard.width)
                            onResetRequested: pg.provReset(prow.r)

                            Loader {
                                anchors.fill: parent
                                sourceComponent: {
                                    switch (prow.r.ctl) {
                                    case "sw": return pSwC;
                                    case "step": return pStepC;
                                    case "slid": return pSlidC;
                                    case "seg": return pSegC;
                                    case "chips": return pChipsC;
                                    case "color": return pColorC;
                                    default: return pTextC;
                                    }
                                }
                            }
                            Component { id: pSwC
                                Sw { anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                    on: !!pg.hv(prow.r.key); onToggled: (v) => pg.he(prow.r.key, v) } }
                            Component { id: pStepC
                                Step { anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                    value: Number(pg.hv(prow.r.key)) || 0
                                    from: prow.r.lo !== undefined ? Number(prow.r.lo) : 0
                                    to: prow.r.hi !== undefined ? Number(prow.r.hi) : 100
                                    onModified: (v) => pg.he(prow.r.key, v) } }
                            Component { id: pSlidC
                                Slid { anchors.fill: parent
                                    value: Number(pg.hv(prow.r.key)) || 0
                                    from: prow.r.lo !== undefined ? Number(prow.r.lo) : 0
                                    to: prow.r.hi !== undefined ? Number(prow.r.hi) : 1
                                    onModified: (v) => pg.he(prow.r.key, v) } }
                            Component { id: pSegC
                                Seg { anchors.left: parent.left; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                    options: prow.r.opts || []; labels: prow.r.optLabels || ({})
                                    current: String(pg.hv(prow.r.key)); onChose: (k) => pg.he(prow.r.key, k) } }
                            Component { id: pChipsC
                                Chips { anchors.fill: parent
                                    options: prow.r.opts || []; labels: prow.r.optLabels || ({})
                                    current: String(pg.hv(prow.r.key)); onChose: (k) => pg.he(prow.r.key, k) } }
                            Component { id: pColorC
                                ColorField { anchors.fill: parent
                                    value: String(pg.hv(prow.r.key)); onChosen: (v) => pg.he(prow.r.key, v) } }
                            Component { id: pTextC
                                Field { anchors.fill: parent; tabular: true; placeholder: prow.r.eg || ""
                                    text: { var v = pg.hv(prow.r.key); return v === undefined ? "" : String(v); }
                                    onCommitted: (v) => pg.he(prow.r.key, v) } }
                        }
                    }
                }
            }
        }
    }

    // no-match state, mirroring the schema sheet
    Column {
        anchors.centerIn: flick
        visible: pg.query !== "" && !pg.anyVisible
        spacing: Tokens.s2
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("NO MATCH"); color: Tokens.inkDim; font.family: Tokens.ui
            font.pixelSize: Tokens.fSmall; font.letterSpacing: 2
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            text: I18n.tr("nothing here matches “%1”").arg(pg.query)
            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
        }
    }

    // ── catalogue overlay ────────────────────────────────────────────────────
    QtObject {
        id: pk
        property var opts: null
        property string current: ""
        property string title: ""
        property var cb: null
    }
    Item {
        anchors.fill: parent
        visible: pk.opts !== null
        z: 800

        Rectangle {
            anchors.fill: parent
            color: Tokens.paper
            opacity: 0.55
            TapHandler { onTapped: pg.closePicker() }
        }
        Rectangle {
            anchors.centerIn: parent
            width: 300
            height: Math.min(360, 56 + (pk.opts ? pk.opts.length : 0) * 32)
            radius: Tokens.radius
            color: Tokens.paperLift
            border.width: Tokens.border
            border.color: Tokens.lineStrong

            Column {
                anchors.fill: parent
                anchors.margins: Tokens.s3
                spacing: Tokens.s2

                Row {
                    width: parent.width
                    Text {
                        id: pkTitle
                        text: pk.title.toUpperCase(); color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
                    }
                    Item { width: parent.width - pkTitle.width - pkCount.width; height: 1 }
                    Text {
                        id: pkCount
                        text: I18n.tr("%1 ENTRIES").arg(pk.opts ? pk.opts.length : 0)
                        color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                    }
                }
                Flickable {
                    width: parent.width
                    height: parent.height - pkTitle.height - Tokens.s2
                    contentHeight: pkList.height
                    clip: true
                    ScrollBar.vertical: ScrollRail { }
                    WheelScroll { }
                    Column {
                        id: pkList
                        width: parent.width
                        Repeater {
                            model: pk.opts
                            delegate: Rectangle {
                                required property var modelData
                                width: pkList.width
                                height: 32
                                color: rh.hovered ? Tokens.bone : "transparent"
                                Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                Text {
                                    anchors { left: parent.left; leftMargin: Tokens.s2; right: dot.left; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                                    text: I18n.tr(modelData.label)
                                    elide: Text.ElideRight
                                    color: rh.hovered ? Tokens.inkOnBone : (modelData.key === pk.current ? Tokens.ink : Tokens.inkDim)
                                    font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                                }
                                Rectangle {
                                    id: dot
                                    anchors { right: parent.right; rightMargin: Tokens.s2; verticalCenter: parent.verticalCenter }
                                    visible: modelData.key === pk.current
                                    width: 4; height: 4; radius: 2
                                    color: rh.hovered ? Tokens.inkOnBone : Tokens.ink
                                }
                                HoverHandler { id: rh; cursorShape: Qt.PointingHandCursor }
                                TapHandler { onTapped: pg.choosePick(modelData.key) }
                            }
                        }
                    }
                }
            }
        }
    }
}

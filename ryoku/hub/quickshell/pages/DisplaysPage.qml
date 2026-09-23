pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"
import "lib/arrange.js" as Arrange

// Displays (SYSTEM). Detect every connected monitor, arrange them to scale on a
// drag canvas (no coordinate math), and tune resolution, scale, rotation, adaptive
// sync and, where the compositor supports them, colour (HDR) and mirroring per
// monitor. Apply writes the layout to the live session through the window-manager
// seam, which persists it so it returns at next login; a named profile is a stored
// layout replayed the same way, so it comes back when the displays reconnect and
// survives a compositor switch. Edits stage in a draft and only touch real screens
// on Apply, so fiddling never nukes a display. Full-bleed: this page owns the whole
// content region (its backend is the seam's output list and apply, not the shared
// settings store), so it draws its own head, canvas, controls and action bar. The
// mirror and colour controls are gated on the output-mirroring and output-HDR
// capabilities, so a compositor that lacks them shows no dead knob. Every value
// reads from Tokens; monochrome throughout: hairline rects, an ink active border.
Item {
    id: pg

    property var hub
    // A full-bleed page draws the whole content region itself: the shell hides
    // its side panel and global action bar and keeps only the rail.
    readonly property bool fullBleed: true

    // ── state: live baseline (the seam's output list) + the editable draft ──
    // monCount is the canvas Repeater model (an int, deliberately not the draft
    // array) so in-place edits never rebuild tiles; `tick` drives reactivity for
    // those mutations, so a drag never rebuilds a tile mid-drag.
    property var draft: []
    property int monCount: 0
    property int selected: 0
    property int tick: 0
    property string committed: "[]"
    property var profiles: []
    property bool listed: false
    property bool listFailed: false

    property bool dragging: false
    property var frozen: ({ "k": 1, "ox": 0, "oy": 0 })

    // the "main" display: the one at the global origin (0,0), the compositor's
    // primary reference corner (cursor home, XWayland primary). Derived on load
    // and re-anchored by normalize(); "Set as main" re-bases the layout onto it.
    property string mainName: ""

    // which catalogue overlay is open: "" | "mode" | "mirror" | "custom".
    property string pickKind: ""
    // "Custom…" chip in the mode picker opens a W×H@Hz form; the typed mode
    // stages into the draft like any other, and on Apply the provider forces a
    // non-advertised timing where it can and reports it where it cannot.
    readonly property string customLabel: I18n.tr("Custom\u2026")
    // timed keep-changes safety after applying a custom mode: apply() stashes the
    // prior layout here and counts revertSecs down; not kept in time -> re-apply.
    property string revertBaseline: ""
    property int revertSecs: 0

    readonly property var sel: (pg.selected >= 0 && pg.selected < pg.draft.length) ? pg.draft[pg.selected] : null

    // ── per-monitor interface (shell) scale ──────────────────────────────
    // Distinct from the compositor SCALE control below: this is the Ryoku shell
    // chrome's own logical scale on a monitor (shell.json displays.ui_scale,
    // keyed by output name), so one screen's bar, menus and this Hub can shrink
    // or grow without touching the crisp compositor scale apps render at. A shell
    // setting, so it live-patches straight through the daemon (Settings.patch),
    // NOT the output draft/Apply flow. uiScaleLive is an optimistic per-name
    // override so the stepper reads back instantly, before the shell.json round
    // trip lands in Tokens.uiScales.
    property var uiScaleLive: ({})
    function uiScalePct(name) {
        void pg.tick;
        var v = name ? pg.uiScaleLive[name] : undefined;
        if (typeof v === "number")
            return v;
        return Math.round(Tokens.uiScaleFor(name) * 100);
    }
    function setUiScale(name, pct) {
        if (!name)
            return;
        var p = Math.max(50, Math.min(200, Math.round(pct)));
        var m = {};
        for (var k in pg.uiScaleLive)
            m[k] = pg.uiScaleLive[k];
        m[name] = p;
        pg.uiScaleLive = m;
        Settings.patch("displays.ui_scale." + name, p / 100);
        pg.tick++;
    }

    property var barLive: ({})
    property var widgetLive: ({})

    function displayFlag(name, kind) {
        void pg.tick;
        if (!name)
            return true;
        const live = kind === "bar" ? pg.barLive : pg.widgetLive;
        if (typeof live[name] === "boolean")
            return live[name];
        return kind === "bar"
            ? Tokens.barEnabledFor(name)
            : Tokens.widgetsEnabledFor(name);
    }

    function setDisplayFlag(name, kind, enabled) {
        if (!name)
            return;

        const source = kind === "bar" ? pg.barLive : pg.widgetLive;
        const next = {};
        for (var k in source)
            next[k] = source[k];
        next[name] = !!enabled;

        if (kind === "bar")
            pg.barLive = next;
        else
            pg.widgetLive = next;

        Settings.patch("displays." + kind + "." + name, !!enabled);
        pg.tick++;
    }

    // ── night light (a display-wide comfort setting on the daemon topic) ─────
    // The daemon owns the on/off truth (the provider's backend process) and the
    // saved temperature; the page reads the pushed frame and sends the intent
    // back on a second socket, the split the Settings singleton uses. Gated on
    // the nightLight capability, so a compositor with no backend shows no group.
    property bool nightOn: false
    property int nightTemp: 4000

    function applyNightFrame(line) {
        try {
            var f = JSON.parse(line);
            if (f && typeof f === "object" && !Array.isArray(f)) {
                pg.nightOn = f.on === true;
                if (typeof f.temperature === "number" && f.temperature > 0)
                    pg.nightTemp = f.temperature;
            }
        } catch (e) {}
    }

    function sendNight(on, temp) {
        nlCtl.queued += "call nightlight.set " + JSON.stringify({ on: on === true, temperature: temp }) + "\n";
        if (nlCtl.connected)
            nlCtl.flushQueued();
        else
            nlCtl.connected = true;
    }

    // Optimistic so the switch and stepper read back instantly; the confirming
    // frame lands on the same values. Temperature applies live only while on
    // (nightlight.set with on:false just turns off), so the row is inert when off.
    function setNight(on) {
        pg.nightOn = on === true;
        pg.sendNight(pg.nightOn, pg.nightTemp);
    }
    function setNightTemp(temp) {
        pg.nightTemp = temp;
        if (pg.nightOn)
            pg.sendNight(true, temp);
    }

    Socket {
        id: nlSub
        path: Settings.sockPath
        parser: SplitParser { onRead: line => pg.applyNightFrame(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe nightlight\n");
                flush();
            } else {
                nlRetry.restart();
            }
        }
    }
    Timer {
        id: nlRetry
        interval: 2000
        onTriggered: if (!nlSub.connected) nlSub.connected = true
    }
    Socket {
        id: nlCtl
        path: Settings.sockPath
        property string queued: ""
        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }

    // ── data load: the seam's output list + saved profiles ──────────────────
    Process {
        id: listProc
        command: ["ryoku-hub", "outputs"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    var arr = JSON.parse(this.text);
                    if (!Array.isArray(arr))
                        throw "not an array";
                    var d = [];
                    for (var i = 0; i < arr.length; i++)
                        d.push(pg.clone(arr[i]));
                    pg.draft = d;
                    pg.monCount = d.length;
                    pg.mainName = pg.deriveMain();
                    // committed is the LIVE baseline (what the compositor has
                    // now); a gapped live layout stays the baseline so tidying it
                    // below reads as a pending Apply, not a silent no-op.
                    pg.committed = JSON.stringify(pg.specsAll());
                    // A layout left separated can strand a display where the
                    // cursor cannot cross the gap. Pull any detached display flush
                    // so opening Displays proposes the fix.
                    if (pg.tidyGaps()) pg.normalize();
                    if (pg.selected >= d.length)
                        pg.selected = 0;
                    pg.listFailed = false;
                    pg.listed = true;
                    pg.tick++;
                } catch (e) {
                    pg.listFailed = true;
                    pg.listed = true;
                    console.log("hub: display enumeration failed: " + e);
                }
            }
        }
    }

    Process {
        id: profilesProc
        command: ["ryoku-hub", "outputs", "profiles"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try { pg.profiles = JSON.parse(this.text); } catch (e) { pg.profiles = []; }
            }
        }
    }

    Process { id: applyProc }
    Process { id: profileProc } // save | load | rm

    function reload() { listProc.running = true; profilesProc.running = true; }
    function reloadProfiles() { profilesProc.running = true; }

    // ── model helpers ───────────────────────────────────────────────────────
    function parseMode(s) {
        var m = /^(\d+)x(\d+)@([\d.]+)/.exec(s);
        if (!m)
            return null;
        var w = parseInt(m[1]), h = parseInt(m[2]);
        return { "key": w + "x" + h + "@" + m[3], "label": w + " \u00d7 " + h + "  \u00b7  " + Math.round(parseFloat(m[3])) + " Hz", "w": w, "h": h, "rate": parseFloat(m[3]) };
    }
    function modeOptions(mon) {
        var seen = ({}), out = [];
        var arr = mon && mon.modes ? mon.modes : [];
        for (var i = 0; i < arr.length; i++) {
            var p = pg.parseMode(arr[i]);
            if (!p || seen[p.key])
                continue;
            seen[p.key] = true;
            out.push(p);
        }
        out.sort((a, b) => (b.w * b.h - a.w * a.h) || (b.rate - a.rate));
        return out;
    }
    function pickCurrentMode(mon) {
        var arr = pg.modeOptions(mon);
        for (var i = 0; i < arr.length; i++)
            if (arr[i].w === mon.width && arr[i].h === mon.height && Math.round(arr[i].rate) === Math.round(mon.refresh))
                return arr[i].key;
        for (i = 0; i < arr.length; i++)
            if (arr[i].w === mon.width && arr[i].h === mon.height)
                return arr[i].key;
        return mon.width + "x" + mon.height + "@" + mon.refresh;
    }
    function clone(mon) {
        return {
            "name": mon.name, "modes": (mon.modes || []),
            "width": mon.width, "height": mon.height, "refresh": mon.refresh,
            "physicalWidth": (mon.physicalWidth || 0),
            "mode": (mon.mode || pg.pickCurrentMode(mon)),
            "scale": mon.scale, "x": mon.x, "y": mon.y, "transform": mon.transform || 0,
            "vrr": (mon.vrr === true),
            "mirror": (mon.mirror && mon.mirror !== "none") ? mon.mirror : "",
            "colorMode": (mon.colorMode || "srgb"), "sdrBrightness": (mon.sdrBrightness || 1.0),
            "disabled": mon.disabled === true
        };
    }
    // A crisp scale divides the mode's pixels into whole logical pixels. The
    // stepper walks that per-resolution ladder instead of doing +-25% arithmetic,
    // which almost always lands between valid values (the compositor substitutes
    // its own and the readback looks like noise). computeLadder derives the ladder
    // for the selected mode, and a compositor that accepts a finer scale still
    // accepts every value it offers.
    function scaleLadder(m) {
        return pg.computeLadder(m.width, m.height, (m.transform & 1) ? 1 : 0);
    }
    // Whole-logical-pixel scales for a mode: k/120 (k in 30..720) dividing both
    // dimensions to whole logical pixels, floored at 1x and never shrinking the
    // logical desktop below 640×360. The floor applies to the on-screen
    // rectangle, so a rotated panel is capped by its (shorter) vertical width
    // rather than being offered landscape-appropriate scales.
    function computeLadder(w, h, tf) {
        var lw = tf ? h : w, lh = tf ? w : h;
        var out = [];
        for (var k = 30; k <= 720; k++)
            if ((w * 120) % k === 0 && (h * 120) % k === 0) {
                var s = Math.round(k / 120 * 10000) / 10000;
                if (s >= 1 && s <= 3 && (lw / s) >= 640 && (lh / s) >= 360)
                    out.push(s);
            }
        return out.length ? out : [1];
    }
    function nearestScaleIdx(l, s) {
        var best = 0;
        for (var i = 1; i < l.length; i++)
            if (Math.abs(l[i] - s) < Math.abs(l[best] - s))
                best = i;
        return best;
    }
    function footW(m) {
        var lw = Math.round(m.width / m.scale), lh = Math.round(m.height / m.scale);
        return (m.transform & 1) ? lh : lw;
    }
    function footH(m) {
        var lw = Math.round(m.width / m.scale), lh = Math.round(m.height / m.scale);
        return (m.transform & 1) ? lw : lh;
    }
    function specsAll() {
        var out = [];
        for (var i = 0; i < pg.draft.length; i++) {
            var m = pg.draft[i];
            out.push({
                "name": m.name, "enabled": !m.disabled, "mode": m.mode,
                "scale": m.scale, "x": m.x, "y": m.y,
                "transform": m.transform, "vrr": !!m.vrr,
                "mirror": m.mirror, "colorMode": m.colorMode, "sdrBrightness": m.sdrBrightness
            });
        }
        return out;
    }
    // whole-document string diff, gated on monCount so an empty read is never dirty.
    readonly property bool dirty: {
        void pg.tick;
        return pg.monCount > 0 && JSON.stringify(pg.specsAll()) !== pg.committed;
    }

    function setField(i, key, val) {
        if (i < 0 || i >= pg.draft.length)
            return;
        pg.draft[i][key] = val;
        pg.tick++;
    }
    function setMode(i, key) {
        var p = pg.parseMode(key);
        if (!p)
            return;
        pg.draft[i].mode = key;
        pg.draft[i].width = p.w;
        pg.draft[i].height = p.h;
        pg.draft[i].refresh = p.rate;
        // the ladder is per-resolution: re-snap the scale so a mode change
        // never stages a scale the new mode cannot hold.
        var l = pg.scaleLadder(pg.draft[i]);
        pg.draft[i].scale = l[pg.nearestScaleIdx(l, pg.draft[i].scale)];
        pg.tick++;
    }
    // stage a hand-entered resolution into the draft. The mode is "WxH@Hz"; on
    // Apply the provider forces a non-advertised one where it can. Scale re-snaps
    // to the new mode's valid ladder.
    function setCustomMode(i, w, h, hz) {
        if (i < 0 || i >= pg.draft.length)
            return;
        w = Math.round(w); h = Math.round(h); hz = Math.round(hz);
        if (!(w >= 320 && h >= 200 && hz >= 20 && w <= 16384 && h <= 16384 && hz <= 480))
            return;
        pg.draft[i].mode = w + "x" + h + "@" + hz;
        pg.draft[i].width = w;
        pg.draft[i].height = h;
        pg.draft[i].refresh = hz;
        var l = pg.scaleLadder(pg.draft[i]);
        pg.draft[i].scale = l[pg.nearestScaleIdx(l, pg.draft[i].scale)];
        pg.tick++;
    }
    function mirrorOptions() {
        void pg.tick;
        var out = [{ "key": "", "label": I18n.tr("None") }];
        for (var i = 0; i < pg.draft.length; i++) {
            if (i === pg.selected || pg.draft[i].disabled)
                continue;
            out.push({ "key": pg.draft[i].name, "label": pg.draft[i].name });
        }
        return out;
    }

    // ── canvas geometry ─────────────────────────────────────────────────────
    function bbox() {
        var minX = 1e9, minY = 1e9, maxX = -1e9, maxY = -1e9, any = false;
        for (var i = 0; i < pg.draft.length; i++) {
            var m = pg.draft[i];
            if (m.disabled)
                continue;
            any = true;
            minX = Math.min(minX, m.x);
            minY = Math.min(minY, m.y);
            maxX = Math.max(maxX, m.x + pg.footW(m));
            maxY = Math.max(maxY, m.y + pg.footH(m));
        }
        if (!any)
            return { "x": 0, "y": 0, "w": 1, "h": 1 };
        return { "x": minX, "y": minY, "w": Math.max(1, maxX - minX), "h": Math.max(1, maxY - minY) };
    }
    // fit the bounding box of all ENABLED monitors to 84% of the canvas, centred.
    function computeView(cw, ch) {
        var b = pg.bbox();
        if (cw <= 0 || ch <= 0)
            return { "k": 0.05, "ox": 0, "oy": 0 };
        var k = Math.min(cw * 0.84 / b.w, ch * 0.84 / b.h);
        if (!isFinite(k) || k <= 0)
            k = 0.05;
        return { "k": k, "ox": (cw - b.w * k) / 2 - b.x * k, "oy": (ch - b.h * k) / 2 - b.y * k };
    }
    // during a drag the fit is frozen so the canvas does not rescale under the cursor.
    function view() {
        void pg.tick;
        return pg.dragging ? pg.frozen : pg.computeView(canvasArea.width, canvasArea.height);
    }

    function nearestSnap(val, cands, th) {
        var best = val, bd = th;
        for (var i = 0; i < cands.length; i++) {
            var d = Math.abs(val - cands[i]);
            if (d < bd) { bd = d; best = cands[i]; }
        }
        return best;
    }
    function dragMonitor(i, dxLogical, dyLogical) {
        if (i < 0 || i >= pg.draft.length)
            return;
        if (!pg.dragging) {
            pg.frozen = pg.computeView(canvasArea.width, canvasArea.height);
            pg.dragging = true;
        }
        pg.draft[i].x += dxLogical;
        pg.draft[i].y += dyLogical;
        pg.tick++;
    }
    // build the pure-geometry view arrange.js works on: logical footprints.
    function monsView() {
        var out = [];
        for (var i = 0; i < pg.draft.length; i++) {
            var m = pg.draft[i];
            out.push({ name: m.name, x: m.x, y: m.y, w: pg.footW(m), h: pg.footH(m), disabled: m.disabled });
        }
        return out;
    }
    // write arranged x/y back onto the draft, matched by connector name.
    function applyMons(mons) {
        for (var i = 0; i < pg.draft.length; i++)
            for (var j = 0; j < mons.length; j++)
                if (mons[j].name === pg.draft[i].name) { pg.draft[i].x = mons[j].x; pg.draft[i].y = mons[j].y; break; }
    }
    // On drop, fine-snap this display's near edge to 0 or a neighbour's edges
    // (zoom-independent ~22 screen px), then guarantee contiguity via the
    // unit-tested arrange lib (a gap the cursor cannot cross strands a display),
    // then re-anchor on the main display.
    function endDrag(i) {
        if (i < 0 || i >= pg.draft.length) {
            pg.dragging = false;
            return;
        }
        var m = pg.draft[i];
        var k = pg.computeView(canvasArea.width, canvasArea.height).k;
        var th = Math.max(8, 22 / Math.max(0.0001, k));
        var xs = [0], ys = [0];
        for (var j = 0; j < pg.draft.length; j++) {
            if (j === i || pg.draft[j].disabled)
                continue;
            var o = pg.draft[j];
            xs.push(o.x, o.x + pg.footW(o), o.x - pg.footW(m));
            ys.push(o.y, o.y + pg.footH(o), o.y - pg.footH(m));
        }
        m.x = Math.round(pg.nearestSnap(m.x, xs, th));
        m.y = Math.round(pg.nearestSnap(m.y, ys, th));
        var mons = pg.monsView();
        if (!Arrange.touchesAny(mons, i)) {
            Arrange.attachFlush(mons, i);
            pg.applyMons(mons);
        }
        pg.normalize();
        pg.dragging = false;
        pg.tick++;
    }
    // ensure every enabled display is part of one connected block (no gaps).
    function tidyGaps() {
        var mons = pg.monsView();
        var changed = Arrange.tidyGaps(mons);
        if (changed)
            pg.applyMons(mons);
        return changed;
    }
    // the main display is the one at the global origin (fallback: top-left).
    function deriveMain() {
        return Arrange.deriveMain(pg.monsView());
    }
    function setMain(i) {
        if (i < 0 || i >= pg.draft.length || pg.draft[i].disabled)
            return;
        pg.mainName = pg.draft[i].name;
        pg.normalize();
        pg.tick++;
    }
    // re-base so the main display sits at the global origin (0,0), the compositor's
    // primary reference corner; other displays keep their relative offsets (may go
    // negative, which the compositor accepts).
    function normalize() {
        var mons = pg.monsView();
        Arrange.rebaseToMain(mons, pg.mainName);
        pg.applyMons(mons);
    }

    // ── quick actions: layouts computed from the enumerated set, then applied ──
    // clear mirroring and lay every enabled display in one flush left-to-right row.
    function extend() {
        var order = [];
        for (var i = 0; i < pg.draft.length; i++)
            order.push(i);
        order.sort((a, b) => pg.draft[a].x - pg.draft[b].x);
        var x = 0;
        for (var k = 0; k < order.length; k++) {
            var m = pg.draft[order[k]];
            m.mirror = "";
            if (m.disabled)
                continue;
            m.x = x; m.y = 0;
            x += pg.footW(m);
        }
        pg.mainName = pg.deriveMain();
        pg.tick++;
        pg.apply();
    }
    // a scale derived from the panel's pixel density (px / physical mm), snapped to
    // the mode's valid ladder. Mirrors the buckets the shell uses at login.
    function dpiScale(m) {
        var mm = m.physicalWidth || 0, px = m.width || 0, target = 1.0;
        if (mm > 0 && px > 0) {
            var dpi = px * 25.4 / mm;
            if (dpi < 40 || dpi > 700) target = 1.0;
            else if (dpi <= 120) target = 1.0;
            else if (dpi <= 150) target = 1.25;
            else if (dpi <= 230) target = 1.5;
            else if (dpi <= 290) target = 1.75;
            else target = 2.0;
        }
        var l = pg.scaleLadder(m);
        return l[pg.nearestScaleIdx(l, target)];
    }
    function dpiAutoscale() {
        for (var i = 0; i < pg.draft.length; i++) {
            var m = pg.draft[i];
            if (m.disabled)
                continue;
            m.scale = pg.dpiScale(m);
        }
        // a scale change resizes footprints, so re-flush before applying.
        if (pg.tidyGaps()) pg.normalize();
        pg.tick++;
        pg.apply();
    }
    // clone every other enabled display onto the main one (output mirroring: only
    // offered where CapOutputMirror is present).
    function mirrorAll() {
        var src = pg.mainName;
        if (!src)
            for (var i = 0; i < pg.draft.length; i++)
                if (!pg.draft[i].disabled) { src = pg.draft[i].name; break; }
        if (!src)
            return;
        for (var j = 0; j < pg.draft.length; j++) {
            var m = pg.draft[j];
            if (m.disabled)
                continue;
            m.mirror = (m.name === src) ? "" : src;
        }
        pg.tick++;
        pg.apply();
    }

    // ── apply ───────────────────────────────────────────────────────────────
    function apply() {
        var prev = pg.committed;
        var next = JSON.stringify(pg.specsAll());
        applyProc.command = ["ryoku-hub", "outputs", "apply", next];
        applyProc.running = true;
        pg.committed = next;
        pg.tick++;
        // read back what the compositor actually accepted (it may still adjust
        // a value); the delayed reload keeps "matches your displays" honest.
        listRefresh.start();
        // a custom (non-advertised) mode can come up wrong; offer a timed
        // keep-or-revert like every other display manager.
        if (pg.hasCustomMode())
            pg.startRevert(prev);
    }
    // any enabled monitor whose staged mode is a typed WxH@rate not in the
    // panel's advertised list -- the case that wants the keep-or-revert net.
    function hasCustomMode() {
        for (var i = 0; i < pg.draft.length; i++) {
            var m = pg.draft[i];
            if (m.disabled || !/^\d+x\d+@/.test(m.mode))
                continue;
            var opts = pg.modeOptions(m), found = false;
            for (var j = 0; j < opts.length; j++)
                if (opts[j].key === m.mode) { found = true; break; }
            if (!found)
                return true;
        }
        return false;
    }
    function startRevert(baseline) {
        pg.revertBaseline = baseline;
        pg.revertSecs = 15;
        revertTimer.restart();
    }
    function keepChanges() {
        revertTimer.stop();
        pg.revertSecs = 0;
        pg.revertBaseline = "";
    }
    function doRevert() {
        revertTimer.stop();
        pg.revertSecs = 0;
        if (pg.revertBaseline === "")
            return;
        applyProc.command = ["ryoku-hub", "outputs", "apply", pg.revertBaseline];
        applyProc.running = true;
        pg.committed = pg.revertBaseline;
        pg.revertBaseline = "";
        listRefresh.start();
    }

    // ── profiles: a stored layout, replayed through the seam ────────────────
    function saveProfile(name) {
        if (name.trim() === "")
            return;
        profileProc.command = ["ryoku-hub", "outputs", "save", name.trim(), JSON.stringify(pg.specsAll())];
        profileProc.running = true;
        pg.committed = JSON.stringify(pg.specsAll());
        pg.tick++;
        nameField.text = "";
        profileRefresh.start();
    }
    function loadProfile(name) {
        profileProc.command = ["ryoku-hub", "outputs", "load", name];
        profileProc.running = true;
        listRefresh.start();
    }
    function deleteProfile(name) {
        profileProc.command = ["ryoku-hub", "outputs", "rm", name];
        profileProc.running = true;
        profileRefresh.start();
    }

    // fixed-delay refreshes after an apply, so the readback reflects the settled layout.
    Timer { id: listRefresh; interval: 700; onTriggered: pg.reload() }
    Timer { id: profileRefresh; interval: 300; onTriggered: pg.reloadProfiles() }
    // counts the keep-or-revert window down; if it reaches zero unconfirmed, the
    // prior layout is re-applied (doRevert).
    Timer {
        id: revertTimer
        interval: 1000
        repeat: true
        onTriggered: { pg.revertSecs--; if (pg.revertSecs <= 0) pg.doRevert(); }
    }

    // ── presentation helpers (rotation labels, colour mapping) ──
    function rotLabel(t) { return (t * 90) + "\u00b0"; }
    function rotKey(label) { return parseInt(label) / 90; }
    // colour management: the mode drives HDR. sRGB is the safe default, Wide is
    // wide-gamut (BT2020), HDR turns on the PQ transfer + 10-bit. Only shown where
    // the output-HDR capability is present.
    readonly property var cmLabels: ["sRGB", "Wide", "HDR"]
    readonly property var cmKeys: ["srgb", "wide", "hdr"]
    function cmLabel(v) { var i = pg.cmKeys.indexOf(v); return i < 0 ? "sRGB" : pg.cmLabels[i]; }
    function cmKey(label) { var i = pg.cmLabels.indexOf(label); return i < 0 ? "srgb" : pg.cmKeys[i]; }
    // SDR content brightness in HDR: 1.0x-6.0x. Fine steps through the usual range,
    // then coarser to the top for a bright panel (miniLED, high-nit OLED) where SDR
    // content stays dim in HDR well past 2x.
    readonly property var sdrLadder: [1.0, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9, 2.0,
        2.25, 2.5, 2.75, 3.0, 3.5, 4.0, 4.5, 5.0, 5.5, 6.0]
    function labelForKey(opts, key) {
        for (var i = 0; i < opts.length; i++)
            if (opts[i].key === key)
                return opts[i].label;
        return "";
    }
    function keyForLabel(opts, label) {
        for (var i = 0; i < opts.length; i++)
            if (opts[i].label === label)
                return opts[i].key;
        return "";
    }
    function openPick(kind) { pg.pickKind = kind; picker.open(); }

    // ── head: eyebrow, Fraunces title with quick actions, blurb ─────────────
    Column {
        id: head
        anchors { left: parent.left; right: parent.right; top: parent.top }
        anchors.topMargin: Tokens.s6
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "\u529b"; color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: Tokens.fMicro; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("DEVICES"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }

        Item {
            id: titleRow
            width: parent.width
            height: title.implicitHeight

            Text {
                id: title
                anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Displays"); color: Tokens.ink
                font.family: Tokens.display; font.pixelSize: Tokens.fTitle
            }
            // the page's utility actions sit beside the title, per DESIGN section
            // 8. Each computes a layout and applies it through the seam. MIRROR is
            // gated on the output-mirroring capability, so it is absent where the
            // compositor cannot clone one output onto another.
            Row {
                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                spacing: Tokens.s2
                Btn { visible: Settings.supports("outputMirror"); text: I18n.tr("MIRROR"); armed: pg.monCount > 1; onAct: pg.mirrorAll() }
                Btn { text: I18n.tr("EXTEND"); armed: pg.monCount > 1; onAct: pg.extend() }
                Btn { text: I18n.tr("DPI AUTO-SCALE"); armed: pg.monCount > 0; onAct: pg.dpiAutoscale() }
            }
        }

        Text {
            width: Math.min(parent.width, 720)
            text: I18n.tr("Arrange your displays and set each one's mode. Save a layout to reuse it.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── body: detected readout, drag canvas, per-monitor controls ───────────
    Item {
        id: body
        anchors { left: parent.left; right: parent.right; top: head.bottom; bottom: bar.top }
        anchors.leftMargin: Tokens.s6; anchors.rightMargin: Tokens.s6
        anchors.topMargin: Tokens.s5; anchors.bottomMargin: Tokens.s5

        // detected-count readout: singular/plural on monCount.
        Text {
            id: detected
            anchors.left: parent.left; anchors.top: parent.top
            text: (pg.monCount === 1 ? I18n.tr("1 DISPLAY DETECTED") : I18n.tr("%1 DISPLAYS DETECTED").arg(pg.monCount))
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }

        // ── the drag-arrange canvas ──
        Rectangle {
            id: canvasArea
            anchors.left: parent.left; anchors.top: detected.bottom
            anchors.bottom: parent.bottom; anchors.right: controls.left
            anchors.topMargin: Tokens.s3; anchors.rightMargin: Tokens.s5
            radius: Tokens.radius
            color: "transparent"
            border.width: Tokens.border
            border.color: Tokens.line
            clip: true

            Repeater {
                model: pg.monCount

                // one monitor on the canvas: a draggable rect labelled with the
                // connector and resolution. The page owns all geometry (position
                // and size from the logical layout via view()); the tile only
                // reports drag deltas in logical px and selection. Selected gets a
                // 2px ink ring, disabled greys to 0.5.
                delegate: Rectangle {
                    id: tile
                    required property int index
                    readonly property var m: pg.draft[index]
                    readonly property var v: pg.view()
                    readonly property real k: tile.v.k
                    readonly property bool on: pg.selected === index
                    readonly property bool live: tile.m ? !tile.m.disabled : true

                    visible: !!tile.m
                    x: { void pg.tick; return tile.m ? (tile.m.x * tile.v.k + tile.v.ox) : 0; }
                    y: { void pg.tick; return tile.m ? (tile.m.y * tile.v.k + tile.v.oy) : 0; }
                    width: { void pg.tick; return tile.m ? Math.max(36, pg.footW(tile.m) * tile.v.k) : 36; }
                    height: { void pg.tick; return tile.m ? Math.max(28, pg.footH(tile.m) * tile.v.k) : 28; }
                    radius: Tokens.radius
                    antialiasing: false
                    color: tile.on ? Tokens.tint10 : (th.hovered ? Tokens.tint5 : "transparent")
                    border.width: tile.on ? 2 : Tokens.border
                    border.color: tile.on ? Tokens.ink : Tokens.line
                    opacity: tile.live ? 1 : 0.5
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                    Behavior on border.color { ColorAnimation { duration: Tokens.snap } }

                    HoverHandler { id: th; cursorShape: Qt.PointingHandCursor }

                    Column {
                        anchors.centerIn: parent
                        spacing: Tokens.s1
                        width: parent.width - Tokens.s3

                        Text {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            elide: Text.ElideRight
                            text: tile.m ? tile.m.name : ""
                            color: tile.live ? Tokens.ink : Tokens.inkDim
                            font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                            font.weight: Font.Medium
                        }
                        Text {
                            width: parent.width
                            horizontalAlignment: Text.AlignHCenter
                            elide: Text.ElideRight
                            // a resolution spec reads as file-truth, so mono.
                            text: tile.live ? (tile.m ? (tile.m.width + "\u00d7" + tile.m.height) : "") : I18n.tr("OFF")
                            color: Tokens.inkMuted
                            font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                        }
                        Text {
                            visible: tile.m ? (tile.m.mirror !== "") : false
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: I18n.tr("MIRROR")
                            color: Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
                            font.letterSpacing: Tokens.trackLabel
                        }
                    }

                    // main-display marker: a corner tag, ink on the monochrome tile.
                    Text {
                        visible: tile.m ? (tile.m.name === pg.mainName && !tile.m.disabled) : false
                        anchors.left: parent.left; anchors.top: parent.top; anchors.margins: Tokens.s2
                        text: I18n.tr("MAIN")
                        color: Tokens.ink; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark
                    }

                    // target null: the page moves the tile from reported deltas so
                    // the frozen fit and edge snap stay authoritative. Drag also
                    // selects (a tap fires on drag start).
                    DragHandler {
                        id: dh
                        target: null
                        property real lastX: 0
                        property real lastY: 0
                        onActiveChanged: {
                            if (active) {
                                lastX = 0;
                                lastY = 0;
                                pg.selected = tile.index;
                            } else {
                                pg.endDrag(tile.index);
                            }
                        }
                        onTranslationChanged: {
                            var dx = translation.x - lastX;
                            var dy = translation.y - lastY;
                            lastX = translation.x;
                            lastY = translation.y;
                            pg.dragMonitor(tile.index, dx / tile.k, dy / tile.k);
                        }
                    }
                    TapHandler { onTapped: pg.selected = tile.index; cursorShape: Qt.PointingHandCursor }
                }
            }

            // empty / loading / error state (monCount === 0).
            Column {
                anchors.centerIn: parent
                visible: pg.monCount === 0
                spacing: Tokens.s3
                width: Math.min(parent.width - Tokens.s6 * 2, 360)

                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    width: parent.width
                    horizontalAlignment: Text.AlignHCenter
                    text: !pg.listed ? I18n.tr("Detecting displays\u2026")
                        : pg.listFailed ? I18n.tr("Couldn't read your displays.")
                        : I18n.tr("No displays detected.")
                    color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                }
                Text {
                    anchors.horizontalCenter: parent.horizontalCenter
                    width: parent.width
                    visible: pg.listFailed
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    text: I18n.tr("The display backend looks out of date. Run 'ryoku deploy' (or update the desktop) and retry.")
                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }
                Btn {
                    anchors.horizontalCenter: parent.horizontalCenter
                    visible: pg.listed
                    text: I18n.tr("RETRY")
                    onAct: pg.reload()
                }
            }

            // arrangement hint, parked in the canvas corner off the reading path.
            Text {
                anchors.left: parent.left; anchors.bottom: parent.bottom
                anchors.margins: Tokens.s3
                text: I18n.tr("DRAG A DISPLAY TO ARRANGE IT \u00b7 EDGES SNAP")
                color: Tokens.inkFaint; font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                font.letterSpacing: Tokens.trackLabel
            }
        }

        // ── per-monitor controls + profiles ──
        Flickable {
            id: controls
            anchors.right: parent.right; anchors.top: detected.bottom; anchors.bottom: parent.bottom
            anchors.topMargin: Tokens.s3
            width: 392
            contentWidth: width
            contentHeight: Math.max(ctlCol.height, height)
            clip: true
            boundsBehavior: Flickable.StopAtBounds
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            Column {
                id: ctlCol
                width: controls.width - Tokens.s3   // reserve a lane for the scroll rail
                spacing: Tokens.s5

                // empty-selection placeholder: both per-monitor cards hide.
                Text {
                    visible: !pg.sel
                    text: I18n.tr("Select a display to configure it.")
                    color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                }

                // per-monitor group; the card title is the selected connector name.
                SettingCard {
                    width: ctlCol.width
                    visible: !!pg.sel
                    title: pg.sel ? pg.sel.name : I18n.tr("DISPLAY")

                    // the main (primary) display: put this screen at the global
                    // origin so the cursor and new workspaces start here.
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        label: I18n.tr("MAIN DISPLAY")
                        footH: 32
                        Item {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            Text {
                                visible: { void pg.tick; return pg.sel ? pg.sel.name === pg.mainName : false; }
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("MAIN"); color: Tokens.ink; font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackMark
                            }
                            Btn {
                                visible: { void pg.tick; return pg.sel ? (pg.sel.name !== pg.mainName && !pg.sel.disabled) : false; }
                                anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                                text: I18n.tr("SET AS MAIN")
                                armed: true
                                onAct: pg.setMain(pg.selected)
                            }
                        }
                    }

                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("ENABLED")
                        controlWidth: 54
                        Sw {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            on: { void pg.tick; return pg.sel ? !pg.sel.disabled : false; }
                            onToggled: (v) => pg.setField(pg.selected, "disabled", !v)
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("RESOLUTION")
                        footH: 32
                        PickBar {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            value: { void pg.tick; return pg.sel ? pg.labelForKey(pg.modeOptions(pg.sel), pg.sel.mode) : ""; }
                            count: { void pg.tick; return pg.sel ? pg.modeOptions(pg.sel).length : 0; }
                            onOpened: pg.openPick("mode")
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("SCALE")
                        value: { void pg.tick; return pg.sel ? (pg.sel.scale.toFixed(2) + "\u00d7") : ""; }
                        controlWidth: 58
                        Step {
                            id: scaleStep
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            // value is an index into the per-resolution ladder of
                            // valid whole-logical-pixel scales (see scaleLadder).
                            readonly property var ladder: { void pg.tick; return pg.sel ? pg.scaleLadder(pg.sel) : [1]; }
                            from: 0; to: ladder.length - 1; stepBy: 1
                            value: { void pg.tick; return pg.sel ? pg.nearestScaleIdx(scaleStep.ladder, pg.sel.scale) : 0; }
                            onModified: (v) => pg.setField(pg.selected, "scale",
                                scaleStep.ladder[Math.max(0, Math.min(scaleStep.ladder.length - 1, v))])
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        // The SCALE above is the compositor (hardware) scale apps
                        // render at; this is the Ryoku shell's own interface scale
                        // on this monitor, shrinking or growing its chrome without
                        // touching that crisp compositor scale. Live shell setting.
                        label: I18n.tr("INTERFACE SCALE")
                        value: { void pg.tick; return pg.sel ? (pg.uiScalePct(pg.sel.name) + "%") : ""; }
                        controlWidth: 58
                        Step {
                            id: uiScaleStep
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            from: 50; to: 200; stepBy: 5
                            value: { void pg.tick; return pg.sel ? pg.uiScalePct(pg.sel.name) : 100; }
                            onModified: (v) => pg.setUiScale(pg.sel ? pg.sel.name : "", v)
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("SHOW BAR")
                        desc: I18n.tr("Show the active Ryoku bar style on this display.")
                        source: "shell.json"
                        controlWidth: 54
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: {
                                void pg.tick;
                                return pg.sel ? pg.displayFlag(pg.sel.name, "bar") : true;
                            }
                            onToggled: value => pg.setDisplayFlag(
                                pg.sel ? pg.sel.name : "", "bar", value)
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("DESKTOP WIDGETS")
                        desc: I18n.tr("Show desktop widgets and desktop-widget plugins on this display.")
                        source: "shell.json"
                        controlWidth: 54
                        Sw {
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            on: {
                                void pg.tick;
                                return pg.sel ? pg.displayFlag(pg.sel.name, "widgets") : true;
                            }
                            onToggled: value => pg.setDisplayFlag(
                                pg.sel ? pg.sel.name : "", "widgets", value)
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("ROTATION")
                        block: true
                        Seg {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: ["0\u00b0", "90\u00b0", "180\u00b0", "270\u00b0"]
                            current: { void pg.tick; return pg.sel ? pg.rotLabel(pg.sel.transform) : "0\u00b0"; }
                            onChose: (label) => pg.setField(pg.selected, "transform", pg.rotKey(label))
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: I18n.tr("ADAPTIVE SYNC")
                        controlWidth: 54
                        Sw {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            on: { void pg.tick; return pg.sel ? !!pg.sel.vrr : false; }
                            onToggled: (v) => pg.setField(pg.selected, "vrr", v)
                        }
                    }
                    // COLOUR + SDR BRIGHTNESS: only where the output-HDR behaviour
                    // exists, so a compositor without an HDR pipeline shows no dead
                    // colour knob.
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        visible: Settings.supports("outputHdr")
                        label: I18n.tr("COLOUR")
                        block: true
                        Seg {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            options: pg.cmLabels
                            current: { void pg.tick; return pg.sel ? pg.cmLabel(pg.sel.colorMode) : "sRGB"; }
                            onChose: (label) => pg.setField(pg.selected, "colorMode", pg.cmKey(label))
                        }
                    }
                    // SDR brightness only bites in HDR (it maps SDR content into the
                    // HDR range); hidden otherwise so it never reads as a dead knob.
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        visible: { void pg.tick; return Settings.supports("outputHdr") && pg.sel && pg.sel.colorMode === "hdr"; }
                        label: I18n.tr("SDR BRIGHTNESS")
                        value: { void pg.tick; return pg.sel ? (pg.sel.sdrBrightness.toFixed(1) + "\u00d7") : ""; }
                        controlWidth: 58
                        Step {
                            id: sdrStep
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            readonly property var ladder: pg.sdrLadder
                            from: 0; to: ladder.length - 1; stepBy: 1
                            value: { void pg.tick; return pg.sel ? pg.nearestScaleIdx(sdrStep.ladder, pg.sel.sdrBrightness) : 0; }
                            onModified: (v) => pg.setField(pg.selected, "sdrBrightness",
                                sdrStep.ladder[Math.max(0, Math.min(sdrStep.ladder.length - 1, v))])
                        }
                    }
                    // MIRROR OF: only where the output-mirroring behaviour exists.
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        visible: Settings.supports("outputMirror")
                        label: I18n.tr("MIRROR OF")
                        footH: 32
                        PickBar {
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            value: { void pg.tick; return pg.sel ? (pg.labelForKey(pg.mirrorOptions(), pg.sel.mirror) || I18n.tr("None")) : I18n.tr("None"); }
                            count: { void pg.tick; return pg.mirrorOptions().length; }
                            onOpened: pg.openPick("mirror")
                        }
                    }
                }

                // position: dragging is primary; these coordinates are editable
                // for a precise nudge. Fields push the stored value on change so a
                // drag keeps them in sync without fighting a mid-type edit.
                SettingCard {
                    width: ctlCol.width
                    visible: !!pg.sel
                    title: I18n.tr("POSITION")

                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        label: "X"
                        controlWidth: 128
                        Row {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Field {
                                width: 96
                                anchors.verticalCenter: parent.verticalCenter
                                tabular: true
                                placeholder: "0"
                                readonly property int modelX: { void pg.tick; return pg.sel ? pg.sel.x : 0; }
                                onModelXChanged: text = String(modelX)
                                Component.onCompleted: text = String(modelX)
                                onCommitted: (v) => { var n = parseInt(v); if (!isNaN(n)) pg.setField(pg.selected, "x", n); }
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "px"; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                            }
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        label: "Y"
                        controlWidth: 128
                        Row {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            spacing: Tokens.s2
                            Field {
                                width: 96
                                anchors.verticalCenter: parent.verticalCenter
                                tabular: true
                                placeholder: "0"
                                readonly property int modelY: { void pg.tick; return pg.sel ? pg.sel.y : 0; }
                                onModelYChanged: text = String(modelY)
                                Component.onCompleted: text = String(modelY)
                                onCommitted: (v) => { var n = parseInt(v); if (!isNaN(n)) pg.setField(pg.selected, "y", n); }
                            }
                            Text {
                                anchors.verticalCenter: parent.verticalCenter
                                text: "px"; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                            }
                        }
                    }
                }

                // profiles: a stored layout that returns when the same displays
                // reconnect. Hub-owned (a saved OutputLayout replayed through the
                // seam), so it works on every compositor and survives a switch.
                SettingCard {
                    width: ctlCol.width
                    title: I18n.tr("PROFILES")

                    Text {
                        width: parent.width
                        leftPadding: Tokens.s4; rightPadding: Tokens.s4
                        topPadding: Tokens.s3; bottomPadding: Tokens.s1
                        wrapMode: Text.WordWrap
                        text: I18n.tr("Save this layout under a name so it returns when you plug the same displays in again.")
                        color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                    }

                    Item {
                        width: parent.width
                        height: 32 + Tokens.s2
                        Btn {
                            id: saveBtn
                            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("SAVE")
                            onAct: pg.saveProfile(nameField.text)
                        }
                        Field {
                            id: nameField
                            anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                            anchors.right: saveBtn.left; anchors.rightMargin: Tokens.s2
                            anchors.verticalCenter: parent.verticalCenter
                            placeholder: I18n.tr("Profile name\u2026")
                            onCommitted: (v) => pg.saveProfile(v)
                        }
                    }

                    Column {
                        width: parent.width - Tokens.s4 * 2
                        x: Tokens.s4
                        spacing: Tokens.s2

                        // dynamic data from `ryoku-hub outputs profiles`.
                        Repeater {
                            model: pg.profiles

                            delegate: Item {
                                id: prof
                                required property var modelData
                                required property int index
                                width: parent.width
                                // the row law the settings cards use: one row
                                // height, a hairline between rows, no per-row box
                                height: Tokens.rowH

                                Rectangle {
                                    anchors.fill: parent
                                    anchors.topMargin: 1; anchors.bottomMargin: 1
                                    radius: Tokens.radius
                                    color: phov.hovered ? Tokens.tint5 : "transparent"
                                    Behavior on color { ColorAnimation { duration: Tokens.snap } }
                                }
                                Rectangle {
                                    anchors { left: parent.left; right: parent.right; top: parent.top }
                                    anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                                    height: 1
                                    color: Tokens.lineSoft
                                    visible: prof.index > 0
                                }
                                // an ink edge marks the profile whose displays are
                                // connected now; emphasis without colour.
                                Rectangle {
                                    visible: prof.modelData.matches
                                    anchors { left: parent.left; verticalCenter: parent.verticalCenter }
                                    width: 2
                                    height: parent.height - Tokens.s3
                                    color: Tokens.ink
                                }

                                HoverHandler { id: phov }

                                Row {
                                    anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2

                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: prof.modelData.name
                                        color: Tokens.ink; font.family: Tokens.ui
                                        font.pixelSize: Tokens.fSmall; font.weight: Font.Medium
                                    }
                                    Text {
                                        anchors.verticalCenter: parent.verticalCenter
                                        visible: prof.modelData.matches
                                        text: I18n.tr("CONNECTED")
                                        color: Tokens.inkMuted; font.family: Tokens.ui
                                        font.pixelSize: Tokens.fTiny; font.weight: Font.Medium
                                        font.letterSpacing: Tokens.trackLabel
                                    }
                                }

                                Row {
                                    anchors.right: parent.right; anchors.rightMargin: Tokens.s3
                                    anchors.verticalCenter: parent.verticalCenter
                                    spacing: Tokens.s2

                                    Btn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("APPLY")
                                        onAct: pg.loadProfile(prof.modelData.name)
                                    }
                                    // a paired minus, not a trash icon: remove is
                                    // not danger and there is no red on the sheet
                                    // to carry one.
                                    IconBtn {
                                        anchors.verticalCenter: parent.verticalCenter
                                        glyph: "\u2212"
                                        onAct: pg.deleteProfile(prof.modelData.name)
                                    }
                                }
                            }
                        }
                    }

                    // breathing room below the list so it clears the card border.
                    Item { width: parent.width; height: Tokens.s3 }
                }

                // Night light: a display-wide comfort setting, not per-monitor,
                // so it sits in its own card. On/off and temperature ride the
                // daemon `nightlight` topic; gated on the capability so a
                // compositor with no backend shows nothing here.
                SettingCard {
                    width: ctlCol.width
                    visible: Settings.supports("nightLight")
                    title: I18n.tr("NIGHT LIGHT")

                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        label: I18n.tr("WARM SCREEN")
                        desc: I18n.tr("Cut blue light with a warmer screen tint.")
                        controlWidth: 54
                        Sw {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            on: pg.nightOn
                            onToggled: (v) => pg.setNight(v)
                        }
                    }
                    SettingRow {
                        anchors.left: parent.left; anchors.right: parent.right
                        divider: true
                        enabled: pg.nightOn
                        label: I18n.tr("TEMPERATURE")
                        value: pg.nightTemp + " K"
                        controlWidth: 58
                        Step {
                            anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                            from: 1000; to: 6500; stepBy: 100
                            value: pg.nightTemp
                            onModified: (v) => pg.setNightTemp(v)
                        }
                    }
                }
            }
        }
    }

    // ── action bar: the transactional status surface, full width, pinned ────
    Rectangle {
        id: bar
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: 60
        color: "transparent"

        Rectangle { anchors.top: parent.top; width: parent.width; height: 1; color: Tokens.line }

        Row {
            anchors.left: parent.left; anchors.leftMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Rectangle {
                width: 6; height: 6
                anchors.verticalCenter: parent.verticalCenter
                color: Tokens.ink
                // a heartbeat while dirty, not an alarm: 600ms each way.
                SequentialAnimation on opacity {
                    running: pg.dirty
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.3; duration: 600 }
                    NumberAnimation { to: 1.0; duration: 600 }
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: pg.dirty ? I18n.tr("UNAPPLIED LAYOUT CHANGES") : I18n.tr("LAYOUT MATCHES YOUR DISPLAYS")
                color: pg.dirty ? Tokens.ink : Tokens.inkDim
                font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium; font.letterSpacing: Tokens.trackLabel
            }
        }

        Row {
            anchors.right: parent.right; anchors.rightMargin: Tokens.s6
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3

            Btn { text: I18n.tr("REVERT"); armed: pg.dirty; onAct: pg.reload() }
            // Apply writes to the live displays and re-commits the baseline; the
            // armed primary inverts to bone while dirty.
            Btn { text: I18n.tr("APPLY"); primary: true; armed: pg.dirty; onAct: pg.apply() }
        }

    }

    // ── the resolution / mirror catalogue overlay, shared across controls ───
    MouseArea {
        id: scrim
        anchors.fill: parent
        visible: pg.pickKind === "mode" || pg.pickKind === "mirror"
        z: 100
        // a bare click-catcher: no fill, since translucency is banned on app
        // surfaces (DESIGN section 6).
        onClicked: pg.pickKind = ""

        Picker {
            id: picker
            anchors.centerIn: parent
            title: pg.pickKind === "mode" ? I18n.tr("Resolution") : I18n.tr("Mirror of")
            options: {
                if (pg.pickKind === "mode")
                    return pg.sel ? pg.modeOptions(pg.sel).map(function (o) { return o.label; }).concat([pg.customLabel]) : [];
                return pg.mirrorOptions().map(function (o) { return o.label; });
            }
            current: {
                if (!pg.sel)
                    return "";
                if (pg.pickKind === "mode")
                    return pg.labelForKey(pg.modeOptions(pg.sel), pg.sel.mode);
                return pg.labelForKey(pg.mirrorOptions(), pg.sel.mirror);
            }
            onChose: (label) => {
                if (pg.pickKind === "mode") {
                    if (label === pg.customLabel) { pg.pickKind = "custom"; return; }
                    var mk = pg.keyForLabel(pg.modeOptions(pg.sel), label);
                    if (mk)
                        pg.setMode(pg.selected, mk);
                } else {
                    pg.setField(pg.selected, "mirror", pg.keyForLabel(pg.mirrorOptions(), label));
                }
                pg.pickKind = "";
            }
            onDismissed: pg.pickKind = ""

            // absorb clicks inside the card so the scrim does not treat a
            // header/padding tap as an outside dismiss.
            MouseArea { anchors.fill: parent; z: -1 }
        }
    }

    // ── custom-resolution form: type W × H @ Hz. Stages into the draft like any
    // mode; the provider forces a non-advertised timing where it can on Apply,
    // and Apply arms the keep-or-revert banner below. ─────────────────────────
    MouseArea {
        id: customScrim
        anchors.fill: parent
        visible: pg.pickKind === "custom"
        z: 101
        onClicked: pg.pickKind = ""
        // prefill from the selected monitor's current mode when the form opens.
        onVisibleChanged: if (customScrim.visible && pg.sel) {
            cwField.text = "" + pg.sel.width;
            chField.text = "" + pg.sel.height;
            chzField.text = "" + Math.round(pg.sel.refresh);
        }

        Rectangle {
            anchors.centerIn: parent
            width: 320
            height: cform.implicitHeight + Tokens.s5 * 2
            radius: Tokens.radius
            color: Tokens.paper
            border.width: Tokens.border
            border.color: Tokens.line
            MouseArea { anchors.fill: parent }  // absorb clicks inside the card

            Column {
                id: cform
                anchors.centerIn: parent
                width: parent.width - Tokens.s5 * 2
                spacing: Tokens.s4

                Text {
                    text: I18n.tr("Custom resolution")
                    color: Tokens.ink
                    font.family: Tokens.ui; font.pixelSize: Tokens.fBody
                }
                Row {
                    spacing: Tokens.s3
                    Field { id: cwField; width: 74; tabular: true; placeholder: I18n.tr("Width"); onAccepted: addBtn.submit() }
                    Text { text: "\u00d7"; color: Tokens.inkDim; font.family: Tokens.ui; font.pixelSize: Tokens.fBody; anchors.verticalCenter: parent.verticalCenter }
                    Field { id: chField; width: 74; tabular: true; placeholder: I18n.tr("Height"); onAccepted: addBtn.submit() }
                    Field { id: chzField; width: 56; tabular: true; placeholder: "Hz"; onAccepted: addBtn.submit() }
                }
                Row {
                    spacing: Tokens.s3
                    anchors.right: parent.right
                    Btn { text: I18n.tr("CANCEL"); onAct: pg.pickKind = "" }
                    Btn {
                        id: addBtn
                        text: I18n.tr("ADD"); primary: true
                        function submit() {
                            var w = parseInt(cwField.text), h = parseInt(chField.text), hz = parseInt(chzField.text);
                            if (!(w > 0 && h > 0 && hz > 0))
                                return;
                            pg.setCustomMode(pg.selected, w, h, hz);
                            pg.pickKind = "";
                        }
                        onAct: submit()
                    }
                }
            }
        }
    }

    // ── keep-or-revert banner: a custom mode can come up wrong, so Apply arms a
    // countdown that re-applies the prior layout unless kept. ─────────────────
    Rectangle {
        visible: pg.revertSecs > 0
        z: 102
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Tokens.s7
        width: rrow.implicitWidth + Tokens.s5 * 2
        height: rrow.implicitHeight + Tokens.s4 * 2
        radius: Tokens.radius
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.ink

        Row {
            id: rrow
            anchors.centerIn: parent
            spacing: Tokens.s4
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Keep this resolution? Reverting in %1s").arg(pg.revertSecs)
                color: Tokens.ink
                font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
            }
            Btn { text: I18n.tr("KEEP"); primary: true; onAct: pg.keepChanges() }
            Btn { text: I18n.tr("REVERT NOW"); onAct: pg.doRevert() }
        }
    }
}

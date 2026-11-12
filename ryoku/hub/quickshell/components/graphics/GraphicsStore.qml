import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the graphics hardware as a Changes store over `ryoku-hub gpu`: which card
// renders the desktop ("mode"), which card the built-in screen is wired to
// ("mux"), and this session's tuning ("tune|<slot>|<id>"; "tune|all" for a
// preset and "tune|reset" for a reset, whose undo puts every knob back). A knob
// with an automatic state (no clock lock, the fan on automatic) reads "auto",
// which the backend takes back too. Read once on open.
QtObject {
    id: store

    property var caps: ({})
    property string capsError: ""
    property bool capsRead: false
    property string mode: ""
    property bool performanceAvailable: true
    property string performanceReason: ""
    property var mux: ({})
    property var tune: []
    property bool tuneRead: false
    property string tuneError: ""
    property string modeError: ""
    // false when the running window manager picks its own card and ignores the mode
    property bool modeApplies: true
    property var presets: []
    property var passthroughAvailability: null
    // one reading of the render card: { ok, asleep, temp, util }
    property var live: ({})
    property var busy: ({})
    property var queued: ({})
    // Changes' quit path waits while a write is queued or running
    readonly property bool unsettled: Object.keys(store.queued).length > 0 || Object.keys(store.busy).length > 0

    readonly property bool twoCards: !!(store.caps.host && store.caps.passthrough)
    readonly property var renderCard: {
        var p = store.caps.passthrough, h = store.caps.host;
        if (p && p.drivesDisplay)
            return p;
        if (h && h.drivesDisplay)
            return h;
        return h || p || null;
    }

    function key(slot, id) {
        return "tune|" + slot + "|" + id;
    }

    function knob(slot, id) {
        for (var i = 0; i < store.tune.length; i++)
            if (store.tune[i].gpu === slot && store.tune[i].id === id)
                return store.tune[i];
        return null;
    }

    function typed(t) {
        if (!t)
            return undefined;
        if (t.kind === "toggle")
            return t.value === "on";
        if (store.autoKnobs.indexOf(t.id) >= 0 && t.auto === true)
            return "auto";
        if (t.kind === "slider" || t.kind === "stepper")
            return Number(t.current || 0);
        return t.value;
    }

    // limit knobs with an automatic state that no number stands for: they read
    // and write "auto" (`gpu tune set <gpu> <id> auto`), so Undo can put it back
    readonly property var autoKnobs: ["clock_lock", "fan_pct"]

    // a tool's error without the program names and the bare exit status
    function cleaned(err) {
        return String(err || "").split("\n").map(l => l.trim().replace(/^ryoku-hub:\s*/, "")
            .replace(/^(ryoku[\w-]*|pkexec|nvidia-smi)\b[^:]*:\s*/, "").replace(/:?\s*exit status \d+$/, ""))
            .filter(l => l !== "").join(" ").replace(/[.\s]+$/, "");
    }

    // a read that failed, as the whole sentence the page shows next to Retry
    function readFailure(what, err, code) {
        if (code === -1)
            return what + ". " + String(err || "");
        var e = store.cleaned(err);
        return e !== "" ? I18n.tr("%1: %2. Retry once that is fixed.").arg(what).arg(e)
            : I18n.tr("%1. Retry, and if it keeps failing, run ryoku doctor.").arg(what);
    }

    // a privileged step that failed, in plain words with the next step; code -1 is
    // a program that is missing or ran out of time, which Changes words itself
    function failure(err, what, code) {
        if (code === -1)
            return I18n.tr("%1 did not change.").arg(what) + " " + String(err || "");
        if (/dismissed|not authorized|authentication failed|exit status 12[67]/i.test(String(err || "")))
            return I18n.tr("%1 did not change because the password prompt was closed. Retry to ask for your password again.").arg(what);
        var e = store.cleaned(err);
        if (e === "")
            return I18n.tr("%1 did not change. Retry, and if it keeps failing, run ryoku doctor.").arg(what);
        return I18n.tr("%1 did not change: %2. Retry once that is fixed.").arg(what).arg(e);
    }

    function snapshot() {
        var out = {};
        for (var i = 0; i < store.tune.length; i++)
            out[store.tune[i].gpu + "|" + store.tune[i].id] = store.typed(store.tune[i]);
        return out;
    }

    function read(k) {
        if (k === "mode")
            return store.mode === "" ? undefined : store.mode;
        if (k === "mux")
            return store.mux.capable ? store.mux.mode : undefined;
        if (k === "tune|all" || k === "tune|reset")
            return store.tuneRead ? store.snapshot() : undefined;
        var p = String(k).split("|");
        return store.typed(store.knob(p[1], p[2]));
    }

    function raw(t, v) {
        if (t.kind === "toggle")
            return v === true ? "on" : "off";
        if (typeof v === "number")
            return String(Math.round(v));
        return String(v);
    }

    function write(k, v, done) {
        if (k === "mode") {
            store.mark(k, true);
            // the mode is a file in your config, with no password to wait for, so a hang ends as an error
            store.run(["ryoku-hub", "gpu", "mode", "set", String(v)], function (ok, out, err, code) {
                store.mark(k, false);
                if (ok)
                    store.mode = String(v);
                Changes.touch();
                Attention.probe("graphics-mode");
                done(ok, ok ? "" : store.failure(err, I18n.tr("The graphics mode"), code));
            }, 15000);
            return;
        }
        if (k === "mux") {
            store.writeMux(String(v), done);
            return;
        }
        if (k === "tune|all" || k === "tune|reset") {
            store.writeAll(v, done);
            return;
        }
        var p = String(k).split("|");
        var t = store.knob(p[1], p[2]);
        if (!t) {
            done(false, I18n.tr("This graphics card does not offer that setting."));
            return;
        }
        store.patch(t, v);
        var q = Object.assign({}, store.queued);
        q[k] = { argv: ["ryoku-hub", "gpu", "tune", "set", t.gpu, t.id, store.raw(t, v)], done: (q[k] ? q[k].done : []).concat([done]) };
        store.queued = q;
        Changes.touch();
        rest.restart();
    }

    // the card the screen is wired to, in words
    function wiredTo(m) {
        return m === "discrete" ? (store.caps.passthrough ? store.caps.passthrough.model : I18n.tr("the discrete graphics card"))
            : I18n.tr("the integrated graphics");
    }

    // the wiring write may wait on a password prompt the user is answering, so it has no
    // time limit: past 1 s the page says it is working, past 10 s it offers Cancel, which
    // stops it (the stop goes on to the password prompt)
    function writeMux(target, done) {
        store.mark("mux", true);
        store.muxStopping = false;
        store.muxSlow = false;
        store.muxVerySlow = false;
        store.muxSlowTimer.restart();
        store.muxVerySlowTimer.restart();
        store.muxJob = store.run(["ryoku-hub", "gpu", "mux", "set", target], function (ok, out, err, code) {
            var stopped = store.muxStopping;
            store.muxJob = null;
            store.muxSlowTimer.stop();
            store.muxVerySlowTimer.stop();
            store.muxSlow = false;
            store.muxVerySlow = false;
            store.muxStopping = false;
            var finish = function (took, message) {
                store.mark("mux", false);
                Attention.probe("display-wiring");
                done(took, message);
            };
            if (ok) {
                var st = store.json(out);
                if (st)
                    store.mux = st;
                finish(true, "");
                return;
            }
            if (!stopped) {
                finish(false, store.failure(err, I18n.tr("The display wiring"), code));
                return;
            }
            // stopped: read the wiring back, so the page says what it is now
            store.run(["ryoku-hub", "gpu", "mux", "get"], function (rok, rout) {
                var now = rok ? store.json(rout) : null;
                if (now)
                    store.mux = now;
                if (now && now.mode === target)
                    finish(true, "");
                else if (now)
                    finish(false, I18n.tr("Stopped. Nothing changed, so the screen stays wired to %1.").arg(store.wiredTo(String(now.mode))));
                else
                    finish(false, I18n.tr("Stopped. Retry to change the wiring."));
            }, 10000);
        });
        Changes.touch();
    }

    function cancelMux() {
        if (!store.muxJob)
            return;
        store.muxStopping = true;
        store.muxJob.signal(15);
        Changes.touch();
    }

    property var muxJob: null
    property bool muxSlow: false
    property bool muxVerySlow: false
    property bool muxStopping: false
    property Timer muxSlowTimer: Timer {
        interval: 1000
        onTriggered: {
            store.muxSlow = true;
            Changes.touch();
        }
    }
    property Timer muxVerySlowTimer: Timer {
        interval: 10000
        onTriggered: {
            store.muxVerySlow = true;
            Changes.touch();
        }
    }

    // a preset, a reset, or (for undo) every knob's earlier value
    function writeAll(v, done) {
        var steps = [];
        if (v && v.preset)
            steps.push(["ryoku-hub", "gpu", "tune", "preset", "apply", String(v.preset)]);
        else if (v && v.reset)
            steps.push(["ryoku-hub", "gpu", "tune", "reset"]);
        else {
            var now = store.snapshot();
            for (var id in v || {}) {
                var p = id.split("|");
                var t = store.knob(p[0], p[1]);
                if (t && JSON.stringify(now[id]) !== JSON.stringify(v[id]))
                    steps.push(["ryoku-hub", "gpu", "tune", "set", t.gpu, t.id, store.raw(t, v[id])]);
            }
        }
        store.mark("tune|all", true);
        store.chain(steps, function (ok, err, code) {
            store.mark("tune|all", false);
            store.loadTune();
            done(ok, ok ? "" : store.failure(err, I18n.tr("The tuning"), code));
        });
    }

    function chain(steps, cb) {
        if (steps.length === 0) {
            cb(true, "");
            return;
        }
        store.run(steps[0], function (ok, out, err, code) {
            if (!ok) {
                cb(false, err, code);
                return;
            }
            store.chain(steps.slice(1), cb);
        });
    }

    function flush() {
        for (var k in store.queued)
            if (!store.busy[k])
                store.send(k);
    }

    function send(k) {
        var job = store.queued[k];
        var q = Object.assign({}, store.queued);
        delete q[k];
        store.queued = q;
        if (!job)
            return;
        store.mark(k, true);
        store.run(job.argv, function (ok, out, err, code) {
            store.mark(k, false);
            for (var i = 0; i < job.done.length; i++)
                job.done[i](ok, ok ? "" : store.failure(err, I18n.tr("The tuning"), code));
            if (store.queued[k])
                store.send(k);
            else
                store.loadTune();
        });
    }

    property Timer rest: Timer {
        interval: 300
        onTriggered: store.flush()
    }

    function patch(t, v) {
        store.tune = store.tune.map(function (x) {
            if (x !== t)
                return x;
            var n = Object.assign({}, x);
            if (x.kind === "toggle")
                n.value = v === true ? "on" : "off";
            else if (v === "auto")
                n.auto = true;
            else if (x.kind === "slider" || x.kind === "stepper") {
                n.current = Number(v);
                n.auto = false;
            } else
                n.value = String(v);
            return n;
        });
        Changes.touch();
    }

    function mark(k, on) {
        var b = Object.assign({}, store.busy);
        if (on)
            b[k] = true;
        else
            delete b[k];
        store.busy = b;
        Changes.touch();
    }

    function json(out) {
        try {
            return JSON.parse(out);
        } catch (e) {
            return null;
        }
    }

    function refresh() {
        store.run(["ryoku-hub", "gpu", "caps"], function (ok, out, err, code) {
            var c = ok ? store.json(out) : null;
            store.caps = c || {};
            store.capsError = c ? "" : store.readFailure(I18n.tr("Could not read your graphics hardware"), err, code);
            store.capsRead = true;
            Changes.touch();
        }, 10000);
        store.run(["ryoku-hub", "gpu", "mode", "get"], function (ok, out, err, code) {
            var m = ok ? store.json(out) : null;
            store.modeError = m ? "" : store.readFailure(I18n.tr("Could not read the graphics mode"), err, code);
            if (m) {
                store.mode = String(m.mode || "hybrid");
                store.modeApplies = m.applies !== false;
                store.performanceAvailable = m.performance_available !== false;
                store.performanceReason = String(m.performance_reason || "");
            }
            Changes.touch();
        }, 10000);
        store.run(["ryoku-hub", "gpu", "mux", "get"], function (ok, out) {
            store.mux = (ok ? store.json(out) : null) || {};
            Changes.touch();
        }, 10000);
        store.run(["ryoku-hub", "gpu", "apply", "availability"], function (ok, out) {
            var a = ok ? store.json(out) : null;
            store.passthroughAvailability = { available: !!(a && a.available === true), reason: a ? String(a.reason || "") : "" };
        }, 10000);
        store.loadTune();
        store.readLive();
    }

    function loadTune() {
        store.run(["ryoku-hub", "gpu", "tune", "caps"], function (ok, out, err, code) {
            var t = ok ? store.json(out) : null;
            store.tune = Array.isArray(t) ? t : [];
            store.tuneError = Array.isArray(t) ? "" : store.readFailure(I18n.tr("Could not read the tuning settings"), err, code);
            store.tuneRead = true;
            Changes.touch();
        }, 10000);
        store.run(["ryoku-hub", "gpu", "tune", "preset", "list"], function (ok, out) {
            var p = ok ? store.json(out) : null;
            store.presets = Array.isArray(p) ? p : [];
        }, 10000);
    }

    // temperature and load of the render card, read once: a sleeping discrete
    // card is reported, never woken, since asking it would cost it its sleep
    function readLive() {
        store.run(["bash", "-c", store.liveScript], function (ok, out) {
            var raw = out.trim();
            if (raw === "asleep") {
                store.live = { ok: false, asleep: true };
                return;
            }
            var p = raw.split(/\s+/);
            store.live = p.length >= 2 ? { ok: true, asleep: false, temp: parseInt(p[0]) || 0, util: parseInt(p[1]) || 0 } : {};
        }, 10000);
    }

    readonly property string liveScript: `
for st in /sys/bus/pci/drivers/nvidia/*/power/runtime_status; do
  [ -r "$st" ] || continue
  IFS= read -r s < "$st"
  [ "$s" = suspended ] && { echo asleep; exit 0; }
  break
done
g=$(nvidia-smi --query-gpu=temperature.gpu,utilization.gpu --format=csv,noheader,nounits 2>/dev/null | head -1)
if [ -n "$g" ]; then echo "$g" | tr ',' ' '; exit 0; fi
for d in /sys/class/drm/card*/device; do
  [ -r "$d/gpu_busy_percent" ] || continue
  u=$(cat "$d/gpu_busy_percent" 2>/dev/null)
  t=$(cat "$d"/hwmon/hwmon*/temp1_input 2>/dev/null | head -1)
  echo "$((t/1000)) $u"; exit 0
done
`

    // reads give up after limit ms, so a hung tool shows its error and Retry; a
    // missing program answers at once
    function run(argv, cb, limit) {
        return Changes.run(argv, cb, { timeout: limit || 0 });
    }

    Component.onCompleted: {
        Changes.registerStore("graphics", store);
        store.refresh();
    }
}

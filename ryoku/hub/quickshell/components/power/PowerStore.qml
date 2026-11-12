import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// what each power mode does to the processor, the battery's charge limit, PCIe
// power saving and the idle timers, as a Changes store over `ryoku-hub cpu`.
// keys: "profile.<mode>.<knob>", "battery.<knob>", "idle.<knob>"
QtObject {
    id: store

    property string active: ""
    property var profiles: []
    // mode -> its knob list as the backend reports it
    property var caps: ({})
    // the last report, which carries the battery and idle knobs for every mode
    property var shared: []
    property bool ready: false
    property string error: ""
    // key -> true while a write runs
    property var busy: ({})
    property var started: ({})
    // Changes' quit path waits while a write is queued or running
    readonly property bool unsettled: Object.keys(store.queued).length > 0 || Object.keys(store.busy).length > 0

    function knob(list, id) {
        for (var i = 0; i < (list || []).length; i++)
            if (list[i].id === id)
                return list[i];
        return null;
    }

    // the backend's description of a key, or null when this machine lacks it
    function tunable(key) {
        var parts = String(key).split(".");
        if (parts[0] === "profile")
            return store.knob(store.caps[parts[1]], parts.slice(2).join("."));
        var id = parts.slice(1).join(".");
        var t = store.knob(store.shared, id);
        return t && t.gpu === parts[0] ? t : null;
    }

    function typed(t) {
        if (!t)
            return undefined;
        if (t.kind === "toggle")
            return t.value === "on";
        if (t.kind === "slider" || t.kind === "stepper")
            return Number(t.current || 0);
        if (t.kind === "segment" && t.gpu === "battery" && t.id === "chargeLimit")
            return Number(t.value);
        return t.value;
    }

    function read(key) {
        return store.typed(store.tunable(key));
    }

    function raw(t, v) {
        if (t.kind === "toggle")
            return v === true ? "on" : "off";
        if (typeof v === "number")
            return String(Math.round(v));
        return String(v);
    }

    // a drag sends many values: the latest one per key goes out once the hand rests
    property var queued: ({})

    function write(key, v, done) {
        var t = store.tunable(key);
        if (!t) {
            done(false, I18n.tr("This computer does not offer that setting."));
            return;
        }
        var parts = String(key).split(".");
        var before = store.read(key);
        // the control shows the new value at once; a failure puts the old one back
        store.patch(parts[0] === "profile" ? parts[1] : "", t.id, v);
        var q = Object.assign({}, store.queued);
        var prior = q[key] ? q[key].done : [];
        q[key] = { value: v, before: q[key] ? q[key].before : before, done: prior.concat([done]) };
        store.queued = q;
        Changes.touch();
        rest.restart();
    }

    function flush() {
        for (var key in store.queued)
            if (!store.busy[key])
                store.send(key);
    }

    function send(key) {
        var job = store.queued[key];
        var q = Object.assign({}, store.queued);
        delete q[key];
        store.queued = q;
        var t = store.tunable(key);
        if (!job)
            return;
        if (!t) {
            for (var j = 0; j < job.done.length; j++)
                job.done[j](false, I18n.tr("This computer no longer offers that setting."));
            return;
        }
        var parts = String(key).split(".");
        var scope = parts[0] === "profile" ? parts[1] : parts[0];
        var mode = parts[0] === "profile" ? parts[1] : "";
        store.mark(key, true);
        // a write may wait on a password prompt, so it gets longer than a read, but not forever
        store.run(["ryoku-hub", "cpu", "set", scope, t.id, store.raw(t, job.value)], function (ok, out, err, code, late) {
            store.mark(key, false);
            // a re-read can fail too (the tool is missing), so the old value goes back first
            if (!ok && !store.queued[key])
                store.patch(mode, t.id, job.before);
            for (var i = 0; i < job.done.length; i++)
                job.done[i](ok, ok ? "" : store.failure(err, code, late));
            if (store.queued[key])
                store.send(key);
            else
                store.load(mode || store.active);
        }, 30000);
    }

    property Timer rest: Timer {
        interval: 300
        onTriggered: store.flush()
    }

    function patch(mode, id, v) {
        var copy = function (list) {
            return (list || []).map(function (t) {
                if (t.id !== id)
                    return t;
                var n = Object.assign({}, t);
                if (t.kind === "toggle")
                    n.value = v === true ? "on" : "off";
                else if (t.kind === "slider" || t.kind === "stepper")
                    n.current = Number(v);
                else
                    n.value = String(v);
                return n;
            });
        };
        if (mode !== "") {
            var c = Object.assign({}, store.caps);
            c[mode] = copy(c[mode]);
            store.caps = c;
        } else {
            store.shared = copy(store.shared);
        }
        Changes.touch();
    }

    function mark(key, on) {
        var b = Object.assign({}, store.busy);
        var s = Object.assign({}, store.started);
        if (on) {
            b[key] = true;
            s[key] = Date.now();
        } else {
            delete b[key];
            delete s[key];
        }
        store.busy = b;
        store.started = s;
        Changes.touch();
    }

    function refresh() {
        store.run(["ryoku-hub", "cpu", "active"], function (ok, out, err, code) {
            var j = null;
            try { j = ok ? JSON.parse(out) : null; } catch (e) { j = null; }
            if (!j) {
                store.error = store.readFailure(err, code);
                store.ready = true;
                Changes.touch();
                return;
            }
            store.error = "";
            store.active = String(j.activeProfile || "");
            store.profiles = Array.isArray(j.profiles) ? j.profiles : [];
            store.load(store.active || store.profiles[0] || "");
        }, 10000);
    }

    // one mode's knobs; the report also refreshes the battery and idle knobs
    function load(mode) {
        var argv = ["ryoku-hub", "cpu", "caps"];
        if (mode)
            argv.push(mode);
        store.run(argv, function (ok, out, err, code) {
            var list = null;
            try { list = ok ? JSON.parse(out) : null; } catch (e) { list = null; }
            if (!Array.isArray(list)) {
                store.error = store.readFailure(err, code);
            } else {
                store.error = "";
                var c = Object.assign({}, store.caps);
                c[mode] = list.filter(t => t.gpu === "cpu");
                store.caps = c;
                store.shared = list.filter(t => t.gpu !== "cpu");
            }
            store.ready = true;
            Changes.touch();
        }, 10000);
    }

    // a tool's error without the program names and the bare exit status
    function cleaned(err) {
        return String(err || "").split("\n").map(l => l.trim().replace(/^ryoku-hub:\s*/, "")
            .replace(/^(ryoku[\w-]*|pkexec|powerprofilesctl)\b[^:]*:\s*/, "").replace(/:?\s*exit status \d+$/, ""))
            .filter(l => l !== "").join(" ").replace(/[.\s]+$/, "");
    }

    // a read that failed, as the whole sentence the page shows; code -1 is a program
    // that is missing or ran out of time, which Changes words itself
    function readFailure(err, code) {
        if (code === -1)
            return I18n.tr("Could not read the processor, battery and idle settings.") + " " + String(err || "");
        var e = store.cleaned(err);
        return e !== "" ? I18n.tr("Could not read the processor, battery and idle settings: %1. Retry once that is fixed.").arg(e)
            : I18n.tr("Could not read the processor, battery and idle settings. Retry, and if it keeps failing, run ryoku doctor.");
    }

    // a write that failed, in plain words with the next step; code -1 is a program
    // that is missing, which Changes words itself, or one stopped after its deadline
    // (late), whose change may still land
    function failure(err, code, late) {
        var raw = String(err || "");
        if (late === true)
            return I18n.tr("The power service did not answer in time, so the change may not have been made. Retry, and if it keeps failing, run ryoku doctor.");
        if (code === -1)
            return I18n.tr("Nothing changed.") + " " + raw;
        if (/dismissed|not authorized|authentication failed|exit status 12[67]/i.test(raw))
            return I18n.tr("Nothing changed because the password prompt was closed. Retry to ask for your password again.");
        var e = store.cleaned(raw);
        return e !== "" ? I18n.tr("The power service did not take the change: %1. Retry once that is fixed.").arg(e)
            : I18n.tr("The power service did not take the change. Retry, and if it keeps failing, run ryoku doctor.");
    }

    // reads and writes give up after limit ms, so a hung service shows its error and
    // Retry; a missing program answers at once
    function run(argv, cb, limit) {
        Changes.run(argv, cb, { timeout: limit || 0 });
    }

    Component.onCompleted: {
        Changes.registerStore("power", store);
        store.refresh();
    }
}

pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import shell.services
import "../modules/Registry.js" as Registry

Singleton {
    id: prefs

    // A choice shows from _pending until shell.json agrees with it. Writes are
    // acknowledged by the daemon; nothing reverts on a timer, so a slow or lost
    // file event can never undo a choice that reached the disk.
    property var _pending: ({})
    property var _previews: ({})
    property var _writeDue: ({})
    property var _inflight: ({})
    property var _ackedAt: ({})
    property int _nextId: 0
    property int _revision: 0
    property string saveError: ""
    readonly property string saveState: saveError.length > 0 ? "failed"
        : Object.keys(_inflight).length > 0 || Object.keys(_writeDue).length > 0
            ? "saving" : "saved"

    readonly property var raw: Config.orpheus && typeof Config.orpheus === "object"
        ? Config.orpheus : ({})

    function clone(value) {
        try {
            return JSON.parse(JSON.stringify(value));
        } catch (error) {
            return null;
        }
    }

    function same(left, right) {
        try {
            return JSON.stringify(left) === JSON.stringify(right);
        } catch (error) {
            return left === right;
        }
    }

    function own(object, key) {
        return object && Object.prototype.hasOwnProperty.call(object, key);
    }

    function clamp(value, low, high, fallback) {
        const number = Number(value);
        return Math.max(low, Math.min(high, isFinite(number) ? number : fallback));
    }

    function choice(value, choices, fallback) {
        return typeof value === "string" && choices.indexOf(value) >= 0
            ? value : fallback;
    }

    function flag(value, fallback) {
        return typeof value === "boolean" ? value : fallback;
    }

    function subset(value, allowed, fallback) {
        const copy = clone(value);
        const source = Array.isArray(copy) ? copy : fallback;
        const result = [];
        for (let i = 0; i < source.length; ++i) {
            const entry = source[i];
            if (typeof entry === "string" && allowed.indexOf(entry) >= 0
                    && result.indexOf(entry) < 0)
                result.push(entry);
        }
        return result;
    }

    function cloneLayout(value) {
        return {
            left: value.left.slice(),
            center: value.center.slice(),
            right: value.right.slice()
        };
    }

    function normaliseLayout(value) {
        const copy = clone(value);
        const valid = copy && typeof copy === "object" && !Array.isArray(copy);
        const source = valid ? copy : Registry.defaultLayout;
        const names = ["left", "center", "right"];
        let hadVoice = false;
        for (let n = 0; n < names.length; ++n) {
            const values = Array.isArray(source[names[n]]) ? source[names[n]] : [];
            if (values.indexOf("voice") >= 0)
                hadVoice = true;
        }
        const preV4 = valid && !hadVoice;
        const result = { left: ["logo"], center: ["voice"], right: [] };
        const seen = { logo: true, voice: true };
        for (let n = 0; n < names.length; ++n) {
            const name = names[n];
            const values = Array.isArray(source[name])
                ? source[name] : Registry.defaultLayout[name];
            for (let i = 0; i < values.length; ++i) {
                const id = values[i];
                if (typeof id !== "string" || seen[id] || Registry.find(id) === null)
                    continue;
                if (preV4 && ["media", "weather", "clock"].indexOf(id) >= 0)
                    continue;
                result[name].push(id);
                seen[id] = true;
            }
        }
        return result;
    }

    function defaults() {
        return {
            span: "islands",
            form: "liquid",
            scale: 1,
            opacity: 0.94,
            rim: true,
            autoHide: "off",
            hoverOpen: false,
            hoverSwitch: true,
            layout: cloneLayout(Registry.defaultLayout),
            gain: 1,
            depth: 22,
            mirror: true,
            feel: "fluid",
            workspaceStyle: "dots",
            workspaceShowEmpty: true,
            workspaceSpare: true,
            showDate: true,
            seconds: false,
            voiceWeather: true,
            mediaWidth: 220,
            coverSpin: true,
            wavy: true,
            spectrum: true,
            windowWidth: 240,
            sysmonItems: ["cpu", "mem", "temp"],
            actions: ["launcher", "capture", "clipboard"],
            trayLimit: 6,
            statusItems: ["wifi", "bluetooth", "volume", "mic"],
            aiTools: ["claude", "codex", "opencode"]
        };
    }

    function value(key) {
        const revision = prefs._revision;
        if (own(prefs._previews, key))
            return prefs._previews[key];
        if (own(prefs._pending, key))
            return prefs._pending[key];
        if (key === "gain" && !own(prefs.raw, "gain") && own(prefs.raw, "reaction")) {
            const reaction = prefs.raw.reaction;
            return reaction === "calm" ? 0.55 : reaction === "still" ? 0 : 1;
        }
        if (own(prefs.raw, key))
            return prefs.raw[key];
        return prefs.defaults()[key];
    }

    function clean(key, value) {
        switch (key) {
        case "span": {
            const span = value === "fit" ? "fused" : value;
            return choice(span, ["islands", "fused", "full"], "islands");
        }
        case "form": return choice(value, ["liquid", "square"], "liquid");
        case "scale": return clamp(value, 0.85, 1.3, 1);
        case "opacity": return clamp(value, 0.6, 1, 0.94);
        case "rim": return flag(value, true);
        case "autoHide": return choice(value, ["off", "smart", "always"], "off");
        case "hoverOpen": return flag(value, false);
        case "hoverSwitch": return flag(value, true);
        case "layout": return normaliseLayout(value);
        case "gain": return clamp(value, 0, 3, 1);
        case "depth": return clamp(value, 8, 40, 22);
        case "mirror": return flag(value, true);
        case "feel": return choice(value, ["fluid", "tight"], "fluid");
        case "workspaceStyle": return choice(value,
            ["dots", "numbers", "kanji", "greek", "dashes", "icons"], "dots");
        case "workspaceShowEmpty": return flag(value, true);
        case "workspaceSpare": return flag(value, true);
        case "showDate": return flag(value, true);
        case "seconds": return flag(value, false);
        case "voiceWeather": return flag(value, true);
        case "mediaWidth": return clamp(value, 120, 320, 220);
        case "coverSpin": return flag(value, true);
        case "wavy": return flag(value, true);
        case "spectrum": return flag(value, true);
        case "windowWidth": return clamp(value, 120, 420, 240);
        case "sysmonItems": return subset(value,
            ["cpu", "mem", "temp", "gpu", "net"], ["cpu", "mem", "temp"]);
        case "actions": return subset(value,
            ["launcher", "ask", "overview", "clipboard", "capture", "wallpaper", "hub"],
            ["launcher", "capture", "clipboard"]);
        case "trayLimit": return Math.round(clamp(value, 3, 12, 6));
        case "statusItems": return subset(value,
            ["wifi", "bluetooth", "volume", "mic"],
            ["wifi", "bluetooth", "volume", "mic"]);
        case "aiTools": return subset(value,
            ["claude", "codex", "opencode"], ["claude", "codex", "opencode"]);
        default: return value;
        }
    }

    readonly property string span: clean("span", value("span"))
    readonly property string form: clean("form", value("form"))
    readonly property real scale: clean("scale", value("scale"))
    readonly property real opacity: clean("opacity", value("opacity"))
    readonly property bool rim: clean("rim", value("rim"))
    readonly property string autoHide: clean("autoHide", value("autoHide"))
    readonly property bool hoverOpen: clean("hoverOpen", value("hoverOpen"))
    readonly property bool hoverSwitch: clean("hoverSwitch", value("hoverSwitch"))
    readonly property var layout: clean("layout", value("layout"))
    readonly property real gain: clean("gain", value("gain"))
    readonly property real depth: clean("depth", value("depth"))
    readonly property bool mirror: clean("mirror", value("mirror"))
    readonly property string feel: clean("feel", value("feel"))
    readonly property string workspaceStyle: clean("workspaceStyle", value("workspaceStyle"))
    readonly property bool workspaceShowEmpty: clean("workspaceShowEmpty", value("workspaceShowEmpty"))
    readonly property bool workspaceSpare: clean("workspaceSpare", value("workspaceSpare"))
    readonly property bool showDate: clean("showDate", value("showDate"))
    readonly property bool seconds: clean("seconds", value("seconds"))
    readonly property bool voiceWeather: clean("voiceWeather", value("voiceWeather"))
    readonly property real mediaWidth: clean("mediaWidth", value("mediaWidth"))
    readonly property bool coverSpin: clean("coverSpin", value("coverSpin"))
    readonly property bool wavy: clean("wavy", value("wavy"))
    readonly property bool spectrum: clean("spectrum", value("spectrum"))
    readonly property real windowWidth: clean("windowWidth", value("windowWidth"))
    readonly property var sysmonItems: clean("sysmonItems", value("sysmonItems"))
    readonly property var actions: clean("actions", value("actions"))
    readonly property int trayLimit: clean("trayLimit", value("trayLimit"))
    readonly property var statusItems: clean("statusItems", value("statusItems"))
    readonly property var aiTools: clean("aiTools", value("aiTools"))
    readonly property bool clock12h: own(_previews, "clock12h")
        ? _previews.clock12h === true : !TimeFormat.is24h

    readonly property real waveStiffness: feel === "tight" ? 320 : 170
    readonly property real waveDamping: feel === "tight" ? 26 : 13

    function lane(name) {
        return ["left", "center", "right"].indexOf(name) >= 0
            ? prefs.layout[name].slice() : [];
    }

    function hasModule(id) {
        return prefs.layout.left.indexOf(id) >= 0
            || prefs.layout.center.indexOf(id) >= 0
            || prefs.layout.right.indexOf(id) >= 0;
    }

    function setLayout(layout) {
        set("layout", normaliseLayout(layout));
    }

    function set(key, value) {
        if (key === "clock12h") {
            const previews = Object.assign({}, prefs._previews);
            delete previews.clock12h;
            prefs._previews = previews;
            TimeFormat.set24h(value !== true);
            ++prefs._revision;
            return;
        }
        if (!own(prefs.defaults(), key))
            return;
        const pending = Object.assign({}, prefs._pending);
        pending[key] = clean(key, clone(value));
        prefs._pending = pending;
        const due = Object.assign({}, prefs._writeDue);
        due[key] = Date.now() + 80;
        prefs._writeDue = due;
        ++prefs._revision;
        settleTimer.start();
    }

    function preview(key, value) {
        if (key !== "clock12h" && !own(prefs.defaults(), key))
            return;
        const previews = Object.assign({}, prefs._previews);
        previews[key] = key === "clock12h" ? value === true : clean(key, clone(value));
        prefs._previews = previews;
        ++prefs._revision;
    }

    function commit(key) {
        if (!own(prefs._previews, key))
            return;
        const value = prefs._previews[key];
        const previews = Object.assign({}, prefs._previews);
        delete previews[key];
        prefs._previews = previews;
        ++prefs._revision;
        set(key, value);
    }

    function reset() {
        const values = defaults();
        prefs._pending = clone(values);
        prefs._previews = ({});
        prefs._writeDue = ({});
        ++prefs._revision;
        prefs._send("orpheus", values, "");
        settleTimer.start();
    }

    // Sends every debounced write now; the settings window calls this on close.
    function flush() {
        const due = Object.assign({}, prefs._writeDue);
        const keys = Object.keys(due);
        for (let i = 0; i < keys.length; ++i)
            due[keys[i]] = 0;
        prefs._writeDue = due;
        prefs._settle();
    }

    function _busy(key) {
        if (own(prefs._writeDue, key))
            return true;
        const ids = Object.keys(prefs._inflight);
        for (let i = 0; i < ids.length; ++i) {
            const entry = prefs._inflight[ids[i]];
            if (entry.key === key || entry.key === "")
                return true;
        }
        return false;
    }

    // A pending choice clears once the file holds it. Only a file change that
    // lands well after the daemon confirmed the write may override it (someone
    // edited the key since); a missed or slow file event never reverts a choice.
    function _reconcile(fromFile) {
        const pending = Object.assign({}, prefs._pending);
        const now = Date.now();
        let changed = false;
        const keys = Object.keys(pending);
        for (let i = 0; i < keys.length; ++i) {
            const key = keys[i];
            if (prefs._busy(key))
                continue;
            const onDisk = own(prefs.raw, key) ? clean(key, prefs.raw[key]) : undefined;
            const acked = Math.max(Number(prefs._ackedAt[key] || 0),
                Number(prefs._ackedAt["*"] || 0));
            if (same(onDisk, pending[key])) {
                delete pending[key];
                changed = true;
            } else if (fromFile === true && acked > 0 && now - acked > 250) {
                console.warn("orpheus: " + key + " changed in shell.json after it was saved");
                delete pending[key];
                changed = true;
            }
        }
        if (changed) {
            prefs._pending = pending;
            ++prefs._revision;
        }
    }

    function _send(path, value, key) {
        const id = "orpheus-" + (++prefs._nextId);
        const inflight = Object.assign({}, prefs._inflight);
        inflight[id] = { path: path, value: value, key: key, sentAt: Date.now(), tries: 1 };
        prefs._inflight = inflight;
        prefs._write(id, path, value);
    }

    function _write(id, path, value) {
        control.queued += "call settings.patch "
            + JSON.stringify({ id: id, path: path, value: value }) + "\n";
        if (control.connected)
            control.flushQueued();
        else
            control.connected = true;
    }

    function _onReply(line) {
        let reply = null;
        try {
            reply = JSON.parse(line);
        } catch (error) {
            return;
        }
        if (!reply || typeof reply.id !== "string" || !own(prefs._inflight, reply.id))
            return;
        const entry = prefs._inflight[reply.id];
        const inflight = Object.assign({}, prefs._inflight);
        delete inflight[reply.id];
        prefs._inflight = inflight;
        if (reply.ok === false) {
            prefs.saveError = String(reply.error || "The shell service refused the change.");
            console.warn("orpheus: could not save " + entry.path + ": " + prefs.saveError);
            const refused = {};
            if (entry.key === "")
                Object.assign(refused, entry.value);
            else
                refused[entry.key] = entry.value;
            const pending = Object.assign({}, prefs._pending);
            let dropped = false;
            const keys = Object.keys(refused);
            for (let i = 0; i < keys.length; ++i) {
                if (!prefs._busy(keys[i]) && same(pending[keys[i]], refused[keys[i]])) {
                    delete pending[keys[i]];
                    dropped = true;
                }
            }
            if (dropped) {
                prefs._pending = pending;
                ++prefs._revision;
            }
            return;
        }
        prefs.saveError = "";
        const acked = Object.assign({}, prefs._ackedAt);
        acked[entry.key === "" ? "*" : entry.key] = Date.now();
        prefs._ackedAt = acked;
        settleTimer.start();
    }

    // Unanswered writes go out again on a fresh connection, backing off to every
    // 30 s; they are idempotent, and the choice stays applied meanwhile.
    function _resendStalled(now) {
        const ids = Object.keys(prefs._inflight);
        let stalled = false;
        for (let i = 0; i < ids.length; ++i) {
            const entry = prefs._inflight[ids[i]];
            if (now - entry.sentAt > Math.min(30000, 2000 * Math.pow(2, entry.tries - 1)))
                stalled = true;
        }
        if (!stalled)
            return;
        const inflight = Object.assign({}, prefs._inflight);
        control.queued = "";
        for (let i = 0; i < ids.length; ++i) {
            const entry = Object.assign({}, inflight[ids[i]]);
            entry.tries += 1;
            entry.sentAt = now;
            inflight[ids[i]] = entry;
            control.queued += "call settings.patch "
                + JSON.stringify({ id: ids[i], path: entry.path, value: entry.value }) + "\n";
            if (entry.tries === 3) {
                prefs.saveError = "The shell service is not answering.";
                console.warn("orpheus: still saving " + entry.path);
            }
        }
        prefs._inflight = inflight;
        control.connected = false;
        Qt.callLater(function() { control.connected = true; });
    }

    function _settle() {
        const now = Date.now();
        const due = Object.assign({}, prefs._writeDue);
        let dueChanged = false;
        const keys = Object.keys(due);
        for (let i = 0; i < keys.length; ++i) {
            const key = keys[i];
            if (now < due[key])
                continue;
            delete due[key];
            dueChanged = true;
            if (own(prefs._pending, key))
                prefs._send("orpheus." + key, prefs._pending[key], key);
        }
        if (dueChanged)
            prefs._writeDue = due;
        prefs._resendStalled(now);
        prefs._reconcile(false);
        // Choices still waiting on the file need no ticking: the file change
        // reconciles them.
        if (Object.keys(prefs._writeDue).length === 0
                && Object.keys(prefs._inflight).length === 0)
            settleTimer.stop();
    }

    onRawChanged: _reconcile(true)

    Timer {
        id: settleTimer
        interval: Object.keys(prefs._writeDue).length > 0 ? 20 : 250
        repeat: true
        onTriggered: prefs._settle()
    }

    readonly property string _sockPath:
        (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    Socket {
        id: control
        path: prefs._sockPath
        property string queued: ""

        function flushQueued() {
            if (!connected || queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        parser: SplitParser {
            onRead: line => prefs._onReply(line)
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}

import QtQuick
import Quickshell.Io
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"

// mouse buttons and the touchpad switch as a Changes store. The shell daemon's
// remapper owns the bindings ("button:<device>|<code>"); the window manager
// owns the touchpad ("touchpad")
QtObject {
    id: store

    property var devices: []
    property var maps: ({})
    // the daemon answered at least once
    property bool seen: false
    // the last press a capture reported: { device, button, at }
    property var pressed: null
    property bool touchpadOn: true

    function key(device, code) {
        return "button:" + device + "|" + code;
    }

    // an unset button reads as null, a value of its own, so undoing its first binding clears it again
    function target(device, code) {
        var m = store.maps[device];
        var t = m ? m[String(code)] : undefined;
        return t === undefined ? null : t;
    }

    function read(k) {
        if (k === "touchpad")
            return store.touchpadOn;
        if (k.indexOf("button:") !== 0)
            return undefined;
        var rest = k.slice(7), bar = rest.lastIndexOf("|");
        return store.target(rest.slice(0, bar), rest.slice(bar + 1));
    }

    function write(k, v, done) {
        if (k === "touchpad") {
            if (store.touchpadBusy) {
                done(false, I18n.tr("The touchpad is still changing. Try again when it has finished."));
                return;
            }
            store.touchpadOn = v === true;
            store.touchpadBusy = true;
            Changes.touch();
            Changes.run(["ryoku", "wm", "act", "input.touchpad", v === true ? "on" : "off"], function (ok, out, err, code) {
                store.touchpadBusy = false;
                if (!ok) {
                    console.warn("touchpad switch failed:", err.trim());
                    store.touchpadOn = v !== true;
                    Changes.touch();
                }
                // code -1: the program is missing or never finished, which its own words say
                done(ok, ok ? "" : code === -1 ? err.trim() : I18n.tr("The touchpad did not change. Run ryoku doctor in a terminal, then retry."));
                store.touchpadStatus.running = true;
            }, { timeout: 10000 });
            return;
        }
        var rest = k.slice(7), bar = rest.lastIndexOf("|");
        var args = { device: rest.slice(0, bar), button: Number(rest.slice(bar + 1)), target: v === undefined ? null : v };
        store.waiting(k, true);
        Changes.call("mouse.map", args, function (ok, error, result) {
            store.waiting(k, false);
            if (result && result.devices)
                store.applyFrame(result);
            if (!ok)
                console.warn("mouse.map failed:", error);
            done(ok, ok ? "" : I18n.tr("The button did not change. Run ryoku doctor in a terminal, then retry."));
        });
    }

    // button keys whose change has waited past a second, so their row says it is working
    property var slowKeys: ({})
    property var waitingSince: ({})

    function waiting(k, on) {
        var w = Object.assign({}, store.waitingSince);
        var s = Object.assign({}, store.slowKeys);
        if (on)
            w[k] = Date.now();
        else {
            delete w[k];
            delete s[k];
        }
        store.waitingSince = w;
        store.slowKeys = s;
        store.slowCheck.running = Object.keys(w).length > 0;
        Changes.touch();
    }

    property Timer slowCheck: Timer {
        interval: 250
        repeat: true
        onTriggered: {
            var now = Date.now(), s = Object.assign({}, store.slowKeys), grew = false;
            for (var k in store.waitingSince) {
                if (!s[k] && now - store.waitingSince[k] >= 1000) {
                    s[k] = true;
                    grew = true;
                }
            }
            if (grew) {
                store.slowKeys = s;
                Changes.touch();
            }
        }
    }

    // shared bus frame, never mutate it
    function applyFrame(f) {
        if (!f || !f.devices)
            return;
        store.seen = true;
        store.devices = f.devices;
        store.maps = f.maps || {};
        if (f.pressed)
            store.pressed = { device: String(f.pressed.device || ""), button: Number(f.pressed.button || 0), at: Date.now() };
        Changes.touch();
    }

    function refresh() {
        Bus.call("mouse.state", {}, function (ok, result) {
            if (ok)
                store.applyFrame(result);
        });
        if (Settings.supports("touchpadToggle") && !store.touchpadStatus.running)
            store.touchpadStatus.running = true;
    }

    // waits for the next mouse press; the answer arrives as `pressed`
    function capture(timeoutMs) {
        store.pressed = null;
        Bus.call("mouse.capture", { timeoutMs: timeoutMs }, function (ok, result) {
            if (ok)
                store.applyFrame(result);
        });
    }

    function test(target, done) {
        Changes.call("mouse.test", { target: target }, function (ok, error) {
            done(ok, ok ? "" : error);
        });
    }

    property QtObject feed: RU.BusTopic {
        topic: "mouse"
        onUpdated: store.applyFrame(store.feed.data)
    }

    property bool touchpadBusy: false

    property Process touchpadStatus: Process {
        command: ["ryoku", "wm", "act", "input.touchpad", "status"]
        stdout: StdioCollector {
            onStreamFinished: {
                store.touchpadOn = String(this.text).trim() !== "off";
                Changes.touch();
            }
        }
    }

    Component.onCompleted: {
        Changes.registerStore("mouse-touchpad", store);
        store.refresh();
    }
}

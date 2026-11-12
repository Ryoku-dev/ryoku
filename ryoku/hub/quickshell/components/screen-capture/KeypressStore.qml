import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"

// the key press overlay's look as a Changes store: keypresses.json is read
// here, the shell daemon writes it through keypress.settings, and the keypress
// topic says whether the overlay or its sample is on the desktop
QtObject {
    id: store

    property string theme: "dark"
    property string mode: "all"
    property bool loaded: false
    // the daemon's overlay state: disabled, starting, ready or error
    property string backend: "disabled"
    property string backendError: ""
    readonly property bool showing: store.backend !== "disabled"
    property real revision: 0
    // where the overlay was dragged to, or null while it sits in its default place
    property var placement: null

    function read(key) {
        if (!store.loaded)
            return undefined;
        if (key === "theme")
            return store.theme;
        if (key === "mode")
            return store.mode;
        if (key === "placement")
            return store.placement;
        return undefined;
    }

    // "placement" null puts the overlay back in its default place; Undo writes the old spot back
    function write(key, v, done) {
        if (key === "placement") {
            var spot = v && typeof v === "object" && typeof v.px === "number" && typeof v.py === "number" ? v : null;
            var args = spot ? { px: spot.px, py: spot.py, monitor: String(spot.monitor || "") } : { resetPlacement: true };
            Changes.call("keypress.settings", args, function (ok, error) {
                if (ok) {
                    store.placement = spot ? { px: spot.px, py: spot.py, monitor: String(spot.monitor || "") } : null;
                    Changes.touch();
                    done(true, "");
                    return;
                }
                var said = store.sentence(error);
                done(false, [spot ? I18n.tr("The overlay could not go back to where it was.") : I18n.tr("The overlay could not move back to its usual place."),
                             said, /try again/i.test(said) ? "" : I18n.tr("Press Retry to try again.")].filter(t => t !== "").join(" "));
            });
            return;
        }
        if (key !== "theme" && key !== "mode") {
            done(false, "");
            return;
        }
        var before = store[key];
        store[key] = String(v);
        Changes.touch();
        if (store.showing)
            store.showSample(true);
        var args = {};
        args[key] = store[key];
        Changes.call("keypress.settings", args, function (ok, error) {
            if (ok) {
                done(true, "");
                return;
            }
            store[key] = before;
            Changes.touch();
            var said = store.sentence(error);
            done(false, [I18n.tr("The overlay's look was not saved."), said,
                         /try again/i.test(said) ? "" : I18n.tr("Press Retry to try again.")].filter(t => t !== "").join(" "));
        });
    }

    // the sample takes the look being edited and can be dragged into place
    function showSample(show) {
        store.revision = Math.max(store.revision + 1, Date.now() * 1000);
        var rev = String(store.revision);
        if (show)
            Bus.ipc("keypresses", "activate", [store.theme, store.mode, rev]);
        else
            Bus.ipc("keypresses", "deactivate", [rev]);
    }

    // a daemon reason as its own sentence, or "" when it gave none
    function sentence(text) {
        var t = String(text || "").trim();
        if (t === "")
            return "";
        t = t.charAt(0).toUpperCase() + t.slice(1);
        return /[.!?]$/.test(t) ? t : t + ".";
    }

    function adopt(text) {
        var o = {};
        try {
            o = JSON.parse(text) || {};
        } catch (e) {
            o = {};
        }
        store.theme = o.theme === "light" ? "light" : "dark";
        store.mode = o.mode === "shortcuts" ? "shortcuts" : "all";
        store.placement = typeof o.px === "number" && typeof o.py === "number"
            ? { px: o.px, py: o.py, monitor: String(o.monitor || "") } : null;
        store.loaded = true;
        Changes.touch();
    }

    property FileView file: FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/keypresses.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: store.adopt(text())
        onLoadFailed: store.adopt("")
    }

    // shared bus frame, never mutate it
    property QtObject feed: RU.BusTopic {
        topic: "keypress"
        onUpdated: {
            var f = store.feed.data;
            if (!f || typeof f !== "object")
                return;
            store.backend = String(f.status || "disabled");
            store.backendError = String(f.error || "");
        }
    }

    Component.onCompleted: Changes.registerStore("keypress", store)
}

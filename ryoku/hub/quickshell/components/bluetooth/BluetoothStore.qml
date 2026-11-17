import QtQuick
import Quickshell.Io
import Quickshell.Bluetooth
import Ryoku.Ui.Singletons
import "../../Singletons"

// the Bluetooth radio as a Changes store: BlueZ is the truth, the power helper the
// write (it also records the choice for the next start), so undo works from any page.
// headsetMic is WirePlumber's own setting, read and written through ryoku-bt-audio mic;
// it stays unknown until a read lands, so a missing wpctl or a stopped WirePlumber shows
QtObject {
    id: store

    property bool busy: false
    property bool micKnown: false
    property bool micOn: true
    readonly property var adapter: Bluetooth.defaultAdapter

    function read(key) {
        if (key === "power")
            return !!store.adapter && store.adapter.enabled === true;
        if (key === "headsetMic")
            return store.micKnown ? store.micOn : undefined;
        return undefined;
    }

    // what WirePlumber holds now; a failed read leaves the switch off with its reason
    function loadMic() {
        Changes.run(["ryoku-bt-audio", "mic"], function (ok, out) {
            var v = String(out || "").trim();
            store.micKnown = ok && (v === "on" || v === "off");
            if (store.micKnown)
                store.micOn = v === "on";
            Changes.touch();
        }, { timeout: 10000 });
    }

    function writeMic(v, done) {
        if (!store.micKnown) {
            done(false, I18n.tr("Needs WirePlumber running and wpctl installed."));
            return;
        }
        var was = store.micOn;
        store.micOn = v === true;
        Changes.touch();
        Changes.run(["ryoku-bt-audio", "mic", v === true ? "on" : "off"], function (ok) {
            if (!ok)
                store.micOn = was;
            Changes.touch();
            done(ok, ok ? "" : I18n.tr("The setting could not be saved. Make sure WirePlumber is running, then press Retry."));
            store.loadMic();
        }, { timeout: 10000 });
    }

    function write(key, v, done) {
        if (key === "headsetMic") {
            store.writeMic(v, done);
            return;
        }
        if (key !== "power") {
            done(false, "");
            return;
        }
        // one change at a time, so every write hears back exactly once
        if (store.busy) {
            done(false, I18n.tr("The radio is still changing. Try again when it finishes."));
            return;
        }
        store.busy = true;
        // the helper gives BlueZ a few seconds; past 20 s something hangs, and the row says so
        Changes.run(["ryoku-bluetooth-power", v ? "on" : "off"], function (ok, out, err, code, late) {
            store.busy = false;
            Changes.touch();
            if (ok) {
                done(true, "");
                return;
            }
            // a missing helper (code -1) already reads in plain words with its next step
            if (late || code === -1) {
                done(false, late ? I18n.tr("The radio did not change in time. Try again.") : err);
                return;
            }
            var lines = (err + "\n" + out).split("\n").map(l => l.trim()).filter(l => l.length > 0);
            // the helper's own name is not news to anyone
            var why = lines.length > 0 ? lines[lines.length - 1].replace(/^[\w-]+:\s*/, "") : "";
            done(false, why !== "" ? I18n.tr("The radio did not change: %1").arg(why) : I18n.tr("The radio did not change."));
        }, { timeout: 20000 });
    }

    property Connections feed: Connections {
        target: store.adapter
        ignoreUnknownSignals: true
        function onEnabledChanged() { Changes.touch(); }
    }

    onAdapterChanged: Changes.touch()

    Component.onCompleted: Changes.registerStore("bluetooth", store)
}

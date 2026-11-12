import QtQuick
import Quickshell.Io
import Quickshell.Bluetooth
import Ryoku.Ui.Singletons
import "../../Singletons"

// the Bluetooth radio as a Changes store: BlueZ is the truth, the power helper the
// write (it also records the choice for the next start), so undo works from any page
QtObject {
    id: store

    property bool busy: false
    readonly property var adapter: Bluetooth.defaultAdapter

    function read(key) {
        if (key === "power")
            return !!store.adapter && store.adapter.enabled === true;
        return undefined;
    }

    function write(key, v, done) {
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

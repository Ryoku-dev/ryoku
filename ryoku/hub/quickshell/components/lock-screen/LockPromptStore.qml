import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// the lock screen's password prompt choice as a Changes store: qylock/auth-retry holds on
// or off, and the lock reads it each time it starts, so a change applies from the next lock
QtObject {
    id: store

    property bool known: false
    property bool retry: false
    property int writesRunning: 0
    readonly property bool busy: store.writesRunning > 0

    function read(key) {
        return key === "retry" && store.known ? store.retry : undefined;
    }

    function write(key, v, done) {
        store.writesRunning++;
        var was = store.retry;
        store.retry = v === true;
        store.known = true;
        Changes.touch();
        var argv = ["sh", "-c", "d=\"${XDG_CONFIG_HOME:-$HOME/.config}/qylock\"; mkdir -p \"$d\" && printf '%s\\n' \"$1\" > \"$d/auth-retry\"",
                    "sh", v === true ? "on" : "off"];
        Changes.run(argv, function (ok) {
            store.writesRunning--;
            if (!ok)
                store.retry = was;
            Changes.touch();
            done(ok, ok ? "" : I18n.tr("The setting could not be saved. Check that your config folder can be written, then press Retry."));
            store.load();
        }, { timeout: 10000 });
    }

    // what the file holds now; a missing file is off, the lock's own default
    function load() {
        if (store.busy)
            return;
        var argv = ["sh", "-c", "f=\"${XDG_CONFIG_HOME:-$HOME/.config}/qylock/auth-retry\"; [ -r \"$f\" ] && [ -f \"$f\" ] && head -c 64 -- \"$f\"; true"];
        Changes.run(argv, function (ok, out) {
            if (!ok)
                return;
            store.retry = String(out || "").trim() === "on";
            store.known = true;
            Changes.touch();
        }, { timeout: 10000 });
    }

    Component.onCompleted: {
        store.load();
        Changes.registerStore("lockprompt", store);
    }
}

import QtQuick
import "../../Singletons"
import "../shortcuts/binds.js" as Binds

// the key that opens each app, keyed by its shipped chord: reads the chord in
// effect ("" while turned off) and writes the same rebind and turn-off lists
// the Keyboard shortcuts page writes
QtObject {
    id: store

    readonly property var rebindsBind: ({ store: "desktop", key: "keybindRebinds" })
    readonly property var offBind: ({ store: "desktop", key: "keybindDisabled" })

    function read(def) {
        if (!Changes.desktopReady)
            return undefined;
        if (Binds.isOff(Changes.value(store.offBind), def))
            return "";
        return Binds.toStore(Binds.effective(Changes.value(store.rebindsBind), def));
    }

    // "" turns the key off; any chord turns it back on
    function write(def, chord, done) {
        var rebinds = Changes.value(store.rebindsBind) || ({});
        var off = Changes.value(store.offBind) || [];
        var want = String(chord || "");
        if (want.length === 0) {
            Changes.write(store.offBind, Binds.withOff(off, def, true), "", "default-apps");
        } else {
            var next = Binds.withRebind(rebinds, def, Binds.toStore(want));
            if (JSON.stringify(next) !== JSON.stringify(rebinds))
                Changes.write(store.rebindsBind, next, "", "default-apps");
            if (Binds.isOff(off, def))
                Changes.write(store.offBind, Binds.withOff(off, def, false), "", "default-apps");
        }
        // the failure belongs to this row, not to the shortcut lists' desktop rows
        Changes.afterDesktopSave(function (ok, error) {
            if (!ok)
                [store.rebindsBind, store.offBind].forEach(b => {
                    if (Changes.error(b) !== "")
                        Changes.setError(b, "");
                });
            done(ok, error);
        });
    }

    Component.onCompleted: Changes.registerStore("launch-keys", store)
}

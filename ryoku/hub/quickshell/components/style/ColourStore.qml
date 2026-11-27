import QtQuick
import "../../Singletons"

// The colour family as one state. Ryogami's theme.policy is canonical: the Hub
// and the wallpaper picker both read it, so neither can show a stale choice.
// theme.colours stays a one-way shadow for readers of shell.json.
QtObject {
    id: store

    property string policy: ""
    property int writeGeneration: 0

    function family() {
        var p = store.policy;
        if (p === "wallpaper" || p === "fixed")
            return "wallpaper";
        return "ryoku";
    }

    function read(key) {
        if (key === "family")
            return store.family();
        return undefined;
    }

    // Changes keeps undo entries in the value space of the store. Keep the full
    // policy there, so undoing Ryoku restores a fixed scheme instead of turning
    // it into follow-wallpaper.
    function changeValue(key, value, before) {
        if (key !== "family")
            return value;
        if (value === "ryoku")
            return "off";
        if (value === "wallpaper")
            return store.policy === "fixed" ? "fixed" : "wallpaper";
        return value; // already a full policy, coming back from undo
    }

    function write(key, v, done) {
        if (key !== "family") {
            done(false, "");
            return;
        }
        var before = store.policy || "off";
        var want = v === "ryoku" ? "off" : v;
        // A fixed scheme is still the desktop palette family. Do not replace it
        // just because the Hub chose Wallpaper again.
        if (want === before || (want === "wallpaper" && before === "fixed")) {
            done(true, "");
            return;
        }
        var generation = ++store.writeGeneration;
        store.policy = want;
        Changes.touch();
        Changes.run(["ryogami", "settings", "set", "theme.policy", want], function (ok) {
            // A newer click owns the displayed policy now. Letting an older
            // failure restore its own before value would undo that choice.
            if (generation !== store.writeGeneration) {
                done(true, "");
                return;
            }
            if (!ok) {
                store.policy = before;
                Changes.touch();
            }
            done(ok, ok ? "" : I18n.tr("The colour choice did not change. Run ryoku doctor in a terminal, then retry."));
        });
    }

    property var config: Live.config("ryogami.json")
    onConfigChanged: {
        var p = store.config && store.config.theme ? store.config.theme.policy : undefined;
        store.policy = typeof p === "string" ? p : "";
        Changes.touch();
    }

    Component.onCompleted: Changes.registerStore("colours", store)
}

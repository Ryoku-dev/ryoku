import QtQuick
import "../../Singletons"

// The colour family as one state. Ryogami's theme.policy is canonical: the Hub
// and the wallpaper picker both read it, so neither can show a stale choice.
// theme.colours stays a one-way shadow for readers of shell.json.
QtObject {
    id: store

    property string policy: ""

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

    function write(key, v, done) {
        if (key !== "family") {
            done(false, "");
            return;
        }
        var before = store.policy;
        var want = v === "wallpaper" ? "wallpaper" : "off";
        // A fixed scheme is still the desktop palette family. Do not replace it
        // just because the Hub chose Wallpaper again.
        if (want === "wallpaper" && (before === "wallpaper" || before === "fixed")) {
            done(true, "");
            return;
        }
        store.policy = want;
        Changes.touch();
        Changes.run(["ryogami", "settings", "set", "theme.policy", want], function (ok) {
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

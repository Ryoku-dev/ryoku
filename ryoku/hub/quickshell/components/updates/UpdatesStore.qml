import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// the update choices kept in hub.toml as a Changes store, so undo works from any page:
// interval is update_interval (the daemon reads it for its background checks),
// system is update_system ("on" makes Update now pass --system)
QtObject {
    id: store

    property string interval: ""
    property var system: undefined

    function read(key) {
        if (key === "interval")
            return store.interval === "" ? undefined : store.interval;
        if (key === "system")
            return store.system;
        return undefined;
    }

    function configKey(key) {
        return key === "system" ? "update_system" : "update_interval";
    }

    function write(key, v, done) {
        var arg = key === "system" ? (v === true ? "on" : "off") : String(v);
        var was = key === "system" ? store.system : store.interval;
        if (key === "system")
            store.system = v === true;
        else
            store.interval = arg;
        Changes.touch();
        var argv = ["ryoku-hub", "config", "set", store.configKey(key), arg];
        Changes.run(argv, function (ok, out, err, code) {
            // a refused write changed nothing, so the control and Retry start from the old value
            if (!ok) {
                if (key === "system")
                    store.system = was;
                else
                    store.interval = was;
                Changes.touch();
            }
            // code -1 is a missing program, or a write stopped after 10 s; a refusal's own words name files, so it gets plain ones
            done(ok, ok ? "" : code === -1
                ? (err === Changes.missingText(argv[0])
                    ? I18n.tr("The setting could not be saved because a Ryoku tool is missing. Update Ryoku, then try again.")
                    : I18n.tr("Saving the setting took too long, so Settings stopped waiting. Press Retry."))
                : I18n.tr("The setting could not be saved. Press Retry, or run ryoku doctor in a terminal if it keeps failing."));
            store.load(key);
        }, { timeout: 10000 });
    }

    // what hub.toml holds now, so the control shows what was really applied; a failed read keeps the last value
    function load(key) {
        Changes.run(["ryoku-hub", "config", "get", store.configKey(key)], function (ok, out) {
            var v = String(out || "").trim();
            if (ok && key === "system")
                store.system = v === "on";
            else if (ok && ["off", "hourly", "daily", "weekly"].indexOf(v) >= 0)
                store.interval = v;
            Changes.touch();
        }, { timeout: 10000 });
    }

    Component.onCompleted: {
        store.load("interval");
        store.load("system");
        Changes.registerStore("updates", store);
    }
}

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// every setting back to how Ryoku ships it, as a Changes store: writing
// "defaults" resets and keeps what it replaced, writing "yours" puts that back
QtObject {
    id: root

    property string state: "yours"
    // path -> value before the reset (undefined: it was not stored); desktop holds
    // the whole window manager store, launcher the whole launcher store
    property var shell: ({})
    property var desktop: null
    property var launcher: null

    // stays: colours and wallpaper (Ryogami's), the language, formats and clock,
    // where you are, and the displays
    readonly property var kept: ["wallpaper", "themePalette", "language", "formatLocale", "general", "weatherLocation",
                                 "weatherUnit", "displays"]
    // the theme namespace holds colours too; only these parts of it reset
    readonly property var themeParts: ["theme.motion", "theme.attributes"]

    // the default setup a new install lands on: a fresh install is the daemon's
    // defaults with this setup's shell and launcher values over them
    readonly property string riceFile: (Quickshell.env("RYOKU_SYSTEM_RICES_DIR") || "/usr/share/ryoku/rices") + "/default/rice.json"
    property var shipped: ({ shell: {}, launcher: {} })
    property FileView riceView: FileView {
        path: root.riceFile
        blockLoading: true
        printErrors: false
        onLoaded: {
            try {
                var look = JSON.parse(text()).look || {};
                root.shipped = { shell: look.shell || {}, launcher: look.launcher || {} };
            } catch (e) {
                root.shipped = { shell: {}, launcher: {} };
            }
        }
    }

    // what shell.json stores: the settings frame also carries runtime data that is not a setting
    property FileView storedView: FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
    }
    function stored() {
        try {
            var o = JSON.parse(root.storedView.text() || "{}");
            return o && typeof o === "object" && !Array.isArray(o) ? o : {};
        } catch (e) {
            return {};
        }
    }

    function read(key) {
        return root.state;
    }

    function clone(v) {
        return v === undefined ? undefined : JSON.parse(JSON.stringify(v));
    }

    function pathGet(obj, path) {
        var parts = path.split(".");
        for (var i = 0; i < parts.length; i++) {
            if (!obj || typeof obj !== "object")
                return undefined;
            obj = obj[parts[i]];
        }
        return obj;
    }

    function write(key, value, done) {
        if (value === "defaults")
            root.reset(done);
        else
            root.restore(done);
    }

    // the settings this resets: every stored or shipped top-level key but the ones that stay
    function paths() {
        var tops = Object.keys(root.stored());
        Object.keys(root.shipped.shell).forEach(k => { if (tops.indexOf(k) < 0) tops.push(k); });
        var out = [];
        for (var i = 0; i < tops.length; i++) {
            if (tops[i] === "theme")
                out = out.concat(root.themeParts);
            else if (root.kept.indexOf(tops[i]) < 0)
                out.push(tops[i]);
        }
        return out;
    }

    // one completion for many writes: done runs once, after the last answer
    function tracker(done) {
        var t = { waiting: 1, failures: [] };
        t.hold = function () { t.waiting++; };
        t.settle = function (ok, what) {
            if (!ok && what && t.failures.indexOf(what) < 0)
                t.failures.push(what);
            if (--t.waiting === 0)
                done(t.failures);
        };
        return t;
    }

    // reset clears the key (the shipped default, or nothing stored), then the value
    // lands on it whole, so a merge never leaves sub-keys of the old value behind
    function put(path, value, t) {
        t.hold();
        Settings.reset(path, function (ok, error) {
            if (!ok) {
                t.settle(false, path);
                return;
            }
            if (value === undefined) {
                t.settle(true, path);
                return;
            }
            Settings.patch(path, root.clone(value), function (ok) { t.settle(ok, path); });
        });
    }

    function launcherStore() {
        Changes.ensureStore("launcher", Qt.resolvedUrl("../launcher/LauncherStore.qml"));
        return Changes.stores["launcher"] || null;
    }

    // the launcher file as the source has it, in one save: a key the source lacks goes back to
    // the launcher's own default; saves in quick succession race the file's re-read and lose keys
    function writeLauncher(source, t) {
        var store = root.launcherStore();
        if (!store)
            return;
        var want = root.clone(source) || {};
        if (Changes.stable(want) === Changes.stable(store.data || {}))
            return;
        var keys = Object.keys(want);
        var last = keys.length > 0 ? keys[keys.length - 1] : Object.keys(store.data || {})[0];
        var rest = Object.assign({}, want);
        delete rest[last];
        if (keys.length === 0)
            rest[last] = store.data[last];
        store.data = rest;
        t.hold();
        store.write(last, want[last], function (ok) { t.settle(ok, I18n.tr("launcher")); });
    }

    // the whole window manager store, top-level key by key, through the one save path
    function writeDesktop(source) {
        var groups = ["desktop", "wm"];
        for (var g = 0; g < groups.length; g++) {
            var want = source[groups[g]] || {};
            var have = Changes.desktopDraft[groups[g]] || {};
            var keys = Object.keys(want);
            Object.keys(have).forEach(k => { if (keys.indexOf(k) < 0 && groups[g] === "desktop") keys.push(k); });
            for (var i = 0; i < keys.length; i++) {
                var key = groups[g] === "desktop" ? keys[i] : "wm." + keys[i];
                Changes.write({ store: "desktop", key: key }, root.clone(want[keys[i]]), "", "backup-import");
            }
        }
    }

    function reset(done) {
        var store = root.launcherStore();
        if (!Changes.desktopReady || !Changes.desktopDefaultsReady || (store && !store.loaded)) {
            done(false, I18n.tr("Your settings have not finished loading yet. Wait a moment, then try again."));
            return;
        }
        // a retry after a partial reset keeps what the first attempt replaced, so Undo still reaches it
        var shell = Object.assign({}, root.shell);
        var t = root.tracker(function (failures) {
            root.shell = shell;
            if (failures.length === 0)
                root.state = "defaults";
            Changes.touch();
            done(failures.length === 0, failures.length === 0 ? ""
                : I18n.tr("Some settings did not reset: %1. Press Retry.").replace("%1", failures.join(", ")));
        });
        var stored = root.stored();
        var list = root.paths();
        for (var i = 0; i < list.length; i++) {
            if (!shell.hasOwnProperty(list[i]))
                shell[list[i]] = root.clone(root.pathGet(stored, list[i]));
            root.put(list[i], root.pathGet(root.shipped.shell, list[i]), t);
        }
        if (root.launcher === null)
            root.launcher = store ? root.clone(store.data || {}) : null;
        root.writeLauncher(root.shipped.launcher, t);
        if (root.desktop === null)
            root.desktop = root.clone(Changes.desktopDraft);
        root.writeDesktop(Changes.desktopDefaults);
        t.hold();
        Changes.afterDesktopSave(function (ok) { t.settle(ok, I18n.tr("window manager")); });
        t.settle(true, "");
    }

    function restore(done) {
        var saved = root.shell;
        var desktop = root.desktop;
        var launcher = root.launcher;
        root.desktop = null;
        root.launcher = null;
        var t = root.tracker(function (failures) {
            // what did not come back stays, so Retry puts back only that
            var left = {};
            failures.forEach(p => { if (saved.hasOwnProperty(p)) left[p] = saved[p]; });
            root.shell = left;
            if (failures.indexOf(I18n.tr("window manager")) >= 0)
                root.desktop = desktop;
            if (failures.indexOf(I18n.tr("launcher")) >= 0)
                root.launcher = launcher;
            root.state = failures.length === 0 ? "yours" : "defaults";
            Changes.touch();
            done(failures.length === 0, failures.length === 0 ? ""
                : I18n.tr("Some settings did not come back: %1. Press Retry.").replace("%1", failures.join(", ")));
        });
        for (var path in saved)
            root.put(path, saved[path], t);
        if (desktop) {
            root.writeDesktop(desktop);
            t.hold();
            Changes.afterDesktopSave(function (ok) { t.settle(ok, I18n.tr("window manager")); });
        }
        if (launcher)
            root.writeLauncher(launcher, t);
        t.settle(true, "");
    }

    Component.onCompleted: {
        Changes.registerStore("reset", root);
        root.launcherStore();
    }
}

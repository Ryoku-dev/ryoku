import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// performance.json as a Changes store: the shell's power saver, effect and memory
// switches. A write changes one key and keeps every other key in the file, so
// keys other readers own (disabledComponents, the shell's own toggles) survive
QtObject {
    id: store

    // what the file holds; a missing key reads as the shell's default
    property var data: ({})
    property bool loaded: false
    property int writing: 0
    // Changes' quit path waits until the file is written
    readonly property bool unsettled: store.writing > 0

    readonly property var defaults: ({
        "lowPowerMode": false,
        "powerProfileEffects": true,
        "autoPowerSaverOnBattery": false,
        "saverDimKeyboard": true,
        "saverPanel60": true,
        "saverDimScreen": true,
        "ambientBarMotion": false,
        "unloadVisualizerWhenSilent": true,
        "unloadWidgetsWhenCovered": true,
        "unloadLauncherWhenIdle": true,
        "unloadOverviewWhenIdle": true
    })
    // read by the window manager when it parses its config
    readonly property var compositorKeys: ["lowPowerMode"]

    function read(key) {
        if (!store.loaded)
            return undefined;
        var v = store.data[key];
        return v === undefined ? store.defaults[key] : v;
    }

    function write(key, v, done) {
        var had = Object.prototype.hasOwnProperty.call(store.data, key), before = store.data[key];
        var next = Object.assign({}, store.data);
        if (v === undefined)
            delete next[key];
        else
            next[key] = v;
        store.data = next;
        var reload = store.compositorKeys.indexOf(key) >= 0;
        // puts the switch back to what it held, unless a newer change replaced it meanwhile
        var putBack = function () {
            if (JSON.stringify(store.data[key]) !== JSON.stringify(v))
                return null;
            var back = Object.assign({}, store.data);
            if (had)
                back[key] = before;
            else
                delete back[key];
            store.data = back;
            return back;
        };
        var finish = function (ok, reason) {
            store.writing--;
            Changes.touch();
            done(ok, reason);
        };
        store.writing++;
        Changes.touch();
        Changes.writeText(file, JSON.stringify(next, null, 2) + "\n", function (ok, error) {
            // the file view's own reason is generic ("an unknown error"), so the page names the file instead
            if (!ok) {
                putBack();
                finish(false, I18n.tr("The performance settings were not saved because %1 could not be written. Make sure its folder can be written to, then Retry.").arg(String(store.file.path)));
                return;
            }
            if (!reload || !Settings.supports("liveConfigEval")) {
                finish(true, "");
                return;
            }
            // the window manager reads the file at parse time, so it re-reads it once the write landed
            Settings.send("wm.act", { action: "config.reload", args: [] }, function (rok, rerror) {
                if (rok) {
                    finish(true, "");
                    return;
                }
                // it kept the old value, so the file goes back to it too and Retry is a whole change again
                var reason = Changes.desktopReason(rerror), back = putBack();
                if (!back) {
                    finish(false, reason);
                    return;
                }
                Changes.writeText(file, JSON.stringify(back, null, 2) + "\n", function () {
                    finish(false, reason);
                });
            });
        });
    }

    function parse(text) {
        var v = {};
        try {
            v = text.trim().length > 0 ? JSON.parse(text) : {};
        } catch (e) {
            return;
        }
        if (v === null || typeof v !== "object" || Array.isArray(v))
            return;
        store.data = v;
        store.loaded = true;
        Changes.touch();
    }

    property FileView file: FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/performance.json"
        watchChanges: true
        atomicWrites: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: store.parse(text())
        onLoadFailed: error => {
            // no file yet: every key reads as its default until the first write makes one
            store.loaded = true;
            Changes.touch();
        }
    }

    Component.onCompleted: Changes.registerStore("performance", store)
}

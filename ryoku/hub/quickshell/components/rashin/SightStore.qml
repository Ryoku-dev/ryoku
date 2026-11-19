import QtQuick
import Quickshell.Io
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"
import "sight.js" as Sight

QtObject {
    id: store

    property bool active: false
    property var installed: []
    property bool languagesReady: false
    property string languageError: ""
    property var requestedLanguages: []
    property var installDone: null
    property var providerModels: ({ routes: [], models: [] })
    property bool modelListRequested: false
    readonly property string savedModel: String(ShellSettings.get("sight.model", ""))
    property string modelIssue: ""
    property string checkedModel: ""
    property var checkedModels: ({})
    property var checkedProviders: []
    readonly property var modelIdentity: Sight.modelIdentity(store.checkedModels, store.savedModel, store.checkedProviders)
    property var gestureKeys: []
    property string modelError: ""
    readonly property var job: installJob
    readonly property var frame: feed.data || ({})
    readonly property string statusLine: store.frame.watching === true ? I18n.tr("Watching for Theia gestures")
        : store.frame.reason === "no-keyboard-access" ? I18n.tr("Needs keyboard access") : I18n.tr("Off")
    readonly property string tapLabel: store.keyLabel("tapKey", "Alt_R", "Right Alt")
    readonly property string highlightLabel: store.keyLabel("highlightKey", "Alt_L", "Left Alt")
    readonly property var defaults: Sight.defaults()
    property bool savingCollection: false
    property bool gestureWriting: false
    property var gestureQueue: []
    readonly property bool unsettled: store.savingCollection || store.gestureWriting || store.gestureQueue.length > 0 || installJob.running

    function keyLabel(setting, fallback, label) {
        ShellSettings.data;
        var name = String(ShellSettings.get("sight." + setting, fallback));
        var key = store.gestureKeys.find(k => k.Name === name)
            || store.gestureKeys.find(k => k.Name === fallback);
        return I18n.tr(key ? key.Label : label);
    }

    function loadGestureKeys() {
        Changes.run(["ryoku-hub", "keybinds"], (ok, out) => {
            if (!store.active || !ok)
                return;
            try {
                var legend = JSON.parse(out);
                store.gestureKeys = legend.gestureKeys || [];
            } catch (e) {}
        }, { timeout: 15000 });
    }

    function read(key) {
        ShellSettings.data;
        var value = ShellSettings.get("sight." + key, store.defaults[key]);
        return key === "target" ? Sight.targetCode(value, I18n.langs) : value;
    }

    function changeValue(key, value, before) {
        if (value && typeof value === "object") {
            if (["tapKey", "highlightKey", "inplace.key"].indexOf(key) >= 0 && value.restoring === undefined)
                return Object.assign({}, value, { restoring: false });
            return value;
        }
        if (["tapKey", "highlightKey", "inplace.key"].indexOf(key) >= 0) {
            var keys = {};
            ["tapKey", "highlightKey", "inplace.key"].forEach(function (name) { keys[name] = store.read(name); });
            if (!before) {
                var owner = store.gestureOwner(key, value);
                if (owner && value === store.defaults[key])
                    keys[owner] = keys[key];
                keys[key] = value;
            }
            return keys;
        }
        if (key === "features.save") {
            var collections = Changes.stores.rashinFeatures;
            return {
                save: value === true,
                collections: before ? collections && collections.read("collections") : value === true
            };
        }
        return value;
    }

    function gestureOwner(key, value) {
        var keys = ["tapKey", "highlightKey", "inplace.key"];
        for (var i = 0; i < keys.length; i++) {
            if (keys[i] !== key && store.read(keys[i]) === value)
                return keys[i];
        }
        return "";
    }

    function writeGesture(key, value, done) {
        store.gestureQueue = store.gestureQueue.concat([{ key: key, value: value, restoring: !!value && typeof value === "object" && value.restoring !== false, done: Sight.once(done) }]);
        store.pumpGesture();
    }

    function pumpGesture() {
        if (store.gestureWriting || store.gestureQueue.length === 0)
            return;
        var entry = store.gestureQueue[0];
        store.gestureQueue = store.gestureQueue.slice(1);
        store.gestureWriting = true;
        var desired = store.changeValue(entry.key, entry.value, false);
        var key = desired[entry.key];
        var owner = store.gestureOwner(entry.key, key);
        var finish = Sight.once(function (ok, error) {
            store.gestureWriting = false;
            entry.done(ok, error);
            store.pumpGesture();
        });
        if (typeof key !== "string" || key === "") {
            finish(false, I18n.tr("Choose a key for this Theia gesture."));
            return;
        }
        if (!owner) {
            store.patch(entry.key, key, finish);
            return;
        }
        var handover = entry.restoring || key === store.defaults[entry.key]
            ? store.read(entry.key) : desired[owner];
        var third = store.gestureOwner(entry.key, handover);
        if (typeof handover !== "string" || handover === key || (third !== "" && third !== owner)) {
            finish(false, I18n.tr("Another Theia gesture already uses this key. Choose a different key first."));
            return;
        }
        var previousOwner = store.read(owner);
        store.patch(owner, handover, function (ok, error) {
            if (!ok) {
                finish(false, error);
                return;
            }
            store.patch(entry.key, key, function (saved, saveError) {
                if (saved) {
                    finish(true, "");
                    return;
                }
                store.patch(owner, previousOwner, function (restored, restoreError) {
                    finish(false, saveError + (restored ? "" : " " + restoreError));
                });
            });
        });
    }

    function patch(key, value, done) {
        var finish = Sight.once(done);
        ShellSettings.patch("sight." + key, value, function (ok, result, error) {
            finish(ok, ok ? "" : String(error || I18n.tr("The Theia setting could not be saved.")));
        });
    }

    function write(key, value, done) {
        done = Sight.once(done);
        if (key === "features.save") {
            store.writeSave(value, done);
            return;
        }
        if (["tapKey", "highlightKey", "inplace.key"].indexOf(key) >= 0) {
            store.writeGesture(key, value, done);
            return;
        }
        if (key !== "ocrLanguages") {
            store.patch(key, value, done);
            return;
        }
        if (installJob.running || !store.languagesReady) {
            done(false, I18n.tr("Wait for the text recognition languages to finish loading."));
            return;
        }
        var selected = Array.from(value || []);
        var missing = Sight.missingLanguages(selected, store.installed);
        if (missing.length === 0) {
            store.patch(key, selected, done);
            return;
        }
        store.requestedLanguages = selected;
        store.installDone = done;
        var names = missing.map(code => "tesseract-data-" + code);
        installJob.start("sight-languages", "install", I18n.tr("Text recognition languages"),
            ["pkexec", "ryoku-host", "pkg", "install"].concat(names));
    }

    function writeSave(value, done) {
        var collections = Changes.stores.rashinFeatures;
        if (!collections || !collections.loaded || collections.error || store.savingCollection) {
            done(false, collections && collections.error ? collections.error : I18n.tr("Wait for the collection setting to finish loading or saving."));
            return;
        }
        var desired = store.changeValue("features.save", value, false);
        if (typeof desired.save !== "boolean" || typeof desired.collections !== "boolean") {
            done(false, I18n.tr("The collection setting could not be read. Try again."));
            return;
        }
        var previous = { save: store.read("features.save"), collections: collections.read("collections") };
        store.savingCollection = true;
        var finish = Sight.once(function (ok, error) {
            store.savingCollection = false;
            done(ok, error);
        });
        collections.write("collections", desired.collections, function (ok, error) {
            if (!ok) {
                finish(false, error);
                return;
            }
            store.patch("features.save", desired.save, function (saved, saveError) {
                if (saved) {
                    finish(true, "");
                    return;
                }
                store.patch("features.save", previous.save, function (shellRestored, shellError) {
                    collections.write("collections", previous.collections, function (restored, restoreError) {
                        finish(false, saveError + (shellRestored ? "" : " " + shellError) + (restored ? "" : " " + restoreError));
                    });
                });
            });
        });
    }

    function refresh() {
        if (!store.active)
            return;
        store.languageError = "";
        Bus.call("sight.languages", {}, function (ok, result, error) {
            if (!store.active)
                return;
            store.languagesReady = ok && result && result.installed !== undefined;
            if (store.languagesReady)
                store.installed = Array.from(result.installed);
            else
                store.languageError = I18n.tr("Text recognition languages could not be read.") + " " + String(error || "");
        });
    }

    function modelOptions(query) {
        if (!store.modelListRequested) return [];
        return Sight.modelSuggestions(store.providerModels, query);
    }

    function checkModel() {
        store.modelIssue = "";
        if (!store.active || !store.savedModel
                || ["auto", "auto:fast", "auto:smart"].indexOf(store.savedModel) >= 0)
            return;
        if (savedModelProc.running || providerProc.running)
            return;
        store.checkedModel = store.savedModel;
        savedModelProc.command = ["rashin", "call", "models.list", JSON.stringify({ provider: store.savedModel.split("/")[0] })];
        savedModelProc.running = true;
    }

    onSavedModelChanged: checkModel()
    function loadModels() {
        if (!store.active) return;
        store.modelListRequested = true;
        if (modelProc.running) return;
        store.modelError = "";
        modelProc.running = true;
    }

    onActiveChanged: {
        if (store.active) {
            store.loadGestureKeys();
            store.refresh();
            store.checkModel();
        }
        else {
            modelProc.running = false;
            savedModelProc.running = false;
            providerProc.running = false;
        }
    }

    property QtObject feed: RU.BusTopic {
        topic: "sight"
        active: store.active
    }
    property QtObject settingsConnection: Connections {
        target: ShellSettings
        function onChanged(paths) {
            if (store.active && ShellSettings.touches("sight", paths))
                Changes.touch();
        }
    }
    property QtObject install: RU.PackageJob {
        id: installJob
        onFinished: (target, action, ok) => {
            var done = store.installDone;
            store.installDone = null;
            if (!ok) {
                if (done)
                    done(false, installJob.error);
                return;
            }
            // recheck trained data before enabling recognition
            Bus.call("sight.languages", {}, function (readOK, result, error) {
                if (readOK && result && result.installed !== undefined) {
                    store.installed = Array.from(result.installed);
                    store.languagesReady = true;
                }
                if (!readOK || Sight.missingLanguages(store.requestedLanguages, store.installed).length > 0) {
                    store.languageError = I18n.tr("The installed text recognition languages are not available yet. Try again.");
                    if (done)
                        done(false, store.languageError);
                    return;
                }
                store.languageError = "";
                if (done)
                    store.patch("ocrLanguages", store.requestedLanguages, done);
                else
                    Changes.set({ store: "sight", key: "ocrLanguages" }, store.requestedLanguages,
                        I18n.tr("Text recognition languages"), "rashin");
            });
        }
    }
    property Process models: Process {
        id: modelProc
        command: ["rashin", "call", "models.list", "{}"]
        environment: Spawn.env
        stdout: StdioCollector { id: modelOut }
        stderr: StdioCollector {}
        onExited: (code, status) => {
            if (!store.active)
                return;
            try {
                if (code !== 0) throw new Error("models");
                const data = JSON.parse(modelOut.text);
                store.providerModels = { routes: data.routes || [], models: (data.models || []).filter(m => m.available === true) };
            } catch (e) {
                store.modelError = I18n.tr("Models could not be listed. You can still enter a model name.");
            }
        }
    }

    property Process savedModelCheck: Process {
        id: savedModelProc
        environment: Spawn.env
        stdout: StdioCollector { id: savedModelOut }
        onExited: (code, status) => {
            if (!store.active) return;
            if (store.checkedModel !== store.savedModel) { store.checkModel(); return; }
            try {
                if (code !== 0) throw new Error("models");
                store.checkedModels = JSON.parse(savedModelOut.text);
                store.modelIssue = Sight.modelUnavailable(store.checkedModels, store.savedModel, []);
                if (store.modelIssue) providerProc.running = true;
            } catch (e) {
                store.modelIssue = "unavailable";
            }
        }
    }
    property Process providerCheck: Process {
        id: providerProc
        command: ["rashin", "call", "providers.list", "{}"]
        environment: Spawn.env
        stdout: StdioCollector { id: providerOut }
        onExited: (code, status) => {
            if (!store.active) return;
            if (store.checkedModel !== store.savedModel) { store.checkModel(); return; }
            if (code === 0) {
                try {
                    const data = JSON.parse(providerOut.text);
                    store.checkedProviders = data.providers || [];
                    store.modelIssue = Sight.modelUnavailable(store.checkedModels, store.savedModel, data.providers || []);
                } catch (e) {}
            }
        }
    }

    Component.onCompleted: Changes.registerStore("sight", store)
}

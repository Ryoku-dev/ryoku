import QtQuick
import Quickshell.Io
import Ryoku.Ui as RU
import Ryoku.Ui.Singletons
import "../../Singletons"
import "sight.js" as Sight

QtObject {
    id: store

    property bool active: false
    property var roster: []
    property string askHarness: ""
    property var installed: []
    property bool languagesReady: false
    property string languageError: ""
    property var requestedLanguages: []
    property var installDone: null
    property var providerModels: []
    property var gestureKeys: []
    property string modelError: ""
    readonly property var job: installJob
    readonly property var frame: feed.data || ({})
    readonly property string statusLine: store.frame.watching === true ? I18n.tr("Watching for Sight gestures")
        : store.frame.reason === "no-keyboard-access" ? I18n.tr("Needs keyboard access") : I18n.tr("Off")
    readonly property string tapLabel: store.keyLabel("tapKey", "Alt_R", "Right Alt")
    readonly property string highlightLabel: store.keyLabel("highlightKey", "Alt_L", "Left Alt")
    readonly property string engine: String(ShellSettings.get("sight.engine", "ask"))
    readonly property string effectiveEngine: store.engine === "ask" ? store.askHarness || "providers" : store.engine
    readonly property var engines: [{ label: "Same as Ask", value: "ask" }, { label: "Rashin's providers", value: "providers" }]
        .concat(store.roster.map(h => ({ label: h.name || h.id, value: h.id })))
    readonly property var suggestions: {
        if (store.effectiveEngine === "providers")
            return store.providerModels;
        var harness = store.roster.find(h => h.id === store.effectiveEngine);
        return harness && harness.models ? harness.models : [];
    }
    readonly property var defaults: ({ enabled: true, tapKey: "Alt_R", highlightKey: "Alt_L", highlight: true, mode: "explain", target: "", ocrLanguages: ["eng"], localOnly: false, engine: "ask", model: "" })

    function keyLabel(setting, fallback, label) {
        ShellSettings.data;
        var name = String(ShellSettings.get("sight." + setting, fallback));
        var key = store.gestureKeys.find(k => k.Name === name)
            || store.gestureKeys.find(k => k.Name === fallback);
        return I18n.tr(key ? key.Label : label);
    }

    function loadGestureKeys() {
        Changes.run(["ryoku-hub", "keybinds"], (ok, out) => {
            if (!ok)
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

    function patch(key, value, done) {
        ShellSettings.patch("sight." + key, value, function (ok, result, error) {
            if (done)
                done(ok, ok ? "" : String(error || I18n.tr("The Sight setting could not be saved.")));
        });
    }

    function write(key, value, done) {
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

    function refresh() {
        if (!store.active)
            return;
        store.languageError = "";
        Bus.call("sight.languages", {}, function (ok, result, error) {
            store.languagesReady = ok && result && result.installed !== undefined;
            if (store.languagesReady)
                store.installed = Array.from(result.installed);
            else
                store.languageError = I18n.tr("Text recognition languages could not be read.") + " " + String(error || "");
        });
    }

    function loadModels() {
        if (!store.active || store.effectiveEngine !== "providers" || modelProc.running)
            return;
        store.modelError = "";
        modelProc.running = true;
    }

    onActiveChanged: {
        if (store.active) {
            store.loadGestureKeys();
            store.refresh();
        }
        else
            modelProc.running = false;
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
            // Recheck the daemon's inventory before enabling OCR; a successful package
            // transaction does not necessarily provide the requested trained data.
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
        command: ["rashin", "models"]
        environment: Spawn.env
        stdout: StdioCollector { id: modelOut }
        stderr: StdioCollector {}
        onExited: (code, status) => {
            if (!store.active)
                return;
            if (code === 0)
                store.providerModels = Sight.modelSuggestions(modelOut.text);
            else
                store.modelError = I18n.tr("Models could not be listed. You can still enter a model name.");
        }
    }

    Component.onCompleted: Changes.registerStore("sight", store)
}

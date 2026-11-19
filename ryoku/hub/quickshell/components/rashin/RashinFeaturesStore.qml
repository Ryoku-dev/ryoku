import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"
import "sight.js" as Sight

QtObject {
    id: store

    property bool active: false
    property bool loaded: false
    property bool reading: false
    property bool writing: false
    property string error: ""
    property var features: ({})
    readonly property int enabledCount: Sight.rashinFeatureIDs.filter(id => store.features[id] === true).length
    property var queue: []
    property var readers: []
    property bool refreshPending: false
    readonly property bool unsettled: store.writing || store.queue.length > 0

    function read(key) {
        return store.loaded && Sight.rashinFeatureIDs.indexOf(key) >= 0 ? store.features[key] === true : undefined;
    }

    function retry() {
        store.refresh();
    }

    function refresh(done) {
        if (done)
            store.readers = store.readers.concat([Sight.once(done)]);
        if (!store.active && store.readers.length === 0)
            return;
        if (store.writing || store.queue.length > 0) {
            store.refreshPending = true;
            return;
        }
        if (store.reading)
            return;
        store.reading = true;
        store.refreshPending = false;
        Changes.run(["rashin", "status", "--json"], function (ok, out, err) {
            store.reading = false;
            var waiting = store.readers;
            store.readers = [];
            var visibleRead = store.active || waiting.length > 0 || store.queue.length > 0;
            var message = "";
            if (ok && visibleRead) {
                try {
                    var snapshot = JSON.parse(out);
                    if (!snapshot || !snapshot.settings || typeof snapshot.settings !== "object")
                        throw new Error("Missing settings snapshot");
                    store.features = Sight.readFeatures(snapshot.settings);
                    store.loaded = true;
                } catch (e) {
                    ok = false;
                }
            }
            if (!ok)
                message = I18n.tr("Rashin's feature settings could not be read.") + (err ? " " + String(err).trim() : "");
            if (visibleRead) {
                store.error = message;
                if (store.active)
                    Changes.touch();
            }
            waiting.forEach(function (finish) { finish(ok, message); });
            store.pump();
        }, { timeout: 15000, env: Spawn.env });
    }

    function write(key, value, done) {
        var finish = Sight.once(done);
        if (Sight.rashinFeatureIDs.indexOf(key) < 0 || typeof value !== "boolean") {
            finish(false, I18n.tr("This Rashin feature setting is not valid."));
            return;
        }
        if (!store.loaded || store.error !== "") {
            finish(false, store.error || I18n.tr("Wait for Rashin's feature settings to finish loading."));
            return;
        }
        store.queue = store.queue.concat([{ key: key, value: value, done: finish }]);
        store.pump();
    }

    function pump() {
        if (store.writing || store.reading)
            return;
        if (store.queue.length === 0) {
            if (store.refreshPending)
                store.refresh();
            return;
        }
        var entry = store.queue[0];
        store.queue = store.queue.slice(1);
        store.writing = true;
        var patch = {};
        patch[entry.key] = entry.value;
        Changes.run(["rashin", "call", "settings.set", JSON.stringify({ patch: { features: patch } })], function (ok, out, err) {
            var message = "";
            if (ok) {
                var next = Object.assign({}, store.features);
                next[entry.key] = entry.value;
                store.features = next;
            } else {
                message = I18n.tr("The Rashin feature setting could not be saved.") + (err ? " " + String(err).trim() : "");
            }
            store.error = message;
            store.writing = false;
            Changes.touch();
            entry.done(ok, message);
            store.pump();
        }, { timeout: 15000, env: Spawn.env });
    }

    onActiveChanged: {
        if (store.active)
            store.refresh();
    }

    Component.onCompleted: Changes.registerStore("rashinFeatures", store)
}

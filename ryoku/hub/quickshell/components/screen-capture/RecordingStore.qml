import QtQuick
import Ryoku.Ui.Singletons
import "../../Singletons"

// the recorder's settings as a Changes store: the shell daemon owns
// recording.json and is its only writer, record.settings reads and writes it
QtObject {
    id: store

    // loading, ready or unavailable; nothing shows as live until a read lands
    property string status: "loading"
    property string error: ""
    property var values: ({})
    property int serial: 0
    property int readId: 0

    // the daemon's contract: these ride the wire as numbers and booleans
    readonly property var intKeys: ["fps", "bitrate", "audioBitrate", "keyint"]
    readonly property var boolKeys: ["cursor"]
    readonly property var keys: ["fps", "framerateMode", "quality", "bitrateMode", "bitrate", "codec", "encoder",
        "cursor", "container", "audioCodec", "audioBitrate", "colorRange", "keyint", "maxResolution", "directory"]

    function coerce(key, v) {
        if (store.intKeys.indexOf(key) >= 0)
            return Math.round(Number(v));
        if (store.boolKeys.indexOf(key) >= 0)
            return v === true || v === "true";
        return v === undefined || v === null ? "" : String(v);
    }

    function read(key) {
        if (key === "status")
            return store.status;
        if (key === "error")
            return store.error;
        return store.status === "ready" ? store.values[key] : undefined;
    }

    function write(key, v, done) {
        if (store.status !== "ready" || store.keys.indexOf(key) < 0) {
            done(false, I18n.tr("Recording settings cannot be changed until they are read."));
            return;
        }
        var before = store.values[key];
        var next = Object.assign({}, store.values);
        next[key] = store.coerce(key, v);
        store.values = next;
        Changes.touch();
        var one = {};
        one[key] = next[key];
        Changes.call("record.settings", { settings: one }, function (ok, error) {
            if (ok) {
                done(true, "");
                return;
            }
            var back = Object.assign({}, store.values);
            back[key] = before;
            store.values = back;
            Changes.touch();
            var said = String(error || "").trim();
            said = said === "" ? "" : said.charAt(0).toUpperCase() + said.slice(1) + (/[.!?]$/.test(said) ? "" : ".");
            done(false, [I18n.tr("The recorder did not take the change."), said,
                         /try again/i.test(said) ? "" : I18n.tr("Press Retry to try again.")].filter(t => t !== "").join(" "));
        });
    }

    // a later read supersedes an earlier one
    function reload() {
        var id = ++store.serial;
        store.readId = id;
        if (store.status !== "ready")
            store.status = "loading";
        Changes.touch();
        Changes.call("record.settings", {}, function (ok, error, result) {
            if (id !== store.readId)
                return;
            if (ok && result && typeof result === "object" && !Array.isArray(result)) {
                var v = {};
                for (var i = 0; i < store.keys.length; i++)
                    if (result[store.keys[i]] !== undefined && result[store.keys[i]] !== null)
                        v[store.keys[i]] = store.coerce(store.keys[i], result[store.keys[i]]);
                store.values = v;
                store.status = "ready";
                store.error = "";
            } else if (store.status !== "ready") {
                store.status = "unavailable";
                store.error = ok ? I18n.tr("The desktop answered without your recording settings.") : error;
            }
            Changes.touch();
        });
    }

    property Connections busWatch: Connections {
        target: Bus
        function onConnectedChanged() {
            if (Bus.connected)
                store.reload();
        }
    }

    Component.onCompleted: {
        Changes.registerStore("recording", store);
        store.reload();
    }
}

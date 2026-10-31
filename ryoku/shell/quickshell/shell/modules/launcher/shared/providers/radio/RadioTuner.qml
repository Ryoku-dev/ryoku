import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../lib/radio.js" as RadioLib
import ".."

// Live-radio provider on the "@" prefix: "@" lists the stations, "@lofi" tunes
// Lofi Girl in, "@stop" tunes out. Rows come from the engine's station catalog
// crossed with the Radio singleton's live status, so the playing station leads
// with Stop, an aside one leads with Resume, and the fallback promise is
// spelled out before the first note plays. Enter runs the row's primary verb.
Provider {
    id: tuner

    providerId: "radio"
    prefix: "@"
    defaultProvider: false

    property var stations: []

    Connections {
        target: Radio
        function onTuningChanged() { Dispatcher.notifyAsync(); }
        function onAvailableChanged() { Dispatcher.notifyAsync(); }
        function onAvailabilityKnownChanged() { Dispatcher.notifyAsync(); }
    }

    function rowFor(r) {
        var acts = [];
        if (r.verb === "stop")
            acts.push({ id: "stop", name: I18n.tr("Stop"), icon: "", execute: function () { Radio.stop(); } });
        else if (r.verb === "resume")
            acts.push({ id: "resume", name: I18n.tr("Resume"), icon: "", execute: function () { Radio.resume(); } });
        else
            acts.push({ id: "tune", name: I18n.tr("Tune in"), icon: "", execute: (function (id) {
                return function () { Radio.start(id); };
            })(r.id) });
        return {
            id: "radio:" + r.id,
            title: r.on ? I18n.tr("LIVE · %1").arg(r.label) : r.label,
            subtitle: I18n.tr(r.note),
            icon: "",
            type: "Radio",
            score: r.score,
            actions: acts
        };
    }

    function missingPlayerRow() {
        return [{
            id: "radio:install-mpv",
            title: I18n.tr("Radio is not installed"),
            subtitle: I18n.tr("Install mpv to use the radio"),
            icon: "",
            type: "Radio",
            score: -30,
            actions: [{
                id: "install",
                name: I18n.tr("Install mpv"),
                icon: "",
                execute: function () {
                    Spawn.runApp(["kitty", "-e", "gpk", "install", "mpv", "mpv-mpris"], { label: "kitty" });
                }
            }]
        }];
    }

    function query(text) {
        var t = (text || "").trim();
        if (Radio.availabilityKnown && !Radio.available)
            return missingPlayerRow();
        // "@stop" / "@off" tunes out, or lets a parked station go, from
        // anywhere, no list browsing needed; "@resume" picks a parked one up.
        if (t === "stop" || t === "off") {
            if (Radio.on)
                return [{
                    id: "radio:stop",
                    title: I18n.tr("Stop the radio"),
                    subtitle: Radio.tuning ? I18n.tr("%1 is tuning in").arg(Radio.label) : I18n.tr("LIVE · %1 is on air").arg(Radio.label),
                    icon: "",
                    type: "Radio",
                    score: -30,
                    actions: [{ id: "stop", name: I18n.tr("Stop"), icon: "", execute: function () { Radio.stop(); } }]
                }];
            if (Radio.aside)
                return [{
                    id: "radio:dismiss",
                    title: I18n.tr("Let the parked radio go"),
                    subtitle: I18n.tr("%1 is set aside").arg(Radio.aside.label || I18n.tr("the radio")),
                    icon: "",
                    type: "Radio",
                    score: -30,
                    actions: [{ id: "dismiss", name: I18n.tr("Dismiss"), icon: "", execute: function () { Radio.stop(); } }]
                }];
        }
        if (t === "resume" && Radio.aside && !Radio.on)
            return [{
                id: "radio:resume",
                title: I18n.tr("Resume %1").arg(Radio.aside.label || I18n.tr("the radio")),
                subtitle: I18n.tr("set aside for your music"),
                icon: "",
                type: "Radio",
                score: -30,
                actions: [{ id: "resume", name: I18n.tr("Resume"), icon: "", execute: function () { Radio.resume(); } }]
            }];
        var status = {
            on: Radio.on,
            station: Radio.station,
            fellBack: Radio.fellBack,
            tuning: Radio.tuning,
            aside: Radio.aside
        };
        var rows = RadioLib.stationRows(tuner.stations, t, status);
        var out = [];
        for (var i = 0; i < rows.length; i++)
            out.push(rowFor(rows[i]));
        return out;
    }

    // the station catalog never changes at runtime; one read at load.
    Process {
        id: stationsProc
        running: true
        command: ["ryoku-cmd-radio", "stations"]
        stdout: StdioCollector {
            onStreamFinished: {
                var rows = [];
                var lines = text.split("\n").filter(l => l.trim().length > 0);
                try {
                    for (const l of lines)
                        rows.push(JSON.parse(l));
                    tuner.stations = rows;
                    Dispatcher.notifyAsync();
                } catch (e) {}
            }
        }
    }

    Component.onCompleted: Dispatcher.register(tuner)
}

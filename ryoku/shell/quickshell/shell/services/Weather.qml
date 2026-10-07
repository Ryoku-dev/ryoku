pragma Singleton
import QtQuick
import Ryoku.Ui.Singletons
import Quickshell
import Quickshell.Io
import "lib/weather.js" as Model

// QML view of the daemon `weather` topic. The Open-Meteo fetch, geocoding, IP
// fallback, retry ladder, WMO-code and icon mapping and unit conversion all live
// in ryoku-shell (weather.go), so QML makes no HTTP call: `subscribe weather`
// streams a full frame on every change, and location/unit intent rides back over
// a second connection. The daemon ships every display string ready to bind; this
// singleton just parses the frame and re-exposes it, deriving the compact glyph
// and short label for the quick-settings and calendar views from the WMO code.
Singleton {
    id: root

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    // The whole frame, plus the parts callers bind directly.
    property var frame: ({ status: "loading", hourly: [], daily: [] })
    readonly property string status: frame.status || "loading"
    readonly property string errorKind: frame.errorKind || ""
    readonly property string errorText: frame.error || ""
    readonly property string location: frame.location || ""
    readonly property bool hasData: frame.hasData === true
    readonly property var current: frame.current || null
    readonly property var hourly: frame.hourly || []
    readonly property var daily: frame.daily || []
    readonly property var moon: frame.moon || null
    readonly property var air: frame.air || null
    readonly property string updatedAt: frame.updatedAt || ""

    // Compact contract for the quick-settings and calendar views, derived from
    // the full weather frame.
    readonly property bool available: root.status === "loaded" && root.current !== null
    readonly property string temp: root.current ? root.current.temperature : ""
    readonly property int tempNow: root.current ? root.current.temp : 0
    readonly property int humidity: root.current ? root.current.humidity : 0
    readonly property int wind: root.current ? root.current.windValue : 0
    readonly property int feels: root.current ? root.current.feels : 0
    readonly property bool isDay: root.current ? root.current.isDay : true
    readonly property string city: frame.city || ""
    readonly property string condition: root.current ? Model.labelFor(root.current.code) : ""
    readonly property string glyph: root.current ? Model.glyphFor(root.current.code) : "cloud"

    function apply(line) {
        try {
            const f = JSON.parse(line);
            if (!Array.isArray(f.hourly))
                f.hourly = [];
            if (!Array.isArray(f.daily))
                f.daily = [];
            root.frame = f;
        } catch (e) {
            // A malformed frame must never wedge the readout; keep the last good one.
        }
    }

    // The error-state Retry button re-kicks the daemon's poll.
    function retry() { root.send("weather.retry", {}); }

    // Switch the temperature unit from the surface. The persisted weatherUnit is
    // patched through the daemon's settings seam (the sole writer of shell.json,
    // so themePalette and the other keys survive), and a re-fetch in the new unit
    // is kicked now so the surface re-renders live without waiting on the file
    // round-trip; Config's file watch fires sendConfigure again once the patch
    // lands, which the daemon dedupes.
    function setUnit(unit) {
        root.send("settings.patch", { path: "weatherUnit", value: unit });
        root.send("weather.configure", { location: Config.weatherLocation, unit: unit, clock24: TimeFormat.is24h });
    }

    // Push the configured location, unit and clock format to the daemon, which
    // owns the fetch. "auto" units are resolved daemon-side from the locale.
    function sendConfigure() {
        root.send("weather.configure", {
            location: Config.weatherLocation,
            unit: Config.weatherUnit,
            clock24: TimeFormat.is24h
        });
    }

    function send(method, args) {
        ctl.queued += "call " + method + " " + JSON.stringify(args) + "\n";
        if (ctl.connected)
            ctl.flushQueued();
        else
            ctl.connected = true;
    }

    Connections {
        target: TimeFormat
        function onIs24hChanged() { root.sendConfigure(); }
    }

    Connections {
        target: Config
        function onWeatherLocationChanged() { root.sendConfigure(); }
        function onWeatherUnitChanged() { root.sendConfigure(); }
    }

    Socket {
        id: sub
        path: root.sockPath
        parser: SplitParser { onRead: line => root.apply(line) }
        Component.onCompleted: connected = true
        onConnectionStateChanged: {
            if (connected) {
                write("subscribe weather\n");
                flush();
                root.sendConfigure();
            } else {
                retry.restart();
            }
        }
    }

    // The daemon may be down when the shell loads (or restart under it); retry
    // quietly so the readout repopulates once it returns.
    Timer {
        id: retry
        interval: 2000
        onTriggered: if (!sub.connected) sub.connected = true
    }

    Socket {
        id: ctl
        path: root.sockPath
        property string queued: ""

        function flushQueued() {
            if (queued.length === 0)
                return;
            write(queued);
            flush();
            queued = "";
        }

        onConnectionStateChanged: if (connected) flushQueued()
    }
}

pragma Singleton
import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons
import Quickshell
import "lib/weather.js" as Model

// QML view of the daemon `weather` topic. The Open-Meteo fetch, geocoding, IP
// fallback, retry ladder, WMO-code and icon mapping and unit conversion all live
// in ryoku-shell (weather.go), so QML makes no HTTP call: the bus streams a full
// frame on every change, and location/unit intent rides back as bus calls. The
// daemon ships every display string ready to bind; this
// singleton just keeps the frame and re-exposes it, deriving the compact glyph
// and short label for the quick-settings and calendar views from the WMO code.
Singleton {
    id: root

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

    function apply(f) {
        if (!f || typeof f !== "object")
            return;
        if (Array.isArray(f.hourly) && Array.isArray(f.daily))
            root.frame = f;
        else
            root.frame = Object.assign({}, f, {
                hourly: Array.isArray(f.hourly) ? f.hourly : [],
                daily: Array.isArray(f.daily) ? f.daily : []
            });
    }

    // The error-state Retry button re-kicks the daemon's poll.
    function retry() { root.send("weather.retry", {}); }

    // Switch the temperature unit from the surface. The persisted weatherUnit is
    // patched through the daemon's settings seam (the sole writer of shell.json,
    // so themePalette and the other keys survive), and a re-fetch in the new unit
    // is kicked now so the surface re-renders live without waiting on the
    // settings round-trip; Config's weatherUnit change fires sendConfigure again
    // once the patch lands, which the daemon dedupes.
    function setUnit(unit) {
        ShellSettings.patch("weatherUnit", unit);
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
        Bus.call(method, args);
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

    BusTopic {
        id: topicFeed
        topic: "weather"
        onUpdated: root.apply(topicFeed.data)
    }

    // the daemon owns the fetch but not the location, so resend it on reconnect
    Connections {
        target: Bus
        function onConnectedChanged() {
            if (Bus.connected)
                root.sendConfigure();
        }
    }

    Component.onCompleted: if (Bus.connected) root.sendConfigure()
}

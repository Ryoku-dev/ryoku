pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import "../lib/weather.js" as Model

// Live weather for HeroShutter. Open-Meteo needs no API key; location resolves
// once via a keyless IP lookup and is cached at
// ~/.local/state/ryoku/weather-loc.json so a restart skips the coord round-trip.
// replaced the old wttr.in scrape (rate-limited, re-located on every poll).
// public contract = temp / condition / glyph / available, unchanged. hourly /
// daily / humidity / city for richer panes. all parsing + the WMO-code ->
// glyph/label map live in lib/weather.js (unit-tested under node); this
// singleton just fetches and assigns. unit follows the locale (F for US/LR/MM,
// C elsewhere), matching the old IP-located feel.
Singleton {
    id: root

    readonly property string stateDir: (Quickshell.env("XDG_STATE_HOME") || (Quickshell.env("HOME") + "/.local/state")) + "/ryoku"
    // unit override from the launcher config: "" follows the locale (Fahrenheit
    // only for US/LR/MM), "C"/"F" force it. a change refetches in the new unit.
    property string unitOverride: ""
    function unitValue() {
        return unitOverride === "C" ? "celsius"
            : unitOverride === "F" ? "fahrenheit"
            : Model.unitFor(Quickshell.env("LC_MEASUREMENT") || Quickshell.env("LANG") || "")
    }
    onUnitOverrideChanged: if (root.located) root.fetchWeather()

    // public contract, identical to the old wttr.in version.
    property string temp: ""
    property string condition: ""
    property string glyph: "cloud"
    property bool available: false

    // richer data, ready for an hourly / 5-day pane.
    property int tempNow: 0
    property int humidity: 0
    property bool isDay: true
    property int sunrise: -1
    property int sunset: -1
    property string city: ""
    property var hourly: []
    property var daily: []

    property real lat: 0
    property real lon: 0
    property bool located: false
    // set true once construction finishes; the cache FileView's initial
    // blockLoading load fires before ipProc/wxProc exist, so loadLoc must wait.
    property bool ready: false

    function fetchWeather() {
        if (!root.located || wxProc.running)
            return;
        // build the URL with the unit wanted right now, so a unit switch can't
        // race the command binding and fetch the old unit.
        wxProc.command = ["curl", "-s", "--max-time", "10",
            "https://api.open-meteo.com/v1/forecast?latitude=" + root.lat
            + "&longitude=" + root.lon
            + "&current=temperature_2m,weather_code,is_day,relative_humidity_2m"
            + "&hourly=temperature_2m,weather_code&forecast_hours=24"
            + "&daily=weather_code,temperature_2m_max,temperature_2m_min,sunrise,sunset&forecast_days=5"
            + "&timezone=auto&temperature_unit=" + root.unitValue()];
        wxProc.running = true;
    }

    function applyForecast(text) {
        var json = Model.parseJson(text);
        // format in the unit the response is actually in (Open-Meteo echoes it
        // in current_units), so a reading never wears the wrong symbol after a
        // unit switch or a cached replay.
        var respUnit = json && json.current_units && json.current_units.temperature_2m === "\u00b0F"
            ? "fahrenheit" : "celsius";
        var f = Model.parseForecast(json, respUnit);
        if (!f.available)
            return;
        root.tempNow = f.tempNow;
        root.temp = f.temp;
        root.condition = f.condition;
        root.glyph = f.glyph;
        root.humidity = f.humidity;
        root.isDay = f.isDay;
        root.sunrise = f.sunrise;
        root.sunset = f.sunset;
        root.hourly = f.hourly;
        root.daily = f.daily;
        root.available = true;
        wxCache.setText(text);           // cache the forecast so the next start opens on weather, not a bare date
    }

    // Load the resolved location from the shell's shared cache (keyed by the
    // explicit weatherLocation), so a location change in
    // Settings reaches the launcher live and across restarts. The launcher used
    // to read this once at startup and never honour a later change, so it kept
    // showing the previously-located city.
    function loadLoc() {
        if (!root.ready)
            return;
        var c = Model.parseJson(locCache.text());
        if (!c || typeof c.lat !== "number" || typeof c.lon !== "number")
            return;
        if (root.located && c.lat === root.lat && c.lon === root.lon)
            return;
        root.city = c.city || "";
        root.lat = c.lat;
        root.lon = c.lon;
        root.located = true;
        root.fetchWeather();
    }

    Component.onCompleted: {
        root.ready = true;
        // Replay the last cached forecast so the hero opens on weather
        // rather than a bare date; the fresh fetch below overwrites it a moment later.
        var w = wxCache.text();
        if (w && w.length > 0)
            root.applyForecast(w);
        root.loadLoc();
        // No cache yet (fresh profile): locate by IP for this session. The shell
        // owns the shared cache, so the launcher never
        // clobbers the explicit-location resolution.
        if (!root.located)
            ipProc.running = true;
    }

    // fresh profile may not have the state dir; mkdir before the forecast cache is written.
    Process {
        command: ["mkdir", "-p", root.stateDir]
        running: true
    }

    FileView {
        id: locCache
        path: root.stateDir + "/weather-loc.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: root.loadLoc()
    }

    FileView {
        id: wxCache
        path: root.stateDir + "/weather-cache.json"
        blockLoading: true
        printErrors: false
    }

    Process {
        id: ipProc
        command: ["curl", "-s", "--max-time", "8", "http://ip-api.com/json/?fields=status,city,lat,lon"]
        stdout: StdioCollector {
            onStreamFinished: {
                var loc = Model.parseLoc(Model.parseJson(this.text));
                if (loc) {
                    root.city = loc.city;
                    root.lat = loc.lat;
                    root.lon = loc.lon;
                    root.located = true;
                    root.fetchWeather();
                }
            }
        }
    }

    Process {
        id: wxProc
        stdout: StdioCollector {
            onStreamFinished: root.applyForecast(this.text)
        }
    }

    Timer {
        interval: 900000
        running: true
        repeat: true
        // a failed first geolocate (offline boot, DNS hiccup) used to leave
        // weather dead until restart; retry the IP lookup on the same cadence.
        onTriggered: {
            if (!root.located) {
                if (!ipProc.running)
                    ipProc.running = true;
                return;
            }
            root.fetchWeather();
        }
    }
}

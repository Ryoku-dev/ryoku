pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia
import Caelestia.Config
import Caelestia.Services as CServices
import caelestia.services
import caelestia.utils

Singleton {
    id: root

    readonly property string city: CServices.Weather.city
    readonly property string loc: CServices.Weather.loc
    readonly property string icon: CServices.Weather.icon
    readonly property string description: CServices.Weather.description
    readonly property string temp: CServices.Weather.temp
    readonly property real tempC: CServices.Weather.tempC
    readonly property string feelsLike: CServices.Weather.feelsLike
    readonly property real feelsLikeC: CServices.Weather.feelsLikeC
    readonly property int humidity: CServices.Weather.humidity
    readonly property real windSpeed: CServices.Weather.windSpeed
    readonly property string sunrise: CServices.Weather.sunrise
    readonly property string sunset: CServices.Weather.sunset
    readonly property string maxTemp: CServices.Weather.maxTemp
    readonly property string minTemp: CServices.Weather.minTemp
    readonly property var forecast: CServices.Weather.forecast
    readonly property var hourlyForecast: CServices.Weather.hourlyForecast
    readonly property bool hasWeather: CServices.Weather.hasWeather
    readonly property bool loading: CServices.Weather.loading

    readonly property var cc: CServices.Weather.hasWeather ? ({
        weatherCode: CServices.Weather.weatherCode,
        weatherDesc: CServices.Weather.description,
        tempC: CServices.Weather.tempC,
        feelsLikeC: CServices.Weather.feelsLikeC,
        humidity: CServices.Weather.humidity,
        windSpeed: CServices.Weather.windSpeed,
        isDay: true,
        sunrise: CServices.Weather.sunrise,
        sunset: CServices.Weather.sunset
    }) : null

    property string locationSearchQuery: ""
    property bool locationSearchLoading: false
    property string locationSearchError: ""
    property list<var> locationSearchResults: []
    property int locationSearchToken: 0

    function formatTemp(temp, compact = false) {
        const unit = GlobalConfig.services.weatherUnit;
        const value = temp === undefined || temp === null || isNaN(temp) ? "--" : Math.round(Units.toTemperature(temp, unit));
        return Units.formatTemp(value, unit, compact);
    }

    function reload() {
        CServices.Weather.reload();
    }

    function refresh() {
        CServices.Weather.refresh();
    }

    function normalizeCoords(lat, lon) {
        if (!isFinite(lat) || !isFinite(lon))
            return "";

        return `${lat.toFixed(4)},${lon.toFixed(4)}`;
    }

    function buildLocationLabel(result) {
        const parts = [];

        if (result?.name)
            parts.push(result.name);
        if (result?.admin1 && result.admin1 !== result.name)
            parts.push(result.admin1);
        if (result?.country)
            parts.push(result.country);

        return parts.join(", ");
    }

    function queueLocationSearch(query) {
        locationSearchQuery = (query ?? "").trim();
        locationSearchError = "";

        if (locationSearchQuery.length < 2) {
            locationSearchToken++;
            locationSearchLoading = false;
            locationSearchResults = [];
            locationSearchDebounce.stop();
            return;
        }

        locationSearchDebounce.restart();
    }

    function searchLocations(query) {
        const trimmed = (query ?? "").trim();
        if (trimmed.length < 2) {
            locationSearchLoading = false;
            locationSearchResults = [];
            locationSearchError = "";
            return;
        }

        const token = ++locationSearchToken;
        locationSearchLoading = true;
        locationSearchError = "";

        const url = `https://geocoding-api.open-meteo.com/v1/search?name=${encodeURIComponent(trimmed)}&count=10&language=en&format=json`;

        const onSuccess = function(text) {
            if (token !== locationSearchToken)
                return;

            locationSearchLoading = false;

            let json;
            try {
                json = JSON.parse(text);
            } catch (e) {
                locationSearchResults = [];
                locationSearchError = qsTr("Couldn't parse location results. Check your connection and try again.");
                return;
            }
            const results = [];

            if (json.results) {
                for (const result of json.results) {
                    const item = {
                        name: result.name ?? "",
                        admin1: result.admin1 ?? "",
                        country: result.country ?? "",
                        timezone: result.timezone ?? "",
                        latitude: result.latitude,
                        longitude: result.longitude
                    };
                    item.label = buildLocationLabel(item);
                    results.push(item);
                }
            }

            locationSearchResults = results;
        };

        const onError = function() {
            if (token !== locationSearchToken)
                return;

            locationSearchLoading = false;
            locationSearchResults = [];
            locationSearchError = qsTr("Couldn't fetch locations. Check your connection and try again.");
        };

        Requests.get(url, onSuccess, onError);
    }

    function applyLocationResult(result) {
        if (!result)
            return false;

        const coords = normalizeCoords(Number(result.latitude), Number(result.longitude));
        if (!coords)
            return false;

        const label = result.label || buildLocationLabel(result) || result.name || "";

        CServices.Weather.setLocation(coords, label);
        GlobalConfig.services.weatherLocation = coords;
        return true;
    }

    function resetToAutoLocation() {
        GlobalConfig.services.weatherLocation = "";
        CServices.Weather.setLocation("", "");
        CServices.Weather.reload();
    }

    CServices.ServiceRef {
        service: CServices.Weather
    }

    Timer {
        id: locationSearchDebounce

        interval: 300
        repeat: false
        onTriggered: searchLocations(root.locationSearchQuery)
    }
}

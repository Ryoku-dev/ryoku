.pragma library

// Pure time arithmetic for the automatic light/dark schedule. Kept free of
// config and service dependencies so it can be reasoned about on its own.

/// "HH:MM" to minutes since local midnight, or -1 if unparseable.
function parseTime(str) {
    const m = /^\s*(\d{1,2}):(\d{2})\s*$/.exec(str || "");
    if (!m)
        return -1;
    const h = Number(m[1]);
    const min = Number(m[2]);
    if (h > 23 || min > 59)
        return -1;
    return h * 60 + min;
}

/// Parses the "lat,lon" string the weather service already stores. Null if it
/// is empty or malformed.
function parseCoords(raw) {
    const parts = String(raw || "").trim().split(",");
    if (parts.length !== 2)
        return null;
    const lat = Number(parts[0].trim());
    const lon = Number(parts[1].trim());
    if (!isFinite(lat) || !isFinite(lon) || Math.abs(lat) > 90 || Math.abs(lon) > 180)
        return null;
    return { lat: lat, lon: lon };
}

/// Sunrise and sunset for `date` at `lat`/`lon`, as minutes since local
/// midnight. Null during polar day or polar night, when there is no sunrise or
/// sunset to switch on.
///
/// NOAA's low-precision solar position algorithm; accurate to about a minute,
/// far tighter than this feature needs.
function solarTimes(date, lat, lon) {
    const rad = Math.PI / 180;
    const epoch = Date.UTC(2000, 0, 1, 12);

    const days = Math.floor((date.getTime() - epoch) / 86400000);
    const meanSolarNoon = days + 0.0008 - lon / 360;

    const anomaly = (357.5291 + 0.98560028 * meanSolarNoon) % 360;
    const centre = 1.9148 * Math.sin(anomaly * rad) + 0.02 * Math.sin(2 * anomaly * rad) + 0.0003 * Math.sin(3 * anomaly * rad);
    const eclipticLon = (anomaly + centre + 180 + 102.9372) % 360;

    const transit = meanSolarNoon + 0.0053 * Math.sin(anomaly * rad) - 0.0069 * Math.sin(2 * eclipticLon * rad);

    const sinDecl = Math.sin(eclipticLon * rad) * Math.sin(23.44 * rad);
    const cosDecl = Math.cos(Math.asin(sinDecl));

    // -0.833° accounts for atmospheric refraction and the solar disc radius.
    const cosHourAngle = (Math.sin(-0.833 * rad) - Math.sin(lat * rad) * sinDecl) / (Math.cos(lat * rad) * cosDecl);
    if (cosHourAngle > 1 || cosHourAngle < -1)
        return null;

    const hourAngle = Math.acos(cosHourAngle) / rad;

    function toLocalMinutes(julian) {
        const d = new Date(epoch + julian * 86400000);
        return d.getHours() * 60 + d.getMinutes();
    }

    return {
        sunrise: toLocalMinutes(transit - hourAngle / 360),
        sunset: toLocalMinutes(transit + hourAngle / 360)
    };
}

/// Whether `minutes` falls in the light window. Handles a window that wraps
/// past midnight, which a high latitude or a deliberately inverted pair of
/// fixed times produces.
function isLightAt(minutes, light, dark) {
    if (light < dark)
        return minutes >= light && minutes < dark;
    return minutes >= light || minutes < dark;
}

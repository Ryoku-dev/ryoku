// no Date.now() or Math.random() here, both throw in the Quickshell JS engine

function displayTemp(shown, celsius, override) {
    var text = String(shown || "");
    if (text.length === 0)
        return "";
    var frameUnit = text.indexOf("F") >= 0 ? "F" : "C";
    if (override !== "C" && override !== "F")
        return text;
    if (override === frameUnit)
        return text;
    if (celsius === null || celsius === undefined || celsius === "")
        return text;
    var c = Number(celsius);
    if (!isFinite(c))
        return text;
    if (override === "F")
        return Math.round(c * 9 / 5 + 32) + "\u00b0F";
    return Math.round(c) + "\u00b0C";
}

function clockToSec(clock) {
    var m = /^\s*(\d{1,2}):(\d{2})\s*(AM|PM)?\s*$/i.exec(String(clock || ""));
    if (!m)
        return -1;
    var hh = Number(m[1]);
    var mm = Number(m[2]);
    if (mm > 59)
        return -1;
    if (m[3]) {
        if (hh < 1 || hh > 12)
            return -1;
        hh = hh % 12 + (m[3].toUpperCase() === "PM" ? 12 : 0);
    } else if (hh > 23) {
        return -1;
    }
    return hh * 3600 + mm * 60;
}

// Fraction through the current solar phase and whether it is day, from the
// current second-of-day and today's sunrise/sunset (also seconds of local day).
// Day runs sunrise..sunset; night wraps midnight (after sunset to the next
// sunrise, before sunrise from the prior evening's sunset). Sunrise/sunset drift
// under a minute across one day, so today's values proxy the adjacent night's
// ends within a pixel. Returns null when the times are unusable (polar day/night
// or a missing feed) so the caller falls back to a plain clock.
function sunFrac(nowSec, sunriseSec, sunsetSec) {
    if (!isFinite(nowSec) || sunriseSec < 0 || sunsetSec < 0 || sunsetSec <= sunriseSec)
        return null;
    var day = 86400;
    if (nowSec >= sunriseSec && nowSec < sunsetSec)
        return { frac: (nowSec - sunriseSec) / (sunsetSec - sunriseSec), isDay: true };
    if (nowSec >= sunsetSec)
        return { frac: (nowSec - sunsetSec) / ((sunriseSec + day) - sunsetSec), isDay: false };
    return { frac: (nowSec - (sunsetSec - day)) / (sunriseSec - (sunsetSec - day)), isDay: false };
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { displayTemp, clockToSec, sunFrac };
}

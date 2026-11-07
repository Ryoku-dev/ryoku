// Pure helpers for the launcher's view of the shell weather frame: the hero's
// own unit override and the sun arc. No QML and no network, so node can test
// them. Avoids Date.now()/Math.random() (both throw in the Quickshell JS engine).

// The frame's display string in the launcher's unit. `shown` is the daemon's
// formatted reading ("21°C"), `whole` its rounded number in that same unit, and
// `override` the launcher setting ("" follows the frame, "C"/"F" force a scale).
function displayTemp(shown, whole, override) {
    var text = String(shown || "");
    if (text.length === 0)
        return "";
    var frameUnit = text.indexOf("F") >= 0 ? "F" : "C";
    if (override !== "C" && override !== "F")
        return text;
    if (override === frameUnit)
        return text;
    var t = Number(whole);
    if (!isFinite(t))
        return text;
    if (override === "F")
        return Math.round(t * 9 / 5 + 32) + "\u00b0F";
    return Math.round((t - 32) * 5 / 9) + "\u00b0C";
}

// "05:24" or "5:24 AM" (the daemon's 24h or 12h clock) -> seconds since local
// midnight, or -1 when unparseable.
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

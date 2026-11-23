// Whether Rashin is switched on, read from its rashin.json the way `rashin
// ensure` reads it: a missing or unreadable file means off, a version 1 file
// counts only when it was on and the user had not opted out.

var VERSION = 2;

function rashinEnabled(text) {
    var raw = String(text == null ? "" : text).trim();
    if (raw.length === 0)
        return false;
    var parsed;
    try {
        parsed = JSON.parse(raw);
    } catch (error) {
        return false;
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
        return false;
    var version = typeof parsed.version === "number" ? parsed.version : 0;
    if (version < VERSION && Object.keys(parsed).length > 0)
        return parsed.enabled === true && parsed.optedOut !== true;
    return parsed.enabled === true;
}

if (typeof module !== "undefined" && module.exports) {
    module.exports = { rashinEnabled: rashinEnabled };
}

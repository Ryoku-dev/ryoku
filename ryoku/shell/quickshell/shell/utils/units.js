// Compact readouts for the Game Bar cards: bytes, rates, storage, uptime,
// refresh and temperature. Decimal for network rates (what a link is sold in)
// and binary for memory (what the kernel reports), so the numbers stay honest
// to their source.

function bytes(v) {
    const n = Number(v) || 0;
    if (n < 1000) return Math.round(n) + " B";
    if (n < 1000 * 1000) return (n / 1000).toFixed(1) + " kB";
    if (n < 1000 * 1000 * 1000) return (n / 1000000).toFixed(1) + " MB";
    return (n / 1000000000).toFixed(1) + " GB";
}

function rate(bps) {
    const n = Number(bps) || 0;
    return n <= 0 ? "0" : bytes(n) + "/s";
}

function gib(v) {
    return (Number(v) || 0).toFixed(1);
}

function pct(v) {
    return Math.round((Number(v) || 0) * 100) + "%";
}

function uptime(sec) {
    let s = Math.max(0, Math.floor(Number(sec) || 0));
    const d = Math.floor(s / 86400);
    s %= 86400;
    const h = Math.floor(s / 3600);
    s %= 3600;
    const m = Math.floor(s / 60);
    if (d > 0) return d + "d " + h + "h";
    if (h > 0) return h + "h " + m + "m";
    return m + "m";
}

function hz(mode) {
    const m = /@([0-9]+(?:\.[0-9]+)?)/.exec(String(mode || ""));
    return m ? Math.round(parseFloat(m[1])) : 0;
}

// A machine reading arrives in Celsius; the shell stores the user's choice in
// weatherUnit, where "auto" follows the locale. The three Fahrenheit-holdout
// locales are the ones the weather daemon resolves for the same setting.
function tempUnit(configured, locale) {
    if (configured === "celsius" || configured === "fahrenheit")
        return configured;
    return /(^|[_.@-])(US|LR|MM)([_.@-]|$)/.test(String(locale || ""))
        ? "fahrenheit" : "celsius";
}

function temp(celsius, unit) {
    const c = Number(celsius);
    if (!isFinite(c)) return "n/a";
    return unit === "fahrenheit"
        ? Math.round(c * 9 / 5 + 32) + " °F"
        : Math.round(c) + " °C";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { bytes, rate, gib, pct, uptime, hz, tempUnit, temp };

// Figures for a transfer: sizes, rates, time left, progress and the 64-bucket
// piece map. Numbers only; QML adds the translated words around them.

var UNITS = ["B", "KiB", "MiB", "GiB", "TiB"];

function bytes(value) {
    var n = Number(value);
    if (!(n >= 0) || !isFinite(n))
        return "";
    var unit = 0;
    while (n >= 1024 && unit < UNITS.length - 1) {
        n /= 1024;
        unit++;
    }
    var digits = unit === 0 || n >= 100 ? 0 : n >= 10 ? 1 : 2;
    return n.toFixed(digits) + " " + UNITS[unit];
}

function bytePair(done, size) {
    var left = bytes(done);
    var right = bytes(size);
    var split = left.lastIndexOf(" ");
    if (split >= 0 && left.slice(split) === right.slice(right.lastIndexOf(" ")))
        left = left.slice(0, split);
    return [left, right];
}

function speed(value) {
    var text = bytes(value);
    return text.length > 0 ? text + "/s" : "";
}

// eta splits seconds into the two largest units worth reading:
// {unit: "s"|"m"|"h"|"d", a, b} where b is the next unit down (0 when dropped).
function eta(seconds) {
    var n = Math.round(Number(seconds));
    if (!(n >= 0) || !isFinite(n))
        return null;
    if (n < 60)
        return { unit: "s", a: n, b: 0 };
    if (n < 3600)
        return { unit: "m", a: Math.floor(n / 60), b: n < 600 ? n % 60 : 0 };
    if (n < 86400)
        return { unit: "h", a: Math.floor(n / 3600), b: Math.floor((n % 3600) / 60) };
    return { unit: "d", a: Math.floor(n / 86400), b: Math.floor((n % 86400) / 3600) };
}

// fraction is 0..1, or -1 when the size is not known yet.
function fraction(done, size) {
    var d = Number(done);
    var s = Number(size);
    if (!(s > 0) || !(d >= 0))
        return -1;
    return Math.max(0, Math.min(1, d / s));
}

function percent(done, size) {
    var f = fraction(done, size);
    if (f < 0)
        return "";
    // a transfer is only 100% once every byte is there
    return (f >= 1 ? 100 : Math.min(99, Math.floor(f * 100))) + "%";
}

var B64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

// pieces decodes the task's piece map into exactly 64 fills of 0..1.
function pieces(encoded) {
    var text = String(encoded || "").replace(/[^A-Za-z0-9+\/]/g, "");
    var out = [];
    var buffer = 0;
    var bits = 0;
    for (var i = 0; i < text.length && out.length < 64; i++) {
        buffer = (buffer << 6) | B64.indexOf(text.charAt(i));
        bits += 6;
        if (bits >= 8) {
            bits -= 8;
            out.push(((buffer >> bits) & 255) / 255);
        }
    }
    while (out.length < 64)
        out.push(0);
    return out;
}

// ago buckets a timestamp against now: {unit: "now"|"m"|"h"|"d", n}.
function ago(stampMs, nowMs) {
    var minutes = Math.max(0, Math.round((Number(nowMs) - Number(stampMs)) / 60000));
    if (!isFinite(minutes))
        return null;
    if (minutes < 1)
        return { unit: "now", n: 0 };
    if (minutes < 60)
        return { unit: "m", n: minutes };
    if (minutes < 1440)
        return { unit: "h", n: Math.round(minutes / 60) };
    return { unit: "d", n: Math.round(minutes / 1440) };
}

function duration(seconds) {
    var n = Math.round(Number(seconds));
    if (!(n > 0) || !isFinite(n))
        return "";
    var h = Math.floor(n / 3600);
    var m = Math.floor((n % 3600) / 60);
    var s = n % 60;
    var two = function(value) { return value < 10 ? "0" + value : String(value); };
    return h > 0 ? h + ":" + two(m) + ":" + two(s) : m + ":" + two(s);
}

var REASONS = ["network", "private", "unavailable", "unsupported", "rate_limited", "invalid"];

// reasonKey: the kind of failure a reader can act on, from the engine's code.
// busy, internal and anything unknown are all "other"; "engine" is the bar's own
// (the daemon could not be reached), whose message is already a sentence.
function reasonKey(error) {
    var code = String(error && error.code || "");
    if (code === "engine")
        return "engine";
    return REASONS.indexOf(code) >= 0 ? code : "other";
}

// reason: the one plain sentence for a failure. sentences maps a reasonKey to
// its translated sentence (DownloadsClient holds them); a row shows this and
// never the engine's own words.
function reason(error, sentences) {
    var key = reasonKey(error);
    var map = sentences || {};
    if (key === "engine")
        return String(error.message || map.other || "");
    return String(map[key] || map.other || "");
}

// detail: the engine's own words, without the code they may lead with
// ("network: ..."), for a second line where there is room. "" when it said
// nothing more.
function detail(error) {
    var code = String(error && error.code || "");
    var said = String(error && error.message || "");
    if (code === "engine")
        return "";
    if (code.length > 0 && said.indexOf(code + ":") === 0)
        said = said.slice(code.length + 1);
    return said.trim();
}

// count shortens a tally the way a result list shows stars or votes: 1.2k, 34k, 1.1M.
function count(value) {
    var n = Number(value);
    if (!isFinite(n))
        return "";
    var abs = Math.abs(n);
    if (abs < 1000)
        return String(Math.round(n));
    if (abs < 1000000)
        return (n / 1000).toFixed(abs < 10000 ? 1 : 0).replace(/\.0$/, "") + "k";
    return (n / 1000000).toFixed(abs < 10000000 ? 1 : 0).replace(/\.0$/, "") + "M";
}

if (typeof module !== "undefined" && module.exports)
    module.exports = {
        bytes: bytes, bytePair: bytePair, speed: speed, eta: eta, fraction: fraction, percent: percent,
        pieces: pieces, ago: ago, duration: duration, count: count,
        reasonKey: reasonKey, reason: reason, detail: detail
    };

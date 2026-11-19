.pragma library

// Steam Hardware Survey (October 2026) calibration, not benchmark measurements.
// Modal components: RTX 3060, 16 GB RAM, 1920x1080, 8 physical CPU cores.
// Steam does not report one exact CPU SKU or a joint whole-PC configuration.
// We use Ryzen 7 5700X / 16 threads and 60 Hz as explicit reference assumptions.
// A reference machine scores 52, in the middle (Epic) tier.
var baseline = {
    survey: "Steam Hardware & Software Survey, October 2026",
    gpu: "NVIDIA GeForce RTX 3060",
    cpu: "AMD Ryzen 7 5700X",
    threads: 16,
    ramGb: 16,
    displays: [{ width: 1920, height: 1080, refresh: 60 }]
};

var tiers = [
    { id: "common",      name: "Common",      mark: "●",    min: 0 },
    { id: "uncommon",    name: "Uncommon",    mark: "◆",    min: 18 },
    { id: "rare",        name: "Rare",        mark: "★",    min: 27 },
    { id: "ultra-rare",  name: "Ultra Rare",  mark: "✦",    min: 38 },
    { id: "epic",        name: "Epic",        mark: "✧",    min: 49 },
    { id: "legendary",   name: "Legendary",   mark: "✶",    min: 60 },
    { id: "mythical",    name: "Mythical",    mark: "✹",    min: 72 },
    { id: "holographic", name: "Holographic", mark: "◈",    min: 85 },
    { id: "master",      name: "Master",      mark: "✺",    min: 97 }
];

function step(value, bounds) {
    var n = 0;
    for (var i = 0; i < bounds.length; i++)
        if (Number(value) >= bounds[i])
            n = i + 1;
    return n;
}

function integrated(name) {
    var s = String(name || "").toLowerCase();
    return /\b(uhd|iris|vega|hd graph|xe graph|arc grap|integrated)|^graphics|radeon graphics|phoenix|rembrandt|raphael|cezanne|renoir|lucienne|barcelo|mendocin|van gogh|hawk poi|strix|krackan|picasso|\b[6-9]\d0m\b|\b80[56]0s\b|\barc 1[34]0v\b/.test(s);
}

function gpuPerformance(name) {
    var s = String(name || "").toLowerCase();
    if (!s || s === "-")
        return 0;
    if (integrated(s)) {
        if (/\b80[56]0s\b/.test(s))
            return 11;
        if (/\b(780m|880m|890m|1[34]0v)\b/.test(s))
            return 9;
        return /\b(680m|7\d0m|8\d0m|arc grap)\b/.test(s) ? 7 : 3;
    }

    var m = /\brtx\s*(\d{4})\b/.exec(s);
    if (m) {
        var family = Math.floor(Number(m[1]) / 100);
        var model = Number(m[1]) % 100;
        var table = {
            20: { 50: 8, 60: 15, 70: 19, 80: 24 },
            30: { 50: 12, 60: 19, 70: 25, 80: 31, 90: 34 },
            40: { 50: 12, 60: 20, 70: 29, 80: 37, 90: 40 },
            50: { 50: 16, 60: 23, 70: 32, 80: 39, 90: 44 }
        };
        var scores = table[family];
        if (!scores || scores[model] === undefined)
            return 0;
        var value = scores[model];
        if (/\bti\b/.test(s))
            value += model === 60 ? 3 : model === 70 ? 3 : model === 80 ? 2 : 1;
        if (/\bsuper\b/.test(s))
            value += 2;
        if (/\b(laptop|mobile|max-q)\b/.test(s))
            value -= Math.max(2, Math.round(value * 0.14));
        return Math.max(1, Math.min(44, value));
    }

    m = /\bgtx\s*(\d{3,4})\b/.exec(s);
    if (m) {
        var gtx = Number(m[1]);
        if (gtx >= 1650 && gtx < 1700)
            return gtx >= 1660 ? (/\b(super|ti)\b/.test(s) ? 13 : 12) : 8;
        if (gtx >= 1000 && gtx < 1100)
            return gtx >= 1080 ? (/\bti\b/.test(s) ? 20 : 17) : gtx >= 1070 ? 13 : 9;
        return gtx >= 900 ? 6 : 3;
    }

    m = /\brx\s*(\d{3,4})\b/.exec(s);
    if (m) {
        var rx = Number(m[1]);
        var base = 0;
        if (rx >= 9000 && rx < 10000) {
            base = rx >= 9070 ? 30 : rx >= 9060 ? 23 : 17;
            if (rx >= 9070 && /\bxt\b/.test(s)) base = 34;
        } else if (rx >= 7000 && rx < 8000) {
            base = rx >= 7900 ? 33 : rx >= 7800 ? 31 : rx >= 7700 ? 27 : 18;
            if (rx >= 7900 && /\bxtx\b/.test(s)) base = 37;
            else if (rx >= 7900 && /\bxt\b/.test(s)) base = 35;
            else if (rx >= 7600 && /\bxt\b/.test(s)) base += 1;
        } else if (rx >= 6000 && rx < 7000) {
            base = rx >= 6900 ? 33 : rx >= 6800 ? 28 : rx >= 6700 ? 22 :
                   rx >= 6600 ? 16 : rx >= 6500 ? 8 : 7;
            if (/\bxtx\b/.test(s)) base += 2;
            else if (/\bxt\b/.test(s)) base += rx >= 6800 ? 3 : 2;
            if (rx === 6650 || rx === 6750 || rx === 6950) base += 1;
        } else if (rx >= 5000 && rx < 6000) {
            base = rx >= 5700 ? 17 : rx >= 5600 ? 12 : 9;
            if (/\bxt\b/.test(s)) base += 2;
        } else if (rx >= 400 && rx < 600) {
            base = rx >= 580 ? 9 : 5;
        }
        if (/\b(laptop|mobile)\b/.test(s) || /\brx\s*\d{4}m\b/.test(s))
            base -= Math.max(2, Math.round(base * 0.14));
        return Math.max(0, Math.min(44, base));
    }

    m = /\barc\s*([ab])(\d{3})\b/.exec(s);
    if (m) {
        var n = Number(m[2]);
        return m[1] === "b" ? (n >= 580 ? 20 : 17) :
            n >= 770 ? 19 : n >= 750 ? 16 : n >= 580 ? 13 : 8;
    }
    if (/\brtx\s*pro|\ba[456]000\b|\bh[12]00\b|\bb200\b/.test(s))
        return 28; // Conservative workstation guess, not a gaming benchmark.
    return 0;
}

// The wire format still stores a 0..5 class. It must not be changed without
// migrating card code and the signed record protocol together.
function gpuClass(name) {
    var s = String(name || "").toLowerCase();
    if (/rtx\s*pro|\ba[456]000\b|\bh100\b|\bh200\b|\bb200\b/.test(s))
        return 5;
    var m = /(rtx|gtx)\s*(\d{3,4})/.exec(s);
    if (m) {
        var tier = Number(m[2]) % 100;
        var c = tier >= 90 ? 5 : tier >= 80 ? 4 : tier >= 70 ? 3 : tier >= 60 ? 2 : 1;
        return m[1] === "gtx" ? Math.max(1, c - 1) : c;
    }
    m = /\brx\s*(\d{4})/.exec(s);
    if (m) {
        var n = Number(m[1]);
        if (n >= 9000 && n < 10000)
            return n >= 9070 ? (/\bxt\b/.test(s) ? 4 : 3) : n >= 9060 ? 2 : 1;
        var h = Math.floor(n % 1000 / 100);
        return h >= 9 ? 5 : h >= 8 ? 4 : h >= 7 ? 3 : h >= 6 ? 2 : 1;
    }
    if (/\barc\s*[ab]\d{3}/.test(s))
        return 2;
    if (integrated(s))
        return /\b80[56]0s\b/.test(s) ? 3 :
            /\b(680m|7\d0m|8\d0m|1[34]0v)\b/.test(s) ? 2 : 1;
    return 0;
}

function strongest(names) {
    var best = "";
    for (var i = 0; i < names.length; i++) {
        var name = String(names[i] || "");
        if (!name || name === "-")
            continue;
        var score = gpuPerformance(name), current = gpuPerformance(best);
        if (!best || score > current || (score === current && integrated(best) && !integrated(name)))
            best = name;
    }
    return best;
}

function discrete(cls, name) {
    return cls > 0 && !integrated(name);
}

function bothGraphics(names) {
    var built = false, card = false;
    for (var i = 0; i < names.length; i++) {
        var name = String(names[i] || "");
        if (integrated(name))
            built = true;
        else if (gpuPerformance(name) > 0)
            card = true;
    }
    return built && card;
}

function cpuPoints(threads, model) {
    var n = Math.max(0, Math.floor(Number(threads) || 0));
    var table = [[2, 3], [4, 5], [6, 7], [8, 10], [12, 14], [16, 17],
                 [20, 19], [24, 22], [32, 24], [48, 27], [64, 29]];
    var score = n ? 1 : 0;
    for (var i = 0; i < table.length; ++i)
        if (n >= table[i][0]) score = table[i][1];
    var s = String(model || "").toLowerCase(), bonus = 0;
    var intel = /\b(?:core\s*)?i[3579][-\s]?(\d{4,5})/.exec(s);
    if (intel) {
        var digits = intel[1];
        var gen = digits.length === 5 ? Number(digits.slice(0, 2)) : Number(digits.slice(0, 1));
        bonus = gen >= 15 ? 6 : gen >= 12 ? 5 : gen >= 10 ? 3 : gen >= 8 ? 2 : gen >= 6 ? 1 : 0;
    } else if (/\bcore\s*ultra\s*[3579]\b/.test(s)) {
        bonus = /\b(?:2[0-9]{2}|3[0-9]{2})\b/.test(s) ? 6 : 5;
    } else {
        var amd = /\bryzen\s*(?:[3579]|threadripper)\s*(\d{4,5})/.exec(s);
        if (amd) {
            var series = Number(amd[1].charAt(0));
            bonus = series >= 9 ? 6 : series >= 7 ? 5 : series >= 5 ? 4 :
                series >= 3 ? 2 : series >= 1 ? 1 : 0;
        }
    }
    return Math.min(30, score + bonus);
}

function ramPoints(gb) {
    var n = Math.max(0, Number(gb) || 0);
    var table = [[8,2], [12,4], [16,6], [24,8], [32,10],
                 [48,11], [64,12], [96,13], [128,14]];
    var score = 0;
    for (var i = 0; i < table.length; ++i)
        if (n >= table[i][0]) score = table[i][1];
    return score;
}

function oneDisplay(output) {
    if (!output) return 0;
    var width = Math.max(0, Number(output.width) || 0);
    var height = Math.max(0, Number(output.height) || 0);
    var refresh = Math.max(0, Number(output.refresh) || 0);
    if (!width || !height) return 0;
    if (refresh > 1000) refresh /= 1000; // Accept Hz and millihertz.
    var pixels = width * height;
    var resolution = pixels >= 7680 * 4320 ? 16 :
        pixels >= 5120 * 2880 ? 14 : pixels >= 3840 * 2160 ? 12 :
        pixels >= 3440 * 1440 ? 10 : pixels >= 2560 * 1440 ? 9 :
        pixels >= 1920 * 1080 ? 6 : pixels >= 1366 * 768 ? 4 :
        pixels >= 1280 * 720 ? 3 : 1;
    var hz = refresh >= 360 ? 5 : refresh >= 240 ? 4 : refresh >= 165 ? 3 :
        refresh >= 120 ? 2 : refresh >= 90 ? 1 : 0;
    return resolution + hz;
}

function displayStats(machine) {
    var m = machine || {};
    var seen = {};
    var outputs = (Array.isArray(m.displays) ? m.displays : []).filter(function(o) {
        if (!o || o.disabled || !(Number(o.width) > 0) || !(Number(o.height) > 0))
            return false;
        var name = String(o.name || "");
        if (name && seen[name]) return false;
        if (name) seen[name] = true;
        return true;
    });
    if (!outputs.length) {
        var width = Number(m.width) || 0, height = Number(m.height) || 0;
        if (!height && width) height = Math.round(width * 9 / 16);
        var legacy = { width: width, height: height, refresh: Number(m.refresh) || 0 };
        return { points: oneDisplay(legacy), count: width > 0 ? 1 : 0,
                 best: legacy, extras: 0, screens: width > 0 ?
                 [{ name: "", width: width, height: height, refresh: legacy.refresh,
                    points: oneDisplay(legacy), primary: true }] : [] };
    }
    var best = outputs[0];
    for (var i = 1; i < outputs.length; ++i) {
        var candidate = outputs[i];
        if (candidate.width * candidate.height > best.width * best.height ||
            (candidate.width * candidate.height === best.width * best.height &&
             Number(candidate.refresh) > Number(best.refresh)))
            best = candidate;
    }
    // Count each physical output. Additional screens earn resolution/refresh-aware
    // diminishing credit: even a 1080p secondary matters, but six old panels
    // cannot beat a powerful GPU. Beyond six displays the reward is capped.
    var rest = outputs.filter(function(o) { return o !== best; })
        .sort(function(a, b) { return oneDisplay(b) - oneDisplay(a); });
    var basePoints = oneDisplay(best);
    var screens = [{ name: best.name || "", width: best.width, height: best.height,
                     refresh: Number(best.refresh) || 0, points: basePoints, primary: true }];
    var extras = 0;
    for (var j = 0; j < rest.length; ++j) {
        var fraction = j === 0 ? 0.32 : j === 1 ? 0.20 : 0.12;
        var cap = j === 0 ? 4 : j === 1 ? 3 : 2;
        var available = Math.max(0, Math.min(8 - extras, 25 - basePoints - extras));
        var award = j < 5 ? Math.min(available, Math.max(1,
            Math.min(cap, Math.round(oneDisplay(rest[j]) * fraction)))) : 0;
        extras += award;
        screens.push({ name: rest[j].name || "", width: rest[j].width, height: rest[j].height,
                       refresh: Number(rest[j].refresh) || 0, points: award, primary: false });
    }
    return { points: Math.min(25, basePoints + extras),
             count: outputs.length, best: best, extras: extras, screens: screens };
}

function of(m) {
    m = m || {};
    var carried = typeof m.gpuClass === "number";
    var gpu = carried ? [0, 12, 19, 26, 34, 40][Math.max(0, Math.min(5, Math.floor(m.gpuClass)))] : 0;
    var names = carried ? [] : (Array.isArray(m.gpus) ? m.gpus : []);
    if (names.length)
        gpu = names.reduce(function(top, name) { return Math.max(top, gpuPerformance(name)); }, 0);
    else if (m.gpu && gpuPerformance(m.gpu) > 0 &&
             (!carried || gpuClass(m.gpu) === m.gpuClass))
        gpu = gpuPerformance(m.gpu);
    var beside = carried ? m.igpu === true : bothGraphics(names);
    var display = displayStats(m);
    var parts = [
        { id: "cpu", points: cpuPoints(m.threads, m.cpu) },
        { id: "ram", points: ramPoints(m.ramGb) },
        { id: "gpu", points: Math.min(44, gpu + (beside ? 1 : 0)) },
        { id: "display", points: display.points }
    ];
    var points = parts.reduce(function(sum, part) { return sum + part.points; }, 0);
    var level = 0;
    for (var i = 0; i < tiers.length; ++i)
        if (points >= tiers[i].min) level = i;
    var next = tiers[level + 1] || null;
    return { points: points, tier: tiers[level], level: level, next: next,
             toNext: next ? next.min - points : 0, parts: parts,
             displays: display };
}

function rarest(a, b) {
    return of(b).points - of(a).points;
}

// Battle HP is a separate legacy value owned by the TCG. Do not rebalance here.
function power(threads, ramGb, hasDiscrete) {
    return Math.max(30, Math.min(340, Math.round((threads * 8 + ramGb * 3 + (hasDiscrete ? 60 : 0)) / 10) * 10));
}

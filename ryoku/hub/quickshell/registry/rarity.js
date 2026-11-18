.pragma library

// Hardware rarity v2. The numbers are deliberately coarse performance bands,
// not model suffixes or prices. Unknown hardware receives conservative credit.
var tiers = [
    { id: "common", name: "Common", mark: "●", min: 0 },
    { id: "uncommon", name: "Uncommon", mark: "◆", min: 5 },
    { id: "rare", name: "Rare", mark: "★", min: 9 },
    { id: "holo", name: "Holo rare", mark: "★", min: 13 },
    { id: "ultra", name: "Ultra rare", mark: "★★", min: 18 },
    { id: "secret", name: "Secret rare", mark: "★★★", min: 23 }
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
            return 3;
        return /\b(680m|7\d0m|8\d0m|1[34]0v)\b/.test(s) ? 2 : 1;
    }

    var m = /\brtx\s*(\d{4})\b/.exec(s);
    if (m) {
        var n = Number(m[1]);
        var gen = Math.floor(n / 100);
        var model = n % 100;
        var rank = {
            20: [0, 0, 2, 3, 4],
            30: [0, 2, 3, 4, 5, 6],
            40: [0, 2, 3, 4, 6, 7],
            50: [0, 2, 4, 5, 7, 8]
        }[gen];
        if (!rank)
            return 0;
        var index = model >= 90 ? 5 : model >= 80 ? 4 : model >= 70 ? 3 : model >= 60 ? 2 : model >= 50 ? 1 : 0;
        var score = rank[index] || 0;
        if (/\b(laptop|max-q|mobile)\b/.test(s) && score >= 3)
            score--;
        return score;
    }

    m = /\bgtx\s*(\d{3,4})\b/.exec(s);
    if (m) {
        var gtx = Number(m[1]);
        if (gtx >= 1650 && gtx < 1700)
            return gtx >= 1660 ? 2 : 1;
        if (gtx >= 1000 && gtx < 1100)
            return gtx >= 1080 ? 3 : gtx >= 1070 ? 2 : 1;
        return 1;
    }

    m = /\brx\s*(\d{3,4})\b/.exec(s);
    if (m) {
        var rx = Number(m[1]);
        var family = Math.floor(rx / 1000);
        var segment = Math.floor((rx % 1000) / 100);
        var bands = { 5: [0, 1, 1, 1, 1, 1, 2, 2],
                      6: [0, 1, 1, 1, 1, 2, 2, 3, 4, 5],
                      7: [0, 1, 1, 1, 1, 2, 3, 4, 5, 6],
                      9: [0, 1, 1, 1, 1, 2, 4, 5, 6, 7] };
        if (family === 9 && rx >= 9060 && rx <= 9090)
            return rx >= 9070 ? (/\bxt\b/.test(s) ? 6 : 5) : 4;
        var familyRanks = bands[family];
        if (!familyRanks)
            return rx >= 400 && rx <= 600 ? 1 : 0;
        var value = familyRanks[segment] || 0;
        if (rx === 9070 && /\bxt\b/.test(s))
            value = 6;
        if (rx === 7900 && /\bxtx\b/.test(s))
            value = 7;
        if (/\b(laptop|mobile)\b/.test(s) && value >= 4)
            value--;
        return value;
    }

    m = /\barc\s*([ab])(\d{3})\b/.exec(s);
    if (m)
        return m[1] === "b" ? (Number(m[2]) >= 570 ? 3 : 2) : (Number(m[2]) >= 750 ? 3 : 2);
    if (/\brtx\s*pro|\ba[456]000\b|\bh[12]00\b|\bb200\b/.test(s))
        return 5;
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
        var h = Math.floor(Number(m[1]) % 1000 / 100);
        return h >= 9 ? 5 : h >= 8 ? 4 : h >= 7 ? 3 : h >= 6 ? 2 : 1;
    }
    if (/\barc\s*[ab]\d{3}/.test(s))
        return 2;
    return integrated(s) ? Math.min(3, gpuPerformance(s)) : 0;
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
    var base = step(threads, [4, 8, 12, 16, 24, 32]);
    var s = String(model || "").toLowerCase();
    var bonus = 0;
    var intel = /\b(?:core\s*)?i[3579][-\s]?(\d{4,5})/.exec(s);
    if (intel) {
        var digits = intel[1];
        var gen = digits.length === 5 ? Number(digits.slice(0, 2)) : Number(digits.slice(0, 1));
        bonus = gen >= 12 ? 2 : gen >= 8 ? 1 : 0;
    } else if (/\bcore\s*ultra\s*[3579]\b/.test(s)) {
        bonus = 2;
    } else {
        var amd = /\bryzen\s*(?:[3579]|threadripper)\s*(\d{4,5})/.exec(s);
        if (amd) {
            var series = Number(amd[1].charAt(0));
            bonus = series >= 7 ? 2 : series >= 5 ? 1 : 0;
        }
    }
    return Math.min(8, base + bonus);
}

function oneDisplay(output) {
    var width = Math.max(0, Number(output.width) || 0);
    var height = Math.max(0, Number(output.height) || 0);
    var refresh = Math.max(0, Number(output.refresh) || 0);
    var pixels = width * height;
    var resolution = step(pixels, [1920 * 1080, 2560 * 1440, 3840 * 2160]);
    return resolution + step(refresh, [120, 240]);
}

function displayStats(m) {
    var outputs = (m.displays || []).filter(function(output) {
        return output && !output.disabled && Number(output.width) > 0 && Number(output.height) > 0;
    });
    if (!outputs.length) {
        var width = Number(m.width) || 0, height = Number(m.height) || 0;
        if (!height && width)
            height = Math.round(width * 9 / 16);
        var legacy = { width: width, height: height, refresh: Number(m.refresh) || 0 };
        return { points: oneDisplay(legacy), count: width ? 1 : 0, best: legacy };
    }
    var best = outputs[0];
    for (var i = 1; i < outputs.length; i++)
        if (outputs[i].width * outputs[i].height > best.width * best.height ||
            (outputs[i].width * outputs[i].height === best.width * best.height && Number(outputs[i].refresh) > Number(best.refresh)))
            best = outputs[i];
    // Extra monitors add modest credit; a screen-count arms race cannot dictate rarity.
    return { points: Math.min(6, oneDisplay(best) + Math.min(2, outputs.length - 1)),
             count: outputs.length, best: best };
}

function of(m) {
    m = m || {};
    var carried = typeof m.gpuClass === "number";
    var gpu = carried ? Math.max(0, Math.min(5, m.gpuClass)) : 0;
    var names = carried ? [] : m.gpus || [];
    if (names.length)
        gpu = names.reduce(function(top, name) { return Math.max(top, gpuPerformance(name)); }, 0);
    else if (m.gpu && gpuPerformance(m.gpu) > 0 && (!carried || gpuClass(m.gpu) === m.gpuClass))
        gpu = gpuPerformance(m.gpu);
    var beside = typeof m.gpuClass === "number" ? m.igpu === true : bothGraphics(names);
    var display = displayStats(m);
    var parts = [
        { id: "cpu", points: cpuPoints(m.threads, m.cpu) },
        { id: "ram", points: step(m.ramGb, [8, 16, 32, 64]) },
        { id: "gpu", points: Math.min(8, gpu + (beside ? 1 : 0)) },
        { id: "display", points: display.points }
    ];
    var points = parts.reduce(function(total, part) { return total + part.points; }, 0);
    var level = 0;
    for (var i = 0; i < tiers.length; i++)
        if (points >= tiers[i].min)
            level = i;
    var next = tiers[level + 1] || null;
    return { points: points, tier: tiers[level], level: level, next: next,
             toNext: next ? next.min - points : 0, parts: parts };
}

function rarest(a, b) {
    return of(b).points - of(a).points;
}

// Battle HP is a separate legacy value owned by the TCG. Do not rebalance here.
function power(threads, ramGb, hasDiscrete) {
    return Math.max(30, Math.min(340, Math.round((threads * 8 + ramGb * 3 + (hasDiscrete ? 60 : 0)) / 10) * 10));
}

.pragma library

// how rare a system card is, from the machine it shows: points for the processor's threads,
// the memory, the strongest graphics card and the display, and a tier as a trading card's
// rarity. A faster machine draws a shinier card, from plain Common to the gilded Secret rare

var tiers = [
    { id: "common", name: "Common", mark: "●", min: 0 },
    { id: "uncommon", name: "Uncommon", mark: "◆", min: 6 },
    { id: "rare", name: "Rare", mark: "★", min: 9 },
    { id: "holo", name: "Holo rare", mark: "★", min: 12 },
    { id: "ultra", name: "Ultra rare", mark: "★★", min: 15 },
    { id: "secret", name: "Secret rare", mark: "★★★", min: 18 }
];

function step(v, bounds) {
    var n = 0;
    for (var i = 0; i < bounds.length; i++)
        if (v >= bounds[i])
            n = i + 1;
    return n;
}

// graphics built into the processor, from its full name or the eight-character one a card
// carries ("Vega Ser", "UHD Grap", "Phoenix1"): AMD names them by their chip, Intel by its series
function integrated(name) {
    var s = String(name || "").toLowerCase();
    return /\b(uhd|iris|vega|hd graph|xe graph|arc grap|integrated)|^graphics|radeon graphics|phoenix|rembrandt|raphael|cezanne|renoir|lucienne|barcelo|mendocin|van gogh|hawk poi|strix|krackan|picasso|\b[6-9]\d0m\b|\b80[56]0s\b|\barc 1[34]0v\b/.test(s);
}

// a graphics card's class from its name: 1 for graphics built into the processor (2 for the
// strong ones, 3 for Strix Halo's), 1 to 5 for a discrete card from entry to flagship, 0 unknown
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
    if (!integrated(s))
        return 0;
    if (/\b80[56]0s\b/.test(s))
        return 3;
    return /\b(680m|7\d0m|8\d0m|1[34]0v)\b/.test(s) ? 2 : 1;
}

// the card that counts among several: the highest class, a discrete one when they tie
function strongest(names) {
    var best = "";
    for (var i = 0; i < names.length; i++) {
        var n = String(names[i] || "");
        var c = gpuClass(n), b = gpuClass(best);
        if (n !== "" && (best === "" || c > b || (c === b && integrated(best) && !integrated(n))))
            best = n;
    }
    return best;
}

// a discrete card earns power on top of its rarity points; built-in graphics only the points.
// cls is the class a card carries, name its graphics card's (short) name
function discrete(cls, name) {
    return cls > 0 && !integrated(name);
}

// built-in graphics beside a discrete card, which earn one point on top of the card's class
function bothGraphics(names) {
    var built = false, card = false;
    for (var i = 0; i < names.length; i++) {
        var n = String(names[i] || "");
        if (integrated(n))
            built = true;
        else if (gpuClass(n) > 0)
            card = true;
    }
    return built && card;
}

// m: { threads, ramGb, gpuClass and igpu (as a card carries them) or gpus: [names], refresh, width }
// -> { points, tier, next, toNext, parts: [{ id, points }] }
function of(m) {
    var carried = typeof m.gpuClass === "number";
    var best = carried ? m.gpuClass : (m.gpus || []).reduce((top, g) => Math.max(top, gpuClass(g)), 0);
    var beside = carried ? m.igpu === true : bothGraphics(m.gpus || []);
    var parts = [
        { id: "cpu", points: step(m.threads || 0, [4, 8, 12, 16, 24, 32]) },
        { id: "ram", points: step(m.ramGb || 0, [8, 16, 32, 64, 128]) },
        { id: "gpu", points: best + (beside ? 1 : 0) },
        { id: "display", points: step(m.refresh || 0, [144, 240]) + step(m.width || 0, [2560, 3840]) }
    ];
    var points = parts.reduce((a, p) => a + p.points, 0);
    var at = 0;
    for (var i = 0; i < tiers.length; i++)
        if (points >= tiers[i].min)
            at = i;
    var next = at + 1 < tiers.length ? tiers[at + 1] : null;
    return { points: points, tier: tiers[at], level: at, next: next, toNext: next ? next.min - points : 0, parts: parts };
}

// sort order for machines, the rarest first: negative when a outranks b, 0 when they tie
function rarest(a, b) {
    return of(b).points - of(a).points;
}

// a card's HP: threads, memory and a discrete graphics card, in tens between 30 and 340
function power(threads, ramGb, discrete) {
    return Math.max(30, Math.min(340, Math.round((threads * 8 + ramGb * 3 + (discrete ? 60 : 0)) / 10) * 10));
}

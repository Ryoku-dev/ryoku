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

// a graphics card's class from its name: 0 built in, 1 entry, up to 5 for a flagship
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
    return 0;
}

// m: { threads, ramGb, gpus: [names], refresh, width }
// -> { points, tier, next, toNext, parts: [{ id, points }] }
function of(m) {
    var parts = [
        { id: "cpu", points: step(m.threads || 0, [4, 8, 12, 16, 24, 32]) },
        { id: "ram", points: step(m.ramGb || 0, [8, 16, 32, 64, 128]) },
        { id: "gpu", points: (m.gpus || []).reduce((best, g) => Math.max(best, gpuClass(g)), 0) },
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

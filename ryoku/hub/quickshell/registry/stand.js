.pragma library

// the Stand card style's reading of a machine: its six Stand parameters in the order the
// JoJo volumes print them, each graded A (超スゴイ) to E (超ニガテ) from the machine's own
// numbers, on the steps registry/rarity.js already scores. A grade is the highest letter
// whose floor the value reaches; below the D floor it is E

var letters = "EDCBA";
var grades = [
    { key: "A", ja: "超スゴイ" },
    { key: "B", ja: "スゴイ" },
    { key: "C", ja: "人間と同じ" },
    { key: "D", ja: "ニガテ" },
    { key: "E", ja: "超ニガテ" }
];
// floors for D, C, B and A
var params = [
    { id: "power", ja: "破壊力", en: "Destructive power", from: "GPU class", floors: [2, 3, 4, 5] },
    { id: "speed", ja: "スピード", en: "Speed", from: "threads", floors: [4, 8, 16, 32] },
    { id: "range", ja: "射程距離", en: "Range", from: "display width", floors: [1280, 1920, 2560, 3840] },
    { id: "durability", ja: "持続力", en: "Durability", from: "GB", floors: [8, 16, 32, 64] },
    { id: "precision", ja: "精密動作性", en: "Precision", from: "Hz", floors: [60, 90, 144, 240] },
    // the rarity tier's floors, Uncommon to Ultra rare
    { id: "potential", ja: "成長性", en: "Potential", from: "rarity points", floors: [6, 9, 12, 15] }
];

// 1 (E) to 5 (A)
function grade(value, floors) {
    var n = 1;
    for (var i = 0; i < floors.length; i++)
        if (value >= floors[i])
            n = i + 2;
    return n;
}
function letter(n) {
    return letters.charAt(Math.max(1, Math.min(5, n)) - 1);
}

// m: { gpuClass, threads, width, ramGb, refresh, points } -> six grades, 1 (E) to 5 (A)
function of(m) {
    var v = [m.gpuClass, m.threads, m.width, m.ramGb, m.refresh, m.points];
    return params.map((p, i) => grade(Number(v[i]) || 0, p.floors));
}

var stockStand = "POWER & BEAUTY";
var stop = ["of", "the", "for", "a", "an", "to", "in", "on", "at", "by", "with", "my", "our", "&"];

// a Stand is named after a phrase: the motto's last clause, cut to its last words that fit,
// with "and" set as "&". The stock motto names the stock Stand in any language
function standName(tagline, stockTagline) {
    var s = String(tagline || "").trim();
    if (s === "" || s === String(stockTagline || ""))
        return stockStand;
    var clauses = s.split(/[,;:!?.。、！？]+/).map(c => c.trim()).filter(c => c.length > 0);
    var words = (clauses.length ? clauses[clauses.length - 1] : s).replace(/\band\b/gi, "&").split(/\s+/).filter(w => w.length > 0);
    if (words.length === 1)
        return Array.from(words[0]).slice(-20).join("").toUpperCase();
    var kept = [];
    for (var i = words.length - 1; i >= 0; i--) {
        if ([words[i]].concat(kept).join(" ").length > 20)
            break;
        kept.unshift(words[i]);
    }
    while (kept.length > 1 && stop.indexOf(kept[0].toLowerCase()) >= 0)
        kept.shift();
    if (kept.length === 0)
        kept = [Array.from(words[words.length - 1]).slice(0, 20).join("")];
    return kept.join(" ").toUpperCase();
}

// the English title: "NERO'S BIZARRE MACHINE"
function title(name) {
    var n = String(name || "").trim().toUpperCase();
    return n === "" ? "A BIZARRE MACHINE" : n + (/S$/.test(n) ? "'" : "'S") + " BIZARRE MACHINE";
}

// the part line: a release's major version is its part, Amaterasu's 1 and Hyperion's 2
function part(version) {
    var m = /^v?(\d+)/.exec(String(version || ""));
    return m && Number(m[1]) > 0 ? Number(m[1]) : 1;
}

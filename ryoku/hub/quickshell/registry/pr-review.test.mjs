import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const hub = path.dirname(dir);
const read = file => fs.readFileSync(path.join(hub, file), "utf8");
const load = file => {
    const ctx = vm.createContext({});
    vm.runInContext(read(file).replace(/^(?:\.pragma|\.import) .*$/gm, ""), ctx);
    return ctx;
};
const P = load("registry/progression.js");
const R = load("registry/rarity.js");
const K = load("KeepsakeSequence.js");

test("empty keys reset the keepsake sequence without granting a badge", () => {
    assert.deepEqual(JSON.parse(JSON.stringify(K.step(3, 1000, "", 1500, false, false))),
        { index: 0, at: 0, complete: false });
    let current = { index: 0, at: 0 };
    for (const [i, key] of [...K.CODE].entries()) {
        const result = K.step(current.index, current.at, key, 1000 + i * 100, false, false);
        current = result;
    }
    assert.equal(current.complete, true);
});

test("mobile RX GPU names are recognised with lower scores than their desktop equivalents", () => {
    for (const name of ["Radeon RX 7600M XT", "Radeon RX 7600M", "AMD Radeon RX 7800M XT"]) {
        assert.ok(R.gpuPerformance(name) > 0, name);
    }
    assert.ok(R.gpuPerformance("Radeon RX 7600M XT") <
        R.gpuPerformance("Radeon RX 7600 XT"));
    assert.equal(R.gpuPerformance("Unknown Radeon"), 0);
});

test("Core i3 generations already score differently and need no regex change", () => {
    assert.ok(R.cpuPoints(8, "Intel Core i3-14100") > R.cpuPoints(8, "Intel Core i3-4130"));
    assert.ok(R.cpuPoints(8, "Core i3 14100F") > R.cpuPoints(8, "Core i3 4130"));
});

test("ISO timestamps keep their local calendar dates in non-UTC timezones", () => {
    const code = [
        'const fs = require("node:fs"), vm = require("node:vm");',
        'const c = vm.createContext({});',
        'vm.runInContext(fs.readFileSync(process.argv[1], "utf8").replace(/^\\.(pragma|import).*$/gm, ""), c);',
        'let s = c.startDay({}, process.argv[2]).state;',
        's = c.startDay(s, process.argv[3]).state;',
        'process.stdout.write(JSON.stringify(Object.keys(s.days)));'
    ].join("\n");
    const cases = [
        { tz: "America/Los_Angeles", times: ["2026-11-19T08:30:00.000Z", "2026-11-20T07:30:00.000Z"], expected: ["2026-11-19"] },
        { tz: "Asia/Tokyo", times: ["2026-11-19T14:30:00.000Z", "2026-11-19T15:30:00.000Z"], expected: ["2026-11-19", "2026-11-20"] }
    ];
    for (const {tz, times, expected} of cases) {
        const result = spawnSync(process.execPath, ["-e", code, path.join(dir, "progression.js"), ...times],
            { env: {...process.env, TZ:tz}, encoding:"utf8" });
        assert.equal(result.status, 0, result.stderr);
        assert.deepEqual(JSON.parse(result.stdout), expected, tz);
    }
});

test("unreadable progression files cannot be treated as fresh saves", () => {
    const s = read("Singletons/Progression.qml");
    assert.match(s, /onLoadFailed: \(loadError\) => \{/);
    assert.match(s, /!root\.ready && loadError === FileViewError\.FileNotFound/);
    assert.match(s, /root\.error = "Could not load progression data\./);
    assert.match(s, /function save\(state\) \{\s*if \(!root\.ready \|\| root\.error !== ""\)/);
});

test("startup visits retain their original timestamps and replay after load", () => {
    const s = read("Singletons/Progression.qml");
    assert.match(s, /property var pendingVisits: \[\]/);
    assert.match(s, /root\.pendingVisits = root\.pendingVisits\.concat/);
    assert.match(s, /root\.visitedPage\(visits\[j\]\.page, visits\[j\]\.category, visits\[j\]\.at\)/);
    assert.match(s, /Model\.visit\(root\.data, page, category, when\)/);
});

test("the archive reveal uses runtime i18n, not hard-coded English headings", () => {
    const s = read("components/about/AchievementReveal.qml");
    for (const phrase of ["CLASSIFIED / DECRYPTED", "MASTERY / ACHIEVED", "THE ARCHIVE REMEMBERS",
        "ACHIEVEMENT UNLOCKED", "Return to archive",
        "Collected. Your badge is now available to feature or place on your profile."]) {
        assert.ok(s.includes('I18n.tr("' + phrase + '")'), phrase);
    }
});

test("decoding stays bounded and shop purchases work on a full canvas", () => {
    const seal = read("components/about/ArchiveSeal.qml");
    const shop = read("components/about/ProfileStudio.qml");
    assert.match(seal, /sourceSize\.width: Math\.min\(1024,/);
    assert.match(seal, /sourceSize\.height: Math\.min\(1024,/);
    assert.match(shop, /sourceSize\.width: Math\.max\(1, Math\.round\(width \* 2\)\)/);
    assert.match(shop, /sourceSize\.height: Math\.max\(1, Math\.round\(height \* 2\)\)/);
    assert.match(shop, /enabled: stickerTile\.owned \? Progression\.canvas\.stickers\.length < 24/);
    let state = P.clean({ version: 4, earned: { "ryoku-thirty": true }, claimed: { "ryoku-thirty": true } });
    state = P.claim(P.unlock(state, "welcome").state, "welcome").state;
    for (let i = 0; i < 24; i++) state = P.addSticker(state, "welcome").state;
    assert.equal(state.canvas.stickers.length, 24);
    assert.equal(P.addSticker(state, "welcome").changed, false);
    const item = P.cosmetics.find(c => c.type === "sticker" && !state.owned[c.id]);
    assert.ok(P.purchase(state, item.id).changed, "purchase should work while canvas is full");
});

test("Home surfaces reuse selected Lithic theme roles", () => {
    const s = read("components/Home.qml");
    assert.match(s, /readonly property color surfaceAccent/);
    assert.match(s, /Lithic\.mix\(Lithic\.ground, profileBackground\.surfaceAccent/);
    assert.doesNotMatch(s, /profileBackground\.chosen === "bg-blueprint" \? "#/);
});

test("Hub changelog records the new feature scope", () => {
    const s = fs.readFileSync(path.join(hub, "../CHANGELOG.md"), "utf8");
    assert.match(s, /Hyperion adds Ryoku-wide achievements and a cosmetic Points Shop/);
});

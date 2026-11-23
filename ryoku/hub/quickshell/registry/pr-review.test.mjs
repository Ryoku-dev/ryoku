import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const hub = path.dirname(dir);
const read = file => fs.readFileSync(path.join(hub, file), "utf8");
const load = file => {
    const ctx = vm.createContext({});
    vm.runInContext(read(file).replace(/^(?:\.pragma|\.import) .*$/gm, ""), ctx);
    return ctx;
};
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

test("display contributions already sum to the rounded capped total", () => {
    const R = load("registry/rarity.js");
    for (const count of [1, 2, 3, 5, 6, 8]) {
        const displays = Array.from({ length: count }, (_, i) => ({
            name: "monitor-" + i,
            width: i % 3 ? 1920 : 2560,
            height: i % 3 ? 1080 : 1440,
            refresh: i % 2 ? 60 : 165
        }));
        const scored = R.displayStats({ displays });
        assert.equal(scored.screens.reduce((sum, screen) => sum + screen.points, 0),
            scored.points, String(count) + " displays");
        assert.equal(scored.screens.filter(s => !s.primary)
            .reduce((sum, screen) => sum + screen.points, 0),
            scored.extras);
    }
});

import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const R = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "rarity.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), R);

test("GPU generations change performance without changing shared GPU classes", () => {
    assert.equal(R.gpuClass("GeForce RTX 3060"), R.gpuClass("GeForce RTX 5060"));
    assert.ok(R.gpuPerformance("GeForce RTX 5060") > R.gpuPerformance("GeForce RTX 3060"));
    assert.ok(R.gpuPerformance("GeForce RTX 5090") > R.gpuPerformance("GeForce RTX 4090"));
    assert.ok(R.gpuPerformance("Radeon RX 9070 XT") > R.gpuPerformance("Radeon RX 7600"));
    assert.equal(R.gpuPerformance("unrecognised controller"), 0);
});

test("CPU model generation improves score without rewarding thread count alone", () => {
    assert.ok(R.cpuPoints(8, "Intel Core i3-14100") > R.cpuPoints(8, "Intel Core i3-4130"));
    assert.ok(R.cpuPoints(12, "AMD Ryzen 5 7600X") > R.cpuPoints(12, "AMD Ryzen 5 1600"));
    assert.ok(R.cpuPoints(32, "not-known") >= R.cpuPoints(4, "not-known"));
});

test("two monitors prefer highest physical resolution regardless of focus and order", () => {
    const hi = { width: 2560, height: 1440, refresh: 200, focused: false };
    const lo = { width: 1920, height: 1080, refresh: 100, focused: true };
    for (const displays of [[lo, hi], [hi, lo]]) {
        const result = R.displayStats({ displays });
        assert.equal(result.count, 2);
        assert.equal(result.best.width, 2560);
        assert.equal(result.best.height, 1440);
        assert.equal(result.points, 4);
    }
    assert.equal(R.displayStats({ displays: [hi] }).points, 3);
});

test("disabled displays are excluded and duplicate screen counts have limited impact", () => {
    const hi = { width: 2560, height: 1440, refresh: 144 };
    const disabled = { width: 3840, height: 2160, refresh: 240, disabled: true };
    const only = R.displayStats({ displays: [hi, disabled] });
    assert.equal(only.best.width, 2560);
    assert.equal(only.count, 1);
    const many = R.displayStats({ displays: Array(20).fill(hi) });
    assert.ok(many.points <= 6);
});

test("rarity is deterministic and does not influence battle HP", () => {
    const m = { threads: 16, cpu: "AMD Ryzen 5 5600X", ramGb: 32,
        gpus: ["GeForce RTX 3060"], displays: [{ width: 2560, height: 1440, refresh: 200 }] };
    assert.deepEqual(JSON.parse(JSON.stringify(R.of(m))), JSON.parse(JSON.stringify(R.of(m))));
    assert.equal(R.power(16, 32, true), 280);
});

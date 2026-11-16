import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

// load rarity.js the way the QML engine links it
const here = path.dirname(fileURLToPath(import.meta.url));
const R = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(here, "rarity.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), R);

test("graphics cards rank by their class, laptops and built-in ones included", () => {
    assert.equal(R.gpuClass("GeForce RTX 4090"), 5);
    assert.equal(R.gpuClass("GeForce RTX 4060 Max-Q"), 2);
    assert.equal(R.gpuClass("GeForce GTX 1660 Ti"), 1);
    assert.equal(R.gpuClass("Radeon RX 7900 XTX"), 5);
    assert.equal(R.gpuClass("Radeon RX 6500 XT"), 1);
    assert.equal(R.gpuClass("Phoenix1"), 0);
    assert.equal(R.gpuClass("UHD Graphics 620"), 0);
});

test("the strongest of several graphics cards counts", () => {
    const r = R.of({ threads: 0, ramGb: 0, gpus: ["Phoenix1", "GeForce RTX 4080"], refresh: 0, width: 0 });
    assert.equal(r.parts.find(p => p.id === "gpu").points, 4);
});

test("a traded graphics class takes precedence over names, including class zero", () => {
    const machine = { threads: 16, ramGb: 32, refresh: 144, width: 2560 };
    assert.equal(R.of({ ...machine, gpuClass: 3 }).points, R.of({ ...machine, gpus: ["RTX 4070"] }).points);
    assert.equal(R.of({ ...machine, gpuClass: 0, gpus: ["RTX 4090"] }).parts.find(p => p.id === "gpu").points, 0);
});

test("tiers climb with the machine, from a thin laptop to a workstation", () => {
    const thin = R.of({ threads: 4, ramGb: 8, gpus: ["UHD Graphics 620"], refresh: 60, width: 1920 });
    const gamer = R.of({ threads: 16, ramGb: 46, gpus: ["GeForce RTX 4060 Max-Q", "Phoenix1"], refresh: 165, width: 2560 });
    const beast = R.of({ threads: 32, ramGb: 128, gpus: ["GeForce RTX 5090"], refresh: 240, width: 3840 });
    assert.equal(thin.tier.id, "common");
    assert.equal(gamer.tier.id, "rare");
    assert.equal(gamer.toNext, 1);
    assert.equal(gamer.next.id, "holo");
    assert.equal(beast.tier.id, "secret");
    assert.equal(beast.next, null);
});

test("a tier starts exactly at its minimum", () => {
    // 16 threads (4) + 32 GB (3) + an RTX 4070 (3) + 144 Hz at 2560 (2) = 12
    const r = R.of({ threads: 16, ramGb: 32, gpus: ["RTX 4070"], refresh: 144, width: 2560 });
    assert.equal(r.points, 12);
    assert.equal(r.tier.id, "holo");
});

test("power counts in tens and stays between 30 and 340", () => {
    assert.equal(R.power(16, 46, true), 330);
    assert.equal(R.power(16, 46, false), 270);
    assert.equal(R.power(0, 0, false), 30);
    assert.equal(R.power(64, 256, true), 340);
});

test("a binder sorted by rarity puts the rarest machine first and keeps ties in order", () => {
    const thin = { name: "thin", threads: 4, ramGb: 8, gpuClass: 0, refresh: 60, width: 1920 };
    const gamer = { name: "gamer", threads: 16, ramGb: 32, gpuClass: 3, refresh: 144, width: 2560 };
    const twin = { ...gamer, name: "twin" };
    const beast = { name: "beast", threads: 32, ramGb: 128, gpuClass: 5, refresh: 240, width: 3840 };
    const sorted = [thin, gamer, beast, twin].sort(R.rarest).map(m => m.name);
    assert.deepEqual(sorted, ["beast", "gamer", "twin", "thin"]);
});

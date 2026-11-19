import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const R = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(here, "rarity.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), R);

const screen = (width, height, refresh = 60, name = "") => ({ width, height, refresh, name });
const build = (cpu, threads, gpu, ramGb, displays) => ({
    cpu, threads, gpus: [gpu], ramGb, displays
});
const reference = build("AMD Ryzen 7 5700X", 16, "NVIDIA GeForce RTX 3060", 16,
    [screen(1920, 1080)]);

test("all nine card rarities are ordered and the Steam reference lands in Epic", () => {
    assert.deepEqual([...R.tiers.map(t => t.id)], [
        "common", "uncommon", "rare", "ultra-rare", "epic",
        "legendary", "mythical", "holographic", "master"
    ]);
    for (let i = 1; i < R.tiers.length; ++i)
        assert.ok(R.tiers[i].min > R.tiers[i - 1].min);
    const result = R.of(reference);
    assert.equal(result.points, 52);
    assert.equal(result.tier.id, "epic");
    assert.equal(result.level, 4);
    assert.equal(result.parts.reduce((sum, part) => sum + part.points, 0), result.points);
    assert.equal(result.next.id, "legendary");
    assert.equal(R.of({ ...reference, gpus: [R.baseline.gpu] }).points, result.points);
});

test("common low-end machines do not get inflated to typical gaming rarity", () => {
    const thin = build("Intel Core i3-4130", 4, "Intel UHD Graphics 620", 8,
        [screen(1920, 1080)]);
    const result = R.of(thin);
    assert.equal(result.tier.id, "common");
    assert.ok(result.points < R.tiers[1].min);
});

test("GPU generation, desktop variants and laptops earn distinct credit", () => {
    const score = R.gpuPerformance;
    assert.ok(score("GeForce RTX 5060") > score("GeForce RTX 3060"));
    assert.ok(score("GeForce RTX 3060 Ti") > score("GeForce RTX 3060"));
    assert.ok(score("GeForce RTX 4070 SUPER") > score("GeForce RTX 4070"));
    assert.ok(score("GeForce RTX 4070 Ti SUPER") > score("GeForce RTX 4070 SUPER"));
    assert.ok(score("GeForce RTX 5090") > score("GeForce RTX 4090"));
    assert.ok(score("GeForce RTX 4060 Laptop GPU") < score("GeForce RTX 4060"));
    assert.ok(score("Radeon RX 9070 XT") > score("Radeon RX 9070"));
    assert.ok(score("Radeon RX 7900 XTX") > score("Radeon RX 7600"));
    assert.ok(score("Radeon RX 7600M XT") < score("Radeon RX 7600 XT"));
    assert.ok(score("Intel Arc B580") > score("Intel Arc A380"));
    assert.equal(score("Unidentified Adapter"), 0);
    assert.equal(score(""), 0);
});

test("CPU generations and threads count without treating old i3 and modern i3 as peers", () => {
    assert.ok(R.cpuPoints(8, "Intel Core i3-14100") > R.cpuPoints(8, "Intel Core i3-4130"));
    assert.ok(R.cpuPoints(16, "Intel Core i7-10700K") > R.cpuPoints(16, "Intel Core i7-8700K"));
    assert.ok(R.cpuPoints(16, "Intel Core i7-14700K") > R.cpuPoints(16, "Intel Core i7-10700K"));
    assert.ok(R.cpuPoints(12, "AMD Ryzen 5 9600X") > R.cpuPoints(12, "AMD Ryzen 5 3600X"));
    assert.ok(R.cpuPoints(16, "AMD Ryzen 7 5700X") > R.cpuPoints(12, "AMD Ryzen 5 5600X"));
    assert.ok(R.cpuPoints(24, "Intel Core Ultra 7 265K") > R.cpuPoints(24, "Intel Core i7-10700K"));
    assert.equal(R.cpuPoints(0, ""), 0);
    assert.ok(R.cpuPoints(64, "Ryzen 9 9950X") <= 30);
});

test("RAM is tiered, bounded and its contribution is less than graphics or CPU", () => {
    for (const [a, b] of [[0, 8], [8, 16], [16, 32], [32, 64]])
        assert.ok(R.ramPoints(b) > R.ramPoints(a));
    assert.equal(R.ramPoints(128), R.ramPoints(2048));
    assert.ok(R.ramPoints(128) < R.gpuPerformance("RTX 5090"));
    assert.ok(R.ramPoints(-7) === 0);
});

test("all connected displays contribute, not just the chosen primary screen", () => {
    const one = R.displayStats({ displays: [screen(2560, 1440, 165, "DP-1")] });
    const two = R.displayStats({ displays: [
        screen(2560, 1440, 165, "DP-1"), screen(1920, 1080, 100, "HDMI-1")
    ] });
    const better = R.displayStats({ displays: [
        screen(2560, 1440, 165, "DP-1"), screen(3840, 2160, 144, "HDMI-1")
    ] });
    assert.equal(one.count, 1);
    assert.equal(two.count, 2);
    assert.equal(two.best.name, "DP-1");
    assert.ok(two.extras > 0);
    assert.ok(two.points > one.points);
    assert.ok(better.points > two.points);
    assert.equal(better.best.name, "HDMI-1");
});

test("all first six displays receive diminishing credit and duplicates are ignored", () => {
    const screens = Array.from({ length: 8 }, (_, i) => screen(1920, 1080, 60, "MON-" + i));
    let last = R.displayStats({ displays: screens.slice(0, 1) }).points;
    for (let count = 2; count <= 6; count++) {
        const stat = R.displayStats({ displays: screens.slice(0, count) });
        assert.ok(stat.points > last, "Display " + count + " earns points");
        last = stat.points;
    }
    const all = R.displayStats({ displays: screens });
    assert.equal(all.count, 8);
    assert.equal(all.points, last);
    assert.ok(all.extras <= 8);
    assert.ok(all.points <= 25);
    assert.equal(R.displayStats({ displays: [screens[0], screens[0]] }).count, 1);
    assert.equal(R.displayStats({ displays: [screens[0], { ...screens[1], disabled: true }] }).count, 1);
});

test("4K, refresh and rotation are scored by actual pixels and Hz", () => {
    assert.ok(R.oneDisplay(screen(3840, 2160, 60)) > R.oneDisplay(screen(2560, 1440, 60)));
    assert.ok(R.oneDisplay(screen(2560, 1440, 240)) > R.oneDisplay(screen(2560, 1440, 60)));
    assert.equal(R.oneDisplay(screen(1080, 1920)), R.oneDisplay(screen(1920, 1080)));
    assert.equal(R.oneDisplay(screen(2560, 1440, 165000)), R.oneDisplay(screen(2560, 1440, 165)));
    assert.equal(R.oneDisplay(screen(0, 0)), 0);
    assert.equal(R.displayStats({ displays: [] }).count, 0);
});

test("better GPUs, CPUs, RAM and screens cannot reduce score", () => {
    const baseline = R.of(reference);
    const upgrades = [
        { ...reference, gpus: ["RTX 5060"] },
        { ...reference, cpu: "Ryzen 7 9700X" },
        { ...reference, ramGb: 32 },
        { ...reference, displays: [screen(2560, 1440, 144)] },
        { ...reference, displays: [...reference.displays, screen(1920, 1080, 60)] }
    ];
    for (const candidate of upgrades)
        assert.ok(R.of(candidate).points > baseline.points);
    const stronger = R.of(build("Ryzen 9 9950X", 32, "RTX 5090", 128,
        [screen(3840, 2160, 240)]));
    assert.equal(stronger.tier.id, "master");
    assert.equal(stronger.next, null);
});

test("legacy signed GPU class stays 0..5 and takes precedence over mismatched names", () => {
    assert.equal(R.gpuClass("GeForce RTX 4090"), 5);
    assert.equal(R.gpuClass("GeForce RTX 4060 Max-Q"), 2);
    assert.equal(R.gpuClass("GeForce GTX 1660 Ti"), 1);
    assert.equal(R.gpuClass("Radeon RX 7900 XTX"), 5);
    assert.equal(R.gpuClass("Radeon RX 9070 XT"), 4);
    assert.equal(R.gpuClass("Radeon RX 9060 XT"), 2);
    assert.equal(R.gpuClass("Unidentified Adapter"), 0);

    const signed = { ...reference, gpuClass: 0, gpu: "RTX 4090" };
    assert.equal(R.of(signed).parts.find(part => part.id === "gpu").points, 0);
    const correct = { ...reference, gpuClass: 2, gpu: "RTX 3060" };
    assert.equal(R.of(correct).parts.find(part => part.id === "gpu").points,
        R.gpuPerformance("RTX 3060"));
    const legacy = { ...reference, gpuClass: 2, gpu: "" };
    assert.ok(R.of(legacy).points > 0);
    assert.ok(R.of({ ...reference, gpuClass: 3, gpu: "" }).points >
        R.of({ ...reference, gpuClass: 2, gpu: "" }).points);
});

test("built-in GPU and discrete GPU detection remain compatible with traded cards", () => {
    for (const name of ["Phoenix1", "UHD Graphics 620", "Iris Xe Graphics",
        "Vega Ser", "UHD Grap", "Iris Xe ", "Arc Grap"])
        assert.equal(R.gpuClass(name), 1, name);
    for (const name of ["Radeon 780M", "780M", "880M / 8", "Radeon 680M",
        "Intel Arc Graphics 140V"])
        assert.equal(R.gpuClass(name), 2, name);
    assert.equal(R.gpuClass("Radeon 8060S"), 3);

    assert.equal(R.discrete(1, "Vega Ser"), false);
    assert.equal(R.discrete(1, "GTX 1650"), true);
    assert.equal(R.strongest(["UHD Graphics 630", "RTX 3060"]), "RTX 3060");
    assert.equal(R.of({ gpuClass: 4, igpu: true }).parts.find(p => p.id === "gpu").points,
        R.of({ gpuClass: 4, igpu: false }).parts.find(p => p.id === "gpu").points + 1);
});

test("shared card ordering stays stable and battle HP is untouched", () => {
    const weak = { ...reference, gpuClass: 0, gpu: "" };
    const mid = { ...reference, gpuClass: 2, gpu: "" };
    const great = { ...reference, gpuClass: 5, gpu: "" };
    const same = { ...mid };
    const ordered = [mid, weak, great, same].sort(R.rarest);
    assert.equal(ordered[0], great);
    assert.equal(ordered[1], mid);
    assert.equal(ordered[2], same);
    assert.equal(ordered[3], weak);
    assert.equal(R.power(16, 46, true), 330);
    assert.equal(R.power(16, 46, false), 270);
    assert.equal(R.power(0, 0, false), 30);
    assert.equal(R.power(64, 256, true), 340);
});

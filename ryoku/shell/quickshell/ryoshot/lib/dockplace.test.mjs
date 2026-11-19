import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { dock, popover } = require("./dockplace.js");

const area = { w: 1920, h: 1200 };
const size = { w: 480, h: 84 };

test("a mid-size selection takes the dock 12 below, centred", () => {
    const p = dock({ x: 600, y: 300, w: 700, h: 400 }, size, area, { align: "center" });
    assert.equal(p.side, "below");
    assert.equal(p.y, 712);
    assert.equal(p.x, 600 + 350 - 240);
});

test("lithic hangs the dock from the selection's left edge", () => {
    const p = dock({ x: 600, y: 300, w: 700, h: 400 }, size, area, { align: "left" });
    assert.equal(p.x, 600);
});

test("a selection near the bottom puts the dock above it", () => {
    const p = dock({ x: 600, y: 700, w: 700, h: 450 }, size, area, {});
    assert.equal(p.side, "above");
    assert.equal(p.y, 700 - 12 - 84);
});

test("a wider gap above clears what the selection draws over its top edge", () => {
    const p = dock({ x: 600, y: 700, w: 700, h: 450 }, size, area, { gapAbove: 28 });
    assert.equal(p.side, "above");
    assert.equal(p.y, 700 - 28 - 84);
});

test("a full-screen selection keeps the dock inside its bottom edge", () => {
    const p = dock({ x: 0, y: 0, w: 1920, h: 1200 }, size, area, {});
    assert.equal(p.side, "inside");
    assert.equal(p.y, 1200 - 12 - 84);
});

test("the dock stays 8 inside the output whatever the drag", () => {
    const p = dock({ x: 10, y: 300, w: 100, h: 100 }, size, area, { align: "left", dx: -400, dy: 5000 });
    assert.equal(p.x, 8);
    assert.equal(p.y, 1200 - 84 - 8);
});

test("a popover opens away from the selection when it fits", () => {
    const p = popover({ x: 700, w: 28 }, { top: 712, bottom: 796 }, { w: 200, h: 220 }, area, "down", {});
    assert.equal(p.side, "down");
    assert.equal(p.y, 802);
    assert.equal(p.x, 714 - 100);
});

test("a popover flips up when the bottom edge is too close", () => {
    const p = popover({ x: 700, w: 28 }, { top: 1000, bottom: 1084 }, { w: 200, h: 220 }, area, "down", {});
    assert.equal(p.side, "up");
    assert.equal(p.y, 1000 - 6 - 220);
});

test("a popover aligned to its button's start clamps at the right edge", () => {
    const p = popover({ x: 1880, w: 28 }, { top: 400, bottom: 484 }, { w: 200, h: 100 }, area, "up", { align: "start" });
    assert.equal(p.x, 1920 - 200 - 8);
    assert.equal(p.side, "up");
});

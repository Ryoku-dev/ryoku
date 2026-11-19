import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { GAP, MARGIN, fitSize, slotPosition, clamp, outputFor, initialPosition, zoom } = require("./pinlayout.js");

test("large image fits within a third width and half height, aspect preserved", () => {
    const s = fitSize(4000, 3000, 1920, 1080);
    assert.ok(s.w <= 1920 / 3, "width within a third of the screen");
    assert.ok(s.h <= 1080 / 2, "height within half the screen");
    assert.equal(s.w / s.h, 4000 / 3000, "aspect ratio preserved");
    assert.equal(s.w, Math.floor(s.w), "width is integral");
    assert.equal(s.h, Math.floor(s.h), "height is integral");
});

test("small image is not scaled up", () => {
    assert.deepEqual(fitSize(64, 64, 1920, 1080), { w: 64, h: 64 });
});

test("slot 0 sits at the bottom-right inside the margin", () => {
    const size = { w: 300, h: 200 };
    assert.deepEqual(slotPosition(0, size, 1920, 1080), {
        x: 1920 - MARGIN - size.w,
        y: 1080 - MARGIN - size.h
    });
});

test("slot 1 sits one rowH above slot 0", () => {
    const size = { w: 300, h: 200 };
    const p0 = slotPosition(0, size, 1920, 1080);
    const p1 = slotPosition(1, size, 1920, 1080);
    assert.equal(p1.x, p0.x);
    assert.equal(p0.y - p1.y, size.h + GAP);
});

test("first slot of the second column is size.w + GAP left of slot 0", () => {
    const size = { w: 300, h: 200 };
    const rows = Math.max(1, Math.floor((1080 - 2 * MARGIN) / (size.h + GAP)));
    const p0 = slotPosition(0, size, 1920, 1080);
    const pCol1 = slotPosition(rows, size, 1920, 1080);
    assert.equal(p0.x - pCol1.x, size.w + GAP);
    assert.equal(pCol1.y, p0.y);
});

test("clamp pulls a negative position back to zero", () => {
    assert.deepEqual(clamp({ x: -50, y: -20 }, { w: 300, h: 200 }, 1920, 1080), { x: 0, y: 0 });
});

test("clamp pulls an overflowing position back inside", () => {
    assert.deepEqual(clamp({ x: 5000, y: 5000 }, { w: 300, h: 200 }, 1920, 1080), {
        x: 1920 - 300,
        y: 1080 - 200
    });
});

test("clamp holds the card one inset inside the output", () => {
    assert.deepEqual(clamp({ x: -50, y: -20 }, { w: 300, h: 200 }, 1920, 1080, 28), { x: 28, y: 28 });
    assert.deepEqual(clamp({ x: 5000, y: 5000 }, { w: 300, h: 200 }, 1920, 1080, 28), {
        x: 1920 - 28 - 300,
        y: 1080 - 28 - 200
    });
});

test("clamp with an inset never inverts on an oversized card", () => {
    assert.deepEqual(clamp({ x: 0, y: 0 }, { w: 4000, h: 4000 }, 1920, 1080, 28), { x: 28, y: 28 });
});

const screens = [
    { name: "DP-1", x: 0, y: 0, width: 1920, height: 1080 },
    { name: "DP-2", x: 1920, y: -200, width: 2560, height: 1440 }
];

test("named output wins over the focused output", () => {
    assert.equal(outputFor({ output: "DP-2", x: 2020, y: 0 }, screens, "DP-1"), "DP-2");
});

test("missing outputs and legacy pins fall back to the focused or first output", () => {
    assert.equal(outputFor({ output: "gone" }, screens, "DP-2"), "DP-2");
    assert.equal(outputFor({}, screens, "gone"), "DP-1");
    assert.equal(outputFor({}, [], "gone"), "");
});

test("position without an output selects the screen containing the global point", () => {
    assert.equal(outputFor({ x: 2020, y: -100 }, screens, "DP-1"), "DP-2");
});

test("global placement becomes output-local without adding a gutter", () => {
    assert.deepEqual(initialPosition({ output: "DP-2", x: 2020, y: 0 }, 0,
        { w: 300, h: 200 }, screens[1]), { x: 100, y: 200 });
    assert.deepEqual(initialPosition({ output: "DP-1", x: 0, y: 0 }, 0,
        { w: 300, h: 200 }, screens[0]), { x: 0, y: 0 });
});

test("missing output and malformed positions use the legacy slot rather than stale coordinates", () => {
    const size = { w: 300, h: 200 };
    const expected = slotPosition(0, size, screens[0].width, screens[0].height);
    assert.deepEqual(initialPosition({ output: "gone", x: 5000, y: -100 }, 0, size, screens[0]), expected);
    for (const position of [{ x: null, y: null }, { x: "100", y: 200 }, { x: Infinity, y: 0 }])
        assert.deepEqual(initialPosition(position, 0, size, screens[0]), expected);
});

test("zoom grows around the centre rather than the top-left", () => {
    const result = zoom({ x: 200, y: 200, w: 400, h: 200 }, 1.25, 1920, 1080);
    assert.deepEqual(result, { x: 150, y: 175, w: 500, h: 250 });
});

test("zoom is aspect-preserving and bounded by the output and minimum size", () => {
    const large = zoom({ x: 0, y: 0, w: 400, h: 200 }, 100, 1920, 1080);
    assert.deepEqual(large, { x: 0, y: 0, w: 1920, h: 960 });
    const small = zoom({ x: 200, y: 200, w: 400, h: 200 }, 0.001, 1920, 1080);
    assert.equal(small.h, 16);
    assert.equal(small.w, 32);
});

import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const B = require("./beautify.js");

test("auto keeps the padded shot", () => {
    assert.deepEqual(B.frame(800, 500, 90, "auto"), { w: 980, h: 680 });
});

test("a wide ratio widens, a tall one heightens", () => {
    const wide = B.frame(800, 500, 0, "16:9");
    assert.equal(wide.h, 500);
    assert.ok(Math.abs(wide.w - 500 * 16 / 9) < 1e-9);
    const tall = B.frame(800, 500, 0, "9:16");
    assert.equal(tall.w, 800);
    assert.ok(Math.abs(tall.h - 800 * 16 / 9) < 1e-9);
});

test("the free area leaves the panel and the margins", () => {
    const a = B.freeArea({ w: 1920, h: 1200 }, { w: 340, edge: 16 }, 32, { bottom: 40 });
    assert.deepEqual(a, { x: 32, y: 32, w: 1920 - 340 - 16 - 32 - 32, h: 1200 - 32 - 40 - 32 });
});

test("a small selection keeps its size and grows its background around it", () => {
    const area = { x: 32, y: 32, w: 1500, h: 1100 };
    const r = B.rest({ w: 480, h: 380 }, 0.75, { x: 700, y: 500 }, area);
    assert.equal(r.scale, 0.75);
    assert.equal(r.x, 700);
    assert.equal(r.y, 500);
});

test("a full-screen selection shrinks to fit beside the panel", () => {
    const area = B.freeArea({ w: 1920, h: 1200 }, { w: 320, edge: 0 }, 32, {});
    const comp = B.frame(2560, 1600, 90, "auto");
    const r = B.rest(comp, 0.75, { x: 960, y: 600 }, area);
    assert.ok(r.scale < 0.75);
    assert.ok(r.x + comp.w * r.scale / 2 <= area.x + area.w + 1e-9);
    assert.ok(r.y - comp.h * r.scale / 2 >= area.y - 1e-9);
});

test("a selection under the panel is clamped into the free area", () => {
    const area = { x: 32, y: 32, w: 1500, h: 1100 };
    const r = B.rest({ w: 400, h: 300 }, 1, { x: 1800, y: 40 }, area);
    assert.equal(r.x, 32 + 1500 - 200);
    assert.equal(r.y, 32 + 150);
});

test("a look is chosen while every value it names holds", () => {
    const style = B.complete({});
    assert.ok(B.matches(style, B.looks[0].cfg));
    assert.ok(!B.matches(style, B.looks[1].cfg));
    assert.ok(!B.matches(Object.assign({}, style, { padding: 60 }), B.looks[0].cfg));
    assert.ok(B.matches(Object.assign({}, style, { bgSolid: "#14120F", bgKind: "solid", padding: 72, roundness: 16,
        shadowStrength: 32, shadowBlur: 42, shadowDist: 16 }), B.looks[3].cfg));
});

test("a partial style takes the defaults", () => {
    const s = B.complete({ padding: 10 });
    assert.equal(s.padding, 10);
    assert.equal(s.bgKind, "preset");
    assert.equal(s.watermark, false);
});

test("backgrounds resolve presets and custom gradients alike", () => {
    assert.deepEqual(B.background({ bgKind: "preset", bgPreset: 0 }), { kind: "gradient", a: "#4facfe", b: "#00c6a7", angle: 135, solid: "", image: "" });
    assert.equal(B.background({ bgKind: "gradient", bgGradA: "#000", bgGradB: "#fff", bgGradAngle: 40 }).angle, 40);
    assert.equal(B.background({ bgKind: "none" }).kind, "none");
});

test("dragging past the edge adds image px at the shown scale, within 0..240", () => {
    assert.equal(B.dragPadding(90, 30, 0.5), 150);
    assert.equal(B.dragPadding(90, -200, 0.5), 0);
    assert.equal(B.dragPadding(200, 100, 0.5), 240);
});

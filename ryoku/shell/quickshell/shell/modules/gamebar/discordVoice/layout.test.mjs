import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { PAD, HEADER, NAME, NAME_SIDE, NAME_MAX, SPACE, avatar, beside, nameWidth, cell, rowHeight, columns, size } = require("./layout.js");

test("avatar size is clamped to the setting range", () => {
    assert.equal(avatar(undefined), 52);
    assert.equal(avatar(10), 32);
    assert.equal(avatar(100), 72);
    assert.equal(avatar(48), 48);
});

test("a cell fits an avatar and an optional name", () => {
    assert.equal(cell(52, true), 76);
    assert.equal(cell(52, false), 60);
    assert.equal(cell(72, true), 76);
});

test("columns never exceed the participants, the design cap or the frame", () => {
    assert.equal(columns(300, 0, 52, true), 1);
    assert.equal(columns(300, 2, 52, true), 2);
    assert.equal(columns(300, 8, 52, true), 3);
    assert.equal(columns(2000, 8, 52, true), 8);
});

test("a quiet call is only the header", () => {
    const box = size(300, 0, 52, true);
    assert.equal(box.w, 300);
    assert.equal(box.h, PAD * 2 + HEADER);
});

test("one row adds the gap, avatar and name", () => {
    const box = size(300, 1, 52, true);
    assert.equal(box.w, 300);
    assert.equal(box.h, PAD * 2 + HEADER + 10 + 52 + NAME);
});

test("the panel never grows past the frame the user placed", () => {
    const box = size(300, 8, 52, true);
    assert.equal(box.w, 300);
    const wide = size(720, 8, 52, true);
    assert.equal(wide.w, 8 * 76 + 7 * SPACE + PAD * 2);
});

test("names off leaves the avatar cell tighter", () => {
    const named = size(400, 4, 52, true);
    const plain = size(400, 4, 52, false);
    assert.ok(plain.h < named.h);
});

test("vertical stacks everyone in one column", () => {
    assert.equal(columns(2000, 8, 52, true, true), 1);
    const narrow = size(400, 8, 52, true, false);
    const tall = size(400, 8, 52, true, true);
    assert.ok(tall.h > narrow.h);
});

test("a name beside the avatar takes width and drops the name row", () => {
    assert.equal(beside("left"), true);
    assert.equal(beside("below"), false);
    assert.equal(cell(52, true, "left"), 52 + NAME_SIDE);
    assert.equal(rowHeight(52, true, "below"), 52 + NAME);
    assert.equal(rowHeight(52, true, "left"), 52);
    assert.equal(cell(52, true, "left", 6), 52 + NAME_SIDE);
    assert.equal(cell(52, true, "right", 6), 52 + NAME_SIDE + 6);
    assert.equal(rowHeight(52, true, "below", 6), 52 + NAME + 6);
    const below = size(400, 1, 52, true, false, "below");
    const side = size(400, 1, 52, true, false, "left");
    assert.ok(side.h < below.h);
});

test("a higher character limit widens the name box", () => {
    assert.equal(nameWidth(8), NAME_SIDE);
    assert.equal(nameWidth(20), 20 * 6);
    assert.equal(nameWidth(32), NAME_MAX);
    assert.equal(cell(52, true, "below", 0, 32), NAME_MAX);
});

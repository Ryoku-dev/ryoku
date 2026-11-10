import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { PAD, HEADER, SPACE, avatar, cell, columns, size } = require("./layout.js");

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
    assert.equal(box.h, PAD * 2 + HEADER + 10 + 52 + 15);
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

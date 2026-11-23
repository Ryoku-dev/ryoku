import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { launcherWidth, visibleRows, sheetLevel, columns, beadHeight } = require("./geometry.js");

test("the launcher never gets narrower than 720 or wider than 880", () => {
  assert.equal(launcherWidth(1280), 720);
  assert.equal(launcherWidth(1920), 845);
  assert.equal(launcherWidth(3840), 880);
  assert.equal(launcherWidth(0), 720);
});

test("a short answer keeps four rows of room and a long one scrolls after nine", () => {
  assert.equal(visibleRows(1, 44, 1000), 4);
  assert.equal(visibleRows(6, 44, 1000), 6);
  assert.equal(visibleRows(200, 44, 1000), 9);
});

test("the list never overflows the room the vessel allows", () => {
  assert.equal(visibleRows(9, 44, 200), 4);
  assert.equal(visibleRows(9, 44, 100), 2);
});

test("the level follows the rows and stops at the vessel's ceiling", () => {
  const few = sheetLevel({ count: 2, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 620 });
  const many = sheetLevel({ count: 9, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 620 });
  assert.ok(many.level > few.level);
  assert.equal(few.listHeight, 4 * 44);
  const tight = sheetLevel({ count: 9, rowHeight: 44, chrome: 150, waveRoom: 10, maxHeight: 400 });
  assert.ok(tight.level <= 400);
  assert.equal(tight.rows, 5);
});

test("the music edge adds room below the content", () => {
  const still = sheetLevel({ count: 9, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 2000 });
  const wave = sheetLevel({ count: 9, rowHeight: 44, chrome: 150, waveRoom: 12, maxHeight: 2000 });
  assert.equal(wave.level - still.level, 12);
});

test("a short list grows to show the selected result's actions, up to nine rows", () => {
  const bare = sheetLevel({ count: 2, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 2000 });
  const tall = sheetLevel({ count: 2, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 2000, leadHeight: 300 });
  assert.equal(tall.listHeight, 300);
  assert.equal(tall.level - bare.level, 300 - 4 * 44);
  const huge = sheetLevel({ count: 2, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 2000, leadHeight: 5000 });
  assert.equal(huge.listHeight, 9 * 44);
  const capped = sheetLevel({ count: 2, rowHeight: 44, chrome: 150, waveRoom: 0, maxHeight: 400, leadHeight: 5000 });
  assert.ok(capped.level <= 400);
});

test("the list column is the wider one and the columns fill the width", () => {
  const c = columns(800, 16);
  assert.ok(c.list > c.lead);
  assert.equal(c.list + c.lead, 784);
});

test("the bead stretches with its travel but never past one and a half rows", () => {
  assert.equal(beadHeight(44, 0), 44);
  assert.ok(beadHeight(44, 44) > 44);
  assert.equal(beadHeight(44, 1000), 66);
});

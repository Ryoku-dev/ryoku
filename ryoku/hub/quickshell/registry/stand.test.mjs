import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

// load stand.js the way the QML engine links it
const here = path.dirname(fileURLToPath(import.meta.url));
const S = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(here, "stand.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), S);
const letters = g => Array.from(g).map(n => S.letter(n)).join("");

test("the owner's machine reads D B B B B C", () => {
    // RTX 4060 Max-Q (class 2), 16 threads, 2560 wide, 46 GB, 165 Hz, 11 points (Rare)
    assert.equal(letters(S.of({ gpuClass: 2, threads: 16, width: 2560, ramGb: 46, refresh: 165, points: 11 })), "DBBBBC");
});

test("each grade starts at its floor", () => {
    assert.equal(letters(S.of({ gpuClass: 5, threads: 32, width: 3840, ramGb: 64, refresh: 240, points: 15 })), "AAAAAA");
    assert.equal(letters(S.of({ gpuClass: 4, threads: 31, width: 3839, ramGb: 63, refresh: 239, points: 14 })), "BBBBBB");
    assert.equal(letters(S.of({ gpuClass: 3, threads: 8, width: 1920, ramGb: 16, refresh: 90, points: 9 })), "CCCCCC");
    assert.equal(letters(S.of({ gpuClass: 2, threads: 4, width: 1280, ramGb: 8, refresh: 60, points: 6 })), "DDDDDD");
    assert.equal(letters(S.of({ gpuClass: 1, threads: 3, width: 1279, ramGb: 7, refresh: 59, points: 5 })), "EEEEEE");
});

test("unknown parts grade E, never a gap", () => {
    assert.equal(letters(S.of({})), "EEEEEE");
    assert.equal(letters(S.of({ gpuClass: 0, threads: NaN })), "EEEEEE");
});

test("the Stand is named after the motto", () => {
    const stock = "For the sake of power and beauty.";
    assert.equal(S.standName(stock, stock), "POWER & BEAUTY");
    assert.equal(S.standName("", stock), "POWER & BEAUTY");
    assert.equal(S.standName("Stay hungry, stay foolish.", stock), "STAY FOOLISH");
    assert.equal(S.standName("Built different", stock), "BUILT DIFFERENT");
    assert.equal(S.standName("For the glory of the old gods and new ones", stock), "OLD GODS & NEW ONES");
    assert.equal(S.standName("力と美のために", stock), "力と美のために");
    assert.equal(S.standName("Supercalifragilisticexpialidocious", stock).length, 20);
});

test("the title and the part", () => {
    assert.equal(S.title("nero"), "NERO'S BIZARRE MACHINE");
    assert.equal(S.title("james"), "JAMES' BIZARRE MACHINE");
    assert.equal(S.part("1.4.2"), 1);
    assert.equal(S.part("v2.0.0"), 2);
    assert.equal(S.part(""), 1);
});

import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { order } = require("./btrows.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

const e = (address, label, rank) => ({ address, label, rank });

eq(order([], [e("3", "Tile", 2), e("2", "Beta", 1), e("1", "Zed", 0), e("4", "Alpha", 1)]),
    ["1", "4", "2", "3"], "connected first, then saved by name, then found devices");

const shown = ["1", "7", "5"];
eq(order(shown, [e("1", "Zed", 0), e("5", "Mid", 2), e("7", "Aaa", 2)]), ["1", "7", "5"],
    "found devices keep their place, a name that sorts earlier does not move the row");
eq(order(shown, [e("1", "Zed", 0), e("5", "Mid", 2), e("7", "Aaa", 2), e("2", "New", 2)]),
    ["1", "7", "5", "2"], "a newly named device goes last");
eq(order(shown, [e("1", "Zed", 0), e("5", "Mid", 1), e("7", "Aaa", 2)]), ["1", "5", "7"],
    "a device that gets saved moves up into the saved group");
eq(order(shown, [e("5", "Mid", 2), e("7", "Aaa", 2)]), ["7", "5"],
    "a device that disappears leaves the rest in order");

if (failed > 0) { console.log(failed + " failed"); process.exit(1); }
console.log("ok");

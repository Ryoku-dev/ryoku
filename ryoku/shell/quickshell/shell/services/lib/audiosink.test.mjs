import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { chooseMusicSink } = require("./audiosink.js");

let failed = 0;
function eq(actual, expected, message) {
    if (actual === expected) console.log("PASS " + message);
    else { failed++; console.log("FAIL " + message + "\n  expected " + expected + "\n  got      " + actual); }
}

eq(chooseMusicSink("", "speakers", "", "", false), "speakers",
    "the initial feed uses the concrete default sink");
eq(chooseMusicSink("speakers", "headphones", "speakers", "speakers", true), "headphones",
    "a stale direct link cannot hold the feed on the old default");
eq(chooseMusicSink("headphones", "headphones", "speakers", "speakers", true), "headphones",
    "a moved playback link follows the new default");
eq(chooseMusicSink("", "headphones", "speakers", "speakers", true), "headphones",
    "an idle default switch retargets the feed");
eq(chooseMusicSink("easyeffects_sink", "headphones", "easyeffects_sink", "speakers", true), "easyeffects_sink",
    "a filter sink stays selected across an output switch");
eq(chooseMusicSink("", "headphones", "easyeffects_sink", "speakers", true), "easyeffects_sink",
    "a paused filter sink stays selected across an output switch");
eq(chooseMusicSink("", "headphones", "missing", "speakers", false), "headphones",
    "a removed playback sink falls back to the new default");
eq(chooseMusicSink("", "", "speakers", "speakers", true), "speakers",
    "a transiently missing default does not clear a known sink");
const duringGap = chooseMusicSink("", "", "speakers", "speakers", true);
eq(chooseMusicSink("", "headphones", duringGap, "speakers", true), "headphones",
    "a default that appears after a graph gap still retargets the feed");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

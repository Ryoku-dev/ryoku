import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { touchesAny, attachFlush, tidyGaps, deriveMain, rebaseToMain } = require("./arrange.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) {
        console.log("PASS " + msg);
    } else {
        failed++;
        console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a);
    }
}
function ok(cond, msg) { eq(!!cond, true, msg); }

function mk(name, x, y, w, h, disabled) { return { name, x, y, w, h, disabled: !!disabled }; }

// --- touchesAny: the cursor-crossing predicate --------------------------------
const flush = [mk("A", 0, 0, 1920, 1080), mk("B", 1920, 0, 1920, 1080)];
ok(touchesAny(flush, 1), "flush side-by-side displays touch");
ok(touchesAny(flush, 0), "flush side-by-side, other direction");
ok(!touchesAny([mk("A", 0, 0, 1920, 1080), mk("B", 2020, 0, 1920, 1080)], 1), "a 100px horizontal gap is not touching");
ok(!touchesAny([mk("A", 0, 0, 1920, 1080), mk("B", 1920, 2000, 1920, 1080)], 1), "edge-adjacent but zero perpendicular overlap does not count");
ok(touchesAny([mk("A", 0, 0, 1920, 1080)], 0), "a lone display is fine");
ok(touchesAny([mk("A", 0, 0, 1920, 1080), mk("B", 0, 1080, 1920, 1080)], 1), "vertically stacked displays touch");

// --- attachFlush: pull a detached display flush, keeping overlap --------------
const g = [mk("A", 0, 0, 1920, 1080), mk("B", 2500, 300, 1920, 1080)];
attachFlush(g, 1);
ok(touchesAny(g, 1), "attachFlush makes a detached display touch");
eq(g[1].x, 1920, "attachFlush snaps B flush to A's right edge");

// --- tidyGaps: a scattered layout becomes one connected block -----------------
const t = [mk("A", 0, 0, 1920, 1080), mk("B", 5000, 0, 1920, 1080), mk("C", -3000, 0, 1920, 1080)];
ok(tidyGaps(t), "tidyGaps reports it moved something");
ok(touchesAny(t, 0) && touchesAny(t, 1) && touchesAny(t, 2), "tidyGaps connects every display");
ok(!tidyGaps([mk("A", 0, 0, 1920, 1080), mk("B", 1920, 0, 1920, 1080)]), "tidyGaps is a no-op on an already-contiguous layout");

// --- deriveMain: the origin display -------------------------------------------
eq(deriveMain([mk("A", 0, 0, 1920, 1080), mk("B", 1920, 0, 1920, 1080)]), "A", "main is the display at the origin");
eq(deriveMain([mk("A", 1920, 0, 1920, 1080), mk("B", 0, 0, 1920, 1080)]), "B", "main is the origin display regardless of order");
eq(deriveMain([mk("A", -1920, 0, 1920, 1080), mk("B", 100, 0, 1920, 1080)]), "A", "no display at 0,0: main is the top-left-most");
eq(deriveMain([mk("A", 0, 0, 1920, 1080, true), mk("B", 1920, 0, 1920, 1080)]), "B", "a disabled origin display is not main");

// --- rebaseToMain: main sits at the global origin -----------------------------
const r = [mk("A", -1920, 0, 1920, 1080), mk("B", 0, 0, 1920, 1080)];
rebaseToMain(r, "A");
eq([r[0].x, r[1].x], [0, 1920], "rebaseToMain puts main at 0 and shifts the rest");
const r2 = [mk("A", 0, 0, 1920, 1080), mk("B", 1920, 0, 1920, 1080)];
rebaseToMain(r2, "B");
eq([r2[0].x, r2[1].x], [-1920, 0], "rebaseToMain onto the right display makes the left negative");

if (failed > 0) {
    console.log("\n" + failed + " test(s) FAILED");
    process.exit(1);
}
console.log("\nAll tests PASSED");

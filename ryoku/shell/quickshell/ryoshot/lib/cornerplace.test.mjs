import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { movingCorner, aroundCorner, besidePoint } = require("./cornerplace.js");

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
function overlaps(p, s, q, t) {
    return p.x < q.x + t.w && q.x < p.x + s.w && p.y < q.y + t.h && q.y < p.y + s.h;
}

const sel = { x: 100, y: 100, w: 300, h: 200 };
eq(movingCorner(sel, { x: 100, y: 100 }, ""), { x: 400, y: 300, dx: 1, dy: 1 }, "drag down-right moves br");
eq(movingCorner(sel, { x: 400, y: 300 }, ""), { x: 100, y: 100, dx: -1, dy: -1 }, "drag up-left moves tl");
eq(movingCorner(sel, { x: 100, y: 300 }, ""), { x: 400, y: 100, dx: 1, dy: -1 }, "drag up-right moves tr");
eq(movingCorner({ x: 50, y: 60, w: 0, h: 0 }, { x: 50, y: 60 }, ""), { x: 50, y: 60, dx: 1, dy: 1 }, "zero rect at press");
eq(movingCorner(sel, null, "tl"), { x: 100, y: 100, dx: -1, dy: -1 }, "grip tl");
eq(movingCorner(sel, null, "b"), { x: 250, y: 300, dx: 1, dy: 1 }, "grip b is the bottom midpoint");
eq(movingCorner(sel, null, "l"), { x: 100, y: 200, dx: -1, dy: 1 }, "grip l faces left");
eq(movingCorner(null, null, ""), null, "no selection");

const bounds = { w: 1920, h: 1200 };
const A = { w: 120, h: 34 }, B = { w: 112, h: 132 };
const g = 18, sg = 8, m = 8;

const mid = aroundCorner({ x: 900, y: 600 }, { dx: 1, dy: 1 }, A, B, bounds, g, sg, m);
eq(mid, { readout: { x: 918, y: 618 }, loupe: { x: 918, y: 450 } }, "middle: readout below-right, loupe above-right");

const right = aroundCorner({ x: 1900, y: 600 }, { dx: 1, dy: 1 }, A, B, bounds, g, sg, m);
eq(right.loupe.x + B.w <= 1900 - g, true, "right edge flips the column inward");
eq(right.readout.x + A.w <= 1900 - g, true, "right edge flips the readout too");

const bottom = aroundCorner({ x: 900, y: 1190 }, { dx: 1, dy: 1 }, A, B, bounds, g, sg, m);
eq(bottom.readout.y + A.h <= 1200 - m && bottom.loupe.y + B.h <= 1200 - m, true, "bottom edge stacks both above");
eq(overlaps(bottom.readout, A, bottom.loupe, B), false, "bottom edge stack does not overlap");

const top = aroundCorner({ x: 900, y: 10 }, { dx: -1, dy: -1 }, A, B, bounds, g, sg, m);
eq(top.readout.y >= m && top.loupe.y >= m, true, "top edge stacks both below");
eq(top.loupe.x + B.w <= 900 - g, true, "a left-facing corner keeps the column left");
eq(overlaps(top.readout, A, top.loupe, B), false, "top edge stack does not overlap");

const corner = aroundCorner({ x: 1915, y: 1195 }, { dx: 1, dy: 1 }, A, B, bounds, g, sg, m);
eq(corner.loupe.x + B.w <= 1915 - g && corner.loupe.y >= m, true, "bottom-right corner fits inward and above");

const alone = aroundCorner({ x: 900, y: 600 }, { dx: 1, dy: -1 }, { w: 0, h: 0 }, B, bounds, g, sg, m);
eq(alone, { readout: null, loupe: { x: 918, y: 618 } }, "a lone loupe runs beside the frame's edge");
const aloneLow = aroundCorner({ x: 900, y: 1150 }, { dx: 1, dy: -1 }, { w: 0, h: 0 }, B, bounds, g, sg, m);
eq(aloneLow, { readout: null, loupe: { x: 918, y: 1000 } }, "a lone loupe flips when the edge leaves no room");

for (const cx of [0, 5, 960, 1915, 1920]) {
    for (const cy of [0, 5, 600, 1195, 1200]) {
        for (const d of [{ dx: 1, dy: 1 }, { dx: -1, dy: -1 }, { dx: 1, dy: -1 }, { dx: -1, dy: 1 }]) {
            const r = aroundCorner({ x: cx, y: cy }, d, A, B, bounds, g, sg, m);
            const inside = (p, s) => p.x >= m && p.y >= m && p.x + s.w <= bounds.w - m && p.y + s.h <= bounds.h - m;
            const cursor = { x: cx - 12, y: cy - 12 }, cs = { w: 24, h: 24 };
            if (!inside(r.readout, A) || !inside(r.loupe, B)
                || overlaps(r.readout, A, cursor, cs) || overlaps(r.loupe, B, cursor, cs)
                || overlaps(r.readout, A, r.loupe, B)) {
                failed++;
                console.log("FAIL sweep " + JSON.stringify({ cx, cy, d, r }));
            }
            const lone = aroundCorner({ x: cx, y: cy }, d, { w: 0, h: 0 }, B, bounds, g, sg, m);
            if (lone.readout !== null || !inside(lone.loupe, B) || overlaps(lone.loupe, B, cursor, cs)) {
                failed++;
                console.log("FAIL lone sweep " + JSON.stringify({ cx, cy, d, lone }));
            }
        }
    }
}
console.log("PASS sweep: on screen, off the cursor, apart");

eq(besidePoint({ x: 100, y: 100 }, { w: 80, h: 30 }, bounds, 16, 8), { x: 116, y: 116 }, "tip below-right");
eq(besidePoint({ x: 1900, y: 1190 }, { w: 80, h: 30 }, bounds, 16, 8), { x: 1804, y: 1144 }, "tip flips at the corner");

if (failed > 0) {
    console.log("\n" + failed + " test(s) FAILED");
    process.exit(1);
}
console.log("\nAll tests PASSED");

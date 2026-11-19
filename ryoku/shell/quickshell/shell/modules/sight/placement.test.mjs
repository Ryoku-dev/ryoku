import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { beside, clamp, lens, mergeLines } = require("./placement.js");

let failed = 0;
function eq(actual, expected, message) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + message);
    else { failed++; console.log("FAIL " + message + "\n  expected " + e + "\n  got      " + a); }
}

const area = { x: 12, y: 48, width: 976, height: 740 };
eq(beside({ x: 100, y: 200, width: 180, height: 100 }, 390, 400, area, 12),
    { x: 292, y: 200 }, "prefers the right of the capture");
eq(beside({ x: 700, y: 200, width: 180, height: 100 }, 390, 400, area, 12),
    { x: 298, y: 200 }, "uses the left when the right does not fit");
eq(beside({ x: 100, y: 100, width: 800, height: 100 }, 390, 400, area, 12),
    { x: 100, y: 212 }, "uses the space below a wide capture");
eq(beside({ x: 100, y: 600, width: 800, height: 100 }, 390, 400, area, 12),
    { x: 100, y: 188 }, "uses the space above when below does not fit");
eq(beside({ x: -200, y: -100, width: 300, height: 100 }, 390, 400, area, 12),
    { x: 112, y: 48 }, "clears the reserved bar even when the capture crosses an output edge");
eq(beside({ x: 1800, y: 1900, width: 200, height: 100 }, 390, 400, area, 12),
    { x: 598, y: 388 }, "clamps a capture outside the surviving output");
eq(clamp(100, 12, -50), 12, "a shrunken output never gives a negative clamp range");
eq(lens({ x: 100, y: 200, width: 180, height: 100 }, 390, 120, area, 12, 444),
    { x: 292, y: 200, anchor: 200, bottom: false, side: "right", room: 588 }, "anchors a right-side lens once and reserves its growth room");
eq(lens({ x: 700, y: 650, width: 180, height: 100 }, 390, 120, area, 12, 444),
    { x: 298, y: 630, anchor: 750, bottom: true, side: "left", room: 702 }, "bottom-aligns a low capture so streamed text grows upward");
eq(lens({ x: 100, y: 100, width: 800, height: 100 }, 390, 120, area, 12, 444),
    { x: 100, y: 212, anchor: 212, bottom: false, side: "below", room: 576 }, "puts a wide capture's lens below and caps growth before the output edge");
eq(lens({ x: 100, y: 650, width: 800, height: 100 }, 390, 120, area, 12, 444),
    { x: 100, y: 518, anchor: 638, bottom: true, side: "above", room: 590 }, "anchors an above-plate lens at its bottom edge");
eq(mergeLines("one\ntwo\n\nthree\nfour"), "one two\n\nthree four", "merges wrapped lines while preserving paragraphs");
eq(mergeLines("one\r\n two\n\n\nthree"), "one two\n\nthree", "normalizes OCR whitespace and blank paragraph lines");
eq(mergeLines(""), "", "empty OCR text remains empty");
process.exitCode = failed ? 1 : 0;

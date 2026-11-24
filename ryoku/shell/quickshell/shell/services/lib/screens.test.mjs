import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { uniqueByName, sameOutputs, sliceForName, sliceForScreen, monitorScale } = require("./screens.js");

let failed = 0;
function eq(actual, expected, message) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + message);
    else { failed++; console.log("FAIL " + message + "\n  expected " + e + "\n  got      " + a); }
}
function ok(cond, message) {
    if (cond) console.log("PASS " + message);
    else { failed++; console.log("FAIL " + message); }
}

const scr = (name, width = 2560, height = 1600) => ({ name, width, height });
const names = list => list.map(s => s.name);

eq(names(uniqueByName([])), [], "empty list yields no surfaces");
eq(names(uniqueByName(undefined)), [], "missing list is tolerated");
eq(names(uniqueByName([{ name: "", width: 0, height: 0 }])), [], "nameless 0x0 placeholder is dropped");
eq(names(uniqueByName([scr("eDP-1"), scr("HDMI-A-1")])), ["eDP-1", "HDMI-A-1"], "distinct outputs are preserved in order");
eq(names(uniqueByName([scr("eDP-1"), scr("eDP-1")])), ["eDP-1"], "a duplicate output announce collapses to one surface set");
eq(names(uniqueByName([scr("eDP-1"), { name: "eDP-1", width: 0, height: 0 }])), ["eDP-1"], "an invalid same-name entry never adds a second bar");
eq(names(uniqueByName([{ name: "eDP-1", width: 0, height: 0 }, scr("eDP-1")])), ["eDP-1"], "a valid entry is kept even when an invalid same-name one precedes it");

const first = scr("eDP-1");
const second = scr("eDP-1");
const deduped = uniqueByName([first, second]);
ok(deduped.length === 1 && deduped[0] === first, "keeps the first ShellScreen object, so a duplicate the compositor is about to drop rebuilds nothing");

// --- churn filter for the per-monitor surface fan --------------------------

// The #312 crash: an unplug/replug makes QtWayland signal the screen list
// several times while the real outputs have not changed (a nameless 0x0
// placeholder is added, then removed). Each signal re-evaluated the deduped
// list, and every per-screen Variants rebuilt on the new array.
const a1 = scr("eDP-1"), a2 = scr("HDMI-A-1");
ok(sameOutputs([a1, a2], [a1, a2]), "the same live objects are the same outputs");
ok(sameOutputs([a1, a2], uniqueByName([a1, { name: "", width: 0, height: 0 }, a2])),
    "a placeholder signal leaves the sanitized list identical");
ok(!sameOutputs([a1, a2], [a1]), "losing an output is a change");
ok(!sameOutputs([a1], [a1, a2]), "gaining an output is a change");
ok(!sameOutputs([a1], [scr("eDP-1")]), "a recreated output object is a change, same name or not");
ok(!sameOutputs(undefined, [a1]), "a missing list is never same");

// --- per-monitor slice lookup ---------------------------------------------

const slice = name => ({ modelData: name === null ? null : scr(name) });
const slices = [slice("eDP-1"), slice("HDMI-A-1")];

ok(sliceForName(slices, "HDMI-A-1") === slices[1], "a slice is found by output name");
ok(sliceForName(slices, "DP-3") === null, "an output with no slice built yet is null");
ok(sliceForName(slices, "") === null, "an empty name matches nothing");
ok(sliceForName(undefined, "eDP-1") === null, "a missing slice list is tolerated");
ok(sliceForName([slice(null), slices[0]], "eDP-1") === slices[0], "a slice whose screen went away is skipped, never dereferenced");
ok(sliceForScreen(slices, null) === null, "a null screen resolves to no slice");
ok(sliceForScreen(slices, scr("DP-3")) === null, "a new output gets no slice until its own is built");

// The lid case: an output disabled and re-enabled comes back as a new
// ShellScreen object under the same name. Matching on object identity returned
// nothing here, which left every binding reading the slice on its stale value.
const recreated = scr("eDP-1");
ok(recreated !== slices[0].modelData, "a re-enabled output is a different object");
ok(slices.find(s => s.modelData === recreated) === undefined, "identity matching misses it, which is the bug");
ok(sliceForScreen(slices, recreated) === slices[0], "matching on name still resolves it to its own slice");

// The compositor scales the surface back up by the output scale, so the
// factor must be the LOGICAL height over 1080: the physical result then
// tracks the panel whatever the scale is. Multiplying the scale in as well
// scaled the chrome twice (issue #955).
const logical = (h) => ({ name: "eDP-1", width: 2048, height: h });
eq(monitorScale(logical(1080)), 1, "an unscaled 1080p panel is exactly 1.0");
eq(monitorScale(logical(1280)), 1280 / 1080, "the factor is the logical height over 1080");
eq(monitorScale(logical(1280)) * 1.25, 1600 / 1080,
    "the compositor scales the surface back up: the panel's physical height sets the factor");
eq(monitorScale(logical(1280), 1.2), 1.2, "the second argument is the cap, not the output scale");
eq(monitorScale(logical(1600), 1.2), 1.2, "a tall panel never balloons past the cap");
eq(monitorScale(logical(1440)), 1440 / 1080, "a fractional-resolution panel scales by its own height");
eq(monitorScale(null), 1, "a surface with no screen yet falls back to 1");
eq(monitorScale({ name: "x", width: 0, height: 0 }), 1, "a 0x0 placeholder output is not scaled to zero");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

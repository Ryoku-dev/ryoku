import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { displayTemp, clockToSec, sunFrac } = require("./weather.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}
function ok(cond, msg) { eq(!!cond, true, msg); }
function near(actual, expected, msg, eps) {
    if (eps === undefined) eps = 1e-6;
    if (typeof actual === "number" && isFinite(actual) && Math.abs(actual - expected) <= eps) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected ~" + expected + " (eps " + eps + ")\n  got      " + actual); }
}

// displayTemp: the frame's own reading unless the launcher forces the other
// scale, converted from the celsius measurement (the frame's tempC).
eq(displayTemp("21\u00b0C", 21.2, ""), "21\u00b0C", "no override follows the frame (C)");
eq(displayTemp("70\u00b0F", 21.2, ""), "70\u00b0F", "no override follows the frame (F)");
eq(displayTemp("21\u00b0C", 21.2, "C"), "21\u00b0C", "override equal to the frame unit keeps the string");
eq(displayTemp("70\u00b0F", 21.2, "F"), "70\u00b0F", "F override on an F frame keeps the string");
eq(displayTemp("21\u00b0C", 21.2, "F"), "70\u00b0F", "F override converts a C frame");
eq(displayTemp("22\u00b0C", 21.6, "F"), "71\u00b0F", "F override converts the measurement, not the rounded 22");
eq(displayTemp("-40\u00b0C", -40, "F"), "-40\u00b0F", "F override at -40");
eq(displayTemp("70\u00b0F", 21.2, "C"), "21\u00b0C", "C override converts an F frame");
eq(displayTemp("32\u00b0F", 0, "C"), "0\u00b0C", "C override at freezing");
eq(displayTemp("", 0, "F"), "", "no reading stays empty");
eq(displayTemp(undefined, undefined, ""), "", "missing reading stays empty");
eq(displayTemp("21\u00b0C", undefined, "F"), "21\u00b0C", "no measurement keeps the frame string");
eq(displayTemp("21\u00b0C", null, "F"), "21\u00b0C", "a null measurement keeps the frame string");

// clockToSec: the daemon's sunrise/sunset in either clock format.
eq(clockToSec("05:24"), 5 * 3600 + 24 * 60, "24h clock");
eq(clockToSec("00:00"), 0, "24h midnight");
eq(clockToSec("23:59"), 23 * 3600 + 59 * 60, "24h 23:59");
eq(clockToSec("5:24 AM"), 5 * 3600 + 24 * 60, "12h morning");
eq(clockToSec("8:48 PM"), 20 * 3600 + 48 * 60, "12h evening");
eq(clockToSec("12:05 AM"), 5 * 60, "12h just after midnight");
eq(clockToSec("12:30 PM"), 12 * 3600 + 30 * 60, "12h just after noon");
eq(clockToSec(""), -1, "empty -> -1");
eq(clockToSec(null), -1, "null -> -1");
eq(clockToSec("24:00"), -1, "24:00 -> -1");
eq(clockToSec("13:00 PM"), -1, "13 PM -> -1");
eq(clockToSec("0:15 AM"), -1, "hour 0 with a meridiem -> -1");
eq(clockToSec("5:60"), -1, "minute 60 -> -1");
eq(clockToSec("2026-07-01T05:24"), -1, "an ISO datetime is not a clock");

// sunFrac: sunrise=05:12 (18720), sunset=20:48 (74880). Verified references.
const SR = 18720, SS = 74880, DAY = 86400;
const day = sunFrac(43200, SR, SS);
ok(day && day.isDay === true, "sunFrac noon isDay true");
near(day.frac, (43200 - SR) / (SS - SR), "sunFrac noon frac ~0.4359");
const atRise = sunFrac(SR, SR, SS);
ok(atRise && atRise.isDay === true, "sunFrac at sunrise isDay true");
eq(atRise.frac, 0, "sunFrac at sunrise frac 0");
const jbSunset = sunFrac(SS - 1, SR, SS);
ok(jbSunset && jbSunset.isDay === true, "sunFrac just before sunset isDay true");
near(jbSunset.frac, (SS - 1 - SR) / (SS - SR), "sunFrac just before sunset frac");
const atSet = sunFrac(SS, SR, SS);
ok(atSet && atSet.isDay === false, "sunFrac at sunset handoff isDay false");
eq(atSet.frac, 0, "sunFrac at sunset frac 0 (clean handoff)");
const afterSunset = sunFrac(22 * 3600, SR, SS);
ok(afterSunset && afterSunset.isDay === false, "sunFrac 22:00 isDay false");
near(afterSunset.frac, (22 * 3600 - SS) / ((SR + DAY) - SS), "sunFrac 22:00 frac ~0.1429");
const beforeSunrise = sunFrac(3 * 3600, SR, SS);
ok(beforeSunrise && beforeSunrise.isDay === false, "sunFrac 03:00 isDay false");
near(beforeSunrise.frac, (3 * 3600 - (SS - DAY)) / (SR - (SS - DAY)), "sunFrac 03:00 frac ~0.7381");
eq(sunFrac(43200, -1, SS), null, "sunFrac null on sunrise<0");
eq(sunFrac(43200, SR, -1), null, "sunFrac null on sunset<0");
eq(sunFrac(43200, SS, SR), null, "sunFrac null when sunset<=sunrise (swapped)");
eq(sunFrac(43200, SR, SR), null, "sunFrac null when sunset==sunrise");
eq(sunFrac(NaN, SR, SS), null, "sunFrac null on NaN nowSec");
eq(sunFrac(Infinity, SR, SS), null, "sunFrac null on Infinity nowSec");
eq(sunFrac(-Infinity, SR, SS), null, "sunFrac null on -Infinity nowSec");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

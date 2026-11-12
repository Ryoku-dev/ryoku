import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const M = require("./model.js");

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

const panel = M.clone({
    name: "eDP-1", width: 2560, height: 1600, refresh: 60, scale: 1.3333333333333333, x: 0, y: 0, transform: 6,
    modes: ["2560x1600@60", "2560x1600@165.019", "1920x1200@60", "1920x1080@59.951", "1920x1080@60"]
});

eq(M.sizes(panel).map(s => s.key), ["2560x1600", "1920x1200", "1920x1080"], "sizes are distinct, largest first");
eq(M.rates(panel).map(r => r.key), ["2560x1600@165.019", "2560x1600@60"], "rates at the current size");
eq(M.modeForSize(panel, 1920, 1080), "1920x1080@60", "a new size keeps the current rate");
eq(M.rateText(59.951, [{ rate: 59.951 }, { rate: 60 }]), "59.95 Hz", "rates that read alike keep decimals");
eq(M.rateText(165.019, [{ rate: 165.019 }]), "165 Hz", "a lone rate reads whole");
eq(M.scaleLadder(panel).indexOf(1.3333) >= 0, true, "the ladder holds the current scale");
eq(M.snapScale(panel, 1.3333333333333333), 1.3333, "the current scale snaps onto the ladder");
eq(M.rotation(6), 2, "a flipped panel reads its rotation");
eq(M.withRotation(6, 1), 5, "rotating keeps the flip");

const custom = Object.assign({}, panel, { mode: "2000x1000@75", width: 2000, height: 1000, refresh: 75 });
eq(M.isCustom(custom), true, "a typed mode is custom");
eq(M.isCustom(panel), false, "an advertised mode is not custom");
eq(M.sizes(custom)[0], { key: "2000x1000", w: 2000, h: 1000, custom: true }, "a custom size leads the list");
eq(M.rates(custom).map(r => r.key), ["2000x1000@75"], "a custom rate is offered");

eq(M.clockText(22 * 60, true), "22:00", "24-hour clock");
eq(M.clockText(22 * 60 + 5, false), "10:05 PM", "12-hour clock");
eq(M.clockText(0, false), "12:00 AM", "midnight");
eq(M.parseClock("9:30 pm"), 21 * 60 + 30, "typed afternoon time");
eq(M.parseClock("21:30"), 21 * 60 + 30, "typed 24-hour time");
eq(M.parseClock("7"), 7 * 60, "a bare hour");
eq(M.parseClock("13 pm"), -1, "nonsense stays out");

if (failed > 0) {
    console.log(failed + " failed");
    process.exit(1);
}

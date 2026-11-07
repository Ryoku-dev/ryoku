import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { blockState, settled, nextRecheck, firstRecheck, lastRecheck } = require("./BluetoothRfkill.js");

let failed = 0;
function eq(actual, expected, msg) {
    if (actual === expected) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + expected + "\n  got      " + actual); }
}

const entry = (type, soft, hard) => ({ pending: 0, type, soft, hard });

let s = blockState([entry("bluetooth", "0", "0"), entry("wlan", "1", "1")]);
eq(s.soft || s.hard, false, "a blocked Wi-Fi radio does not block Bluetooth");

s = blockState([entry("bluetooth", "0", "0"), entry("bluetooth", "1", "0")]);
eq(s.soft, true, "a soft-blocked platform switch blocks Bluetooth even when the controller is unblocked");
eq(s.hard, false, "a soft block alone is not reported as a hardware block");

s = blockState([entry("bluetooth", "0", "1")]);
eq(s.hard, true, "a hardware kill switch is reported");

s = blockState([]);
eq(s.soft || s.hard, false, "no rfkill entries means nothing is blocked");

eq(blockState([entry("bluetooth", "1", "0"), { pending: 2, type: "", soft: "", hard: "" }]), null,
    "a rescan still loading keeps the previous state instead of showing unblocked");

eq(settled(-1, false, true), true, "with no choice made there is nothing to re-check");
eq(settled(1, false, false), false, "turning on is unsettled while the radio is still off");
eq(settled(1, true, true), false, "turning on is unsettled while the radio is still blocked");
eq(settled(1, true, false), true, "turning on settles once the radio is on and unblocked");
eq(settled(0, true, false), false, "turning off is unsettled while the radio is still on");
eq(settled(0, false, true), true, "turning off settles once the radio is off, blocked or not");

const schedule = [];
for (let t = firstRecheck; t > 0; t = nextRecheck(t))
    schedule.push(t);
eq(schedule.join(","), "250,500,1000,2000,4000,8000", "re-checks back off from 250 ms and end after 8 s");
eq(schedule[schedule.length - 1], lastRecheck, "the last re-check uses the cap");
eq(schedule.reduce((a, b) => a + b, 0), 15750, "the re-check window is bounded to about 16 s");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

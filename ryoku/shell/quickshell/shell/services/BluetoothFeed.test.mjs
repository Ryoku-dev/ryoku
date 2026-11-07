import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { frameFrom, resolve } = require("./BluetoothFeed.js");

let failed = 0;
function eq(actual, expected, msg) {
    if (actual === expected) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + expected + "\n  got      " + actual); }
}

const up = frameFrom({ running: true, adapters: [{ path: "/org/bluez/hci0", name: "laptop", address: "AA", powered: true }] });
eq(up.running && up.adapters.length, 1, "a running frame keeps its adapter");
eq(frameFrom(null), null, "no frame is not a frame");
eq(frameFrom("err unknown topic: bluetooth"), null, "a stray string is not a frame");
eq(frameFrom({ adapters: [] }), null, "a frame without running is rejected");
eq(frameFrom({ running: true, adapters: [null, { name: "x" }] }).adapters.length, 0, "adapters without a path are dropped");

const shared = { running: true, adapters: [{ path: "/org/bluez/hci0" }, { name: "x" }] };
frameFrom(shared);
eq(shared.adapters.length, 2, "the shared bus frame is never mutated");

const gone = frameFrom({ running: false, adapters: [] });
const offFrame = frameFrom({ running: true, adapters: [{ path: "/org/bluez/hci0", powered: false }] });
const noController = frameFrom({ running: true, adapters: [] });

let r = resolve(true, false, up);
eq(r.useQuickshell && !r.enabled, true, "Quickshell's adapter wins over the daemon frame while BlueZ runs");
r = resolve(true, true, null);
eq(r.useQuickshell && r.enabled, true, "with no daemon frame Quickshell is used as before");
r = resolve(true, true, gone);
eq(!r.useQuickshell && !r.present && !r.enabled, true, "BlueZ gone drops Quickshell's stale adapter to no adapter");
r = resolve(false, false, up);
eq(!r.useQuickshell && r.present && r.enabled, true, "BlueZ started after the shell shows the controller and its power");
r = resolve(false, false, offFrame);
eq(r.present && !r.enabled, true, "a powered-off controller seen only by the daemon reads off");
r = resolve(false, false, noController);
eq(r.present, false, "BlueZ without a controller is no adapter");
r = resolve(false, false, null);
eq(r.present, false, "no Quickshell adapter and no frame is no adapter");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

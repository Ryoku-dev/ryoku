import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { switches } = require("./effects.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}

const on = { blurDisabled: false, shadowsDisabled: false };

eq(switches({}, false), on, "a shell.json without theme.effects keeps glass and shadows on");
eq(switches(null, false), on, "no settings frame yet keeps glass and shadows on");
eq(switches({ theme: { theme: "Default" } }, false), on, "a theme without effects keeps both on");
// this machine: Window style's blur behind windows is off in desktop.json, which
// the shell never reads, so its panel glass stays on
eq(switches({ desktop: { appearance: { blurEnabled: false, shadowEnabled: false } } }, false), on,
    "window blur and shadows off leave the panels' glass and shadows on");
eq(switches({ theme: { effects: { glass: false } } }, false), { blurDisabled: true, shadowsDisabled: false },
    "glass off frosts nothing and leaves shadows on");
eq(switches({ theme: { effects: { shadows: false } } }, false), { blurDisabled: false, shadowsDisabled: true },
    "shadows off leaves glass on");
eq(switches({ theme: { effects: { glass: true, shadows: true } } }, true), { blurDisabled: true, shadowsDisabled: true },
    "low power, Power Saver or Game Mode turn both off whatever the switches say");
eq(switches({ theme: { effects: { glass: "no" } } }, false), on, "only an explicit false turns glass off");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

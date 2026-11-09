import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { parse, isProtected, top } = require("./tasks.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}
function ok(cond, msg) { eq(!!cond, true, msg); }
function near(actual, expected, msg) {
    ok(Math.abs(actual - expected) < 0.001, msg + " (" + actual + ")");
}

const sample = [
    "root       1 systemd                2    3442 16524",
    "dodo    4242 firefox            2400     120 812340",
    "dodo    5150 Web Content        1200      60 340012",
    "dodo    6001 kitty                50     300  51200",
    ""
].join("\n");

const rows = parse(sample);
eq(rows.length, 4, "parses every row");
eq(rows[0].pid, 1, "reads pid");
eq(rows[0].user, "root", "reads the owner");
eq(rows[0].name, "systemd", "reads a plain name");
eq(rows[2].name, "Web Content", "keeps spaces in comm");
eq(rows[2].memMiB, 332.04296875, "rss kB becomes MiB");
near(rows[1].cpu, 2000, "cpu averages cputime over lifetime");
near(rows[3].cpu, 16.666666666666668, "short lived process still gets a rate");
eq(isProtected("niri"), true, "compositor is protected");
eq(isProtected("ryoku-shell"), true, "shell is protected");
eq(isProtected("firefox"), false, "an app is not protected");
eq(top(sample, 2, "dodo").map((r) => r.name), ["firefox", "Web Content"], "top sorts by cpu then memory");
eq(top(sample, 5, "dodo").length, 3, "top keeps only the current user");
eq(top(sample, 5, "root").map((r) => r.name), ["systemd"], "top can select the owner");
eq(parse("").length, 0, "empty input parses to nothing");
eq(parse("garbage\n\n").length, 0, "junk lines are skipped");

const scanMs = 1700000000000;
eq(parse(sample, scanMs)[1].started, 1700000000 - 120, "start is the scan time minus elapsed seconds");
eq(top(sample, 1, "dodo", scanMs)[0].started, 1700000000 - 120, "top forwards the scan time");

if (failed > 0) {
    console.log(failed + " failure(s)");
    process.exit(1);
}
console.log("all tasks tests passed");

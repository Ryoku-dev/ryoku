import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { rank, subsequence, highlight } = require("./fuzzy.js");

let failed = 0;
function eq(actual, expected, msg) {
    const a = JSON.stringify(actual);
    const e = JSON.stringify(expected);
    if (a === e) console.log("PASS " + msg);
    else { failed++; console.log("FAIL " + msg + "\n  expected " + e + "\n  got      " + a); }
}
function ok(cond, msg) { eq(!!cond, true, msg); }

const apps = [
    { id: "spotify.desktop", name: "Spotify", genericName: "Musik", keywords: ["audio", "stream"], noDisplay: false },
    { id: "files.desktop", name: "Files", genericName: "File Manager", keywords: [], noDisplay: false },
    { id: "firefox.desktop", name: "Firefox", genericName: "Web Browser", keywords: ["www", "internet"], noDisplay: false },
    { id: "settings-daemon.desktop", name: "Settings Daemon", genericName: "", keywords: [], noDisplay: true },
    { id: "steam.desktop", name: "Steam", genericName: "Game Launcher", keywords: ["games"], noDisplay: false }
];

const names = (q, usage) => rank(apps, q, usage).map((e) => e.name);

ok(subsequence("spt", "spotify"), "subsequence spt in spotify");
ok(!subsequence("xyz", "spotify"), "subsequence xyz not in spotify");

eq(names(""), ["Files", "Firefox", "Spotify", "Steam"], "empty query lists visible apps alphabetically, drops noDisplay");

eq(names("spo")[0], "Spotify", "prefix match ranks first");
eq(names("fi"), ["Files", "Firefox"], "prefix matches both Files and Firefox alphabetically");

ok(names("musik").indexOf("Spotify") !== -1, "genericName substring matches");
ok(names("internet").indexOf("Firefox") !== -1, "keyword substring matches");

eq(names("gam"), ["Steam"], "keyword substring match (gam -> Steam via games)");

eq(names("zzz"), [], "no match returns empty");

ok(rank(apps, "s").indexOf(apps[3]) === -1, "noDisplay excluded even on match");

eq(names("Spo"), ["Spotify"], "case-insensitive prefix");

eq(names("", { "steam.desktop": 5, "firefox.desktop": 2 }), ["Steam", "Firefox", "Files", "Spotify"], "empty query orders by usage desc then alphabetical");
eq(names(""), ["Files", "Firefox", "Spotify", "Steam"], "ties without usage stay alphabetical");
eq(names("fi", { "firefox.desktop": 3 }), ["Firefox", "Files"], "query results tie-break by usage desc");
eq(names("fi"), ["Files", "Firefox"], "query results stay alphabetical when usage equal");

// highlight(): spans of the query matched against a display name, for the UI's
// char-highlight. Contiguous prefix collapses to one span; scattered subsequence
// returns one span per matched char; no match returns [].
eq(highlight("Firefox", "fir"), [{ start: 0, len: 3 }], "prefix highlight is one contiguous span");
eq(highlight("Spotify", "sty"), [{ start: 0, len: 1 }, { start: 3, len: 1 }, { start: 6, len: 1 }], "subsequence highlight is per-char spans");
eq(highlight("Files", "fis"), [{ start: 0, len: 2 }, { start: 4, len: 1 }], "adjacent matched chars merge into one span");
eq(highlight("Firefox", "FIR"), [{ start: 0, len: 3 }], "highlight is case-insensitive");
eq(highlight("Firefox", "zzz"), [], "no subsequence match returns no spans");
eq(highlight("Firefox", ""), [], "empty query returns no spans");

if (failed > 0) { console.log("\n" + failed + " test(s) FAILED"); process.exit(1); }
console.log("\nAll tests PASSED");

import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const M = require("./memory.js");
const S = require("./sections.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

// a kit.json as the old frame and its tabs wrote it
const old = JSON.stringify({
    "frame.tab": "typing",
    "commands.rail": "group:linux:files",
    "phone.rail": "screen",
    "devtools.tool": "jwt_decoder",
    "devtools.options.password": { length: 32, symbols: false },
    "typing.mode": "words",
    "typing.history.results": [{ wpm: 71.4, mode: "time" }, { wpm: 64, mode: "time" }],
    "typing.history.personalBests": [{ key: "time:30:english_1k:-:-", wpm: 92.6 }, { key: "words:25:english_1k:-:-", wpm: 88 }]
});
const values = M.parse(old);

equal(M.get(values, "commands.rail", ""), "group:linux:files", "the commands page a reader was on is still read");
equal(M.get(values, "phone.rail", ""), "screen", "the phone page too");
equal(M.get(values, "devtools.tool", ""), "jwt_decoder", "the dev tool");
equal(M.get(values, "devtools.options.password", null), { length: 32, symbols: false }, "and a tool's options");
equal(M.get(values, "typing.mode", "time"), "words", "typing settings");
equal(M.get(values, "typing.history.results", []).length, 2, "typing history");
equal(M.get(values, "nothing.here", 7), 7, "a key never written answers the fallback");
equal(M.get(values, "toString", "own"), "own", "an inherited name is not a key");

equal(M.startSection(values, S.ORDER, S.FIRST), "typing", "frame.tab is the section Kit reopens on");
equal(M.startSection({}, S.ORDER, S.FIRST), "keys", "no memory opens the legend");
equal(M.startSection({ "frame.tab": "gone" }, S.ORDER, S.FIRST), "keys", "a tab that no longer exists opens the legend");
equal(M.startSection({ "frame.tab": "downloads" }, S.ORDER, S.FIRST), "downloads", "the new section is remembered under the same key");

equal(M.parse(""), {}, "an empty file is an empty memory");
equal(M.parse("   \n"), {}, "so is a blank one");
equal(M.parse("{broken"), {}, "so is a broken one");
equal(M.parse("[1,2]"), {}, "and one that is not a map");
equal(M.parse("null"), {}, "or null");

const next = M.set(values, "frame.tab", "keys");
equal(next["frame.tab"], "keys", "set writes the key");
equal(values["frame.tab"], "typing", "without touching the map it was given");
equal(next["typing.history.results"], values["typing.history.results"], "and carries every other key over");
equal(Object.keys(next).length, Object.keys(values).length, "no key is lost on a write");
equal(M.set(values, "frame.tab", "typing") === values, true, "writing what is stored changes nothing (no file rewrite)");
equal(M.set(values, "devtools.options.password", { length: 32, symbols: false }) === values, true, "an equal object is no change either");
equal(M.set(values, "devtools.options.password", { length: 16, symbols: false }) === values, false, "a different one is");
equal(M.set(null, "a.b", 1), { "a.b": 1 }, "the first write makes the map");

equal(M.key("keys", "count"), "keys.count", "a section's keys carry its id");
equal(M.count({ "keys.count": 108 }, "keys"), 108, "what a section last counted");
equal(M.count({ "keys.count": "108" }, "keys"), 108, "read leniently");
equal(M.count({}, "commands"), 0, "nothing counted yet is zero, so the tab shows no figure");
equal(M.count({ "commands.count": -4 }, "commands"), 0, "nonsense is zero too");

equal(M.typingBest(values), 93, "the best on record, in whole words per minute");
equal(M.typingBest({}), 0, "no history is no best");
equal(M.typingBest({ "typing.history.personalBests": "oops" }), 0, "a broken list is no best");
equal(M.typingBest({ "typing.history.personalBests": [{ wpm: "x" }, null, { wpm: 41.2 }] }), 41, "bad entries are skipped");
equal(M.typingTests(values), 2, "the tests on record");
equal(M.typingTests({}), 0, "none yet");

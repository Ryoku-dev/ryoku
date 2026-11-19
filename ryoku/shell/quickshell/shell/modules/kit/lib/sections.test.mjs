import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const S = require("./sections.js");
const R = require("./route.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

equal(S.ORDER, ["downloads", "keys", "commands", "phone", "devtools", "typing"], "the rail order of the contract");
equal(R.SECTIONS.slice().sort(), S.ORDER.slice().sort(), "the router knows exactly the rail's sections");
equal(S.ORDER.every(id => typeof S.GLYPHS[id] === "string" && S.GLYPHS[id].length > 0), true, "every section has a glyph");
equal(S.has(S.FIRST), true, "the first-open section exists");
equal(S.FIRST, "keys", "a first open lands on the legend");

equal(S.index("downloads"), 0, "Downloads is Ctrl+1");
equal(S.at(5), "typing", "Typing test is Ctrl+6");
equal(S.at(6), "", "there is no seventh section");
equal(S.at(-1), "", "nor one before the first");

equal(S.step("downloads", 1), "keys", "Ctrl+Tab goes down the rail");
equal(S.step("typing", 1), "downloads", "and round from the last to the first");
equal(S.step("downloads", -1), "typing", "Ctrl+Shift+Tab goes round the other way");
equal(S.step("bogus", 1), "keys", "an unknown section steps to the first-open one");

const pages = [{ id: "test" }, { id: "history" }, { id: "stats" }];
equal(S.stepPage(pages, "test", 1), "history", "Ctrl+PgDn goes to the next page");
equal(S.stepPage(pages, "stats", 1), "stats", "and stops on the last");
equal(S.stepPage(pages, "test", -1), "test", "Ctrl+PgUp stops on the first");
equal(S.stepPage(pages, "gone", 1), "test", "a page that left starts over");
equal(S.stepPage([], "x", 1), "", "a section with no pages has nowhere to go");
equal(S.pageIndex(pages, "stats"), 2, "a page is found by id");
equal(S.pageIndex(pages, "nope"), -1, "or not at all");

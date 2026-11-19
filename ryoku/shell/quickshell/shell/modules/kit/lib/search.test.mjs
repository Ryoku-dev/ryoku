import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Q = require("./search.js");
const S = require("./sections.js");
const List = require("../../ask/lib/list.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

equal(Q.matches("mirror", ["Mirror screen"]), true, "a word matches whatever its case");
equal(Q.matches("screen mirror", ["Mirror screen"]), true, "every word must appear, in any order");
equal(Q.matches("mirror tv", ["Mirror screen"]), false, "one missing word is no match");
equal(Q.matches("  ", ["anything"]), false, "an empty query matches nothing: all of Kit starts blank");
equal(Q.matches("json", ["Formatter", "Format, minify and validate JSON"]), true, "any of a row's texts can carry the word");
equal(Q.matches("x", [undefined, null, ""]), false, "missing texts are just empty");

equal(Q.chordForms(["Super", "Shift", "P"]), ["super+shift+p", "super shift p", "super + shift + p"], "a chord as a reader types it");
equal(Q.chordMatches("super+k", ["Super", "K"]), true, "super+k finds Super K");
equal(Q.chordMatches("Super + K", ["Super", "K"]), true, "spaces round the plus do not matter");
equal(Q.chordMatches("shift+p", ["Super", "Shift", "P"]), true, "part of a chord finds it");
equal(Q.chordMatches("super+k", ["Super", "Shift", "K"]), false, "but not across a key that is not there");
equal(Q.chordMatches("", ["Super", "K"]), false, "nothing typed finds nothing");
equal(Q.chordMatches("super", []), false, "a bind with no keys has no chord");

const legend = [
    { name: "Windows", binds: [
        { desc: "Close window", hint: "", keys: ["Super", "Q"] },
        { desc: "Pin window", hint: "", keys: ["Super", "P"], unhonored: "Not on this desktop" }
    ] },
    { name: "Shell", binds: [
        { desc: "Kit", hint: "Press again to close", keys: ["Super", "K"] },
        { desc: "Launcher", hint: "", keys: ["Super", "Space"] }
    ] }
];
const de = { "Close window": "Fenster schließen", "Kit": "Kit", "Launcher": "Starter", "Windows": "Fenster", "Shell": "Shell" };
const tr = text => de[text] || text;

equal(Q.keyRows(legend, "close", tr).map(row => row.title), ["Fenster schließen", "Kit"], "found by the English name, and by a hint");
equal(Q.keyRows(legend, "schließen", tr).map(row => row.id), ["c0b0"], "and by the name in the reader's language");
equal(Q.keyRows(legend, "super+k", tr).map(row => row.title), ["Kit"], "and by chord");
equal(Q.keyRows(legend, "pin", tr), [], "a shortcut this compositor does not honour is not offered");
const kit = Q.keyRows(legend, "kit", tr)[0];
equal(kit.open, { page: "category-1", query: "Kit" }, "a shortcut opens its category with its name in the field");
equal(kit.trail, "Super + K", "and shows its chord");
equal(kit.subtitle, "Shell", "under its category");
equal(Q.keyRows(null, "kit"), [], "no legend yet is no rows, not an error");

const tools = [{ id: "jwt_decoder", name: "JWT decoder", description: "Read a token" }, { id: "uuid", name: "UUID generator", description: "Random ids" }];
const toolRows = Q.listRows(tools, "token", tool => ({ id: tool.id, title: tool.name, texts: [tool.description], glyph: "build", open: { page: tool.id } }));
equal(toolRows.map(row => row.id), ["jwt_decoder"], "a list row matches on its extra texts");
equal(toolRows[0].open, { page: "jwt_decoder" }, "and carries where it opens");
equal(Q.listRows(tools, "uuid", () => null), [], "an item the reader declines is skipped");

function rows(n, prefix) {
    return Array.from({ length: n }, (_, i) => ({ id: prefix + i, size: "one", title: prefix + " " + i, open: { page: "p" + i } }));
}
const labels = { downloads: "Downloads", keys: "Keys", commands: "Commands", phone: "Phone", devtools: "Dev tools", typing: "Typing test" };
const grouped = Q.group(S.ORDER, labels, { commands: rows(9, "cmd"), keys: rows(2, "key"), typing: [], phone: rows(1, "ph") }, "wifi",
    { limit: 6, more: (n, label) => n + " more in " + label });

equal(grouped.map(section => section.id), ["keys", "commands", "phone"], "groups come in rail order and empty sections are left out");
equal(grouped.map(section => section.title), ["Keys", "Commands", "Phone"], "each headed by its section");
equal(grouped.map(section => section.note), ["2", "9", "1"], "with how many it found, not how many it shows");
equal(grouped[1].rows.length, 7, "a long group shows the limit and one row for the rest");
equal(grouped[1].rows[6].title, "3 more in Commands", "which says how many more");
equal(Q.target(grouped[1].rows[6]), { section: "commands", page: "", query: "wifi", args: null }, "and opens the section searching for the same words");
equal(Q.target(grouped[0].rows[1]), { section: "keys", page: "p1", query: "", args: null }, "Enter opens a row in its own section");
equal(Q.total(grouped), 12, "the total counts everything found");

const ids = grouped.flatMap(section => section.rows.map(row => row.id));
equal(new Set(ids).size, ids.length, "row ids stay unique across sections that reuse an id");
equal(Q.group(S.ORDER, labels, { keys: [{ id: "x", title: "a" }], commands: [{ id: "x", title: "b" }] }, "q").map(s => s.rows[0].id).length, 2, "even the same id in two sections");
equal(Q.target(null), null, "no row leads nowhere");
equal(Q.target({ id: "loose" }), null, "nor a row from outside the groups");

// the groups are what the bar's result list lays out and steps through
const layout = List.build(grouped, { scale: 1 });
equal(layout.rows.length, 2 + 7 + 1, "the list lays out every shown row");
equal(layout.heads.length, 3, "under three heads");
equal(List.jump(layout.rows, 0, 1), 2, "Ctrl+Down jumps from Keys to the first Commands row");

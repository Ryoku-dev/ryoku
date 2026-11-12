import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs
    .readFileSync(new URL("./KeybindsPage.js", import.meta.url), "utf8")
    .replace(/^\.pragma library\s*/, "");
const context = {};
vm.createContext(context);
vm.runInContext(source, context);
const rows = context.rows;

test("every row is a generic finder entry on one of the three tabs", () => {
    const tabs = new Set(["Shortcuts", "Apps", "Custom"]);
    for (const row of rows) {
        // the global finder only surfaces rows with a real label and no
        // action/layout control; these are plain doc rows on a known tab
        assert.ok(tabs.has(row.tab), `unexpected tab ${row.tab}`);
        assert.ok(row.label && row.label.length > 0, "row has a label");
        assert.ok(row.desc && row.desc.length > 0, "row has a description");
        // no store leaf: the finder must never treat a legend row as settable,
        // and a keyed row could be dropped as a dead key on some compositor
        assert.equal(row.key, undefined, `${row.label} names no store key`);
        assert.notEqual(row.ctl, "action");
    }
});

test("the finder reaches the number pad, the screens and the custom editor", () => {
    const opts = rows.flatMap(r => r.opts || []);
    // the three things a user is most likely to type and expect to land here
    assert.ok(opts.includes("number pad"), "number pad is indexed");
    assert.ok(opts.includes("screen"), "screens are indexed");
    assert.ok(rows.some(r => r.tab === "Custom" && r.opts.includes("record")),
        "the custom editor's record flow is indexed");
});

test("the Apps and Custom editors each have exactly one entry", () => {
    assert.equal(rows.filter(r => r.tab === "Apps").length, 1);
    assert.equal(rows.filter(r => r.tab === "Custom").length, 1);
    assert.ok(rows.filter(r => r.tab === "Shortcuts").length >= 6,
        "the shortcut legend is covered section by section");
});

import fs from "node:fs";
import vm from "node:vm";
import assert from "node:assert/strict";
import test from "node:test";

const source = fs.readFileSync(new URL("./DesktopPage.js", import.meta.url), "utf8").replace(/^\.pragma library\s*/, "");
const context = {};
vm.createContext(context);
vm.runInContext(source, context);
const rows = context.rows;

test("reload cover is a brand-backed General setting before Sidebars", () => {
    const index = rows.findIndex(row => row.key === "reloadCover");
    assert.ok(index >= 0);
    const row = rows[index];
    // what the Hub acts on: the row is a brand-backed reload-cover control on
    // General, worded for a user and short enough to read as one line
    assert.equal(row.tab, "General");
    assert.equal(row.group, "SHELL RELOAD");
    assert.equal(row.ctl, "reload-cover");
    assert.equal(row.src, "brand");
    assert.ok(row.label.length > 0);
    assert.ok(row.desc.length > 0 && row.desc.length <= 60, `desc is ${row.desc.length} chars`);
    // and where it sits: after the marking rows, before the Sidebars tab
    assert.equal(rows[index - 1].key, "markTint");
    assert.equal(rows[index + 1].tab, "Sidebars");
    assert.equal(rows[index + 1].key, "sidebars.width");
});

test("Sidebars exposes the complete shell-backed panel and card settings", () => {
    const sidebarRows = rows.filter(row => row.tab === "Sidebars");
    assert.deepEqual(Array.from(sidebarRows, row => [row.key, row.group, row.ctl]), [
        ["sidebars.width", "PANEL", "step"],
        ["sidebars.motion", "PANEL", "seg"],
        ["sidebars.depth", "PANEL", "sw"],
        ["sidebars.push", "PANEL", "sw"],
        ["sidebars.wallpaperSlide", "PANEL", "slid"],
        ["sidebars.left.enabled", "LEFT SIDEBAR", "sw"],
        ["sidebars.left.cards", "LEFT SIDEBAR", "multi"],
        ["sidebars.right.enabled", "RIGHT SIDEBAR", "sw"],
        ["sidebars.right.cards", "RIGHT SIDEBAR", "multi"]
    ]);
    for (const row of sidebarRows) {
        assert.equal(row.src, "shell", `${row.key} writes shell settings`);
        assert.ok(row.label && row.label.length > 0, `${row.key} has a label`);
        assert.ok(row.desc && row.desc.length > 0, `${row.key} has a description`);
    }

    const byKey = key => sidebarRows.find(row => row.key === key);
    assert.deepEqual(
        [byKey("sidebars.width").lo, byKey("sidebars.width").hi, byKey("sidebars.width").unit],
        [280, 560, "px"]
    );
    assert.deepEqual(Array.from(byKey("sidebars.motion").opts), ["quick", "standard", "calm"]);
    assert.deepEqual(
        [byKey("sidebars.wallpaperSlide").lo, byKey("sidebars.wallpaperSlide").hi],
        [1, 1.4]
    );
    assert.deepEqual(Array.from(byKey("sidebars.left.cards").opts),
        ["system", "notifications", "weather", "media", "capture", "stage"]);
    assert.deepEqual(Array.from(byKey("sidebars.right.cards").opts), ["usage", "tools", "chat"]);
});

test("retired quick-settings rows are absent", () => {
    assert.equal(rows.some(row => String(row.key).startsWith("frameBars.menus.quick-settings.")), false);
});

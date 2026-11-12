import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const Model = require("./BarStudioModel.js");
const FrameBars = require("../../../../shell/framebars/FrameBars.js");
const BarCatalog = require("../../../../shell/framebars/BarCatalog.js");
const MenuCatalog = require("../../../../shell/framebars/MenuCatalog.js");

let failed = 0;
function ok(value, message) {
    if (!value) {
        console.error(`FAIL: ${message}`);
        failed++;
    }
}
function eq(actual, expected, message) {
    ok(JSON.stringify(actual) === JSON.stringify(expected), `${message}; got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
}
function fresh(before, after, message) {
    ok(before !== after, `${message} returns a fresh root`);
}

// A controlled fixture: the shipped config with every rail zone emptied, so the
// zone assertions never couple to the default rail contents (which the shell's
// FrameBars.js owns and may retune). defaultConfig still supplies the whole
// shape, so the subtree checks stay honest.
function cfg() {
    const c = FrameBars.defaultConfig(MenuCatalog);
    for (const e of ["top", "left", "bottom", "right"]) for (const z of Model.zones(e)) c.rails[e][z] = [];
    return c;
}

// ── add ──────────────────────────────────────────────────────────────────
const base = cfg();
const added = Model.addZoneItem(base, "top", "start", "battery", BarCatalog);
fresh(base, added, "add zone item");
eq(base.rails.top.start, [], "add leaves source zone unchanged");
eq(added.rails.top.start, ["battery"], "add places compatible widget in the named zone");

// add targets the named zone, not always the first
const addedEnd = Model.addZoneItem(base, "top", "end", "battery", BarCatalog);
eq(addedEnd.rails.top.end, ["battery"], "add honours the target zone");
eq(addedEnd.rails.top.start, [], "add does not touch other zones");

// an incompatible-axis widget is a clean no-op (dock is vertical-only)
eq(Model.addZoneItem(base, "top", "start", "dock", BarCatalog), base, "add rejects a widget that does not fit the rail axis");

// a widget already on the rail (any zone) cannot be added again to that rail
const dockLeft = Model.addZoneItem(base, "left", "top", "dock", BarCatalog);
eq(dockLeft.rails.left.top, ["dock"], "add places a vertical widget on a vertical rail");
eq(Model.addZoneItem(dockLeft, "left", "bottom", "dock", BarCatalog), dockLeft, "add rejects a widget already present elsewhere on the same rail");
eq(Model.addZoneItem(added, "top", "start", "battery", BarCatalog), added, "add rejects a duplicate within the same zone");

// an unknown id is a clean no-op
eq(Model.addZoneItem(base, "top", "start", "not-a-widget", BarCatalog), base, "add rejects an uncatalogued id");

// ── reorder (within a zone) ────────────────────────────────────────────────
const two = Model.addZoneItem(added, "top", "start", "clock", BarCatalog);
eq(two.rails.top.start, ["battery", "clock"], "second add appends within the zone");
const swapped = Model.reorderZoneItem(two, "top", "start", 0, 1);
fresh(two, swapped, "reorder within zone");
eq(two.rails.top.start, ["battery", "clock"], "reorder leaves source untouched");
eq(swapped.rails.top.start, ["clock", "battery"], "reorder moves the item to the target index");
eq(Model.reorderZoneItem(two, "top", "start", 5, 0), two, "reorder rejects an out-of-range index");

// ── remove ─────────────────────────────────────────────────────────────────
const removed = Model.removeZoneItem(two, "top", "start", 0);
fresh(two, removed, "remove zone item");
eq(two.rails.top.start, ["battery", "clock"], "remove leaves source list unchanged");
eq(removed.rails.top.start, ["clock"], "remove deletes the selected item");

// ── rail settings ───────────────────────────────────────────────────────────
const thicker = Model.setRail(base, "left", { size: 64 });
fresh(base, thicker, "set rail size");
eq(thicker.rails.left.size, 64, "set rail applies a rounded size");
eq(Model.setRail(base, "left", { enabled: false }).rails.left.enabled, false, "set rail toggles enabled");
eq(Model.setRail(base, "left", { reveal: false }).rails.left.reveal, false, "set rail toggles reveal");

// ── railWidgets helper ────────────────────────────────────────────────────────
const packed = Model.addZoneItem(Model.addZoneItem(dockLeft, "left", "bottom", "clock", BarCatalog), "left", "bottom", "battery", BarCatalog);
eq(Model.railWidgets(packed, "left"), ["dock", "clock", "battery"], "railWidgets concatenates a rail's zones in order");

// ── subtree preservation ─────────────────────────────────────────────────────
// every mutation clones the whole current config, so an edit can never drop
// the menus or surfaces subtrees. Nested dock state is retired; a legacy copy
// survives normalization only until doctor moves its pins to the universal
// top-level dock store.
const shipped = FrameBars.defaultConfig(MenuCatalog);
const subtrees = ["version", "style", "rails", "menus", "surfaces"];
function preservesAll(config, label) {
    for (const key of subtrees) ok(config[key] !== undefined, `${label} preserves the ${key} subtree`);
}
ok(shipped.dock === undefined, "the current frameBars schema has no nested dock store");
const legacyDock = FrameBars.normalize({ ...shipped, dock: { pinned: ["org.kde.dolphin", 42] } }, BarCatalog, MenuCatalog);
eq(legacyDock.dock, { pinned: ["org.kde.dolphin"] }, "normalization keeps valid legacy pins for doctor migration");
preservesAll(Model.setRail(shipped, "left", { size: 64 }), "a rail thickness edit");
preservesAll(Model.setRail(shipped, "top", { reveal: false }), "a rail visibility edit");
preservesAll(Model.addZoneItem(base, "left", "top", "vpn", BarCatalog), "a zone widget add");
preservesAll(Model.removeZoneItem(dockLeft, "left", "top", 0), "a zone widget remove");
preservesAll(Model.reorderZoneItem(two, "top", "start", 0, 1), "a zone widget reorder");
// the menus and surfaces the page no longer edits still ride through untouched
eq(Model.setRail(shipped, "left", { size: 64 }).menus, shipped.menus, "a rail edit carries the menus subtree verbatim");
eq(Model.addZoneItem(base, "top", "start", "battery", BarCatalog).surfaces, base.surfaces, "a zone add carries the surfaces subtree verbatim");

if (failed > 0) {
    console.error(`\n${failed} test(s) FAILED`);
    process.exit(1);
}
console.log("\nAll tests PASSED");

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import vm from "node:vm";

const Geometry = createRequire(import.meta.url)("../lib/list.js");
const source = readFileSync(new URL("./ResultList.qml", import.meta.url), "utf8");
function method(name) {
    const start = source.indexOf(`function ${name}(`);
    assert.ok(start >= 0, `${name} is implemented by the real QML list`);
    let end = source.indexOf("{", start) + 1;
    let depth = 1;
    while (depth) {
        if (source[end] === "{") depth++;
        if (source[end] === "}") depth--;
        end++;
    }
    return source.slice(start, end);
}
const long = Geometry.build([{ id: "all", title: "Settings", rows: Array.from({ length: 80 }, (_, i) => ({ id: `row-${i}` })) }], { scale: 1 });
const short = Geometry.build([{ id: "filtered", title: "History", rows: [{ id: "row-79" }] }], { scale: 1 });
function list(height, duration = 140) {
    const callbacks = new Set();
    const flick = { contentY: 0, height };
    const root = { layout: long, scrollLayout: long, selectedId: "row-0", held: true, s: 1 };
    Object.defineProperty(root, "count", { get: () => root.layout.rows.length });
    Object.defineProperty(root, "selectedIndex", { get: () => Geometry.indexOf(root.layout.rows, root.selectedId) });
    const glide = {
        running: false, duration, from: 0, to: 0,
        stop() { this.running = false; },
        start() { this.running = this.duration > 0; if (!this.running) flick.contentY = this.to; }
    };
    const ctx = vm.createContext({ root, flick, glide, Geometry, Math, String, Qt: { callLater: fn => callbacks.add(fn) } });
    for (const name of ["samePositions", "settleScroll", "reconcile", "select", "reveal", "move", "page", "selectFirst"])
        root[name] = vm.runInContext(`(${method(name)})`, ctx);
    function flush() { for (const fn of callbacks) fn(); callbacks.clear(); }
    function finish() { if (glide.running) flick.contentY = glide.to; glide.stop(); }
    function visible() {
        const row = root.layout.rows[root.selectedIndex];
        return row && row.y >= flick.contentY && row.y + row.h <= flick.contentY + flick.height;
    }
    return { root, flick, glide, flush, finish, visible };
}
for (const height of [690, 590]) {
    const view = list(height);
    view.root.move(1000); view.finish(); assert.equal(view.root.selectedId, "row-79"); assert.ok(view.visible(), "last row is visible");
    view.root.selectFirst();
    for (let i = 0; i < 100; i++) view.root.page(1);
    view.finish(); assert.ok(view.visible(), "repeated PgDn stops with the last row visible");
    view.root.selectFirst();
    view.root.move(1000); assert.ok(view.glide.running);
    view.flick.contentY = 400;
    view.root.layout = short; view.root.reconcile();
    assert.equal(view.glide.running, false, "a new layout stops the obsolete glide immediately");
    view.flush(); assert.equal(view.flick.contentY, 0); assert.ok(view.visible(), "filtering mid-glide keeps the surviving selected row in view");
    view.root.layout = long; view.root.reconcile(); view.flush(); view.root.move(-1000); view.finish();
    assert.ok(view.visible(), "restoring the long list and returning to its first row stays in view");
    view.root.layout = short; view.root.selectedId = "missing"; view.root.reconcile(); view.flush();
    assert.equal(view.root.selectedId, "row-79"); assert.ok(view.visible(), "a removed selection chooses and reveals the first surviving row");
    view.root.layout = Geometry.build([], {}); view.root.reconcile(); view.flush();
    assert.equal(view.root.selectedId, ""); assert.equal(view.flick.contentY, 0);
}
const resized = list(690); resized.root.move(1000); resized.finish(); resized.flick.height = 590; resized.root.reconcile(true); resized.flush(); assert.ok(resized.visible(), "a smaller viewport keeps the last row visible");
const still = list(590, 0); still.root.move(1000); assert.equal(still.glide.running, false); assert.ok(still.visible(), "zero-duration motion jumps straight to the selected row");
const live = list(590);
const updated = Geometry.build([{ id: "all", title: "Settings", rows: Array.from({ length: 80 }, (_, i) => ({ id: `row-${i}`, title: `Updated ${i}` })) }], { scale: 1 });
live.flick.contentY = 200;
live.root.layout = updated; live.root.reconcile(); live.flush();
assert.equal(live.flick.contentY, 200, "value-only updates preserve manual scrolling");
live.root.move(1000); live.root.layout = long; live.root.reconcile();
assert.equal(live.glide.running, true, "value-only updates leave a valid glide running");
live.finish(); assert.ok(live.visible());
console.log("ResultList: last row, PgDn, layout shrink during glide, removed selection, empty list, resize, value-only updates and reduced motion pass");

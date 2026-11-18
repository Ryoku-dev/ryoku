import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import vm from "node:vm";

const source = fs.readFileSync(new URL("SettingsPage.qml", import.meta.url), "utf8");
function method(name, context) {
    const start = source.indexOf(`    function ${name}(`);
    assert.notEqual(start, -1);
    const end = source.indexOf("\n    }", start) + 6;
    vm.runInContext(source.slice(start, end), context);
    return context[name];
}
function fixture() {
    const page = {
        visible: true, ready: true, stale: false,
        schema: { sections: [{ id: "one", title: "One" }, { id: "two", title: "Two" }] },
        sectionData: {}, layoutSignature: "", bodies: {}, footers: {}, moreBodies: {}, slabBodies: {},
        hub: { rowAvailable: () => true, unhonoredReason: () => "" },
        whenMet: () => true, syncSections: () => {},
        rows: [{ id: "a", section: "one", label: "A" }, { id: "b", section: "two", label: "B", detail: "Old" }],
        allRows() { return this.rows; }
    };
    return { page, context: vm.createContext({ page }) };
}

test("status refreshes preserve unrelated section models and open editors", () => {
    const { page, context } = fixture();
    const rebuild = method("rebuild", context);
    rebuild();
    const first = page.sectionData.one;
    const second = page.sectionData.two;
    page.rows = page.rows.map(row => ({ ...row, detail: row.id === "b" ? "New" : row.detail }));
    rebuild();
    assert.equal(page.sectionData.one, first);
    assert.equal(page.sectionData.one.rows, first.rows);
    assert.notEqual(page.sectionData.two, second);
    assert.equal(page.sectionData.two.rows[0].detail, "New");
});

test("hidden schema changes defer all row work until shown", () => {
    const { page, context } = fixture();
    const rebuild = method("rebuild", context);
    rebuild();
    const oldLayout = page.layout;
    page.visible = false;
    page.allRows = () => { throw new Error("hidden row work"); };
    rebuild();
    assert.equal(page.stale, true);
    assert.equal(page.layout, oldLayout);
});

test("search and outline landings finish preceding sections in order", () => {
    const { page, context } = fixture();
    const loaded = [];
    page.layout = [{ id: "one", rows: [{ id: "a" }] }, { id: "two", rows: [{ id: "b" }] }, { id: "three", rows: [] }];
    page.sectionFrontier = 1;
    page.sectionItems = { one: {}, two: {} };
    page.rowItems = { a: { rowControl: { ready: true } }, b: { rowControl: { ready: true } } };
    context.Loader = { Ready: 1 };
    page.sectionLoaders = Object.fromEntries(page.layout.map(({ id }) => [id, {
        status: 1,
        set asynchronous(value) { assert.equal(value, false); loaded.push(id); }
    }]));
    assert.equal(method("finishSections", context)("two"), true);
    assert.deepEqual(loaded, ["one", "two"]);
    assert.equal(page.sectionFrontier, 2);
    assert.equal(method("finishSections", context)("missing"), false);
    assert.deepEqual(loaded, ["one", "two"]);
});

test("landing waits for nested rows even after the section Loader is ready", () => {
    const { page, context } = fixture();
    page.layout = [{ id: "one", rows: [{ id: "a" }] }];
    page.sectionItems = { one: {} };
    page.rowItems = {};
    page.sectionLoaders = { one: { status: 1, asynchronous: true } };
    context.Loader = { Ready: 1 };
    const finish = method("finishSections", context);
    assert.equal(finish("one"), false);
    assert.equal(page.sectionLoaders.one.requested, true);
    assert.equal(page.sectionLoaders.one.asynchronous, false);
    page.rowItems.a = { rowControl: { ready: false } };
    assert.equal(finish("one"), false);
    page.rowItems.a.rowControl.ready = true;
    assert.equal(finish("one"), true);
});

test("layout flushing skips closed menus and places visible descendants before parents", () => {
    const { page, context } = fixture();
    const order = [];
    page.flushLayout = method("flushLayout", context);
    page.flushLayout({
        visible: true,
        children: [
            { visible: false, forceLayout() { throw new Error("closed menu"); } },
            { visible: true, children: [], forceLayout() { order.push("child"); } }
        ],
        forceLayout() { order.push("parent"); }
    });
    assert.deepEqual(order, ["child", "parent"]);
});

test("an unfinished tail is completed before a landing would hit the old bottom", () => {
    const { page, context } = fixture();
    page.layout = [{ id: "one", rows: [] }, { id: "two", rows: [] }];
    page.rowItems = {};
    page.sectionLoaders = { one: { status: 1 }, two: { status: 0 } };
    context.Loader = { Ready: 1 };
    context.flick = { contentItem: {}, contentHeight: 600, height: 500 };
    context.body = {};
    const finished = [];
    page.finishSections = id => {
        finished.push(id);
        page.sectionLoaders.two.status = 1;
        return true;
    };
    page.flushLayout = () => { context.flick.contentHeight = 1400; };
    const ready = method("prepareLanding", context)({ mapToItem() { return { y: 800 }; } }, 100);
    assert.equal(ready, true);
    assert.deepEqual(finished, ["two"]);
});

test("late control height changes preserve the landed row's viewport position", () => {
    const { page, context } = fixture();
    page.landingAnchor = { mapToItem() { return { y: 900 }; } };
    page.landingOffset = 200;
    context.flick = { contentItem: {}, contentHeight: 1800, height: 600, contentY: 726, moving: false };
    context.scrollAnim = { running: false };
    context.body = { forceLayout() {} };
    method("correctLanding", context)();
    assert.equal(context.flick.contentY, 700);
    assert.equal(page.heldY, 700);
});

test("first-paint deferral never blocks an explicit landing or destroys cached sections", () => {
    const binding = source.match(/active: (kept \|\|[\s\S]*?)\n\s+onLoaded:/)[1];
    const page = { visible: true, asynchronousSections: true, firstSectionPainted: false, sectionFrontier: 2 };
    const context = { page, kept: false, requested: false, index: 1 };
    const active = () => vm.runInNewContext(binding, context);
    assert.equal(active(), false);
    context.index = 0;
    assert.equal(active(), true);
    context.index = 1;
    context.requested = true;
    assert.equal(active(), true);
    context.requested = false;
    page.firstSectionPainted = true;
    assert.equal(active(), true);
    page.visible = false;
    assert.equal(active(), false);
    context.kept = true;
    assert.equal(active(), true);
});

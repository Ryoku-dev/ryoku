import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

// load summaries.js the way the QML engine links it
const here = path.dirname(fileURLToPath(import.meta.url));
const S = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(here, "summaries.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), S);

const dig = (obj, key) => key.split(".").reduce((o, p) => (o == null ? undefined : o[p]), obj);

// a window manager that draws blur (or not), Window style's desktop.json and shell.json
function ctx({ drawsBlur, desktop, settings }) {
    return {
        tr: s => s,
        desktopReady: true,
        desktop: k => dig(desktop, k),
        settings: k => dig(settings, k),
        models: key => key !== "desktop.appearance.blurEnabled" || drawsBlur,
    };
}

const thisMachine = { appearance: { rounding: 8, borderSize: 2, blurEnabled: false } };

test("window blur shows only where the window manager draws it; panel glass shows everywhere", () => {
    assert.equal(S.pageSummary("window-style", ctx({ drawsBlur: true, desktop: thisMachine, settings: {} })),
        "8 px corners, 2 px border, window blur off, panel glass on");
    assert.equal(S.pageSummary("window-style", ctx({ drawsBlur: false, desktop: thisMachine, settings: {} })),
        "8 px corners, 2 px border, panel glass on");
    assert.equal(S.pageSummary("window-style", ctx({ drawsBlur: false, desktop: thisMachine,
        settings: { theme: { effects: { glass: false } } } })), "8 px corners, 2 px border, panel glass off");
});

test("the Appearance sentence names window blur where it exists and the panels' glass on its own", () => {
    assert.equal(S.categorySentence("appearance", ctx({ drawsBlur: true, desktop: thisMachine, settings: {} })),
        "Windows have 8 px corners and no blur. Panels and the dock are frosted.");
    assert.equal(S.categorySentence("appearance", ctx({ drawsBlur: false, desktop: thisMachine, settings: {} })),
        "Windows have 8 px corners. Panels and the dock are frosted.");
    assert.equal(S.categorySentence("appearance", ctx({ drawsBlur: true,
        desktop: { appearance: { rounding: 0, blurEnabled: true } }, settings: { theme: { effects: { glass: false } } } })),
        "Windows have 0 px corners and blur behind them. Panels and the dock are not frosted.");
});

test("the Game bar line says off, or how far it dims the game, and waits for shell.json", () => {
    const line = settings => S.pageSummary("game-bar", ctx({ drawsBlur: false, desktop: {}, settings }));
    assert.equal(line({}), "Overlay, snapping and voice");
    assert.equal(line({ gamebar: { enabled: false, dim: 0.62 } }), "Off");
    assert.equal(line({ gamebar: { enabled: true, dim: 0.62 } }), "On, dims the game by 62%");
    assert.equal(line({ gamebar: { enabled: true, dim: 0 } }), "On, the game is not dimmed");
    assert.equal(line({ gamebar: { enabled: true } }), "On, dims the game by 62%");
});

// registry.js, linked the same way
const R = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(here, "registry.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), R);

test("every listed page is listed once and has an entry in its category, and every alias resolves", () => {
    const ids = R.allPageIds();
    assert.equal(new Set(ids).size, ids.length);
    for (const id of ids)
        assert.equal(R.pages[id].category, R.categories.find(c => c.pages.includes(id)).id, id);
    assert.deepEqual(Object.keys(R.pages).sort(), [...ids].sort());
    for (const alias of Object.keys(R.aliases))
        assert.notEqual(R.resolve(alias), "", alias);
});

test("the Game bar page sits under Desktop and the old id still opens it, with its row", () => {
    assert.equal(R.page("game-bar").category, "desktop");
    assert.equal(R.resolve("gamebar"), "game-bar");
    assert.equal(R.resolve("gamebar/voice-names"), "game-bar");
    assert.equal(R.rowPart("gamebar/voice-names"), "voice-names");
});

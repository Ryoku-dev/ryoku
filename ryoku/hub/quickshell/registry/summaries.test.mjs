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

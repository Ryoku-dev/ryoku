import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

// load game-bar.js the way the QML engine links it
const here = path.dirname(fileURLToPath(import.meta.url));
const S = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(here, "game-bar.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), S);

// the daemon's ranges and defaults (ryoku/shell/ipc/settings.go gameBarSettings)
const daemon = {
    enabled: [true], dim: [0.62, 0, 1], fade: [200, 0, 2000], scale: [1, 0.5, 2],
    snap: [true], snapReach: [9, 0, 64], snapGrid: [24, 4, 128], guideGrid: [true], gridStep: [48, 8, 256],
    gridAlpha: [0.28, 0, 1], spring: [3.2, 0.5, 20], damping: [0.58, 0.05, 1], mass: [1, 0.1, 10],
    voiceAvatars: [8, 1, 8], voiceVertical: [false], voiceAlign: ["center"], voiceNamePosition: ["below"],
    voiceNameGap: [1, 0, 16], voiceNameChars: [12, 4, 32], voiceDim: [true], voiceDimOpacity: [0.5, 0.2, 1], voiceAvatarSize: [52, 32, 72],
    voiceNames: [true], voiceControls: [true], voiceBgOpacity: [0.72, 0, 1]
};

test("one row per gamebar key, with the daemon's default and range", () => {
    const bound = S.rows.filter(r => r.bind);
    assert.deepEqual([...bound.map(r => r.bind.key.replace("gamebar.", ""))].sort(), Object.keys(daemon).sort());
    for (const r of bound) {
        const [def, lo, hi] = daemon[r.bind.key.replace("gamebar.", "")];
        assert.equal(r.bind.store, "settings", r.id);
        assert.equal(r["default"], def, r.id);
        assert.equal(r.bind.fallback, def, r.id);
        if (lo !== undefined) {
            assert.equal(r.min, lo, r.id);
            assert.equal(r.max, hi, r.id);
        }
    }
    assert.equal(new Set(S.rows.map(r => r.id)).size, S.rows.length);
    for (const r of S.rows)
        assert.ok(S.sections.some(s => s.id === r.section), r.id);
});

test("the voice layout choice offers horizontal and vertical", () => {
    const row = S.row("voice-vertical");
    assert.equal(row.label, "Avatar layout");
    assert.equal(row.control, "segmented");
    assert.equal(JSON.stringify(row.options.map(o => [o.label, o.value])),
        JSON.stringify([["Horizontal", false], ["Vertical", true]]));

    const align = S.row("voice-align");
    assert.equal(align.label, "Avatar alignment");
    assert.equal(align.control, "segmented");
    assert.equal(JSON.stringify(align.options.map(o => [o.label, o.value])),
        JSON.stringify([["Center", "center"], ["Left", "left"], ["Right", "right"]]));

    const names = S.row("voice-name-position");
    assert.equal(names.label, "Name position");
    assert.equal(names.control, "segmented");
    assert.equal(JSON.stringify(names.options.map(o => [o.label, o.value])),
        JSON.stringify([["Below", "below"], ["Left", "left"], ["Right", "right"]]));

    const gap = S.row("voice-name-gap");
    assert.equal(gap.control, "stepper");
    assert.equal(gap.min, 0);
    assert.equal(gap.max, 16);

    const chars = S.row("voice-name-chars");
    assert.equal(chars.control, "stepper");
    assert.equal(chars.min, 4);
    assert.equal(chars.max, 32);
});

// a store of the given gamebar values over the defaults
const store = values => bind => {
    const key = bind.key.replace("gamebar.", "");
    return key in values ? values[key] : bind.fallback;
};
const shown = values => [...S.adapt(S.rows, store(values)).filter(r => {
    if (!r.when)
        return true;
    const v = store(values)(r.when.bind);
    return r.when.is.some(x => JSON.stringify(x) === JSON.stringify(v));
}).map(r => r.id)];

test("rows wait on the outermost switch that is off", () => {
    assert.equal(shown({}).length, S.rows.length);
    assert.deepEqual(shown({ enabled: false }), ["enabled", "shortcut"]);
    const noSnap = shown({ snap: false });
    for (const id of ["snap-reach", "snap-grid", "guide-grid", "grid-step", "grid-alpha"])
        assert.ok(!noSnap.includes(id), id);
    assert.ok(noSnap.includes("snap"));
    const noGuides = shown({ guideGrid: false });
    assert.ok(noGuides.includes("guide-grid") && !noGuides.includes("grid-step") && !noGuides.includes("grid-alpha"));
    assert.ok(!shown({ voiceDim: false }).includes("voice-dim-opacity"));
});

test("alignment and name position are hidden in a horizontal layout", () => {
    const horizontal = shown({ voiceVertical: false });
    assert.ok(!horizontal.includes("voice-name-position"));
    assert.ok(!horizontal.includes("voice-align"));
    const vertical = S.adapt(S.rows, store({ voiceVertical: true }));
    assert.equal(vertical.find(r => r.id === "voice-name-position").options.length, 2);
    assert.equal(JSON.stringify(vertical.find(r => r.id === "voice-align").options.map(o => o.value)),
        JSON.stringify(["left", "right"]));
});

test("adaptive rows carry no baked value", () => {
    // Ghost Button: a value frozen into the row goes stale when page.layout
    // is kept because only the value changed.
    for (const v of ["center", "left", "right"]) {
        const rows = S.adapt(S.rows, store({ voiceVertical: true, voiceAlign: v }));
        for (const r of rows) {
            assert.equal(r.value, undefined, r.id);
            assert.equal(r.valueAlias, undefined, r.id);
        }
    }
    assert.equal(S.adapt(S.rows, store({ voiceAlign: "left" }))
        .find(r => r.id === "voice-name-position").mirrorSide, true);
});

test("a side alignment mirrors the available name positions", () => {
    const names = values => S.adapt(S.rows, store(values))
        .find(r => r.id === "voice-name-position").options.map(o => o.value);
    assert.equal(JSON.stringify(names({ voiceAlign: "left" })),
        JSON.stringify(["below", "right"]));
    assert.equal(JSON.stringify(names({ voiceAlign: "right" })),
        JSON.stringify(["below", "left"]));
    // a stored Center acts as Left until a side is chosen
    assert.equal(JSON.stringify(names({ voiceAlign: "center" })),
        JSON.stringify(["below", "right"]));
});

test("a hidden row's condition names the switch a deep link lands on", () => {
    const step = values => S.adapt(S.rows, store(values)).find(r => r.id === "grid-step").when.bind.key;
    assert.equal(step({ enabled: false, snap: false, guideGrid: false }), "gamebar.enabled");
    assert.equal(step({ snap: false, guideGrid: false }), "gamebar.snap");
    assert.equal(step({ guideGrid: false }), "gamebar.guideGrid");
    assert.equal(step({}), "gamebar.guideGrid");
    // the schema's own rows stay as written
    assert.equal(S.row("grid-step").when.bind.key, "gamebar.guideGrid");
});

import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const directory = path.dirname(fileURLToPath(import.meta.url));
const root = path.dirname(directory);
const model = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(directory, "progression.js"), "utf8")
    .replace(/^(\.pragma|\.import) .*$/gm, ""), model);
const P = model;
const read = name => fs.readFileSync(path.join(root, name), "utf8");

test("long-term date history keeps the newest 366 valid entries", () => {
    const days = {};
    for (let i = 0; i < 370; ++i) {
        const date = new Date(Date.UTC(2025, 0, 1 + i));
        days[date.toISOString().slice(0, 10)] = true;
    }
    days.invalid = true;
    days["2026-02-01"] = false;
    const last = new Date(Date.UTC(2025, 0, 370)).toISOString().slice(0, 10);
    const state = P.clean({ version: 4, days, customDays: days });
    assert.equal(Object.keys(state.days).length, 366);
    assert.equal(state.days["2025-01-01"], undefined);
    assert.equal(state.days[last], true);
    assert.equal(state.customDays[last], true);
    const next = new Date(Date.UTC(2025, 0, 371)).toISOString();
    const updated = P.startDay(state, next).state;
    assert.ok(Object.keys(updated.days).length <= 366);
    assert.equal(updated.days[next.slice(0, 10)], true);
});

test("returning to the original background clears a personal image", () => {
    let state = P.customImage({}, "file:///tmp/personal.png").state;
    assert.equal(state.canvas.image, "file:///tmp/personal.png");
    const result = P.background(state, "");
    assert.equal(result.changed, true);
    assert.equal(result.state.canvas.image, "");
    assert.equal(result.state.canvas.background, "");
    assert.equal(P.background(result.state, "").changed, false);
});

test("the original background also clears purchased scenes", () => {
    let state = P.clean({ version: 4, owned: { "bg-graphite": true } });
    state = P.background(state, "bg-graphite").state;
    assert.equal(state.canvas.background, "bg-graphite");
    const restored = P.background(state, "");
    assert.equal(restored.changed, true);
    assert.equal(restored.state.canvas.background, "");
    assert.match(read("components/about/ProfileStudio.qml"),
        /onClicked: Progression\.setBackground\(""\)/);
});

test("achievement titles and descriptions reach the translation runtime", () => {
    const card = read("components/about/AchievementCard.qml");
    const reveal = read("components/about/AchievementReveal.qml");
    assert.match(card, /I18n\.tr\(card\.badge\.title\)/);
    assert.match(card, /I18n\.tr\(card\.badge\.detail\)/);
    assert.match(reveal, /I18n\.tr\(reveal\.badge\.title\)/);
});

test("shop preview colours follow Lithic rather than a second fixed palette", () => {
    const store = read("components/about/ProfileStudio.qml");
    assert.match(store, /readonly property color ground: Lithic\.mix\(Lithic\.ground, tint,/);
    assert.match(store, /modelData\.id === "bg-ember" \|\|/);
    assert.doesNotMatch(store, /modelData\.id === "bg-blueprint" \? "#/);
});

test("sticker artwork and contributor portraits stay bounded by display size", () => {
    const art = read("components/about/StickerArtwork.qml");
    const keepsake = read("pages/KeepsakePage.qml");
    assert.match(art, /sourceSize\.width: Math\.min\(512, Math\.max\(1, Math\.round\(root\.width \* 2\)\)\)/);
    assert.match(art, /sourceSize\.height: Math\.min\(512, Math\.max\(1, Math\.round\(root\.height \* 2\)\)\)/);
    assert.match(keepsake, /source: contributor\.modelData\.avatar\s+sourceSize\.width: Math\.max\(1, Math\.round\(width \* 2\)\)/);
});

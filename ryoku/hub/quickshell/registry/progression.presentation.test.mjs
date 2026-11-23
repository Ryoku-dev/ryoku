import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const P = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "progression.js"), "utf8")
    .replace(/^(\.pragma|\.import) .*$/gm, ""), P);
const serial = value => JSON.parse(JSON.stringify(value));

test("Black Prism is a secret free completion bonus, not a purchase", () => {
    assert.equal(P.cosmetics.length, 32);
    assert.equal(P.cosmetics.some(x => x.id === P.completionScene.id), false);
    assert.equal(P.completionScene.price, 0);
    assert.equal(P.completionSceneUnlocked({}), false);
    assert.equal(P.purchase({}, P.completionScene.id).changed, false);
    assert.equal(P.background({}, P.completionScene.id).changed, false);
});

test("owning all paid cosmetics automatically unlocks the free animated background", () => {
    const owned = Object.fromEntries(P.cosmetics.map(c => [c.id, true]));
    let state = P.clean({ version: 4, owned });
    assert.equal(state.earned["collection-everything"], true);
    assert.equal(P.completionSceneUnlocked(state), true);
    assert.equal(Object.keys(state.owned).length, 32);
    const equipped = P.background(state, P.completionScene.id);
    assert.equal(equipped.changed, true);
    assert.equal(equipped.state.canvas.background, P.completionScene.id);
    assert.equal(equipped.state.balance, state.balance);
    assert.equal(P.purchase(equipped.state, P.completionScene.id).changed, false);
    assert.deepEqual(serial(P.clean(equipped.state)), serial(equipped.state));
});

test("a partial collection cannot spoof access or preserve a locked background", () => {
    const owned = Object.fromEntries(P.cosmetics.slice(0, 31).map(c => [c.id, true]));
    const state = P.clean({ version: 4, owned,
        canvas: { background: P.completionScene.id } });
    assert.equal(state.earned["collection-everything"], undefined);
    assert.equal(state.canvas.background, "");
    assert.equal(P.background(state, P.completionScene.id).changed, false);
});

test("legacy complete collections receive the free bonus without spending", () => {
    const owned = Object.fromEntries(P.cosmetics.map(c => [c.id, true]));
    const migrated = P.clean({ version: 3, owned });
    assert.equal(P.completionSceneUnlocked(migrated), true);
    assert.equal(P.background(migrated, P.completionScene.id).changed, true);
});

test("mapmaker requires eight unique areas as well as 40 distinct pages", () => {
    let state = P.clean({});
    const date = "2026-11-19T15:00:00";
    for (let i = 0; i < 40; ++i)
        state = P.visit(state, "path-" + i,
            ["appearance", "desktop", "windows", "devices", "connections", "apps"][i % 6], date).state;
    assert.equal(state.earned["ryoku-cartographer"], true);
    assert.equal(state.earned["ryoku-mapmaker"], undefined);
    state = P.visit(state, "path-40", "accounts", date).state;
    assert.equal(state.earned["ryoku-mapmaker"], undefined);
    state = P.visit(state, "path-41", "system", date).state;
    assert.equal(state.earned["ryoku-mapmaker"], true);
});

test("Variety Show requires two badges plus three different shop stickers", () => {
    let state = P.clean({ version: 4, earned: { welcome: true, explorer: true },
        claimed: { welcome: true, explorer: true },
        owned: { "sticker-orbit": true, "sticker-flame": true, "sticker-bloom": true } });
    for (const id of ["welcome", "explorer", "sticker-orbit", "sticker-flame"])
        state = P.addSticker(state, id).state;
    assert.equal(state.earned["profile-variety"], undefined);
    state = P.addSticker(state, "sticker-bloom").state;
    assert.equal(state.earned["profile-variety"], true);
});

test("The Long Game is not a duplicate of 14-day Habit Formed", () => {
    let state = P.clean({});
    for (let day = 1; day <= 14; ++day)
        state = P.startDay(state, "2026-11-" + String(day).padStart(2, "0") + "T12:00:00").state;
    assert.equal(state.earned["ryoku-two-weeks"], true);
    assert.equal(state.earned["secret-timekeeper"], undefined);
    for (let day = 15; day <= 21; ++day)
        state = P.startDay(state, "2026-11-" + day + "T12:00:00").state;
    assert.equal(state.earned["secret-timekeeper"], true);
});

test("progressInfo returns real measurements but keeps locked secrets unlisted", () => {
    const zero = P.progressInfo({}, "ryoku-thirty");
    assert.equal(zero.current, 0);
    assert.equal(zero.total, 30);
    assert.equal(zero.fraction, 0);
    assert.equal(P.progressInfo({}, "secret-absolute"), null);
    assert.equal(P.progressInfo({}, "ryoku-first-steps"), null);
    let state = P.startDay({}, "2026-11-19T13:00:00").state;
    const measure = P.progressInfo(state, "ryoku-thirty");
    assert.equal(measure.current, 1);
    assert.equal(measure.total, 30);
    assert.ok(measure.fraction > 0 && measure.fraction < 1);
});

test("collectible card and claim reveal respect reduced motion and preserve hidden entry privacy", () => {
    const components = path.join(dir, "../components/about");
    const card = fs.readFileSync(path.join(components, "AchievementCard.qml"), "utf8");
    const reveal = fs.readFileSync(path.join(components, "AchievementReveal.qml"), "utf8");
    const book = fs.readFileSync(path.join(components, "ProgressionTab.qml"), "utf8");
    assert.match(card, /ArchiveSeal \{/);
    assert.match(card, /Progression\.progressInfoFor/);
    assert.match(card, /badge.category === "Classified"/);
    assert.match(reveal, /Lithic\.reduceMotion/);
    assert.match(reveal, /Keys\.onEscapePressed/);
    assert.match(book, /Progression\.visibleBadges/);
    assert.match(book, /AchievementCard \{/);
});

test("free animated Black Prism uses gated Home rendering and a static shop preview", () => {
    const components = path.join(dir, "../components");
    const scene = fs.readFileSync(path.join(components, "about/BlackPrism.qml"), "utf8");
    const home = fs.readFileSync(path.join(components, "Home.qml"), "utf8");
    const studio = fs.readFileSync(path.join(components, "about/ProfileStudio.qml"), "utf8");
    assert.match(scene, /live && visible && !Lithic\.reduceMotion/);
    assert.match(scene, /AnimatedImage \{/);
    assert.match(scene, /black-prism.gif/);
    assert.match(home, /profileBackground\.chosen === Progression\.completionScene\.id/);
    assert.match(studio, /Progression\.completionSceneUnlocked/);
    assert.match(studio, /BlackPrism \{/);
    assert.match(studio, /live: false/);
    assert.doesNotMatch(studio, /LithicTokens\.cut\.medium/);
});

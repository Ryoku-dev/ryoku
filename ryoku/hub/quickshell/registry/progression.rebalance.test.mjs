import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const model = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "progression.js"), "utf8")
    .replace(/^(\.pragma|\.import) .*$/gm, ""), model);
const P = model;

function date(i, time = "12:00:00") {
    return "2026-11-" + String(i).padStart(2, "0") + "T" + time;
}

test("four category mastery achievements and a new holographic endgame exist", () => {
    for (const id of ["master-ryoku", "master-profile", "master-card", "master-collection"]) {
        assert.equal(P.achievement(id).category, "Mastery");
        assert.equal(P.achievement(id).rarity, "legendary");
        assert.equal(P.unlock({}, id).changed, false);
    }
    assert.equal(P.achievement("master-hyperion").rarity, "holographic");
    assert.equal(P.achievement("secret-absolute").rarity, "holographic");
    assert.equal(P.achievement("secret-absolute").secret, true);
});

test("an old completed archive remains earned without awarding the new 100%", () => {
    const old = { version: 3, earned: { "master-archive": true, "secret-totality": true },
                  claimed: { "master-archive": true, "secret-totality": true } };
    const migrated = P.clean(old);
    assert.equal(migrated.earned["master-archive"], true);
    assert.equal(migrated.earned["secret-totality"], true);
    assert.equal(migrated.earned["secret-absolute"], undefined);
    assert.equal(migrated.earned["master-hyperion"], undefined);
    assert.equal(migrated.balance, P.achievement("master-archive").reward + P.achievement("secret-totality").reward);
});

test("one hundred rapid Hub openings still count as one calendar day", () => {
    let state = P.clean({});
    for (let i = 0; i < 100; ++i)
        state = P.startDay(state, date(1)).state;
    assert.equal(Object.keys(state.days).length, 1);
    assert.equal(state.earned["ryoku-steadfast"], undefined);
    assert.equal(state.earned["ryoku-thirty"], undefined);
    assert.equal(state.earned["master-ryoku"], undefined);
    for (let i = 2; i <= 29; ++i)
        state = P.startDay(state, date(i)).state;
    assert.equal(state.earned["ryoku-two-weeks"], true);
    assert.equal(state.earned["ryoku-thirty"], undefined);
    state = P.startDay(state, date(30)).state;
    assert.equal(state.earned["ryoku-thirty"], true);
    assert.equal(P.progress(state, "ryoku-thirty"), "");
});

test("Points Shop rewards require separate calendar visits", () => {
    let state = P.clean({});
    for (let i = 1; i <= 7; ++i)
        state = P.visit(state, "points-shop", "accounts", date(i)).state;
    assert.equal(state.earned["collection-patron"], true);
    assert.equal(Object.keys(state.shopDays).length, 7);
    assert.equal(Object.keys(state.visited).length, 1);
    assert.equal(P.visit(state, "points-shop", "accounts", date(7)).changed, false);
});

test("only real changes count toward profile customisation days", () => {
    let state = P.claim(P.unlock({}, "welcome").state, "welcome").state;
    state = P.addSticker(state, "welcome").state;
    assert.equal(Object.keys(state.customDays).length, 1);
    assert.equal(P.moveSticker(state, "st1", 0.56, 0.65, 82, 0).changed, false);
    assert.equal(Object.keys(state.customDays).length, 1);
    assert.equal(state.earned["profile-week"], undefined);
});

test("the hidden Keepsake is rewarded exactly once", () => {
    const first = P.keepsake({});
    assert.equal(first.changed, true);
    assert.equal(first.state.keepsakeSeen, true);
    assert.equal(first.state.earned["secret-keepsake"], true);
    assert.equal(first.state.claimed["secret-keepsake"], undefined);
    assert.equal(P.keepsake(first.state).changed, false);
    assert.equal(P.unlock({}, "secret-keepsake").changed, false);
});

test("Classified is never disclosed before discovery", () => {
    const locked = P.visibleAchievements({});
    assert.equal(locked.some(a => a.id === "secret-absolute"), false);
    assert.equal(locked.some(a => a.id === "secret-keepsake"), false);
    const unlocked = P.visibleAchievements(P.keepsake({}).state, "Classified");
    assert.deepEqual(Array.from(unlocked, a => a.id), ["secret-keepsake"]);
});

test("all owned pre-rebalance items keep their actual legacy paid prices", () => {
    const owned = { midnight: true, "sticker-orbit": true, "bg-blueprint": true, "font-mono": true };
    const claimed = { welcome: true, explorer: true, identity: true, portrait: true };
    const migrated = P.clean({ version: 3, earned: claimed, claimed, owned });
    const earned = ["welcome", "explorer", "identity", "portrait"].reduce(
        (sum, id) => sum + P.achievement(id).reward, 0);
    const paid = Object.keys(owned).reduce((sum, id) => sum + P.legacyPrices[id], 0);
    assert.equal(migrated.balance, Math.max(0, earned - paid));
    for (const id of Object.keys(owned))
        assert.equal(migrated.paid[id], P.legacyPrices[id]);
    assert.deepEqual(JSON.parse(JSON.stringify(P.clean(migrated))), JSON.parse(JSON.stringify(migrated)));
});

test("mastery can't be completed merely from repeating identical stickers", () => {
    let state = P.claim(P.unlock({}, "welcome").state, "welcome").state;
    for (let i = 0; i < 24; i++)
        state = P.addSticker(state, "welcome").state;
    assert.equal(state.earned["profile-sanctuary"], undefined);
    assert.equal(state.earned["profile-curation"], undefined);
    assert.equal(state.earned["secret-twelve"], undefined);
    assert.equal(state.earned["master-profile"], undefined);
});

test("master achievements unlock automatically from genuinely completed category records", () => {
    const earned = {};
    for (const a of P.achievements)
        if (a.category === "Ryoku") earned[a.id] = true;
    const state = P.clean({ version: 4, earned });
    assert.equal(state.earned["master-ryoku"], true);
    assert.equal(state.earned["master-profile"], undefined);
    assert.equal(state.earned["master-hyperion"], undefined);
});

test("the catalogue remains finite and has enough overall rewards", () => {
    const totalRewards = P.achievements.reduce((sum, a) => sum + a.reward, 0);
    const totalCost = P.cosmetics.reduce((sum, c) => sum + c.price, 0);
    assert.ok(totalCost > 9000, "Premium shop costs must meaningfully exceed alpha costs");
    assert.ok(totalRewards >= totalCost);
    for (const c of P.cosmetics) assert.ok(c.price > 0);
    assert.equal(new Set(P.cosmetics.map(c => c.id)).size, P.cosmetics.length);
});

test("visible progress gives honest counts without revealing classified challenges", () => {
    const initial = P.clean({});
    assert.equal(P.progress(initial, "ryoku-thirty"), "0 / 30");
    assert.equal(P.progress(initial, "master-ryoku").endsWith("/ 11"), true);
    assert.equal(P.progress(initial, "secret-absolute"), "");
    const oneDay = P.startDay(initial, date(1)).state;
    assert.equal(P.progress(oneDay, "ryoku-thirty"), "1 / 30");
});

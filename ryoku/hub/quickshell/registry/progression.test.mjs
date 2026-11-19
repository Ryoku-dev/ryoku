import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const P = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "progression.js"), "utf8").replace(/^\.(pragma|import) .*$/gm, ""), P);

test("milestones award once even when the same action repeats", () => {
    let state = P.clean({});
    let first = P.unlock(state, "welcome");
    assert.equal(first.changed, true);
    assert.equal(first.state.balance, 50);
    let again = P.unlock(first.state, "welcome");
    assert.equal(again.changed, false);
    assert.equal(again.state.balance, 50);
    assert.equal(P.unlock(state, "nonexistent").changed, false);
});

test("shop purchases are affordable and repeat purchases do not charge twice", () => {
    let state = P.unlock({}, "welcome").state;
    assert.equal(P.purchase(state, "midnight").changed, false);
    for (const id of ["explorer", "identity"])
        state = P.unlock(state, id).state;
    assert.equal(state.balance, 230);
    let bought = P.purchase(state, "midnight");
    assert.equal(bought.changed, true);
    assert.equal(bought.state.balance, 80);
    assert.equal(P.purchase(bought.state, "midnight").changed, false);
    assert.equal(P.equip(bought.state, "ember").changed, false);
    assert.equal(P.equip(bought.state, "midnight").state.active, "midnight");
});

test("card milestones retain existing rewards while global badges add more", () => {
    let state = P.clean({});
    for (const achievement of P.achievements.filter(a => a.category === "Card studio"))
        state = P.unlock(state, achievement.id).state;
    assert.equal(state.balance, 600);
    for (const item of P.cosmetics)
        state = P.purchase(state, item.id).state;
    assert.equal(state.balance, 0);
    assert.equal(Object.keys(state.owned).length, 3);
    assert.equal(P.clean({ version: 1, earned: state.earned, owned: state.owned }).balance, 0);
});

test("global visits award badges once and feature only earned badges", () => {
    let state = P.unlock({}, "ryoku-first-steps").state;
    assert.equal(state.balance, 30);
    for (let i = 0; i < 15; i++) {
        const category = ["appearance", "desktop", "apps", "system"][i % 4];
        state = P.visit(state, "page-" + i, category).state;
    }
    assert.equal(Object.keys(state.visited).length, 15);
    assert.equal(state.earned["ryoku-pathfinder"], true);
    assert.equal(state.earned["ryoku-voyager"], true);
    assert.equal(state.earned["ryoku-versatile"], true);
    assert.equal(P.visit(state, "page-1", "apps").changed, false);
    assert.equal(P.feature({}, "ryoku-voyager").changed, false);
    const featured = P.feature(state, "ryoku-voyager");
    assert.equal(featured.changed, true);
    assert.equal(featured.state.featured, "ryoku-voyager");
    assert.equal(P.feature(featured.state, "").state.featured, "");
});

test("stored balance cannot mint credits and unknown unlocks are discarded", () => {
    let state = P.clean({ balance: 999999, earned: { welcome: true, fake: true },
        owned: { stolen: true }, active: "stolen" });
    assert.equal(state.balance, 50);
    assert.equal(state.active, "");
    assert.equal(state.earned.fake, undefined);
    assert.equal(state.owned.stolen, undefined);
});

test("all earnable cosmetics have valid colours", () => {
    for (const item of P.cosmetics)
        for (const key of ["ink", "plate", "accent", "rim"])
            assert.match(item.palette[key], /^#[0-9a-f]{6}$/i);
});

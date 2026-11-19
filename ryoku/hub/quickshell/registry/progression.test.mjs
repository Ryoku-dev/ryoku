import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const P = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "progression.js"), "utf8").replace(/^(\.pragma|\.import) .*$/gm, ""), P);
const serial = obj => JSON.parse(JSON.stringify(obj));

test("completion and claiming are separate and neither can be farmed", () => {
    const earned = P.unlock({}, "welcome");
    assert.equal(earned.changed, true);
    assert.equal(earned.state.balance, 0);
    assert.equal(earned.state.earned.welcome, true);
    assert.equal(earned.state.claimed.welcome, undefined);
    assert.equal(P.claim({}, "welcome").changed, false);
    const claimed = P.claim(earned.state, "welcome");
    assert.equal(claimed.changed, true);
    assert.equal(claimed.state.balance, 50);
    assert.equal(claimed.state.claimed.welcome, true);
    assert.equal(P.claim(claimed.state, "welcome").changed, false);
    assert.equal(P.unlock(claimed.state, "welcome").changed, false);
    assert.equal(P.unlock({}, "fake-milestone").changed, false);
});

test("purchases spend only claimed credits, and cannot repeat", () => {
    let state = P.clean({});
    for (const id of ["welcome", "explorer", "identity"])
        state = P.claim(P.unlock(state, id).state, id).state;
    assert.equal(state.balance, 230);
    assert.equal(P.purchase(state, "jade").changed, false);
    const bought = P.purchase(state, "midnight");
    assert.equal(bought.changed, true);
    assert.equal(bought.state.balance, 80);
    assert.equal(P.purchase(bought.state, "midnight").changed, false);
    assert.equal(P.equip(bought.state, "ember").changed, false);
    assert.equal(P.equip(bought.state, "midnight").state.active, "midnight");
    assert.equal(P.equip(bought.state, "font-serif").changed, false);
});

test("legacy saved achievement receipts and purchases are never lost", () => {
    const original = { version: 2, earned: { welcome: true, explorer: true, identity: true,
        portrait: true, layout: true, showcase: true }, owned: { midnight: true, ember: true, jade: true },
        active: "jade", featured: "identity" };
    const migrated = P.clean(original);
    assert.equal(migrated.version, 3);
    assert.equal(migrated.balance, 0);
    assert.equal(migrated.featured, "identity");
    assert.equal(migrated.active, "jade");
    assert.equal(Object.keys(migrated.claimed).length, 6);
    assert.deepEqual(serial(P.clean(migrated)), serial(migrated));
    const newer = P.unlock(migrated, "profile-curator").state;
    assert.equal(newer.claimed["profile-curator"], undefined);
    assert.equal(newer.balance, 0);
});

test("global visits award categories once and featured badges must be claimed", () => {
    let state = P.unlock({}, "ryoku-first-steps").state;
    assert.equal(state.balance, 0);
    assert.equal(P.feature(state, "ryoku-first-steps").changed, false);
    state = P.claim(state, "ryoku-first-steps").state;
    assert.equal(state.balance, 30);
    for (let i = 0; i < 15; i++) {
        state = P.visit(state, "page-" + i,
            ["appearance", "desktop", "apps", "system"][i % 4]).state;
    }
    assert.equal(Object.keys(state.visited).length, 15);
    for (const id of ["ryoku-pathfinder", "ryoku-voyager", "ryoku-versatile"]) {
        assert.equal(state.earned[id], true);
        assert.equal(state.claimed[id], undefined);
    }
    assert.equal(P.visit(state, "page-1", "apps").changed, false);
    assert.equal(P.feature(state, "ryoku-voyager").changed, false);
    const featured = P.feature(state, "ryoku-first-steps");
    assert.equal(featured.changed, true);
    assert.equal(featured.state.featured, "ryoku-first-steps");
    assert.equal(featured.state.earned["profile-curator"], true);
    assert.equal(P.feature(featured.state, "").state.featured, "");
});

test("claiming a badge grants its sticker independently of the featured slot", () => {
    const state = P.claim(P.unlock({}, "welcome").state, "welcome").state;
    const available = P.availableStickers(state);
    assert.equal(available.length, 1);
    assert.equal(available[0].id, "welcome");
    assert.equal(P.addSticker({}, "welcome").changed, false);
    const placed = P.addSticker(state, "welcome");
    assert.equal(placed.changed, true);
    assert.equal(placed.state.canvas.stickers[0].item, "welcome");
    assert.equal(placed.state.earned["profile-sticker"], true);
    assert.equal(P.moveSticker(placed.state, "st1", 0.25, 0.35, 110, 20).state.canvas.stickers[0].size, 110);
    assert.equal(P.removeSticker(placed.state, "st1").state.canvas.stickers.length, 0);
});

test("backgrounds and fonts only equip when owned", () => {
    let state = P.clean({});
    assert.equal(P.background(state, "bg-blueprint").changed, false);
    assert.equal(P.font(state, "name", "font-serif").changed, false);
    assert.equal(P.font(state, "invalid-role", "").changed, false);
    for (const a of P.achievements)
        state = P.claim(P.unlock(state, a.id).state, a.id).state;
    const b = P.purchase(state, "bg-blueprint");
    assert.equal(b.changed, true);
    state = P.background(b.state, "bg-blueprint").state;
    assert.equal(state.canvas.background, "bg-blueprint");
    assert.equal(state.earned["profile-background"], true);
    const f = P.purchase(state, "font-serif");
    assert.equal(f.changed, true);
    state = P.font(f.state, "name", "font-serif").state;
    assert.equal(state.canvas.fonts.name, "font-serif");
    assert.equal(state.earned["profile-type"], true);
});

test("personal picture backgrounds accept only local image files", () => {
    const accepted = P.customImage({}, "file:///home/aether/Pictures/studio%20art.png");
    assert.equal(accepted.changed, true);
    assert.equal(accepted.state.canvas.image, "file:///home/aether/Pictures/studio%20art.png");
    assert.equal(accepted.state.earned["profile-background"], true);
    assert.equal(P.customImage({}, "https://example.com/background.png").changed, false);
    assert.equal(P.customImage({}, "file:///home/aether/Pictures/clip.mp4").changed, false);
    assert.equal(P.customImage(accepted.state, "").state.canvas.image, "");
});

test("sticker placement clamps coordinates and limits inventory to 24", () => {
    let state = P.claim(P.unlock({}, "welcome").state, "welcome").state;
    for (let i = 0; i < 24; i++)
        state = P.addSticker(state, "welcome").state;
    assert.equal(state.canvas.stickers.length, 24);
    assert.equal(P.addSticker(state, "welcome").changed, false);
    const first = state.canvas.stickers[0].slot;
    const moved = P.moveSticker(state, first, 999, -20, 12000, 999).state;
    assert.equal(moved.canvas.stickers[0].x, 0.94);
    assert.equal(moved.canvas.stickers[0].y, 0);
    assert.equal(moved.canvas.stickers[0].size, 300);
    assert.equal(moved.canvas.stickers[0].rotation, 45);
    assert.equal(P.removeSticker(moved, first).state.canvas.stickers.length, 23);
});

test("stored balances and inventory cannot mint new items", () => {
    const state = P.clean({ version: 3, balance: 999999, earned: { welcome: true, fake: true },
        claimed: { fake: true }, owned: { stolen: true }, featured: "fake",
        canvas: { background: "bg-blueprint", fonts: { name: "font-serif" },
            stickers: [{ slot: "st1", item: "sticker-orbit", x: 0.5, y: 0.5 }] } });
    assert.equal(state.balance, 0);
    assert.equal(state.featured, "");
    assert.equal(state.earned.fake, undefined);
    assert.equal(state.owned.stolen, undefined);
    assert.equal(state.canvas.background, "");
    assert.equal(state.canvas.fonts.name, "");
    assert.equal(state.canvas.stickers.length, 0);
});

test("finite achievement rewards cover every catalogue cosmetic", () => {
    const reward = P.achievements.reduce((sum, item) => sum + item.reward, 0);
    const price = P.cosmetics.reduce((sum, item) => sum + item.price, 0);
    assert.ok(reward >= price, "catalogue must be achievable without farming");
    const ids = Object.fromEntries(P.achievements.map(a => [a.id, true]));
    let state = P.clean({ version: 3, earned: ids, claimed: ids });
    for (const item of P.cosmetics) {
        const bought = P.purchase(state, item.id);
        assert.equal(bought.changed, true, item.id);
        state = bought.state;
    }
    assert.equal(state.balance, reward - price);
    assert.equal(Object.keys(state.owned).length, P.cosmetics.length);
});

test("rarities and secret visibility are consistent", () => {
    const allowed = new Set(["common", "uncommon", "rare", "epic", "legendary", "mythic", "holographic"]);
    assert.ok(P.achievements.length >= 40);
    assert.ok(P.cosmetics.length >= 25);
    for (const a of P.achievements) {
        assert.ok(allowed.has(a.rarity), a.id);
        assert.ok(a.sigil, a.id);
        assert.equal(!!a.secret, a.rarity === "holographic");
    }
    assert.equal(new Set(P.achievements.map(a => a.id)).size, P.achievements.length);
    assert.equal(new Set(P.cosmetics.map(a => a.id)).size, P.cosmetics.length);
    assert.equal(P.visibleAchievements({}, "Classified").length, 0);
    assert.equal(P.unlock({}, "secret-totality").changed, false);
    assert.equal(P.unlock({}, "master-archive").changed, false);
    assert.equal(P.unlock({}, "secret-monolith").changed, false);
});

test("a secret signal must be discovered rather than manually awarded", () => {
    let state = P.clean({});
    for (let i = 0; i < 6; i++)
        state = P.signal(state).state;
    assert.equal(state.earned["secret-signal"], undefined);
    state = P.signal(state).state;
    assert.equal(state.earned["secret-signal"], true);
    assert.equal(P.signal(state).changed, false);
    assert.equal(P.visibleAchievements(state, "Classified").length, 1);
    assert.equal(state.claimed["secret-signal"], undefined);
});

test("the final seal is hidden and requires every other achievement", () => {
    const all = Object.fromEntries(P.achievements.filter(a => a.id !== "secret-totality")
        .map(a => [a.id, true]));
    const almost = { ...all, "secret-monolith": false };
    assert.equal(P.clean({ version: 3, earned: almost }).earned["secret-totality"], undefined);
    const complete = P.clean({ version: 3, earned: all });
    assert.equal(complete.earned["secret-totality"], true);
    assert.equal(P.visibleAchievements(complete, "Classified").length, 4);
    assert.equal(complete.claimed["secret-totality"], undefined);
});

test("multi-stage achievements reflect real tracked state", () => {
    let state = P.clean({});
    for (let i = 0; i < 30; i++)
        state = P.visit(state, "visit-" + i, ["accounts","desktop","apps","system"][i % 4]).state;
    assert.equal(state.earned["ryoku-tenfold"], true);
    assert.equal(state.earned["ryoku-cartographer"], true);
    assert.equal(state.earned["ryoku-omniscope"], true);
    assert.equal(state.earned["secret-totality"], undefined);
    const claimed = P.claim(P.unlock({}, "welcome").state, "welcome").state;
    let placed = claimed;
    for (let i = 0; i < 10; i++)
        placed = P.addSticker(placed, "welcome").state;
    assert.equal(placed.earned["profile-mosaic"], true);
    assert.equal(placed.earned["profile-gallery"], true);
    assert.equal(placed.earned["secret-sevenfold"], undefined);
});

test("all shop palettes have valid colours, and every badge has a glyph", () => {
    for (const item of P.cosmetics.filter(c => c.type === "palette"))
        for (const key of ["ink", "plate", "accent", "rim"])
            assert.match(item.palette[key], /^#[0-9a-f]{6}$/i);
    for (const item of P.achievements)
        assert.ok(item.glyph && item.category);
});

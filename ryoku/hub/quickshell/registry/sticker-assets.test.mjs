import assert from "node:assert/strict";
import { test } from "node:test";
import { readFileSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const dir = dirname(fileURLToPath(import.meta.url));
const P = vm.createContext({});
vm.runInContext(readFileSync(join(dir, "progression.js"), "utf8").replace(/^\.(pragma|import).*$/gm, ""), P);

test("each shop sticker has its own vector artwork", () => {
    const entries = P.cosmetics.filter(item => item.type === "sticker");
    assert.equal(entries.length, 10);
    const names = entries.map(item => item.art);
    assert.equal(new Set(names).size, entries.length);
    for (const entry of entries) {
        const path = join(dir, "../assets/stickers", entry.art + ".svg");
        assert.ok(existsSync(path), entry.id);
        const svg = readFileSync(path, "utf8");
        assert.match(svg, /<svg\b/);
        assert.match(svg, /viewBox="-180 -180 360 360"/);
        assert.ok(svg.length > 600, entry.id);
        assert.equal(P.stamp(entry.id).art, entry.art);
    }
});

test("owned artwork cannot be placed until purchased and resizes within bounds", () => {
    const sticker = P.cosmetics.find(item => item.type === "sticker");
    assert.equal(P.addSticker({}, sticker.id).changed, false);
    let state = P.clean({ version: 3, owned: { [sticker.id]: true } });
    state = P.addSticker(state, sticker.id).state;
    assert.equal(state.canvas.stickers[0].item, sticker.id);
    assert.equal(P.moveSticker(state, "st1", 0.2, 0.4, 250, -20).state.canvas.stickers[0].size, 250);
    assert.equal(P.moveSticker(state, "st1", 0.2, 0.4, -100, 0).state.canvas.stickers[0].size, 36);
    assert.equal(P.moveSticker(state, "st1", 0.2, 0.4, 999, 0).state.canvas.stickers[0].size, 300);
});

test("Home keeps profile identity clear and delegates editing to Customise", () => {
    const home = readFileSync(join(dir, "../components/Home.qml"), "utf8");
    const studio = readFileSync(join(dir, "../components/about/CardEditor.qml"), "utf8");
    assert.ok(home.indexOf("id: achievementsPlaque") < home.indexOf("id: shopPlaque"));
    assert.ok(home.indexOf("id: shopPlaque") < home.indexOf("id: binderPlaque"));
    assert.match(home, /FEATURED BADGE/);
    assert.match(home, /root\.customise\("layout"\)/);
    assert.match(home, /id !== "achievements" && id !== "points-shop"/);
    assert.doesNotMatch(home, /Change featured seal|Choose featured seal|root\.choosingBadge/);
    assert.equal(home.match(/I18n\.tr\("Points Shop"\)/g)?.length, 1);
    assert.match(studio, /title: I18n\.tr\("Featured badge"\)/);
    assert.match(studio, /title: I18n\.tr\("Decorate profile"\)/);
    assert.match(studio, /Progression\.featureBadge\(badgeChoice\.modelData\.id\)/);
    assert.match(studio, /Progression\.editMode = true/);
    assert.match(home, /id: resizeGrip/);
    assert.match(home, /StickerArtwork \{/);
});

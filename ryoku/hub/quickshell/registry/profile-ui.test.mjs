import assert from "node:assert/strict";
import test from "node:test";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { fileURLToPath } from "node:url";

const dir = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(dir, "..");
const model = vm.createContext({});
vm.runInContext(fs.readFileSync(path.join(dir, "progression.js"), "utf8")
    .replace(/^(\.pragma|\.import) .*$/gm, ""), model);
const P = model;

const source = file => fs.readFileSync(path.join(root, file), "utf8");

test("every badge has its own SVG art, shape and mapped achievement identity", () => {
    const folder = path.join(root, "assets/badges");
    assert.equal(P.achievements.length, 72);
    assert.equal(fs.readdirSync(folder).filter(file => file.endsWith(".svg")).length, 72);
    const artworks = new Set();
    for (const badge of P.achievements) {
        const svg = fs.readFileSync(path.join(folder, badge.id + ".svg"), "utf8");
        assert.match(svg, /<svg xmlns="http:\/\/www\.w3\.org\/2000\/svg"/);
        assert.match(svg, /<path d="/);
        assert.ok(svg.includes("<title>" + badge.title.replaceAll("&", "&amp;") + "</title>"), badge.id);
        assert.ok(!svg.includes("ROMAN"), badge.id);
        assert.doesNotMatch(svg, /<linearGradient|<radialGradient|<filter|<image|<script/);
        assert.match(svg, /Flat Hellenic achievement insignia/);
        artworks.add(svg);
    }
    assert.equal(artworks.size, P.achievements.length);
    for (const group of ["Ryoku", "Profile", "Card studio", "Collection", "Mastery", "Classified"])
        assert.ok(P.achievements.some(badge => badge.category === group));
    const seal = source("components/about/ArchiveSeal.qml");
    assert.match(seal, /assets\/badges/);
    assert.match(seal, /badgeId/);
    assert.doesNotMatch(seal, /arcText/);
});

test("profile backgrounds are distinct from buyable stickers", () => {
    const scenes = P.cosmetics.filter(c => c.type === "background");
    const stickers = P.cosmetics.filter(c => c.type === "sticker");
    assert.equal(scenes.length, 13);
    assert.equal(new Set(scenes.map(scene => scene.art)).size, scenes.length);
    for (const scene of scenes) {
        assert.match(scene.art, /^scenes\/bg-[a-z]+\.svg$/);
        assert.ok(fs.existsSync(path.join(root, "assets", scene.art)), scene.id);
        assert.equal(stickers.some(sticker => sticker.art === scene.art), false);
    }
    const store = source("components/about/ProfileStudio.qml");
    assert.doesNotMatch(store, /SCENE \/ /);
    assert.doesNotMatch(store, /Progression\.setFont\(/);
    const editor = source("components/about/CardEditor.qml");
    assert.match(editor, /Profile typography/);
    assert.match(editor, /Progression\.setFont\(/);
});

test("Black Prism is original animated artwork, reduced motion disables playback", () => {
    const gif = fs.readFileSync(path.join(root, "assets/rewards/black-prism.gif"));
    assert.equal(gif.subarray(0, 6).toString("ascii"), "GIF89a");
    assert.ok(gif.length > 200000);
    const frameMarker = Buffer.from([0x21, 0xf9, 0x04]);
    let frames = 0;
    for (let offset = 0; (offset = gif.indexOf(frameMarker, offset)) !== -1; offset += 3)
        frames++;
    assert.ok(frames >= 20, "The reward must contain animated frames, not a still image");
    const scene = source("components/about/BlackPrism.qml");
    assert.match(scene, /AnimatedImage \{/);
    assert.match(scene, /playing: prism\.moving/);
    assert.match(scene, /!Lithic\.reduceMotion/);
    const shop = source("components/about/ProfileStudio.qml");
    assert.match(shop, /live: false/);
    assert.equal(P.cosmetics.some(c => c.id === P.completionScene.id), false);
});

test("freeform sticker transforms survive serialization and layer order changes", () => {
    let state = P.claim(P.unlock({}, "welcome").state, "welcome").state;
    state = P.addSticker(state, "welcome").state;
    state = P.addSticker(state, "welcome").state;
    state = P.addSticker(state, "welcome").state;
    state = P.moveSticker(state, "st1", -0.5, 1.2, 1500, 273.25).state;
    assert.equal(state.canvas.stickers[0].size, 1500);
    assert.equal(state.canvas.stickers[0].rotation, -86.75);
    assert.equal(state.canvas.stickers[0].x, -0.5);
    assert.equal(state.canvas.stickers[0].y, 1.2);
    state = P.reorderSticker(state, "st1", 1).state;
    assert.deepEqual(Array.from(state.canvas.stickers, s => s.slot), ["st2", "st1", "st3"]);
    assert.equal(P.reorderSticker(state, "st3", 1).changed, false);
    state = P.clean(JSON.parse(JSON.stringify(state)));
    assert.equal(state.canvas.stickers[1].size, 1500);
    assert.equal(state.canvas.stickers[1].rotation, -86.75);
    assert.equal(state.canvas.stickers[1].slot, "st1");
    const home = source("components/Home.qml");
    assert.match(home, /onModelDataChanged/);
    assert.match(home, /id: rotationGrip/);
    assert.match(home, /z: 1 \+ index \/ 100/);
    assert.match(source("components/about/CardEditor.qml"), /Progression\.reorderSticker/);
});

test("archive and cards omit redundant prose and have dedicated tracking", () => {
    const book = source("components/about/ProgressionTab.qml");
    const card = source("components/about/AchievementCard.qml");
    assert.doesNotMatch(book, /Every badge tells a story|THE ACHIEVEMENT ARCHIVE|RECORDED/);
    assert.doesNotMatch(card, /RYOKU \/ |IN YOUR ARCHIVE|DECRYPTED \/ |CHALLENGE ACTIVE/);
    assert.match(card, /id: tracker/);
    assert.match(card, /card\.progress\.current/);
    assert.match(card, /badgeId: card\.badge\.id/);
});

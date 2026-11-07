import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";

const gallery = fs.readFileSync(new URL("../pages/StoreDesignGallery.qml", import.meta.url), "utf8");
const studio = fs.readFileSync(new URL("../pages/BarStudioPage.qml", import.meta.url), "utf8");
const subshell = fs.readFileSync(new URL("../pages/SubshellsPage.qml", import.meta.url), "utf8");

test("resource indicators keep gallery tiles compact", () => {
    assert.match(gallery, /readonly property int tileH: 64/);
    assert.match(gallery, /id: resourceMark[\s\S]*?visible: designCard\.heavy/);
    assert.match(gallery, /id: resourceMark[\s\S]*?anchors\.top: parent\.top/);
    assert.match(gallery, /id: resourceMark[\s\S]*?anchors\.right: parent\.right/);
    assert.match(gallery, /text: "▲"/);
    assert.doesNotMatch(gallery, /May need more CPU, GPU and memory|Tokens\.alert/);
});

test("resource triangles follow catalogue metadata", () => {
    assert.match(gallery, /modelData\.resourceTier === "heavy"/);
    assert.doesNotMatch(gallery, /modelData\.id.*resource/);
});

test("Bar Studio and Subshells share one readable resource legend", () => {
    assert.match(studio, /Uses more resources than other options/);
    assert.match(studio, /page\.subshellsOnly/);
    assert.match(subshell, /subshellsOnly: true/);
});

test("gallery selection and Store actions remain unchanged", () => {
    assert.match(gallery, /gallery\.wear\(designCard\.modelData\.id\)/);
    assert.match(gallery, /"ryostore", "install", item\.category, item\.id/);
    assert.match(gallery, /"ryostore", "open", gallery\.browseCategory/);
});

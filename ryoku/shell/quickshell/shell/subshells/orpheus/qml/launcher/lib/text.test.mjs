import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { ghostFor, matchSpans, escapeHtml } = require("./text.js");

const hitText = spans => spans.filter(s => s.hit).map(s => s.text).join("");
const whole = spans => spans.map(s => s.text).join("");

test("the ghost completes the top app from what was typed", () => {
  assert.equal(ghostFor("fir", "Firefox"), "efox");
  assert.equal(ghostFor("FIR", "Firefox"), "efox");
});

test("no ghost when the title does not start with the query or is already complete", () => {
  assert.equal(ghostFor("fox", "Firefox"), "");
  assert.equal(ghostFor("firefox", "Firefox"), "");
  assert.equal(ghostFor("", "Firefox"), "");
});

test("accepting the ghost spells the full title", () => {
  const typed = "vis";
  assert.equal((typed + ghostFor(typed, "Visual Studio Code")).toLowerCase(), "visual studio code");
});

test("a typed word lights up where it appears, preferring a word start", () => {
  const spans = matchSpans("Visual Studio Code", "code");
  assert.equal(hitText(spans), "Code");
  assert.equal(whole(spans), "Visual Studio Code");
  const second = matchSpans("Dolphin Pho", "ph");
  assert.equal(second.find(s => s.hit).text, "Ph");
  assert.equal(whole(second).indexOf("Ph"), whole(second).length - 3);
});

test("scattered initials light up in order", () => {
  const spans = matchSpans("Visual Studio Code", "vsc");
  assert.equal(hitText(spans), "VSC");
});

test("nothing lights when the query is empty or does not match", () => {
  assert.equal(hitText(matchSpans("Firefox", "")), "");
  assert.equal(hitText(matchSpans("Firefox", "zz")), "");
  assert.equal(whole(matchSpans("Firefox", "zz")), "Firefox");
});

test("titles stay literal text when drawn as styled text", () => {
  assert.equal(escapeHtml("<b>&"), "&lt;b&gt;&amp;");
});

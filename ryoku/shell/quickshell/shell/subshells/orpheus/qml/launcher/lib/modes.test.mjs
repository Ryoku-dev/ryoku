import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { MODES, routeFor, queryForMode, glyphFor } = require("./modes.js");
const { routePrefix } = require("../../../../../modules/launcher/shared/lib/dispatch.js");

const prefixes = { "/": "actions", "=": "calc", "?": "web", "/file": "find", "@": "radio" };
const route = text => routePrefix(text, prefixes);

test("the tabs open on All", () => {
  assert.equal(MODES[0], "all");
  assert.ok(!MODES.includes("ask"));
});

test("an empty All query is the rest state", () => {
  assert.ok(routeFor("all", "", route).rest);
  assert.ok(!routeFor("all", "fire", route).rest);
});

test("typed prefixes in All reach their provider", () => {
  assert.equal(routeFor("all", "=2+2", route).provider, "calc");
  assert.equal(routeFor("all", "?weather", route).provider, "web");
  assert.equal(routeFor("all", "/lock", route).provider, "actions");
  assert.equal(routeFor("all", "/lock", route).query, "lock");
  assert.equal(routeFor("all", "fire", route).provider, "");
});

test("each tab searches its own kind of result", () => {
  assert.equal(routeFor("apps", "fi", route).provider, "apps");
  assert.equal(routeFor("windows", "", route).provider, "windows");
  assert.equal(routeFor("actions", "", route).provider, "actions");
});

test("Files shows recent files until something is typed", () => {
  assert.equal(routeFor("files", "", route).provider, "recent");
  const typed = routeFor("files", "notes", route);
  assert.equal(typed.provider, "find");
  assert.equal(typed.prefix, "/file");
});

test("switching tab drops a routing prefix typed in All", () => {
  assert.equal(queryForMode("actions", "/lock", route), "lock");
  assert.equal(queryForMode("apps", "fire", route), "fire");
  assert.equal(queryForMode("all", "/lock", route), "/lock");
});

test("the search glyph names where the query is going", () => {
  assert.equal(glyphFor(routeFor("all", "fire", route)), "seal");
  assert.equal(glyphFor(routeFor("all", "=1", route)), "calculate");
  assert.equal(glyphFor(routeFor("all", "?x", route)), "public");
  assert.equal(glyphFor(routeFor("all", "/", route)), "bolt");
  assert.equal(glyphFor(routeFor("actions", "", route)), "bolt");
});

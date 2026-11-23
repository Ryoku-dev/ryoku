import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { modesFor, routeFor, queryForMode, glyphFor } = require("./modes.js");
const { routePrefix } = require("../../../../../modules/launcher/shared/lib/dispatch.js");

const prefixes = { "/": "actions", "=": "calc", "?": "web", "/file": "find", "@": "radio" };
const route = text => routePrefix(text, prefixes);

test("the Ask tab only exists when Rashin can answer", () => {
  assert.ok(!modesFor(false).includes("ask"));
  assert.equal(modesFor(true)[modesFor(true).length - 1], "ask");
  assert.equal(modesFor(false)[0], "all");
});

test("an empty All query is the rest state", () => {
  assert.ok(routeFor("all", "", route, true).rest);
  assert.ok(!routeFor("all", "fire", route, true).rest);
});

test("typed prefixes in All reach their provider", () => {
  assert.equal(routeFor("all", "=2+2", route, true).provider, "calc");
  assert.equal(routeFor("all", "?weather", route, true).provider, "web");
  assert.equal(routeFor("all", "/lock", route, true).provider, "actions");
  assert.equal(routeFor("all", "/lock", route, true).query, "lock");
  assert.equal(routeFor("all", "fire", route, true).provider, "");
});

test("a backslash asks Rashin only when Rashin is available", () => {
  const ask = routeFor("all", "\\ what is dns", route, true);
  assert.ok(ask.ask);
  assert.equal(ask.query, "what is dns");
  assert.ok(!routeFor("all", "\\ what", route, false).ask);
});

test("each tab searches its own kind of result", () => {
  assert.equal(routeFor("apps", "fi", route, true).provider, "apps");
  assert.equal(routeFor("windows", "", route, true).provider, "windows");
  assert.equal(routeFor("actions", "", route, true).provider, "actions");
  assert.ok(routeFor("ask", "why", route, true).ask);
});

test("Files shows recent files until something is typed", () => {
  assert.equal(routeFor("files", "", route, true).provider, "recent");
  const typed = routeFor("files", "notes", route, true);
  assert.equal(typed.provider, "find");
  assert.equal(typed.prefix, "/file");
});

test("switching tab drops a routing prefix typed in All", () => {
  assert.equal(queryForMode("actions", "/lock", route), "lock");
  assert.equal(queryForMode("apps", "fire", route), "fire");
  assert.equal(queryForMode("ask", "\\why", route), "why");
  assert.equal(queryForMode("all", "/lock", route), "/lock");
});

test("the search glyph names where the query is going", () => {
  assert.equal(glyphFor(routeFor("all", "fire", route, true)), "seal");
  assert.equal(glyphFor(routeFor("all", "=1", route, true)), "calculate");
  assert.equal(glyphFor(routeFor("all", "?x", route, true)), "public");
  assert.equal(glyphFor(routeFor("all", "/", route, true)), "bolt");
  assert.equal(glyphFor(routeFor("ask", "", route, true)), "auto_awesome");
  assert.equal(glyphFor(routeFor("actions", "", route, true)), "bolt");
});

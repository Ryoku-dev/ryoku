import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { kindKey, glyphFor, parentFolder, indexLabel, countLabel, sameApp } = require("./rows.js");

test("only the first nine rows carry an Alt+N index", () => {
  assert.equal(indexLabel(0, false), "1");
  assert.equal(indexLabel(8, false), "9");
  assert.equal(indexLabel(9, false), "");
  assert.equal(indexLabel(-1, false), "");
});

test("Lithic sets the index and the count as print digits", () => {
  assert.equal(indexLabel(0, true), "01");
  assert.equal(indexLabel(8, true), "09");
  assert.equal(countLabel(41, true), "041");
  assert.equal(countLabel(41, false), "41");
  assert.equal(countLabel(1234, true), "1234");
});

test("each kind of result gets its own symbol when it has no icon", () => {
  const symbols = ["apps", "windows", "calc", "web", "actions", "mpris", "ask"]
    .map(providerId => glyphFor({ providerId }));
  assert.equal(new Set(symbols).size, symbols.length);
  assert.equal(glyphFor({ providerId: "find", type: "Folder" }), "folder");
  assert.notEqual(glyphFor({ providerId: "find", type: "Image" }), glyphFor({ providerId: "find", type: "File" }));
});

test("files and recent files read as one kind", () => {
  assert.equal(kindKey({ providerId: "find" }), kindKey({ providerId: "recent" }));
});

test("a file's kind line names its folder, home shortened", () => {
  assert.equal(parentFolder("/home/u/Pictures/a.png", "/home/u"), "~/Pictures");
  assert.equal(parentFolder("file:///etc/hosts", "/home/u"), "/etc");
  assert.equal(parentFolder("/home/u/x.txt", "/home/u"), "~");
});

test("an open window belongs to its app by id or window class", () => {
  assert.ok(sameApp("firefox", "", "firefox"));
  assert.ok(sameApp("firefox.desktop", "", "Firefox"));
  assert.ok(sameApp("code-oss", "code", "Code"));
  assert.ok(sameApp("org.gnome.Nautilus", "", "nautilus"));
  assert.ok(!sameApp("firefox", "", "kitty"));
  assert.ok(!sameApp("firefox", "", ""));
  assert.ok(!sameApp("org.a.Files", "", "org.b.Files"));
});

test("a refresh that changes nothing visible keeps the same rows", () => {
  const { signature } = require("./rows.js");
  const make = name => [{ resultKey: "k", title: "Spotify", primaryAction: { id: "toggle", name }, secondaryActions: [] }];
  assert.equal(signature(make("Pause")), signature(make("Pause")));
  assert.notEqual(signature(make("Pause")), signature(make("Play")), "a renamed action redraws");
  assert.notEqual(signature([{ resultKey: "a", title: "x" }]), signature([{ resultKey: "a", title: "y" }]));
});

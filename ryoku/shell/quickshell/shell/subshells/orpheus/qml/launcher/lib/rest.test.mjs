import assert from "node:assert/strict";
import { createRequire } from "node:module";
import test from "node:test";
const require = createRequire(import.meta.url);
const { categoryKey, categoryRank, restOrder } = require("./rest.js");

const apps = [
  { appId: "zed", title: "Zed", cats: "Development;IDE;" },
  { appId: "firefox", title: "Firefox", cats: "Network;WebBrowser;" },
  { appId: "kitty", title: "kitty", cats: "System;TerminalEmulator;" },
  { appId: "nautilus", title: "Files", cats: "GNOME;System;FileManager;" },
  { appId: "mpv", title: "mpv", cats: "AudioVideo;Video;Player;" },
  { appId: "calc", title: "Calculator", cats: "Utility;" },
  { appId: "settings", title: "Settings", cats: "Settings;" },
  { appId: "steam", title: "Steam", cats: "Game;" },
  { appId: "gimp", title: "GIMP", cats: "Graphics;2DGraphics;" },
  { appId: "discord", title: "Discord", cats: "Network;InstantMessaging;" },
  { appId: "libre", title: "LibreOffice Writer", cats: "Office;WordProcessor;" },
  { appId: "aaa", title: "Aaa Tool", cats: "" },
  { appId: "chromium", title: "Chromium", cats: "Network;WebBrowser;" }
];
const catsOf = row => row.cats;
const ids = result => result.rows.map(row => row.appId);

test("pinned apps lead in the order the user pinned them", () => {
  const result = restOrder(apps, ["steam", "kitty"], { firefox: 9 }, catsOf, 9);
  assert.deepEqual(ids(result).slice(0, 3), ["steam", "kitty", "firefox"]);
  assert.equal(result.section, "pinned");
});

test("most used apps follow, the most launched first", () => {
  const result = restOrder(apps, [], { mpv: 2, zed: 7, gimp: 2 }, catsOf, 9);
  assert.deepEqual(ids(result).slice(0, 3), ["zed", "gimp", "mpv"]);
  assert.equal(result.section, "mostUsed");
});

test("a pinned app is never listed twice", () => {
  const result = restOrder(apps, ["zed"], { zed: 10 }, catsOf, 9);
  assert.equal(ids(result).filter(id => id === "zed").length, 1);
});

test("a fresh user gets a browser, a terminal and files first, never A to Z", () => {
  const result = restOrder(apps, [], {}, catsOf, 9);
  assert.deepEqual(ids(result).slice(0, 3), ["chromium", "kitty", "nautilus"]);
  assert.ok(!ids(result).includes("firefox"), "a second browser waits for the second round");
  assert.equal(result.rows.length, 9);
  assert.equal(result.section, "");
  assert.notEqual(ids(result)[0], "aaa");
});

test("a fresh user's set spreads across categories before repeating one", () => {
  const result = restOrder(apps, [], {}, catsOf, 9);
  const first = ids(result);
  assert.ok(first.includes("mpv"));
  assert.ok(first.includes("libre"));
  assert.ok(first.includes("discord"));
  assert.ok(first.indexOf("settings") === -1 || first.indexOf("settings") > first.indexOf("discord"));
});

test("the rest state shows nine apps at most", () => {
  assert.equal(restOrder(apps, [], {}, catsOf).rows.length, 9);
  assert.equal(restOrder(apps.slice(0, 3), [], {}, catsOf, 9).rows.length, 3);
});

test("pins of uninstalled apps are skipped", () => {
  const result = restOrder(apps, ["gone", "mpv"], {}, catsOf, 9);
  assert.equal(ids(result)[0], "mpv");
});

test("an app's category reads from its desktop categories", () => {
  assert.equal(categoryKey("Network;WebBrowser;"), "browser");
  assert.equal(categoryKey(["System", "FileManager"]), "files");
  assert.equal(categoryKey(""), "app");
  assert.ok(categoryRank("WebBrowser") < categoryRank("Utility"));
  assert.ok(categoryRank("Utility") < categoryRank("Settings"));
});

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import test from "node:test";

const require = createRequire(import.meta.url);
const Catalog = require("./catalog.js");
const raw = JSON.parse(readFileSync(
  new URL("../../catalog.json", import.meta.url),
  "utf8"
));

test("real catalog exposes only Ryoku launcher variants", () => {
  const catalog = Catalog.normalize(raw);

  assert.deepEqual(
    catalog.variants.map(variant => variant.id),
    ["main", "hero", "okshell"]
  );
  assert.equal(Catalog.defaultEntry(catalog).id, "hero");
  assert.equal(Catalog.fallbackEntry(catalog).id, "okshell");
  assert.equal(Catalog.entry(catalog, "main").id, "main");
  assert.equal(Catalog.entry(catalog, "does-not-exist").id, "hero");
});

test("catalog rejects duplicate IDs", () => {
  assert.throws(() => Catalog.normalize({
    version: 1,
    default: "hero",
    fallback: "hero",
    variants: [
      {
        id: "hero",
        entrypoint: "variants/hero/Main.qml",
        preview: "variants/hero/Preview.qml"
      },
      {
        id: "hero",
        entrypoint: "variants/again/Main.qml",
        preview: "variants/again/Preview.qml"
      }
    ]
  }), /invalid or duplicate id/);
});

test("catalog rejects entrypoints and previews outside variant folders", () => {
  const unsafeValues = [
    "../shell.qml",
    "shared/Main.qml",
    "variants/hero/../Main.qml",
    "variants/Hero/Main.qml",
    "variants/hero/main.qml/extra"
  ];

  for (const unsafeValue of unsafeValues) {
    for (const key of ["entrypoint", "preview"]) {
      const variant = {
        id: "hero",
        entrypoint: "variants/hero/Main.qml",
        preview: "variants/hero/Preview.qml",
        [key]: unsafeValue
      };
      assert.throws(() => Catalog.normalize({
        version: 1,
        default: "hero",
        fallback: "hero",
        variants: [variant]
      }), /unsafe QML path/);
    }
  }
});

test("catalog requires its default and fallback IDs to exist", () => {
  const variant = {
    id: "hero",
    entrypoint: "variants/hero/Main.qml",
    preview: "variants/hero/Preview.qml"
  };

  for (const [defaultId, fallbackId] of [
    ["missing", "hero"],
    ["hero", "missing"]
  ]) {
    assert.throws(() => Catalog.normalize({
      version: 1,
      default: defaultId,
      fallback: fallbackId,
      variants: [variant]
    }), /default and fallback must name entries/);
  }
});

const digest = "a".repeat(64);
const storeRow = (id, extra = {}) => ({
  id,
  version: "1.0.0",
  preview: "Preview.qml",
  view: `launcher-views/${id}/${digest}`,
  ...extra
});

test("installed Store launchers join the catalog and run from their views", () => {
  const catalog = Catalog.withStore(Catalog.normalize(raw), [storeRow("tatami")], "/home/u/.local/state/ryoku/store");
  const tatami = Catalog.entry(catalog, "tatami");

  assert.equal(tatami.id, "tatami");
  assert.equal(tatami.url, `file:///home/u/.local/state/ryoku/store/launcher-views/tatami/${digest}/Main.qml`);
  assert.equal(tatami.previewUrl, `file:///home/u/.local/state/ryoku/store/launcher-views/tatami/${digest}/Preview.qml`);
  assert.equal(Catalog.defaultEntry(catalog).id, "hero");
  assert.equal(Catalog.entry(catalog, "gone").id, "hero");
});

test("a Store launcher without a preview has no preview URL", () => {
  const [entry] = Catalog.storeVariants(Catalog.normalize(raw), [storeRow("tatami", { preview: "" })], "/s");
  assert.equal(entry.previewUrl, "");
});

test("Store rows that are malformed, borrowed or shadowing are dropped", () => {
  const rows = [
    storeRow("hero"),
    storeRow("theft", { view: `launcher-views/tatami/${digest}` }),
    storeRow("escape", { view: `launcher-views/escape/../../../etc` }),
    storeRow("short", { view: "launcher-views/short/abc" }),
    storeRow("unversioned", { version: "" }),
    storeRow("Bad_Id", { view: `launcher-views/Bad_Id/${digest}` }),
    storeRow("tatami"),
    storeRow("tatami", { version: "2.0.0" })
  ];
  const variants = Catalog.storeVariants(Catalog.normalize(raw), rows, "/s");

  assert.deepEqual(variants.map(variant => [variant.id, variant.version]), [["tatami", "1.0.0"]]);
  assert.match(Catalog.entry(Catalog.withStore(Catalog.normalize(raw), rows, "/s"), "hero").entrypoint, /^variants\/hero\//);
});

const shellContribution = (id, prepared, extra = {}) => ({
  id,
  itemId: `owner-${id}`,
  name: `${id} launcher`,
  description: `${id} contribution`,
  prepared,
  entryUrl: prepared ? `file:///state/${id}/Main.qml` : "",
  previewUrl: prepared ? `file:///state/${id}/Preview.qml` : "",
  ...extra
});

test("a prepared shell contribution resolves to its lazy entry URLs", () => {
  const catalog = Catalog.normalize(raw);
  const result = Catalog.resolve(catalog, "spotlight", [
    shellContribution("spotlight", true)
  ]);

  assert.equal(result.pending, false);
  assert.deepEqual(result.entry, {
    id: "spotlight",
    name: "spotlight launcher",
    description: "spotlight contribution",
    contribution: true,
    itemId: "owner-spotlight",
    url: "file:///state/spotlight/Main.qml",
    previewUrl: "file:///state/spotlight/Preview.qml",
    capabilities: []
  });
});

test("an unprepared shell contribution keeps the catalog default pending", () => {
  const result = Catalog.resolve(Catalog.normalize(raw), "python", [
    shellContribution("python", false)
  ]);

  assert.equal(result.pending, true);
  assert.equal(result.entry.id, "hero");
});

test("an unknown contribution selection uses the catalog default", () => {
  const result = Catalog.resolve(Catalog.normalize(raw), "missing", [
    shellContribution("spotlight", true)
  ]);

  assert.equal(result.pending, false);
  assert.equal(result.entry.id, "hero");
});

test("catalog variants cannot be shadowed by shell contributions", () => {
  const result = Catalog.resolve(Catalog.normalize(raw), "hero", [
    shellContribution("hero", true)
  ]);

  assert.equal(result.pending, false);
  assert.equal(result.entry.id, "hero");
  assert.equal(result.entry.contribution, undefined);
});

test("shell contributions take precedence over legacy Store rows", () => {
  const catalog = Catalog.withStore(Catalog.normalize(raw), [storeRow("spotlight")], "/s");
  const result = Catalog.resolve(catalog, "spotlight", [
    shellContribution("spotlight", true)
  ]);

  assert.equal(result.entry.contribution, true);
  assert.equal(result.entry.url, "file:///state/spotlight/Main.qml");
});

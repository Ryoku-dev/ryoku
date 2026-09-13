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

test("real catalog exposes the built-in variants with the promised routing", () => {
  const catalog = Catalog.normalize(raw);

  assert.deepEqual(
    catalog.variants.map(variant => variant.id),
    ["main", "hero", "okshell", "kairos"]
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

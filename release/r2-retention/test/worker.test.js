import assert from "node:assert/strict";
import test from "node:test";

import { runRetention } from "../src/index.js";

class Bucket {
  constructor() {
    this.objects = new Map();
    this.deleted = [];
    this.puts = [];
  }

  add(key, body, uploaded) {
    this.objects.set(key, { key, body, uploaded: new Date(uploaded), size: body.length });
  }

  async list({ prefix = "", delimiter }) {
    const objects = [];
    const prefixes = new Set();
    for (const value of this.objects.values()) {
      if (!value.key.startsWith(prefix)) continue;
      const rest = value.key.slice(prefix.length);
      if (delimiter && rest.includes(delimiter)) {
        prefixes.add(`${prefix}${rest.slice(0, rest.indexOf(delimiter) + 1)}`);
      } else {
        objects.push({ key: value.key, uploaded: value.uploaded, size: value.size });
      }
    }
    return { objects, delimitedPrefixes: [...prefixes], truncated: false };
  }

  async get(key) {
    const value = this.objects.get(key);
    return value ? { text: async () => value.body } : null;
  }

  async delete(keys) {
    this.deleted.push(...keys);
    for (const key of keys) this.objects.delete(key);
  }

  async put(key, body, options) {
    this.puts.push({ key, body, options });
  }
}

function packageBucket() {
  const bucket = new Bucket();
  for (let version = 1; version <= 3; version += 1) {
    const date = `2026-10-0${version}T00:00:00Z`;
    const prefix = `stable/releases/v${version}/x86_64/`;
    bucket.add(`${prefix}release.json`, JSON.stringify({ date }), date);
    bucket.add(`${prefix}ryoku.pkg.tar.zst`, "package", date);
  }
  return bucket;
}

async function quietly(run) {
  const original = console.log;
  console.log = () => {};
  try {
    await run();
  } finally {
    console.log = original;
  }
}

test("reads KEEP from Worker vars", async () => {
  const bucket = packageBucket();
  await quietly(() => runRetention({
    BUCKET: bucket,
    ROOT: "stable/",
    KEEP: "2",
    DRY_RUN: "0",
    OPERATION_BUDGET: "100",
  }));

  assert.deepEqual(bucket.deleted.sort(), [
    "stable/releases/v1/x86_64/release.json",
    "stable/releases/v1/x86_64/ryoku.pkg.tar.zst",
  ]);
});

test("dry-run mode never writes", async () => {
  const bucket = packageBucket();
  await quietly(() => runRetention({
    BUCKET: bucket,
    ROOT: "stable/",
    KEEP: "1",
    DRY_RUN: "1",
    OPERATION_BUDGET: "100",
  }));

  assert.deepEqual(bucket.deleted, []);
  assert.deepEqual(bucket.puts, []);
});

test("discovers Fedora releases and updates their ledgers", async () => {
  const bucket = new Bucket();
  for (let version = 1; version <= 2; version += 1) {
    const date = `2026-10-0${version}T00:00:00Z`;
    const prefix = `stable/fedora/44/releases/v${version}/x86_64/`;
    bucket.add(`${prefix}release.json`, JSON.stringify({ date }), date);
    bucket.add(`${prefix}ryoku.rpm`, "package", date);
  }
  bucket.add("stable/fedora/44/releases/index.json", JSON.stringify({
    schema: 1,
    latest: "v2",
    releases: [
      { tag: "v2", repo: "fedora/44/releases/v2/x86_64" },
      { tag: "v1", repo: "fedora/44/releases/v1/x86_64" },
    ],
  }), "2026-10-02T00:00:00Z");

  await quietly(() => runRetention({
    BUCKET: bucket,
    ROOT: "stable/",
    KEEP: "1",
    DRY_RUN: "0",
    OPERATION_BUDGET: "100",
  }));

  assert.deepEqual(bucket.deleted.sort(), [
    "stable/fedora/44/releases/v1/x86_64/release.json",
    "stable/fedora/44/releases/v1/x86_64/ryoku.rpm",
  ]);
  assert.equal(bucket.puts.length, 1);
  assert.equal(bucket.puts[0].key, "stable/fedora/44/releases/index.json");
  assert.deepEqual(JSON.parse(bucket.puts[0].body).releases.map((release) => release.tag), ["v2"]);
});

import assert from "node:assert/strict";
import test from "node:test";

import { buildPlan, Operations } from "../src/index.js";
import { formatPlan, monthlyCost } from "../src/report.js";
import { RestBucket } from "../src/rest-bucket.js";

const API = "https://api.example.test/client/v4";

// fakeApi serves the Cloudflare R2 REST object routes from a key map, two
// objects per page so pagination is exercised, and records every method used.
function fakeApi(store) {
  const calls = [];
  const fetch = async (input, init = {}) => {
    const url = new URL(input);
    calls.push({ method: init.method ?? "GET", url: url.toString(), auth: init.headers?.Authorization });
    const base = "/client/v4/accounts/acct/r2/buckets/ryoku-iso/objects";
    if (url.pathname === base) {
      const prefix = url.searchParams.get("prefix") ?? "";
      const delimiter = url.searchParams.get("delimiter");
      const start = Number(url.searchParams.get("cursor") ?? 0);
      const objects = [];
      const delimited = new Set();
      for (const [key, value] of [...store.entries()].sort(([a], [b]) => a.localeCompare(b))) {
        if (!key.startsWith(prefix)) continue;
        const rest = key.slice(prefix.length);
        if (delimiter && rest.includes(delimiter)) delimited.add(`${prefix}${rest.slice(0, rest.indexOf(delimiter) + 1)}`);
        else objects.push({ key, size: value.body.length, last_modified: value.uploaded });
      }
      const page = objects.slice(start, start + 2);
      const truncated = start + 2 < objects.length;
      return Response.json({
        success: true,
        result: page,
        result_info: { is_truncated: truncated, cursor: truncated ? String(start + 2) : undefined, delimited: [...delimited] },
      });
    }
    const key = decodeURIComponent(url.pathname.slice(base.length + 1));
    const value = store.get(key);
    if (!value) return new Response("{}", { status: 404 });
    return new Response(value.body);
  };
  return { fetch, calls };
}

function store() {
  const objects = new Map();
  const add = (key, body, uploaded) => objects.set(key, { body, uploaded });
  for (let version = 1; version <= 8; version += 1) {
    const date = `2026-10-0${version}T00:00:00Z`;
    add(`stable/releases/v${version}/x86_64/release.json`, JSON.stringify({ release: `v${version}`, date }), date);
    add(`stable/releases/v${version}/x86_64/ryoku.pkg.tar.zst`, "package-bytes", date);
  }
  add("stable/x86_64/release.json", JSON.stringify({ release: "v8", date: "2026-10-08T00:00:00Z" }), "2026-10-08T00:00:00Z");
  add("stable/releases/index.json", JSON.stringify({
    schema: 1,
    latest: "v8",
    releases: Array.from({ length: 8 }, (_, index) => ({ tag: `v${8 - index}`, repo: `releases/v${8 - index}/x86_64` })),
  }), "2026-10-08T00:00:00Z");
  add("install.sh", "#!/bin/sh", "2026-10-01T00:00:00Z");
  return objects;
}

test("RestBucket pages listings, reports delimited prefixes, and cannot write", async () => {
  const api = fakeApi(store());
  const bucket = new RestBucket({ accountId: "acct", bucket: "ryoku-iso", token: "secret", fetch: api.fetch, api: API });

  const first = await bucket.list({ prefix: "stable/releases/v1/" });
  assert.equal(first.objects.length, 2);
  assert.equal(first.truncated, false);
  const top = await bucket.list({ prefix: "stable/releases/", delimiter: "/" });
  assert.deepEqual(top.objects.map((object) => object.key), ["stable/releases/index.json"]);
  assert.equal(top.delimitedPrefixes.length, 8);
  const paged = await bucket.list({ prefix: "stable/x86_64/" });
  assert.equal(paged.objects[0].uploaded, "2026-10-08T00:00:00Z");
  assert.equal(await bucket.get("stable/missing.json"), null);
  assert.equal(await (await bucket.get("install.sh")).text(), "#!/bin/sh");
  assert.equal(typeof bucket.delete, "undefined");
  assert.equal(typeof bucket.put, "undefined");
  assert.ok(api.calls.every((call) => call.method === "GET" && call.auth === "Bearer secret"));
});

test("plan.js's dry run lists each deletion with its size, then totals, using only GETs", async () => {
  const objects = store();
  const api = fakeApi(objects);
  const bucket = new RestBucket({ accountId: "acct", bucket: "ryoku-iso", token: "secret", fetch: api.fetch, api: API });
  const { plan } = await buildPlan(bucket, new Operations(Infinity), { root: "stable/", now: new Date("2026-10-10T00:00:00Z") });
  const everything = [];
  let cursor;
  do {
    const page = await bucket.list({ cursor });
    everything.push(...page.objects);
    cursor = page.truncated ? page.cursor : undefined;
  } while (cursor);

  const report = formatPlan(plan, everything, new Date("2026-10-10T00:00:00Z"));
  const lines = report.trimEnd().split("\n");
  const entries = lines.filter((line) => !line.startsWith("#"));
  assert.deepEqual(entries, [
    "stable/releases/v1/x86_64/release.json\t46",
    "stable/releases/v1/x86_64/ryoku.pkg.tar.zst\t13",
    "stable/releases/v2/x86_64/release.json\t46",
    "stable/releases/v2/x86_64/ryoku.pkg.tar.zst\t13",
  ]);
  assert.ok(lines.includes("# delete: 4 objects, 0.00 GB (118 bytes)"));
  assert.ok(lines.some((line) => line.startsWith("# keep: 15 objects")));
  assert.ok(lines.some((line) => line.startsWith("# ledger rewrite: stable/releases/index.json keeps 6 entries, latest v8")));
  assert.ok(api.calls.every((call) => call.method === "GET"));
  assert.ok(objects.has("stable/releases/v1/x86_64/release.json"));
});

test("monthly cost charges only beyond the free 10 GB", () => {
  assert.equal(monthlyCost(5e9), 0);
  assert.equal(monthlyCost(110e9).toFixed(2), "1.50");
});

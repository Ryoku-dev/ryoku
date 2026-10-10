import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { planRetention } from "../src/planner.js";

const fixture = JSON.parse(await readFile(new URL("./fixtures/isos.json", import.meta.url), "utf8"));
const NOW = "2026-10-10T12:00:00Z";

function object(key, uploaded = "2026-10-01T00:00:00Z", size = 1) {
  return { key, uploaded, size };
}

function isoFacts(items, sidecars = ["", ".sha256", ".sig", ".json", ".js"]) {
  const objects = [];
  const manifests = {};
  for (const item of items) {
    for (const suffix of sidecars) objects.push(object(`${item.key}${suffix}`, item.at, suffix ? 10 : item.size));
    manifests[`${item.key}.json`] = {
      variant: item.variant,
      installer_ref: item.ref,
      generated_at: item.at,
      files: { iso: item.key.slice("stable/".length) },
    };
  }
  return { objects, manifests, releases: {}, pointers: {}, ledgers: {} };
}

function mergeFacts(...all) {
  return all.reduce((result, facts) => ({
    objects: [...result.objects, ...(facts.objects ?? [])],
    manifests: { ...result.manifests, ...(facts.manifests ?? {}) },
    releases: { ...result.releases, ...(facts.releases ?? {}) },
    pointers: { ...result.pointers, ...(facts.pointers ?? {}) },
    ledgers: { ...result.ledgers, ...(facts.ledgers ?? {}) },
  }), { objects: [], manifests: {}, releases: {}, pointers: {}, ledgers: {} });
}

function packageFacts(names, base = "stable/releases/", dates = {}) {
  const objects = [];
  const releases = {};
  for (const [index, name] of names.entries()) {
    const prefix = `${base}${name}/x86_64/`;
    const date = dates[name] ?? `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`;
    objects.push(object(`${prefix}release.json`, date), object(`${prefix}ryoku.pkg.tar.zst`, date, 100));
    releases[`${prefix}release.json`] = { date };
  }
  return { objects, releases, manifests: {}, pointers: {}, ledgers: {} };
}

test("keeps exactly ten newest package versions and uses names to break date ties", () => {
  const names = Array.from({ length: 12 }, (_, index) => `v1.0.${index}`);
  const dates = Object.fromEntries(names.map((name) => [name, "2026-09-01T00:00:00Z"]));
  const plan = planRetention(packageFacts(names, "stable/releases/", dates), { keep: 10, now: NOW });
  const group = plan.groups.find((item) => item.group === "packages:arch:stable");

  assert.equal(group.kept.length, 10);
  assert.deepEqual(group.dropped, ["v1.0.1", "v1.0.0"]);
  assert.equal(plan.deleteKeys.length, 4);
});

test("uses manifest variant for old CachyOS names and splits stable from unstable", () => {
  const oldCachy = fixture.find((item) => item.variant === "cachyos" && item.ref === "main");
  const unstable = {
    variant: "cachyos",
    ref: "unstable-dev",
    at: "2026-10-01T00:00:00Z",
    key: "stable/ryoku-2026.10.01-r99-1234567-x86_64-unstable-dev.iso",
    size: 7000000000,
  };
  const plan = planRetention(isoFacts([oldCachy, unstable]), { keep: 10, now: NOW });

  assert.equal(plan.groups.find((item) => item.group === "isos:cachyos:stable").kept.length, 1);
  assert.equal(plan.groups.find((item) => item.group === "isos:cachyos:unstable").kept.length, 1);
  assert.equal(plan.groups.some((item) => item.group === "isos:plain:stable"), false);
});

test("deletes only sidecars which exist", () => {
  const items = Array.from({ length: 11 }, (_, index) => ({
    variant: "plain",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/ryoku-${index}.iso`,
    size: 100,
  }));
  const facts = isoFacts(items, ["", ".json", ".sha256"]);
  const plan = planRetention(facts, { keep: 10, now: NOW });

  assert.deepEqual(plan.deleteKeys, ["stable/ryoku-0.iso", "stable/ryoku-0.iso.json", "stable/ryoku-0.iso.sha256"]);
  assert.equal(plan.deleteKeys.some((key) => key.endsWith(".sig") || key.endsWith(".js")), false);
});

test("never drops the ISO named by a pointer", () => {
  const items = Array.from({ length: 11 }, (_, index) => ({
    variant: "plain",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/pointer-${index}.iso`,
    size: 100,
  }));
  const facts = isoFacts(items);
  facts.pointers["stable/latest.json"] = { files: { iso: "pointer-0.iso" } };
  const plan = planRetention(facts, { keep: 10, now: NOW });
  const group = plan.groups.find((item) => item.group === "isos:plain:stable");

  assert.equal(group.kept.length, 11);
  assert.deepEqual(group.dropped, []);
  assert.deepEqual(plan.deleteKeys, []);
});

test("does not plan deletion of heads, ledgers, keys, installer, or unknown objects", () => {
  const safe = [
    "stable/x86_64/ryoku.pkg.tar.zst",
    "stable/channels/testing/x86_64/release.json",
    "stable/void/x86_64/release.json",
    "stable/void/channels/testing/x86_64/release.json",
    "stable/releases/index.json",
    "stable/latest.json",
    "stable/latest.js",
    "stable/ryoku-release-key.pub.asc",
    "install.sh",
    "stable/surprise.data",
  ];
  const facts = { objects: safe.map((key) => object(key)), manifests: {}, releases: {}, pointers: {}, ledgers: {} };
  const plan = planRetention(facts, { keep: 1, now: NOW });

  assert.deepEqual(plan.deleteKeys, []);
});

test("skips an entire group when one release or ISO date is unparseable", () => {
  const packages = packageFacts(["v1", "v2"]);
  packages.releases["stable/releases/v1/x86_64/release.json"].date = "not-a-date";
  const isos = isoFacts([
    { variant: "plain", ref: "main", at: "2026-09-01T00:00:00Z", key: "stable/good.iso", size: 100 },
    { variant: "plain", ref: "main", at: "bad", key: "stable/bad.iso", size: 100 },
  ]);
  const plan = planRetention(mergeFacts(packages, isos), { keep: 1, now: NOW });

  assert.equal(plan.groups.find((item) => item.group === "packages:arch:stable").skipped, true);
  assert.equal(plan.groups.find((item) => item.group === "isos:plain:stable").skipped, true);
  assert.deepEqual(plan.deleteKeys, []);
});

test("drops a legacy manifest-less ISO using its filename and upload time", () => {
  const newer = Array.from({ length: 10 }, (_, index) => ({
    variant: "plain",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/ryoku-2026.09.${String(index + 1).padStart(2, "0")}-r${index + 1}-1234567-x86_64-v1.0.${index}.iso`,
    size: 100,
  }));
  const legacy = "stable/ryoku-2026.08.20-r187-febd279-x86_64-main.iso";
  const facts = mergeFacts(isoFacts(newer), {
    objects: [object(legacy, "2026-08-20T00:00:00Z"), object(`${legacy}.sha256`), object(`${legacy}.sig`)],
  });
  const plan = planRetention(facts, { keep: 10, now: NOW });

  assert.deepEqual(plan.deleteKeys, [legacy, `${legacy}.sha256`, `${legacy}.sig`]);
  assert.equal(plan.skipped.length, 0);
});

test("keeps a brand-new manifest-less ISO while its manifest is uploading", () => {
  const older = Array.from({ length: 10 }, (_, index) => ({
    variant: "cachyos",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/ryoku-2026.09.${String(index + 1).padStart(2, "0")}-r${index + 1}-1234567-x86_64-v1.0.${index}-cachyos.iso`,
    size: 100,
  }));
  const fresh = "stable/ryoku-2026.10.10-r99-abcdef0-x86_64-main-cachyos.iso";
  const facts = mergeFacts(isoFacts(older), { objects: [object(fresh, "2026-10-10T11:59:00Z", 100)] });
  const plan = planRetention(facts, { keep: 10, now: NOW });
  const group = plan.groups.find((item) => item.group === "isos:cachyos:stable");

  assert.equal(group.kept.includes(fresh), true);
  assert.equal(plan.deleteKeys.includes(fresh), false);
  assert.equal(group.dropped.length, 1);
});

test("keeps a manifest-less ISO whose filename is not recognized", () => {
  const facts = { objects: [object("stable/orphan.iso")], manifests: {}, releases: {}, pointers: {}, ledgers: {} };
  const plan = planRetention(facts, { keep: 1, now: NOW });

  assert.deepEqual(plan.deleteKeys, []);
  assert.deepEqual(plan.skipped, [{ group: "stable/orphan.iso", reason: "unrecognized ISO filename" }]);
});

test("removes incomplete version debris only after 48 hours", () => {
  const facts = {
    objects: [
      object("stable/releases/stale/x86_64/package", "2026-10-08T11:59:59Z"),
      object("stable/releases/boundary/x86_64/package", "2026-10-08T12:00:00Z"),
      object("stable/releases/fresh/x86_64/package", "2026-10-09T12:00:00Z"),
    ],
    manifests: {}, releases: {}, pointers: {}, ledgers: {},
  };
  const plan = planRetention(facts, { keep: 10, now: NOW });

  assert.deepEqual(plan.deleteKeys, ["stable/releases/stale/x86_64/package"]);
  assert.equal(plan.debris.length, 1);
});

test("filters missing releases and deleted images while leaving an untouched ledger alone", () => {
  const packages = packageFacts(["v1", "v2"], "stable/releases/", {
    v1: "2026-09-01T00:00:00Z",
    v2: "2026-09-02T00:00:00Z",
  });
  const isos = isoFacts([
    { variant: "plain", ref: "v1.0.0", at: "2026-09-01T00:00:00Z", key: "stable/old.iso", size: 100 },
    { variant: "plain", ref: "v2.0.0", at: "2026-09-02T00:00:00Z", key: "stable/new.iso", size: 100 },
  ]);
  const ledger = {
    schema: 1,
    latest: "missing",
    releases: [
      { tag: "missing", repo: "releases/missing/x86_64" },
      { tag: "v2", repo: "releases/v2/x86_64", images: { plain: { iso: "new.iso" }, cachyos: { iso: "gone.iso" } } },
      { tag: "v1", repo: "releases/v1/x86_64", images: { plain: { iso: "old.iso" } } },
    ],
  };
  const untouched = { schema: 1, latest: "v2", releases: [{ tag: "v2", repo: "releases/v2/x86_64", images: { plain: { iso: "new.iso" } } }] };
  const facts = mergeFacts(packages, isos, {
    ledgers: {
      "stable/releases/index.json": ledger,
      "stable/channels/testing/index.json": untouched,
    },
  });
  const plan = planRetention(facts, { keep: 1, now: NOW });

  assert.equal(plan.ledgerEdits.length, 1);
  const next = plan.ledgerEdits[0].value;
  assert.equal(next.latest, "v2");
  assert.deepEqual(next.releases.map((release) => release.tag), ["v2"]);
  assert.deepEqual(Object.keys(next.releases[0].images), ["plain"]);
});

test("honours a non-default KEEP value", () => {
  const plan = planRetention(packageFacts(["v1", "v2", "v3"]), { keep: "2", now: NOW });
  const group = plan.groups.find((item) => item.group === "packages:arch:stable");

  assert.equal(group.kept.length, 2);
  assert.deepEqual(group.dropped, ["v1"]);
});

test("retains each Fedora release and channel as an independent package group", () => {
  const names = Array.from({ length: 11 }, (_, index) => `v1.0.${index}`);
  const facts = mergeFacts(
    packageFacts(names, "stable/fedora/44/releases/"),
    packageFacts(names, "stable/fedora/44/channels/testing/builds/"),
    packageFacts(names, "stable/fedora/45/releases/"),
    {
      objects: [
        object("stable/fedora/44/x86_64/release.json"),
        object("stable/fedora/44/channels/testing/x86_64/release.json"),
        object("stable/fedora/44/releases/index.json"),
      ],
    },
  );
  const plan = planRetention(facts, { keep: 10, now: NOW });

  for (const groupName of [
    "packages:fedora:44:stable",
    "packages:fedora:44:unstable",
    "packages:fedora:45:stable",
  ]) {
    const group = plan.groups.find((item) => item.group === groupName);
    assert.deepEqual(group.dropped, ["v1.0.0"]);
  }
  assert.equal(plan.deleteKeys.filter((key) => key.includes("/v1.0.0/")).length, 6);
  assert.equal(plan.deleteKeys.some((key) => key.endsWith("/index.json")), false);
  assert.equal(plan.deleteKeys.some((key) => key.includes("/x86_64/release.json") && !key.includes("/v1.0.0/")), false);
});

test("filters a Fedora ledger after its oldest repository is removed", () => {
  const packages = packageFacts(["v1", "v2"], "stable/fedora/44/releases/", {
    v1: "2026-09-01T00:00:00Z",
    v2: "2026-09-02T00:00:00Z",
  });
  const ledger = {
    schema: 1,
    latest: "v2",
    releases: [
      { tag: "v2", repo: "fedora/44/releases/v2/x86_64" },
      { tag: "v1", repo: "fedora/44/releases/v1/x86_64" },
    ],
  };
  const facts = mergeFacts(packages, {
    ledgers: { "stable/fedora/44/releases/index.json": ledger },
  });
  const plan = planRetention(facts, { keep: 1, now: NOW });

  assert.equal(plan.ledgerEdits.length, 1);
  assert.equal(plan.ledgerEdits[0].key, "stable/fedora/44/releases/index.json");
  assert.deepEqual(plan.ledgerEdits[0].value.releases.map((release) => release.tag), ["v2"]);
});

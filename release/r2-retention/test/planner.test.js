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

test("keeps the six newest package directories (current + five rollback) and uses names to break date ties", () => {
  const names = Array.from({ length: 8 }, (_, index) => `v1.0.${index}`);
  const dates = Object.fromEntries(names.map((name) => [name, "2026-09-01T00:00:00Z"]));
  const plan = planRetention(packageFacts(names, "stable/releases/", dates), { now: NOW });
  const group = plan.groups.find((item) => item.group === "packages:arch:stable");

  assert.deepEqual(group.kept, ["v1.0.7", "v1.0.6", "v1.0.5", "v1.0.4", "v1.0.3", "v1.0.2"]);
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
  const plan = planRetention(isoFacts([oldCachy, unstable]), { now: NOW });

  assert.equal(plan.groups.find((item) => item.group === "isos:cachyos:stable").kept.length, 1);
  assert.equal(plan.groups.find((item) => item.group === "isos:cachyos:unstable").kept.length, 1);
  assert.equal(plan.groups.some((item) => item.group === "isos:plain:stable"), false);
});

test("deletes only sidecars which exist", () => {
  const items = Array.from({ length: 4 }, (_, index) => ({
    variant: "plain",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/ryoku-${index}.iso`,
    size: 100,
  }));
  const facts = isoFacts(items, ["", ".json", ".sha256"]);
  const plan = planRetention(facts, { now: NOW });

  assert.deepEqual(plan.deleteKeys, ["stable/ryoku-0.iso", "stable/ryoku-0.iso.json", "stable/ryoku-0.iso.sha256"]);
  assert.equal(plan.deleteKeys.some((key) => key.endsWith(".sig") || key.endsWith(".js")), false);
});

test("never drops the ISO named by a pointer", () => {
  const items = Array.from({ length: 4 }, (_, index) => ({
    variant: "plain",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/pointer-${index}.iso`,
    size: 100,
  }));
  const facts = isoFacts(items);
  facts.pointers["stable/latest.json"] = { files: { iso: "pointer-0.iso" } };
  const plan = planRetention(facts, { now: NOW });
  const group = plan.groups.find((item) => item.group === "isos:plain:stable");

  assert.equal(group.kept.length, 4);
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
  const plan = planRetention(facts, { keepPackages: 1, keepIsoVersions: 1, now: NOW });

  assert.deepEqual(plan.deleteKeys, []);
});

test("skips an entire group when one release or ISO date is unparseable", () => {
  const packages = packageFacts(["v1", "v2"]);
  packages.releases["stable/releases/v1/x86_64/release.json"].date = "not-a-date";
  const isos = isoFacts([
    { variant: "plain", ref: "main", at: "2026-09-01T00:00:00Z", key: "stable/good.iso", size: 100 },
    { variant: "plain", ref: "main", at: "bad", key: "stable/bad.iso", size: 100 },
  ]);
  const plan = planRetention(mergeFacts(packages, isos), { keepPackages: 1, keepIsoVersions: 1, now: NOW });

  assert.equal(plan.groups.find((item) => item.group === "packages:arch:stable").skipped, true);
  assert.equal(plan.groups.find((item) => item.group === "isos:plain:stable").skipped, true);
  assert.deepEqual(plan.deleteKeys, []);
});

test("drops a legacy manifest-less ISO using its filename and upload time", () => {
  const newer = Array.from({ length: 3 }, (_, index) => ({
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
  const plan = planRetention(facts, { now: NOW });

  assert.deepEqual(plan.deleteKeys, [legacy, `${legacy}.sha256`, `${legacy}.sig`]);
  assert.equal(plan.skipped.length, 0);
});

test("keeps a brand-new manifest-less ISO while its manifest is uploading", () => {
  const older = Array.from({ length: 3 }, (_, index) => ({
    variant: "cachyos",
    ref: `v1.0.${index}`,
    at: `2026-09-${String(index + 1).padStart(2, "0")}T00:00:00Z`,
    key: `stable/ryoku-2026.09.${String(index + 1).padStart(2, "0")}-r${index + 1}-1234567-x86_64-v1.0.${index}-cachyos.iso`,
    size: 100,
  }));
  const fresh = "stable/ryoku-2026.10.10-r99-abcdef0-x86_64-main-cachyos.iso";
  const facts = mergeFacts(isoFacts(older), { objects: [object(fresh, "2026-10-10T11:59:00Z", 100)] });
  const plan = planRetention(facts, { now: NOW });
  const group = plan.groups.find((item) => item.group === "isos:cachyos:stable");

  assert.equal(group.kept.includes(fresh), true);
  assert.equal(plan.deleteKeys.includes(fresh), false);
  assert.equal(group.dropped.length, 1);
});

test("keeps a manifest-less ISO whose filename is not recognized", () => {
  const facts = { objects: [object("stable/orphan.iso")], manifests: {}, releases: {}, pointers: {}, ledgers: {} };
  const plan = planRetention(facts, { keepPackages: 1, keepIsoVersions: 1, now: NOW });

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
  const plan = planRetention(facts, { now: NOW });

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
  const plan = planRetention(facts, { keepPackages: 1, keepIsoVersions: 1, now: NOW });

  assert.equal(plan.ledgerEdits.length, 1);
  const next = plan.ledgerEdits[0].value;
  assert.equal(next.latest, "v2");
  assert.deepEqual(next.releases.map((release) => release.tag), ["v2"]);
  assert.deepEqual(Object.keys(next.releases[0].images), ["plain"]);
});

test("honours non-default KEEP_PACKAGES and KEEP_ISO_VERSIONS values", () => {
  const isos = isoFacts(["v1.0.0", "v1.0.1", "v1.0.2"].map((ref, index) => ({
    variant: "plain",
    ref,
    at: `2026-09-0${index + 1}T00:00:00Z`,
    key: `stable/${ref}.iso`,
    size: 100,
  })));
  const plan = planRetention(mergeFacts(packageFacts(["v1", "v2", "v3"]), isos), {
    keepPackages: "2",
    keepIsoVersions: "1",
    now: NOW,
  });

  assert.deepEqual(plan.groups.find((item) => item.group === "packages:arch:stable").dropped, ["v1"]);
  assert.deepEqual(plan.groups.find((item) => item.group === "isos:plain:stable").dropped, [
    "stable/v1.0.1.iso",
    "stable/v1.0.0.iso",
  ]);
  assert.throws(() => planRetention(isos, { keepPackages: "0", now: NOW }), /KEEP_PACKAGES/);
  assert.throws(() => planRetention(isos, { keepIsoVersions: "x", now: NOW }), /KEEP_ISO_VERSIONS/);
});

test("retains each Fedora release and channel as an independent package group", () => {
  const names = Array.from({ length: 7 }, (_, index) => `v1.0.${index}`);
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
  const plan = planRetention(facts, { now: NOW });

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
  const plan = planRetention(facts, { keepPackages: 1, now: NOW });

  assert.equal(plan.ledgerEdits.length, 1);
  assert.equal(plan.ledgerEdits[0].key, "stable/fedora/44/releases/index.json");
  assert.deepEqual(plan.ledgerEdits[0].value.releases.map((release) => release.tag), ["v2"]);
});

test("keeps the newest build of each of the three newest ISO versions", () => {
  const builds = [
    ["v1.0.4", "2026-10-25T23:38:17Z", "stable/ryoku-2026.10.25-r226-ad2d365-x86_64-v1.0.4.iso"],
    ["v1.0.5", "2026-10-26T04:05:19Z", "stable/ryoku-2026.10.26-r227-e344edb-x86_64-v1.0.5.iso"],
    ["v1.0.6", "2026-10-28T20:02:41Z", "stable/ryoku-2026.10.28-r228-e24972a-x86_64-v1.0.6.iso"],
    ["v1.0.6", "2026-10-29T04:55:25Z", "stable/ryoku-2026.10.29-r229-6d573d9-x86_64-v1.0.6.iso"],
    ["v1.0.8", "2026-11-11T06:30:27Z", "stable/ryoku-2026.11.11-r231-2befae3-x86_64-v1.0.8.iso"],
    ["v1.0.9", "2026-11-12T01:48:42Z", "stable/ryoku-2026.11.12-r232-08ff328-x86_64-v1.0.9.iso"],
  ];
  const facts = isoFacts(builds.map(([ref, at, key]) => ({ variant: "plain", ref, at, key, size: 7000000000 })));
  const plan = planRetention(facts, { now: "2026-11-16T12:00:00Z" });
  const group = plan.groups.find((item) => item.group === "isos:plain:stable");

  assert.deepEqual(group.kept, [
    "stable/ryoku-2026.11.12-r232-08ff328-x86_64-v1.0.9.iso",
    "stable/ryoku-2026.11.11-r231-2befae3-x86_64-v1.0.8.iso",
    "stable/ryoku-2026.10.29-r229-6d573d9-x86_64-v1.0.6.iso",
  ]);
  assert.deepEqual(group.dropped, [
    "stable/ryoku-2026.10.28-r228-e24972a-x86_64-v1.0.6.iso",
    "stable/ryoku-2026.10.26-r227-e344edb-x86_64-v1.0.5.iso",
    "stable/ryoku-2026.10.25-r226-ad2d365-x86_64-v1.0.4.iso",
  ]);
  assert.equal(plan.deleteKeys.length, 15);
});

test("counts every unstable ISO build as its own version", () => {
  const builds = Array.from({ length: 4 }, (_, index) => ({
    variant: "cachyos",
    ref: "unstable-dev",
    at: `2026-11-0${index + 1}T00:00:00Z`,
    key: `stable/ryoku-2026.11.0${index + 1}-r${index + 1}-abcdef0-x86_64-unstable-dev-cachyos.iso`,
    size: 100,
  }));
  const plan = planRetention(isoFacts(builds), { now: NOW });
  const group = plan.groups.find((item) => item.group === "isos:cachyos:unstable");

  assert.equal(group.kept.length, 3);
  assert.deepEqual(group.dropped, ["stable/ryoku-2026.11.01-r1-abcdef0-x86_64-unstable-dev-cachyos.iso"]);
});

test("never drops the frozen directory a channel head serves", () => {
  const names = Array.from({ length: 8 }, (_, index) => `v1.0.${index}`);
  const facts = packageFacts(names);
  for (const name of names) {
    facts.releases[`stable/releases/${name}/x86_64/release.json`].release = name;
  }
  facts.heads = { "stable/x86_64/release.json": { release: "v1.0.0" } };
  const plan = planRetention(facts, { now: NOW });
  const group = plan.groups.find((item) => item.group === "packages:arch:stable");

  assert.equal(group.kept.includes("v1.0.0"), true);
  assert.deepEqual(group.dropped, ["v1.0.1"]);
  assert.equal(plan.deleteKeys.some((key) => key.includes("/v1.0.0/")), false);
});

// A trimmed snapshot of the real ryoku-iso bucket on 2026-11-16: every
// root object, heads, ledgers and pointers, and each frozen directory's
// release.json plus two of its files, with their real sizes and dates.
test("plans the real bucket: owner policy, and nothing that must stay", async () => {
  const bucket = JSON.parse(await readFile(new URL("./fixtures/bucket-2026-11-16.json", import.meta.url), "utf8"));
  const plan = planRetention(bucket, { now: "2026-11-16T12:00:00Z" });
  const kept = Object.fromEntries(plan.groups.map((group) => [group.group, group.kept]));
  const dropped = Object.fromEntries(plan.groups.map((group) => [group.group, group.dropped]));
  const stableTags = ["v1.0.9", "v1.0.8", "v1.0.7", "v1.0.6", "v1.0.5", "v1.0.4"];

  assert.deepEqual(plan.skipped, []);
  assert.deepEqual(plan.debris, []);
  for (const group of ["arch", "void", "fedora:44", "fedora:45"]) {
    assert.deepEqual(kept[`packages:${group}:stable`], stableTags);
  }
  assert.deepEqual(dropped["packages:arch:stable"], ["v1.0.3", "v1.0.2", "v1.0.1", "v1.0.0"]);
  assert.deepEqual(dropped["packages:void:stable"], ["v1.0.3", "v1.0.2", "v1.0.1"]);
  assert.deepEqual(kept["packages:arch:unstable"], [
    "v1.1.0.dev.1035_g0389a5e",
    "v1.1.0.dev.1034_g229e6ba",
    "v1.0.9.dev.1032_g59985f7",
    "v1.0.9.dev.1031_g0b1db92",
    "v1.0.9.dev.1030_g9c71328",
    "v1.0.9.dev.1029_gbc873de",
  ]);
  assert.equal(kept["packages:fedora:44:unstable"][0], "v1.1.0.dev.256_g0389a5e");
  assert.equal(kept["packages:fedora:44:unstable"].length, 6);
  assert.equal(dropped["packages:fedora:44:unstable"].length, 81);
  assert.equal(dropped["packages:fedora:45:unstable"].length, 79);

  assert.deepEqual(kept["isos:plain:stable"], [
    "stable/ryoku-2026.11.12-r232-08ff328-x86_64-v1.0.9.iso",
    "stable/ryoku-2026.11.11-r231-2befae3-x86_64-v1.0.8.iso",
    "stable/ryoku-2026.10.29-r229-6d573d9-x86_64-v1.0.6.iso",
  ]);
  assert.deepEqual(kept["isos:cachyos:stable"], [
    "stable/ryoku-2026.11.12-r72-08ff328-x86_64-v1.0.9-cachyos.iso",
    "stable/ryoku-2026.11.11-r71-2befae3-x86_64-v1.0.8-cachyos.iso",
    "stable/ryoku-2026.10.29-r69-6d573d9-x86_64-v1.0.6-cachyos.iso",
  ]);
  assert.deepEqual(kept["isos:void:stable"], [
    "stable/ryoku-2026.11.12-r9-08ff328-x86_64-v1.0.9.iso",
    "stable/ryoku-2026.11.11-r8-2befae3-x86_64-v1.0.8.iso",
    "stable/ryoku-2026.10.28-r6-d5ea9e5-x86_64-v1.0.6.iso",
  ]);
  const droppedIsos = ["plain", "cachyos", "void"].flatMap((variant) => dropped[`isos:${variant}:stable`]);
  assert.equal(droppedIsos.length, 19);

  const deleted = new Set(plan.deleteKeys);
  const keptIsos = ["plain", "cachyos", "void"].flatMap((variant) => kept[`isos:${variant}:stable`]);
  const mustStay = bucket.objects.map((object) => object.key).filter((key) =>
    !key.startsWith("stable/")
    || /^stable\/(?:void\/|fedora\/[0-9]+\/)?(?:channels\/testing\/)?x86_64\//.test(key)
    || key.endsWith("/index.json")
    || /^stable\/latest(?:-[a-z0-9-]+)?\.(?:json|js)$/.test(key)
    || keptIsos.some((iso) => key === iso || key.startsWith(`${iso}.`))
    || [
      "stable/SHA256SUMS",
      "stable/ryoku-release-key.pub.asc",
      "stable/fedora/releases",
    ].includes(key));
  assert.ok(mustStay.includes("install.sh"));
  assert.ok(mustStay.includes("Dismantle7-Decathlon1-Overdrive2-Buddhism1"));
  assert.ok(mustStay.includes("stable/x86_64/release.json"));
  assert.deepEqual(mustStay.filter((key) => deleted.has(key)), []);

  // Every deletion belongs to a dropped frozen directory or a dropped ISO.
  const droppedDirectories = plan.groups
    .filter((group) => group.kind === "packages")
    .flatMap((group) => group.dropped.map((name) => `/${name}/`));
  for (const key of plan.deleteKeys) {
    const fromIso = droppedIsos.some((iso) => key === iso || key.startsWith(`${iso}.`));
    const fromDirectory = droppedDirectories.some((part) => key.includes(part));
    assert.ok(fromIso || fromDirectory, key);
  }

  // Every ledger loses exactly what was deleted and keeps its newest entry.
  assert.equal(plan.ledgerEdits.length, 8);
  for (const edit of plan.ledgerEdits) {
    assert.equal(edit.value.releases.length, 6, edit.key);
    assert.equal(edit.value.latest, bucket.ledgers[edit.key].latest, edit.key);
  }
  const stableLedger = plan.ledgerEdits.find((edit) => edit.key === "stable/releases/index.json").value;
  const ledgerIsos = stableLedger.releases.flatMap((release) => Object.values(release.images ?? {}).map((image) => `stable/${image.iso}`));
  assert.deepEqual(ledgerIsos.filter((iso) => deleted.has(iso)), []);
});

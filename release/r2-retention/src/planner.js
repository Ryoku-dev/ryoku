const STABLE_RELEASE = /^v[0-9]+\.[0-9]+\.[0-9]+(-(alpha|beta|rc)\.[0-9]+)?$/;
const ISO_SIDECARS = ["", ".sha256", ".sig", ".json", ".js"];
const DEBRIS_AGE_MS = 48 * 60 * 60 * 1000;

// The newest frozen repository directory is what a channel serves now; the
// other five are the rollback depth `ryoku rollback` and `ryoku track <tag>`
// can still reach.
export const DEFAULT_KEEP_PACKAGES = 6;
// The current ISO release and the two before it, per variant and channel.
export const DEFAULT_KEEP_ISO_VERSIONS = 3;

function timestamp(value) {
  if (typeof value !== "string" && !(value instanceof Date)) return NaN;
  return Date.parse(value);
}

function newestFirst(a, b) {
  return b.time - a.time || b.name.localeCompare(a.name);
}

function packageGroups(root, objects) {
  const groups = [
    { id: "packages:arch:stable", base: `${root}releases/`, head: `${root}x86_64/release.json` },
    { id: "packages:void:stable", base: `${root}void/releases/`, head: `${root}void/x86_64/release.json` },
    {
      id: "packages:arch:unstable",
      base: `${root}channels/testing/builds/`,
      head: `${root}channels/testing/x86_64/release.json`,
    },
    {
      id: "packages:void:unstable",
      base: `${root}void/channels/testing/builds/`,
      head: `${root}void/channels/testing/x86_64/release.json`,
    },
  ];
  const escapedRoot = root.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const fedoraReleases = new Set();
  const pattern = new RegExp(`^${escapedRoot}fedora/([0-9]+)/(?:releases|channels/testing/builds)/`);
  for (const object of objects) {
    const match = pattern.exec(object.key);
    if (match) fedoraReleases.add(match[1]);
  }
  for (const release of [...fedoraReleases].sort((a, b) => Number(a) - Number(b))) {
    const edition = `${root}fedora/${release}/`;
    groups.push(
      { id: `packages:fedora:${release}:stable`, base: `${edition}releases/`, head: `${edition}x86_64/release.json` },
      {
        id: `packages:fedora:${release}:unstable`,
        base: `${edition}channels/testing/builds/`,
        head: `${edition}channels/testing/x86_64/release.json`,
      },
    );
  }
  return groups;
}

function positiveInteger(value, name) {
  const number = typeof value === "number" ? value : Number.parseInt(value, 10);
  if (!Number.isInteger(number) || number < 1) throw new Error(`${name} must be a positive integer`);
  return number;
}

// Stable ISOs are grouped by their release tag, so a rebuild of the same tag
// counts as the same version. Any other ref (main, unstable-dev) has no
// release identity: each such image is its own version.
function isoVersion(installerRef, key) {
  return STABLE_RELEASE.test(installerRef) ? installerRef : key;
}

function versionDirectory(key, base) {
  if (!key.startsWith(base)) return null;
  const rest = key.slice(base.length);
  const slash = rest.indexOf("/");
  if (slash <= 0) return null;
  const name = rest.slice(0, slash);
  const directory = `${base}${name}/`;
  return { name, directory, releaseKey: `${directory}x86_64/release.json` };
}

function pointerIso(pointer) {
  if (!pointer || typeof pointer !== "object") return null;
  if (typeof pointer.files?.iso === "string") return pointer.files.iso;
  if (typeof pointer.url === "string") {
    try {
      return decodeURIComponent(new URL(pointer.url).pathname.split("/").pop());
    } catch {
      return null;
    }
  }
  return null;
}

function isoFromFilename(key, root) {
  if (!key.startsWith(root) || key.slice(root.length).includes("/")) return null;
  const match = /^ryoku-\d{4}\.\d{2}\.\d{2}-r\d+-[0-9a-f]{7,40}-x86_64-(.+)\.iso$/.exec(key.slice(root.length));
  if (!match) return null;
  let installerRef = match[1];
  let variant = "plain";
  for (const suffix of ["cachyos", "void"]) {
    if (installerRef.endsWith(`-${suffix}`)) {
      installerRef = installerRef.slice(0, -(suffix.length + 1));
      variant = suffix;
      break;
    }
  }
  return installerRef ? { variant, installerRef } : null;
}

function sameJson(a, b) {
  return JSON.stringify(a) === JSON.stringify(b);
}

function filterLedger(key, ledger, objectKeys, deleted, root) {
  if (!ledger || !Array.isArray(ledger.releases)) return null;
  const releases = [];
  for (const original of ledger.releases) {
    if (!original || typeof original.repo !== "string") continue;
    const releaseKey = `${root}${original.repo.replace(/^\/+|\/+$/g, "")}/release.json`;
    if (!objectKeys.has(releaseKey) || deleted.has(releaseKey)) continue;

    const release = { ...original };
    if (release.images && typeof release.images === "object") {
      const images = {};
      for (const [variant, image] of Object.entries(release.images)) {
        if (!image || typeof image.iso !== "string") continue;
        const isoKey = image.iso.startsWith(root) ? image.iso : `${root}${image.iso}`;
        if (objectKeys.has(isoKey) && !deleted.has(isoKey)) images[variant] = image;
      }
      if (Object.keys(images).length) release.images = images;
      else delete release.images;
    }
    releases.push(release);
  }
  const next = { ...ledger, latest: releases[0]?.tag ?? "", releases };
  if (sameJson(ledger, next)) return null;
  return { key, value: next };
}

export function planRetention(facts, options = {}) {
  const root = (options.root ?? "stable/").replace(/\/*$/, "/");
  const keepPackages = positiveInteger(options.keepPackages ?? DEFAULT_KEEP_PACKAGES, "KEEP_PACKAGES");
  const keepIsoVersions = positiveInteger(options.keepIsoVersions ?? DEFAULT_KEEP_ISO_VERSIONS, "KEEP_ISO_VERSIONS");
  const now = timestamp(options.now ?? new Date());
  if (!Number.isFinite(now)) throw new Error("now must be a valid date");

  const objects = Array.isArray(facts.objects) ? facts.objects : [];
  const objectByKey = new Map(objects.map((object) => [object.key, object]));
  const objectKeys = new Set(objectByKey.keys());
  const releases = facts.releases ?? {};
  const manifests = facts.manifests ?? {};
  const pointers = facts.pointers ?? {};
  const ledgers = facts.ledgers ?? {};
  const heads = facts.heads ?? {};
  const groups = [];
  const skipped = [];
  const debris = [];
  const deleteKeys = new Set();

  for (const definition of packageGroups(root, objects)) {
    const directories = new Map();
    for (const object of objects) {
      const found = versionDirectory(object.key, definition.base);
      if (!found) continue;
      const directory = directories.get(found.name) ?? { ...found, objects: [] };
      directory.objects.push(object);
      directories.set(found.name, directory);
    }

    const versions = [];
    let badDate = false;
    for (const directory of directories.values()) {
      if (!objectKeys.has(directory.releaseKey)) {
        const newest = Math.max(...directory.objects.map((object) => timestamp(object.uploaded ?? object.last_modified)));
        if (Number.isFinite(newest) && now - newest > DEBRIS_AGE_MS) {
          const keys = directory.objects.map((object) => object.key).sort();
          for (const key of keys) deleteKeys.add(key);
          debris.push({ group: definition.id, directory: directory.directory, keys });
        }
        continue;
      }
      const release = releases[directory.releaseKey];
      const time = timestamp(release?.date);
      if (!Number.isFinite(time)) badDate = true;
      const tag = typeof release?.release === "string" ? release.release : null;
      versions.push({ name: directory.name, time, tag, objects: directory.objects });
    }

    if (badDate) {
      skipped.push({ group: definition.id, reason: "unparseable release date" });
      groups.push({ group: definition.id, kind: "packages", kept: versions.map((item) => item.name).sort(), dropped: [], skipped: true });
      continue;
    }

    // The frozen directory the moving head serves is never dropped, even when
    // newer frozen directories exist (a publish that froze but never moved).
    const served = typeof heads[definition.head]?.release === "string" ? heads[definition.head].release : null;
    versions.sort(newestFirst);
    const kept = [];
    const dropped = [];
    for (const [index, version] of versions.entries()) {
      if (index < keepPackages || (served !== null && version.tag === served)) kept.push(version);
      else dropped.push(version);
    }
    for (const version of dropped) {
      for (const object of version.objects) deleteKeys.add(object.key);
    }
    groups.push({
      group: definition.id,
      kind: "packages",
      kept: kept.map((item) => item.name),
      dropped: dropped.map((item) => item.name),
      skipped: false,
    });
  }

  const protectedIsos = new Set();
  for (const pointer of Object.values(pointers)) {
    const name = pointerIso(pointer);
    if (name) protectedIsos.add(name.startsWith(root) ? name : `${root}${name}`);
  }

  const isoGroups = new Map();
  const isoPattern = new RegExp(`^${root.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}[^/]+\\.iso$`);
  for (const object of objects) {
    if (!isoPattern.test(object.key)) continue;
    const manifest = manifests[`${object.key}.json`];
    const hasManifestIdentity = manifest
      && typeof manifest === "object"
      && typeof manifest.variant === "string"
      && typeof manifest.installer_ref === "string";
    const fallback = hasManifestIdentity ? null : isoFromFilename(object.key, root);
    if (!hasManifestIdentity && !fallback) {
      skipped.push({ group: object.key, reason: "unrecognized ISO filename" });
      continue;
    }
    const variant = hasManifestIdentity ? manifest.variant : fallback.variant;
    const installerRef = hasManifestIdentity ? manifest.installer_ref : fallback.installerRef;
    const time = timestamp(hasManifestIdentity ? manifest.generated_at : object.uploaded ?? object.last_modified);
    const channel = installerRef === "main" || STABLE_RELEASE.test(installerRef) ? "stable" : "unstable";
    const group = `isos:${variant}:${channel}`;
    const members = isoGroups.get(group) ?? [];
    members.push({ name: object.key, time, version: isoVersion(installerRef, object.key), object });
    isoGroups.set(group, members);
  }

  for (const [group, members] of [...isoGroups.entries()].sort(([a], [b]) => a.localeCompare(b))) {
    if (members.some((member) => !Number.isFinite(member.time))) {
      skipped.push({ group, reason: "unparseable ISO date" });
      groups.push({ group, kind: "isos", kept: members.map((item) => item.name).sort(), dropped: [], skipped: true });
      continue;
    }
    members.sort(newestFirst);
    // Walk newest first: the newest build of each of the newest
    // keepIsoVersions versions survives; older rebuilds of a kept version and
    // every older version go, unless a latest pointer names them.
    const versions = new Set();
    const kept = [];
    const dropped = [];
    for (const member of members) {
      let keepMember = protectedIsos.has(member.name);
      if (!versions.has(member.version) && versions.size < keepIsoVersions) {
        versions.add(member.version);
        keepMember = true;
      }
      if (keepMember) {
        kept.push(member);
        continue;
      }
      dropped.push(member);
      for (const suffix of ISO_SIDECARS) {
        const key = `${member.name}${suffix}`;
        if (objectKeys.has(key)) deleteKeys.add(key);
      }
    }
    groups.push({
      group,
      kind: "isos",
      kept: kept.map((member) => member.name),
      dropped: dropped.map((member) => member.name),
      skipped: false,
    });
  }

  const deleted = new Set(deleteKeys);
  const ledgerEdits = [];
  for (const [key, ledger] of Object.entries(ledgers)) {
    const edit = filterLedger(key, ledger, objectKeys, deleted, root);
    if (edit) ledgerEdits.push(edit);
  }

  return {
    keepPackages,
    keepIsoVersions,
    root,
    groups,
    skipped,
    debris,
    deleteKeys: [...deleteKeys].sort(),
    ledgerEdits,
  };
}

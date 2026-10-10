import { planRetention } from "./planner.js";

const CONNECTIONS = 4;
const CORE_LEDGER_KEYS = [
  "releases/index.json",
  "void/releases/index.json",
  "channels/testing/index.json",
  "void/channels/testing/index.json",
];
const CORE_PACKAGE_BASES = [
  "releases/",
  "void/releases/",
  "channels/testing/builds/",
  "void/channels/testing/builds/",
];

class OperationBudgetError extends Error {}

class Operations {
  constructor(limit) {
    this.limit = limit;
    this.used = 0;
  }

  async run(operation) {
    if (this.used >= this.limit) throw new OperationBudgetError(`operation budget ${this.limit} exhausted`);
    this.used += 1;
    return operation();
  }

  canFit(count) {
    return this.used + count <= this.limit;
  }
}

function rootPath(value) {
  return String(value || "stable/").replace(/\/*$/, "/");
}

function storedObject(object) {
  return {
    key: object.key,
    size: object.size ?? 0,
    uploaded: object.uploaded instanceof Date ? object.uploaded.toISOString() : object.uploaded,
  };
}

async function listAll(bucket, operations, options) {
  const objects = [];
  const prefixes = [];
  let cursor;
  do {
    const page = await operations.run(() => bucket.list({ ...options, cursor }));
    objects.push(...page.objects.map(storedObject));
    prefixes.push(...(page.delimitedPrefixes ?? []));
    cursor = page.truncated ? page.cursor : undefined;
  } while (cursor);
  return { objects, prefixes };
}

async function readJson(bucket, operations, key) {
  const object = await operations.run(() => bucket.get(key));
  if (!object) return null;
  try {
    return JSON.parse(await object.text());
  } catch {
    return null;
  }
}

async function mapConcurrent(values, concurrency, visit) {
  const results = new Array(values.length);
  let next = 0;
  async function worker() {
    while (next < values.length) {
      const index = next++;
      results[index] = await visit(values[index]);
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, values.length) }, worker));
  return results;
}

export async function gatherFacts(bucket, root, operations) {
  const rootListing = await listAll(bucket, operations, { prefix: root, delimiter: "/" });
  const objects = [...rootListing.objects];
  const fedoraListing = await listAll(bucket, operations, { prefix: `${root}fedora/`, delimiter: "/" });
  objects.push(...fedoraListing.objects);
  const escapedFedoraRoot = `${root}fedora/`.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const fedoraPattern = new RegExp(`^${escapedFedoraRoot}([0-9]+)/$`);
  const fedoraReleases = fedoraListing.prefixes
    .map((prefix) => fedoraPattern.exec(prefix)?.[1])
    .filter(Boolean)
    .sort((a, b) => Number(a) - Number(b));
  const packageBases = [...CORE_PACKAGE_BASES];
  const ledgerKeys = [...CORE_LEDGER_KEYS];
  for (const release of fedoraReleases) {
    packageBases.push(
      `fedora/${release}/releases/`,
      `fedora/${release}/channels/testing/builds/`,
    );
    ledgerKeys.push(
      `fedora/${release}/releases/index.json`,
      `fedora/${release}/channels/testing/index.json`,
    );
  }

  const directoryListings = [];
  for (const relativeBase of packageBases) {
    const base = `${root}${relativeBase}`;
    const listing = await listAll(bucket, operations, { prefix: base, delimiter: "/" });
    objects.push(...listing.objects);
    directoryListings.push(...listing.prefixes);
  }

  const versionObjects = await mapConcurrent(directoryListings, CONNECTIONS, async (prefix) => {
    const listing = await listAll(bucket, operations, { prefix });
    return listing.objects;
  });
  for (const listed of versionObjects) objects.push(...listed);

  const byKey = new Map(objects.map((object) => [object.key, object]));
  const uniqueObjects = [...byKey.values()];
  const releaseKeys = uniqueObjects
    .map((object) => object.key)
    .filter((key) => key.endsWith("/x86_64/release.json") && packageBases.some((base) => key.startsWith(`${root}${base}`)));
  const manifestKeys = uniqueObjects
    .map((object) => object.key)
    .filter((key) => key.startsWith(root) && !key.slice(root.length).includes("/") && key.endsWith(".iso.json"));
  const pointerKeys = uniqueObjects
    .map((object) => object.key)
    .filter((key) => new RegExp(`^${root.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}latest(?:-[a-z0-9-]+)?\\.json$`).test(key));

  const reads = [
    ...releaseKeys.map((key) => ({ kind: "release", key })),
    ...manifestKeys.map((key) => ({ kind: "manifest", key })),
    ...pointerKeys.map((key) => ({ kind: "pointer", key })),
    ...ledgerKeys.map((key) => ({ kind: "ledger", key: `${root}${key}` })),
  ];
  const values = await mapConcurrent(reads, CONNECTIONS, (item) => readJson(bucket, operations, item.key));
  const parsed = { release: {}, manifest: {}, pointer: {}, ledger: {} };
  reads.forEach((item, index) => {
    if (item.kind !== "ledger" || values[index]) parsed[item.kind][item.key] = values[index];
  });

  return {
    objects: uniqueObjects,
    releases: parsed.release,
    manifests: parsed.manifest,
    pointers: parsed.pointer,
    ledgers: parsed.ledger,
  };
}

function keysForGroup(group, plan) {
  if (group.kind === "isos") {
    return plan.deleteKeys.filter((key) => group.dropped.some((iso) => key === iso || key.startsWith(`${iso}.`)));
  }
  const definitions = {
    "packages:arch:stable": `${plan.root}releases/`,
    "packages:void:stable": `${plan.root}void/releases/`,
    "packages:arch:unstable": `${plan.root}channels/testing/builds/`,
    "packages:void:unstable": `${plan.root}void/channels/testing/builds/`,
  };
  let base = definitions[group.group];
  if (!base) {
    const match = /^packages:fedora:([0-9]+):(stable|unstable)$/.exec(group.group);
    if (match) {
      const suffix = match[2] === "stable" ? "releases/" : "channels/testing/builds/";
      base = `${plan.root}fedora/${match[1]}/${suffix}`;
    }
  }
  if (!base) return [];
  return plan.deleteKeys.filter((key) => group.dropped.some((name) => key.startsWith(`${base}${name}/`)));
}

function logPlan(plan, facts, dryRun, operations) {
  const sizes = new Map(facts.objects.map((object) => [object.key, object.size ?? 0]));
  for (const group of plan.groups) {
    const keys = keysForGroup(group, plan);
    console.log(JSON.stringify({
      event: "r2_retention_group",
      group: group.group,
      dry_run: dryRun,
      skipped: group.skipped,
      kept: group.kept.length,
      dropped: group.dropped.length,
      objects: keys.length,
      bytes: keys.reduce((sum, key) => sum + (sizes.get(key) ?? 0), 0),
    }));
  }
  if (plan.debris.length) {
    const keys = plan.debris.flatMap((item) => item.keys);
    console.log(JSON.stringify({
      event: "r2_retention_group",
      group: "debris",
      dry_run: dryRun,
      skipped: false,
      kept: 0,
      dropped: plan.debris.length,
      objects: keys.length,
      bytes: keys.reduce((sum, key) => sum + (sizes.get(key) ?? 0), 0),
    }));
  }
  for (const item of plan.skipped) {
    console.log(JSON.stringify({ event: "r2_retention_skip", ...item, dry_run: dryRun }));
  }
  const bytes = plan.deleteKeys.reduce((sum, key) => sum + (sizes.get(key) ?? 0), 0);
  console.log(JSON.stringify({
    event: "r2_retention_total",
    status: "ok",
    dry_run: dryRun,
    kept_versions: plan.groups.reduce((sum, group) => sum + group.kept.length, 0),
    dropped_versions: plan.groups.reduce((sum, group) => sum + group.dropped.length, 0),
    debris: plan.debris.length,
    objects: plan.deleteKeys.length,
    bytes,
    ledger_edits: plan.ledgerEdits.length,
    operations: operations.used,
  }));
}

export async function runRetention(env) {
  const root = rootPath(env.ROOT);
  const keep = Number.parseInt(env.KEEP ?? "10", 10);
  const dryRun = env.DRY_RUN === "1";
  const operationLimit = Number.parseInt(env.OPERATION_BUDGET ?? "900", 10);
  const operations = new Operations(operationLimit);

  try {
    const facts = await gatherFacts(env.BUCKET, root, operations);
    const plan = planRetention(facts, { root, keep, now: new Date() });
    const deleteBatches = Math.ceil(plan.deleteKeys.length / 1000);
    const needed = dryRun ? 0 : deleteBatches + plan.ledgerEdits.length;
    if (!operations.canFit(needed)) {
      console.log(JSON.stringify({
        event: "r2_retention_total",
        status: "operation_budget",
        dry_run: dryRun,
        operations: operations.used,
        needed,
        limit: operationLimit,
      }));
      return;
    }

    if (!dryRun) {
      for (let offset = 0; offset < plan.deleteKeys.length; offset += 1000) {
        const batch = plan.deleteKeys.slice(offset, offset + 1000);
        await operations.run(() => env.BUCKET.delete(batch));
      }
      for (const edit of plan.ledgerEdits) {
        await operations.run(() => env.BUCKET.put(edit.key, `${JSON.stringify(edit.value, null, 2)}\n`, {
          httpMetadata: {
            contentType: "application/json",
            cacheControl: "no-cache, must-revalidate",
          },
        }));
      }
    }
    logPlan(plan, facts, dryRun, operations);
  } catch (error) {
    if (error instanceof OperationBudgetError) {
      console.log(JSON.stringify({
        event: "r2_retention_total",
        status: "operation_budget",
        dry_run: dryRun,
        operations: operations.used,
        limit: operationLimit,
      }));
      return;
    }
    throw error;
  }
}

export default {
  async scheduled(_controller, env) {
    await runRetention(env);
  },
};

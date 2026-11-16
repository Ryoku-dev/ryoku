import { keysForGroup } from "./index.js";

const GB = 1e9;
// R2 Standard storage: the first 10 GB-month are free, then $0.015/GB-month.
const FREE_GB = 10;
const PRICE_PER_GB_MONTH = 0.015;

function gigabytes(bytes) {
  return `${(bytes / GB).toFixed(2)} GB`;
}

export function monthlyCost(bytes) {
  return Math.max(0, bytes / GB - FREE_GB) * PRICE_PER_GB_MONTH;
}

// formatPlan renders a dry-run plan: one "<key>\t<bytes>" line per object to
// delete, then "#" comment lines with each group's decision and the totals.
// bucketObjects is a complete listing of the bucket, so kept bytes include
// what retention never looks at (heads, ledgers, pointers, install.sh).
export function formatPlan(plan, bucketObjects, generatedAt = new Date()) {
  const sizes = new Map(bucketObjects.map((object) => [object.key, object.size ?? 0]));
  const lines = plan.deleteKeys.map((key) => `${key}\t${sizes.get(key) ?? 0}`);
  const sum = (keys) => keys.reduce((total, key) => total + (sizes.get(key) ?? 0), 0);

  lines.push(
    `# ryoku-iso retention plan, ${generatedAt.toISOString()}: dry run, nothing was deleted`,
    `# policy: ${plan.keepPackages} frozen package directories per edition and channel (current + ${plan.keepPackages - 1} rollback), ${plan.keepIsoVersions} ISO versions per variant and channel (current + ${plan.keepIsoVersions - 1}), newest build of each`,
  );
  for (const group of plan.groups) {
    const keys = keysForGroup(group, plan);
    const state = group.skipped ? "SKIPPED" : `drop ${group.dropped.length}`;
    lines.push(`# ${group.group}: keep ${group.kept.length} [${group.kept.join(" ")}], ${state}, ${keys.length} objects, ${gigabytes(sum(keys))}`);
  }
  if (plan.debris.length) {
    const keys = plan.debris.flatMap((item) => item.keys);
    lines.push(`# debris: ${plan.debris.length} incomplete directories, ${keys.length} objects, ${gigabytes(sum(keys))}`);
  }
  for (const item of plan.skipped) lines.push(`# skipped ${item.group}: ${item.reason}`);
  for (const edit of plan.ledgerEdits) {
    lines.push(`# ledger rewrite: ${edit.key} keeps ${edit.value.releases.length} entries, latest ${edit.value.latest || "(none)"}`);
  }

  const deleted = new Set(plan.deleteKeys);
  const deleteBytes = sum(plan.deleteKeys);
  const totalBytes = bucketObjects.reduce((total, object) => total + (object.size ?? 0), 0);
  const keptObjects = bucketObjects.filter((object) => !deleted.has(object.key)).length;
  const keptBytes = totalBytes - deleteBytes;
  lines.push(
    `# bucket now: ${bucketObjects.length} objects, ${gigabytes(totalBytes)}, $${monthlyCost(totalBytes).toFixed(2)}/month`,
    `# delete: ${plan.deleteKeys.length} objects, ${gigabytes(deleteBytes)} (${deleteBytes} bytes)`,
    `# keep: ${keptObjects} objects, ${gigabytes(keptBytes)} (${keptBytes} bytes), $${monthlyCost(keptBytes).toFixed(2)}/month at $${PRICE_PER_GB_MONTH}/GB-month beyond ${FREE_GB} GB free`,
  );
  return `${lines.join("\n")}\n`;
}

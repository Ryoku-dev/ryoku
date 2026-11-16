#!/usr/bin/env node
// plan.js: print the exact delete list the retention Worker would act on,
// without deleting or writing anything. It runs the Worker's own buildPlan
// over a read-only REST adapter (list and get only).
//
//   CLOUDFLARE_ACCOUNT_ID=<id> node release/r2-retention/plan.js [--out FILE]
//
// The token is CLOUDFLARE_API_TOKEN, or Wrangler's OAuth login when that is
// unset (run `npx wrangler whoami` first if the login has expired).
import { readFileSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { parseArgs } from "node:util";

import { buildPlan, Operations } from "./src/index.js";
import { DEFAULT_KEEP_ISO_VERSIONS, DEFAULT_KEEP_PACKAGES } from "./src/planner.js";
import { formatPlan } from "./src/report.js";
import { RestBucket } from "./src/rest-bucket.js";

function wranglerToken() {
  const config = join(process.env.XDG_CONFIG_HOME || join(homedir(), ".config"), ".wrangler", "config", "default.toml");
  let text;
  try {
    text = readFileSync(config, "utf8");
  } catch {
    return null;
  }
  const expires = /^expiration_time\s*=\s*"([^"]+)"/m.exec(text)?.[1];
  if (expires && Date.parse(expires) <= Date.now()) {
    throw new Error("the Wrangler login has expired; run `npx wrangler whoami` to refresh it");
  }
  return /^oauth_token\s*=\s*"([^"]+)"/m.exec(text)?.[1] ?? null;
}

async function listEverything(bucket) {
  const objects = [];
  let cursor;
  do {
    const page = await bucket.list({ cursor });
    objects.push(...page.objects);
    cursor = page.truncated ? page.cursor : undefined;
  } while (cursor);
  return objects;
}

const { values } = parseArgs({
  options: {
    bucket: { type: "string", default: "ryoku-iso" },
    root: { type: "string", default: "stable/" },
    "keep-packages": { type: "string", default: String(DEFAULT_KEEP_PACKAGES) },
    "keep-iso-versions": { type: "string", default: String(DEFAULT_KEEP_ISO_VERSIONS) },
    out: { type: "string" },
  },
});

const accountId = process.env.CLOUDFLARE_ACCOUNT_ID;
if (!accountId) {
  console.error("plan.js: set CLOUDFLARE_ACCOUNT_ID");
  process.exit(2);
}
const token = process.env.CLOUDFLARE_API_TOKEN || wranglerToken();
if (!token) {
  console.error("plan.js: set CLOUDFLARE_API_TOKEN or log in with `npx wrangler login`");
  process.exit(2);
}

const bucket = new RestBucket({ accountId, bucket: values.bucket, token });
const { plan } = await buildPlan(bucket, new Operations(Infinity), {
  root: values.root,
  keepPackages: values["keep-packages"],
  keepIsoVersions: values["keep-iso-versions"],
});
const report = formatPlan(plan, await listEverything(bucket));
if (values.out) {
  writeFileSync(values.out, report);
  console.error(report.split("\n").filter((line) => line.startsWith("# ")).join("\n"));
} else {
  process.stdout.write(report);
}

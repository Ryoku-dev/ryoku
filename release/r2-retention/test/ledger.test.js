import assert from "node:assert/strict";
import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { delimiter, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import test from "node:test";

const execFileAsync = promisify(execFile);
const ledger = fileURLToPath(new URL("../../../bin/ryoku-release-ledger", import.meta.url));

async function release(root, path, metadata) {
  const directory = join(root, path, "x86_64");
  await mkdir(directory, { recursive: true });
  await writeFile(join(directory, "release.json"), `${JSON.stringify(metadata)}\n`);
}

async function fakeRclone(directory) {
  const path = join(directory, "rclone");
  await writeFile(path, `#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");
const [command, target, ...args] = process.argv.slice(2);
if (command === "lsf") {
  if (!fs.existsSync(target)) process.exit(0);
  const dirsOnly = args.includes("--dirs-only");
  const filesOnly = args.includes("--files-only");
  const includeJson = args.includes("--include") && args[args.indexOf("--include") + 1] === "*.json";
  for (const entry of fs.readdirSync(target, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    if (dirsOnly && !entry.isDirectory()) continue;
    if (filesOnly && !entry.isFile()) continue;
    if (includeJson && !entry.name.endsWith(".json")) continue;
    process.stdout.write(entry.name + (entry.isDirectory() ? "/" : "") + "\\n");
  }
} else if (command === "cat") {
  process.stdout.write(fs.readFileSync(target));
} else if (command === "copyto") {
  const destination = args[0];
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  fs.copyFileSync(target, destination);
} else {
  throw new Error("unsupported rclone command: " + command);
}
`);
  await chmod(path, 0o755);
}

test("builds stable and testing ledgers for a Fedora release", async (t) => {
  const work = await mkdtemp(join(tmpdir(), "ryoku-fedora-ledger-"));
  t.after(() => rm(work, { recursive: true, force: true }));
  const remote = join(work, "remote");
  const bin = join(work, "bin");
  await mkdir(bin);
  await fakeRclone(bin);

  await release(remote, "fedora/44/releases/v1.0.0", {
    release: "v1.0.0", name: "Ryoku", version: "1.0.0", commit: "old", date: "2026-09-01T00:00:00Z",
  });
  await release(remote, "fedora/44/releases/v1.1.0", {
    release: "v1.1.0", name: "Ryoku", version: "1.1.0", commit: "new", date: "2026-10-01T00:00:00Z",
  });
  await release(remote, "fedora/44/channels/testing/builds/v1.2.0_dev.7_gabc", {
    release: "v1.2.0.dev.7+gabc", name: "Ryoku", version: "1.2.0.r7.gabc", commit: "abc", date: "2026-10-02T00:00:00Z",
  });

  const env = { ...process.env, PATH: `${bin}${delimiter}${process.env.PATH}` };
  const stable = await execFileAsync("bash", [ledger, remote, "fedora/44", "stable"], { env });
  const testing = await execFileAsync("bash", [ledger, remote, "fedora/44", "testing"], { env });
  const stableLedger = JSON.parse(stable.stdout);
  const testingLedger = JSON.parse(testing.stdout);

  assert.equal(stableLedger.latest, "v1.1.0");
  assert.deepEqual(stableLedger.releases.map((entry) => entry.tag), ["v1.1.0", "v1.0.0"]);
  assert.equal("images" in stableLedger.releases[0], false);
  assert.equal(testingLedger.latest, "v1.2.0.dev.7+gabc");
  assert.equal(testingLedger.releases[0].repo, "fedora/44/channels/testing/builds/v1.2.0_dev.7_gabc/x86_64");

  const written = JSON.parse(await readFile(resolve(remote, "fedora/44/releases/index.json"), "utf8"));
  assert.deepEqual(written, stableLedger);
});

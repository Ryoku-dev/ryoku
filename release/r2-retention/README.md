# R2 retention

This scheduled Worker holds the `ryoku-iso` bucket to the release policy:

- **Packages:** the six newest frozen repository directories per edition and channel: the one the channel serves now plus five to roll back to. This covers Arch and Void stable releases and unstable builds, and both channels of every Fedora release.
- **ISOs:** the three newest versions per variant (plain, CachyOS, Void) and channel: the current release and the two before it. Only the newest build of each version stays, so a rebuild of the same tag replaces the older image. Unstable images have no release tag, so each build counts as its own version.

It removes stale incomplete snapshot uploads after 48 hours. Moving repository heads, ledgers, `latest*` pointers and the ISOs they name, the frozen directory a channel head serves, the `fedora/releases` list, `SHA256SUMS`, signing keys, unknown files, and everything outside `stable/` (such as `install.sh`) are never retention targets. ISO identity normally comes from its manifest, so older CachyOS images without a filename suffix are handled correctly. Legacy images without a manifest fall back to their filename and upload time.

## Retained layout

Package retention runs independently for Arch (`stable/releases/` and
`stable/channels/testing/builds/`), Void (the same paths below `stable/void/`),
and every Fedora release (the same paths below `stable/fedora/<N>/`). Each
version directory and all of its sidecars are kept or removed together, ordered
by the `date` in its `x86_64/release.json`. Moving `x86_64/` heads and
`index.json` ledgers are not version directories and are never direct deletion
targets. A ledger is filtered only when a repository or ISO it references no
longer exists after retention, which is what `ryoku rollback` then offers.

`KEEP_PACKAGES` (default 6) and `KEEP_ISO_VERSIONS` (default 3) in
`wrangler.jsonc` set the depth.

Run the tests from the repository root. The ledger integration test also uses
the repository's standard `bash` and `jq` release-tool dependencies:

```sh
node --test release/r2-retention/test/*.test.js
```

`test/fixtures/bucket-2026-11-16.json` is a trimmed snapshot of the real bucket
(every root object, head, ledger and pointer, and each frozen directory's
`release.json` plus two of its files) that the planner test checks the policy
and the never-delete set against.

## Dry run: the exact delete list

`plan.js` runs the Worker's own planner over a read-only REST adapter (list and
get only; it has no delete or put) and prints one `<key>\t<bytes>` line per
object it would delete, followed by `#` lines with each group's decision, the
ledger rewrites, and the totals and monthly cost before and after:

```sh
CLOUDFLARE_ACCOUNT_ID=<account id> node release/r2-retention/plan.js --out /tmp/r2-plan.txt
```

It uses `CLOUDFLARE_API_TOKEN` when set, otherwise Wrangler's OAuth login (run
`npx wrangler whoami` first if it has expired). `--keep-packages`,
`--keep-iso-versions`, `--bucket`, and `--root` override the defaults.

The `dryrun` Wrangler environment runs the deployed code path itself with an
R2 [remote binding](https://developers.cloudflare.com/workers/local-development/#remote-bindings) and `DRY_RUN=1`, logging per-group totals rather than keys. Start it in one terminal:

```sh
cd release/r2-retention
npx wrangler dev --env dryrun 2>&1 | tee /tmp/r2-dryrun.log
```

Trigger its scheduled handler from another terminal with the current Wrangler endpoint:

```sh
curl 'http://localhost:8787/cdn-cgi/local/scheduled?format=json'
```

A dry run only calls R2 list/get operations; it never calls delete or put.

## Deploy and observe

The `R2 retention` GitHub workflow only runs the tests; nothing deploys the Worker automatically. Deploy from this directory with `npx wrangler deploy` after a change, or the bucket keeps running the previous policy. For a policy change, set the top-level `DRY_RUN` value to `1`, deploy, trigger or wait for one scheduled run, and inspect it with `npx wrangler tail ryoku-r2-retention`. Restore `DRY_RUN` to `0` and deploy again only after the plan is correct. The cron runs every six hours in UTC.

Each invocation logs one JSON line per retention group and one total line. Search Workers Logs for `r2_retention_total`; `dry_run`, `objects`, `bytes`, `ledger_edits`, and `operations` show what happened. A `status` of `operation_budget` means the Worker stopped before writing and will try again at the next run.

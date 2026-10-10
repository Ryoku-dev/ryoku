# R2 retention

This scheduled Worker keeps the ten newest Arch and Void package snapshots in each stable and testing channel, plus the ten newest ISOs for each variant and channel. It removes stale incomplete snapshot uploads after 48 hours. Moving repository heads, ledgers, current ISO pointers, signing keys, unknown files, and the bucket-root `install.sh` are never retention targets. ISO identity normally comes from its manifest, so older CachyOS images without a filename suffix are handled correctly. Legacy images without a manifest fall back to their filename and upload time.

Run the dependency-free tests from the repository root:

```sh
node --test release/r2-retention/test
```

## Dry run against R2

The `dryrun` Wrangler environment uses an R2 [remote binding](https://developers.cloudflare.com/workers/local-development/#remote-bindings) and forces `DRY_RUN=1`. Start it in one terminal:

```sh
cd release/r2-retention
npx wrangler dev --env dryrun 2>&1 | tee /tmp/r2-dryrun.log
```

Trigger its scheduled handler from another terminal with the current Wrangler endpoint:

```sh
curl 'http://localhost:8787/cdn-cgi/local/scheduled?format=json'
```

The older `--test-scheduled` and `/__scheduled` combination is not needed by current Wrangler. A dry run only calls R2 list/get operations; it never calls delete or put.

## Deploy and observe

Deploy from this directory with `npx wrangler deploy`. For the first deployment, set the top-level `DRY_RUN` value to `1`, deploy, trigger or wait for one scheduled run, and inspect it with `npx wrangler tail ryoku-r2-retention`. Restore `DRY_RUN` to `0` and deploy again only after the plan is correct. The cron runs every six hours in UTC.

Each invocation logs one JSON line per retention group and one total line. Search Workers Logs for `r2_retention_total`; `dry_run`, `objects`, `bytes`, `ledger_edits`, and `operations` show what happened. A `status` of `operation_budget` means the Worker stopped before writing and will try again at the next run.

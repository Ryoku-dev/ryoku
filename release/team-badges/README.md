# Team badges gate

Cloudflare Worker for `https://team.ryoku.dev`. It publishes machine-bound Ed25519 badges, revocations, approved team designs, and device-signed system-card records. Public issuer keys are in `wrangler.jsonc`; no private key belongs in the Worker. All writes require signatures, not a shared password.

## Layout

- `src/codec.js`: Crockford text encoding, request CRC, signed-code and card verification, upload authorization, JPEG frame limits.
- `src/index.js`: HTTPS routes, bounded streaming reads, R2 persistence, listing and 60-second Cache API responses.
- `test/*.test.js`: Node WebCrypto tests using fresh test-only signing keys. Optional `test/vectors.json` runs Go cross-language vectors too.
- `wrangler.jsonc`: custom domain, public keys and `BUCKET` binding to `ryoku-team`.

R2 keys: `badges/<id>.txt`, `revoked/<id>.txt`, `designs/<lowercase-login>.txt`, `cards/<fp>.json`, `cards/<fp>/<sha>.jpg`, `pending/<sha>.zip`, `packs/<sha>.zip`. Publishing a design copies its authenticated pending pack into `packs/`. The pending copy remains available for review. Replacing a card picture deletes the old object. Revoked badges remain in the list alongside the signed revocations so clients can verify the history.

## Local test

Requires Node 20+ and Wrangler. From the repository root:

```sh
node --test release/team-badges/test/*.test.js
```

Or run `npm test` in this directory. Tests do not contact Cloudflare. Optional vector files are an array (or `{ "vectors": [...] }`) of `{ "kind": "RYB", "text": "RYB-...", "valid": true }` entries. Kinds also accept `request`, `badge`, `revoke`, `design`, `card`, and `upload`. Signature vectors may provide `ownerKey` / `mikaKey` (or `OWNER_KEY` / `MIKA_KEY`); card vectors supply `record`, `deviceKey`; upload vectors supply `header`, `method`, `path`, `sha`, `now`. Absent keys use the production public keys.

For local R2 simulation, run in this directory:

```sh
npx wrangler dev --local --local-protocol https
curl -k https://127.0.0.1:8787/v1/list
```

Use `--var OWNER_KEY:<test-public-key>` and `--var MIKA_KEY:<test-public-key>` to exercise writes with throwaway test keys. Local requests must use HTTPS too. Never pass real private keys to Wrangler. Stop Wrangler when finished.

## Deploy

An authorized operator provisions the bucket once, then deploys from this directory:

```sh
wrangler r2 bucket create ryoku-team
wrangler deploy
```

The route is the custom domain `team.ryoku.dev`; `workers.dev` is disabled. Check that the configured issuer public keys match the actual issuers before deployment.

## API

Read and verify the public list:

```sh
curl --fail https://team.ryoku.dev/v1/list
```

It returns `{ "v": 1, "badges": ["RYB-..."], "revoked": ["RYX-..."], "designs": ["RYD-..."] }`. It is cached for 60 seconds; listings paginate through all stored objects and cryptographically filter invalid entries. Consumers must verify the codes and apply revocations themselves. Design approvals require a current role 3–5 badge for their GitHub login. Newest approval day wins; equal days use accepted upload order.

Each isolate memoizes the verified inventory for 30 seconds and shares any in-flight refresh. Card and picture GETs, including missing records, use the Cache API for 60 seconds too. Cache keys ignore query strings so they cannot bypass the cache. Writes first check the memo, and verification misses may refresh it at most once every five seconds per isolate. Accepted badge, revocation and design uploads update the local memo immediately, so a new badge can publish its card without waiting. Other isolates can take up to 30 seconds to observe a revocation, and an already-cached read can remain visible for its 60-second lifetime.

Workers rate-limit bindings allow approximately 120 reads and 20 writes per minute per `CF-Connecting-IP`, independently in each Cloudflare location. Exceeded limits return JSON 429 with `Retry-After: 60`. Requests without that header share the `unknown` key, which also covers local development. The limiter is skipped when its binding is absent, allowing Node tests and runtimes without the binding to run. Namespace IDs `2026111501` and `2026111502` must remain unique within the Cloudflare account unless sharing counters is intentional.

| Write | Exact Content-Type | Limit |
| --- | --- | --- |
| `POST /v1/badges`, `/v1/revoke`, `/v1/designs` | `text/plain` | 600 characters (2,400-byte streaming ceiling) |
| `PUT /v1/cards/<fp>` | `application/json` | 600 KiB JSON, 400 KiB decoded JPEG, 1120×780 maximum |
| `PUT /v1/pending/<sha>` | `application/zip` | 8 MiB, signed `X-Ryoku-Upload` header |

Every write needs an explicit `Content-Length`; chunked-only uploads are rejected before consuming their bodies. Card records must be within 600 seconds of server time and strictly newer than stored records. Upload authorization allows a 300-second clock difference and signs the method, exact path, time and body hash. ZIP structure/design-schema validation is performed by the signing `ryoku-hub` CLI before uploading; this gate authenticates issuers and pack digests, and never executes pack contents.

Read individual records at `/v1/cards/<fp>`, pictures at `/v1/cards/<fp>/<sha>.jpg`, review packs at `/v1/pending/<sha>.zip`, and published packs at `/v1/packs/<sha>.zip`. Fingerprints and SHA-256 hashes are lowercase hexadecimal, six and 64 characters respectively. Other paths return JSON 404 errors. Rejected writes return JSON 400, 404 (missing design pack), 413 (size), 415 (content type), or 429 (rate limit), without stack traces.

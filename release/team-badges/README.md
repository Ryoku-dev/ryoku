# Team badges gate

Cloudflare Worker for `https://team.ryoku.dev`. It publishes machine-bound Ed25519 badges, revocations, approved team designs, and device-signed system-card records for anyone who turns live cards on. Public issuer keys are in `wrangler.jsonc`; no private key belongs in the Worker. All writes require signatures, not a shared password.

## Layout

- `src/codec.js`: Crockford text encoding, request CRC, signed-code, card (classic v2 and styled v3, with the style allowlist) and card-delete verification, upload authorization, JPEG frame limits.
- `src/index.js`: HTTPS routes, bounded streaming reads, rate limits, the 60-second Cache API response for the list, and the daily cleanup entry point.
- `src/storage.js`: conditional R2 writes for badges, designs, card records and the card index, plus the cleanups (90-day expiry, styled records without an active badge).
- `test/*.test.js`: Node WebCrypto tests using fresh test-only signing keys. Optional `test/vectors.json` runs Go cross-language vectors too.
- `wrangler.jsonc`: custom domain, public keys, `BUCKET` binding to `ryoku-team`, rate limiters, the cleanup cron and the card limits.

R2 keys: `badges/<id>.txt`, `revoked/<id>.txt`, `designs/<lowercase-login>.txt`, `cards/<fp>.json`, `cards/<fp>/<uuid>/<sha>.jpg`, `cards/index.json`, `pending/<sha>.zip`, `packs/<sha>.zip`. Publishing a design copies its authenticated pending pack into `packs/`. The pending copy remains available for review. Replacing a card picture deletes the old object. Revoked badges remain in the list alongside the signed revocations so clients can verify the history.

## Local test

Requires Node 20+ and Wrangler. From the repository root:

```sh
node --test release/team-badges/test/*.test.js
```

Or run `npm test` in this directory. Tests do not contact Cloudflare. Optional vector files are an array (or `{ "vectors": [...] }`) of `{ "kind": "RYB", "text": "RYB-...", "valid": true }` entries. Kinds also accept `request`, `badge`, `revoke`, `design`, `card`, `deviceCard`, `cardDelete`, and `upload`. Signature vectors may provide `ownerKey` / `mikaKey` (or `OWNER_KEY` / `MIKA_KEY`); card vectors supply `record`, `deviceKey`; device card vectors supply only `record` (it carries its own `device`); card delete vectors supply `id`, `at`, `sig`, `deviceKey`, `now`; upload vectors supply `header`, `method`, `path`, `sha`, `now`. Absent keys use the production public keys.

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

Each isolate memoizes the verified inventory for 30 seconds and shares any in-flight refresh. Card records and pictures are read fresh from R2 on every GET, missing ones included: a Cache API copy is only purged in the data centre that took the write, so friends elsewhere kept a superseded record for a minute and found its picture already deleted. The list's cache keys ignore query strings so they cannot bypass the cache. Writes first check the memo, and verification misses may refresh it at most once every five seconds per isolate, so a badge published through another isolate a moment ago can be refused once with `No active badge verifies this record`; Settings sends the record again six seconds later. Accepted badge, revocation and design uploads update the local memo immediately. Other isolates can take up to 30 seconds to observe a revocation, and an already-cached list can remain visible for its 60-second lifetime.

Workers rate-limit bindings allow approximately 120 reads and 20 writes per minute per `CF-Connecting-IP`, independently in each Cloudflare location. `POST /v1/cards/batch` counts as a read and, unless it carries a valid badge holder proof, also against `PULL_LIMITER`, 10 per minute. Exceeded limits return JSON 429 with `Retry-After: 60`; a refused batch also carries `next`, the server time to try again. Requests without that header share the `unknown` key, which also covers local development. The limiter is skipped when its binding is absent, allowing Node tests and runtimes without the binding to run. Namespace IDs `2026111501`, `2026111502` and `2026111603` must remain unique within the Cloudflare account unless sharing counters is intentional.

| Write | Exact Content-Type | Limit |
| --- | --- | --- |
| `POST /v1/badges`, `/v1/revoke`, `/v1/designs` | `text/plain` | 600 characters (2,400-byte streaming ceiling) |
| `PUT /v1/cards/<fp>` | `application/json` | 600 KiB JSON, 400 KiB decoded JPEG, 1120×780 maximum |
| `DELETE /v1/cards/<fp>` | `application/json` | 1 KiB |
| `POST /v1/cards/batch` | `application/json` | 16 KiB, 1 to 200 ids |
| `PUT /v1/pending/<sha>` | `application/zip` | 8 MiB, signed `X-Ryoku-Upload` header |

Every write needs an explicit `Content-Length`; chunked-only uploads are rejected before consuming their bodies. Card records must be within 600 seconds of server time and strictly newer than stored records. Upload authorization allows a 300-second clock difference and signs the method, exact path, time and body hash. ZIP structure/design-schema validation is performed by the signing `ryoku-hub` CLI before uploading; this gate authenticates issuers and pack digests, and never executes pack contents.

Read individual records at `/v1/cards/<fp>`, pictures at `/v1/cards/<fp>/<sha>.jpg`, review packs at `/v1/pending/<sha>.zip`, and published packs at `/v1/packs/<sha>.zip`. Fingerprints and SHA-256 hashes are lowercase hexadecimal, 16 and 64 characters respectively. Other paths return JSON 404 errors. Rejected writes return JSON 400, 403 (styled card without its active badge), 404 (missing design pack or card), 413 (size), 415 (content type), 429 (rate limit), or 507 (card service full), without stack traces.

## Live cards

A classic (`"v":2`) record that carries `device`, the base64 Ed25519 public key of the machine, needs no badge: it is accepted when the key derives the card id and signs the record. A record without `device` (older clients) still needs an active badge for its id, and a device record may not name a key other than the one an active badge already holds for that id. Device records stay live when a badge is revoked; only team status goes away. Styled `"v":3` records always need the badge, see below.

`DELETE /v1/cards/<fp>` takes `{"id","at","sig"}`, signed by the record's device (or the active badge's device) over `ryoku-card-delete-v1` NUL id | `at` u64, with `at` within 600 seconds. It removes the record, its picture and its index entry. The deleted record's timestamp is refused for an hour afterwards, so a copy someone fetched before the delete cannot put it back.

`POST /v1/cards/batch` takes `{"cards":{"<fp>":<known updated>}}` (0 for never seen) and returns `{"v":1,"now","next","records","gone","badges","designs"}`: records newer than what the binder holds, ids the server no longer has, active badge codes for the requested ids, and approved designs for the team logins among them. `next` is `now + PULL_SECONDS`. The response is `Cache-Control: no-store`. Signatures were checked when each record was stored and binders check them again, so a batch only checks what can change afterwards: a device record's key must still derive its id, and a record without `device` needs an active badge for its id, so it drops out once that badge is revoked.

A batch may also carry `"holder":{"badge":"RYB-...","at":<unix s>,"sig":"<base64>"}`, signed by the badge's device over `ryoku-pull-v1` NUL badge id (8 bytes) | `at` u64. When the badge verifies, is still active and `at` is within 600 seconds, the batch skips `PULL_LIMITER` and answers `next = now`, so holders pull whenever they like; the read limit still applies. A broken proof is ignored and the batch is treated like any other.

`cards/index.json` maps every stored id to its record time. Card writes and deletes update it with etag-conditional writes, rebuilding each touched entry from the record read after the index, so racing writers cannot leave it behind the records. Batches read it once, memoised per isolate for 30 seconds and updated by writes in the same isolate. A missing index is rebuilt from the stored records on first use. `CARD_LIMIT` (20,000) caps the ids in the index: a new id over the cap gets 507 `Card service is full`, updates to existing ids still pass.

A cron at 03:17 UTC deletes records not updated for 90 days, with their pictures and index entries, up to 300 per run so a run stays inside the R2 call budget of a free invocation, then deletes styled records that no active badge holds any more. Friends' binders keep their copies of expired cards.

Cost: a binder makes one batch request per `PULL_SECONDS` (15 minutes) instead of one request per card, and a batch costs one index read (usually memoised) plus one record read per card that actually changed. Each accepted card write costs the record write, at most one picture write and one index write. Idle cards cost nothing and leave storage after 90 days, and `CARD_LIMIT` bounds the index and storage at roughly 20,000 records.

## Card styles are for badge holders only

A built-in card style rides only in a `"v":3` record: the v2 fields plus `style`, an id from the Worker's allowlist (`CARD_STYLES` in `src/codec.js`, currently `["stand"]`). Classic cards stay `"v":2` and never carry `style`; a v2 record with `style`, a v3 record without it, or any other key is refused with 400 `Invalid record fields`. A style off the allowlist, or anything that is not 1–32 ASCII `[a-z0-9-]` (no case folding, look-alike letters, zero-width or control characters), gets 400 `Unknown card style`. The style is signed: the record signs `ryoku-card-v3` NUL, the v2 card payload, the style's UTF-8 byte length as a big-endian u16, then the style bytes.

The Worker, not the client, decides who may store one. `PUT /v1/cards/<fp>` accepts a v3 record only when an active (not revoked) badge exists for that card id and that badge's device key signed it; when the record names `device`, it must be the badge's key. A machine's own key never vouches for a v3 record, so a self-signed device record without a badge cannot carry a style. With no active badge for the id the answer is 403 `Custom card styles need an active team badge on this machine`; with a badge but a signature from any other key, 403 `Invalid card signature`. v2 records are unaffected.

Serving re-checks the badge every time: `GET /v1/cards/<fp>`, its picture and every batch return a v3 record only while an active badge still holds its id, so a revoked badge's styled card is withheld at once (within the 30-second inventory memo of other isolates). It is not downgraded to a classic card, because the classic signature does not exist on the server. `POST /v1/revoke` then deletes the styled record, its picture and index entry straight away, so binders see the id in `gone`, and the 03:17 cron deletes any styled record whose badges are all revoked, catching revocations that reached R2 without the route. A classic record under a revoked badge stays. The machine's next sync uploads its card as v2. A styled record whose badge file was removed from R2 outright, rather than revoked, is withheld the same way and leaves with the 90-day expiry.

Everything else a non-holder can upload stays as before: a still JPEG picture (400 KiB, 1120×780, hash bound into the signed record; moving pictures never reach the Worker), and no team design: approvals need an owner or Mika signature, packs need a signed upload header, and designs are listed and served only for logins with an active role 3–5 badge.

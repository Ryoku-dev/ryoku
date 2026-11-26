# Team badges gate

Cloudflare Worker for `https://team.ryoku.dev`. It publishes machine-bound Ed25519 badges, revocations, approved card designs, and device-signed system-card records for anyone who turns live cards on. Public issuer keys are in `wrangler.jsonc`; no issuer private key belongs in the Worker. The private candidate adds Account-authenticated intake and separately scoped inspector/reviewer credentials, without changing the deployed gate.

## Layout

- `src/codec.js`: Crockford text encoding, request CRC, signed-code (badge v2 and v3, revocation and design approval), card (classic v2, styled v3 and worded v4, with the style allowlist and the words rule), card-delete and holder-proof verification, upload authorization, JPEG frame limits.
- `src/index.js`: HTTPS routes, bounded streaming reads, rate limits, the 60-second Cache API response for the list, and the daily cleanup entry point.
- `src/storage.js`: conditional R2 writes for badges, designs, card records, the card index and badge holders' early-refresh counters, plus the cleanups (90-day expiry, styled or worded records without an active badge).
- `src/inspection-protocol.js`: detached canonical device manifests, inspector certificates and restricted design approval v3 envelopes
- `src/inspection-queue.js`: one Durable Object for private admission, immutable R2 objects, leases, review claims, decisions and design withdrawals
- `test/*.test.js`: Node WebCrypto tests using fresh test-only signing keys. Optional `test/vectors.json` runs Go cross-language vectors too.
- `wrangler.jsonc`: custom domain, public keys, `BUCKET` binding to `ryoku-team`, rate limiters, the cleanup cron, the card limits and the holders' early refreshes.

R2 keys: `badges/<id>.txt`, `revoked/<id>.txt`, `designs/<lowercase-login>.txt`, `cards/<fp>.json`, `cards/<fp>/<uuid>/<sha>.jpg`, `cards/index.json`, `holders/<badge id>.json`, `pending/<sha>.zip`, `packs/<sha>.zip`. Publishing a legacy design copies its authenticated pending pack into `packs/`. Pending reads now need an issuer-signed GET proof, they are not public. Private candidate originals and outputs use `private-inspection/<job-id>/...`, and exact-hash withdrawals use `design-revocations/<sha>.json`. These prefixes must never be exposed through a public R2 bucket or custom domain. Replacing a card picture deletes the old object. Revoked badges remain in the list alongside the signed revocations so clients can verify the history.

## Badge codes

A badge (`RYB-...`) is Crockford base32 over its bytes, and its id is the first eight bytes of SHA-256 over all of them, in hex. Version 2 is `2 | issuer | role | device (32) | day u16 | login length u8 | login | signature (64)`: issuer 1 is the owner and 2 is Mika, who may sign roles 1 (tester) and 2 (contributor) only, and the signature covers everything before it as `ryoku-badge-v2` NUL payload. Version 3 adds the program a tester round tested after the login, `3 | issuer | role | device | day | login length | login | program length u8 | program | signature`, signed as `ryoku-badge-v3`, so changing the program breaks the signature. A program is 1 to 40 ASCII characters matching `^[A-Za-z0-9](?:[A-Za-z0-9 .+-]{0,38}[A-Za-z0-9])?$` with no two spaces in a row; `ryoku-hub` applies the same rule (`badgeProgramValid` in `ryoku/hub/backend/badgecodes.go`) and refuses to sign anything else. `verifyCode('RYB', ...)` returns it as `program`; a v2 badge returns none (`ryoku-hub` reads every v2 tester badge as Ryoku v2 Hyperion, the version the first tester round tested). Only badges have a version 3: requests, revocations and design approvals stay version 2, and any other version gets 400 `Unsupported code version`. Every route that takes a badge (`POST /v1/badges`, the list, revocations, card records, holder proofs, design approvals) takes either version. Deploy the Worker before Mika issues version 3 codes: an older gate refuses them with 400 `Unsupported code version`.

## Local test

Requires Node 20+ and Wrangler. From the repository root:

```sh
node --test release/team-badges/test/*.test.js
```

Or run `npm test` in this directory. Tests do not contact Cloudflare. Optional vector files are an array (or `{ "vectors": [...] }`) of `{ "kind": "RYB", "text": "RYB-...", "valid": true }` entries. Kinds also accept `request`, `badge`, `revoke`, `design`, `card`, `deviceCard`, `cardDelete`, `pullHolder`, `upload`, and `styleOwners`. Signature vectors may provide `ownerKey` / `mikaKey` (or `OWNER_KEY` / `MIKA_KEY`); a valid badge vector's `program` must match the one its code carries, and one without `program` must carry none; card vectors (v2, v3 and v4) supply `record`, `deviceKey`; device card vectors supply only `record` (it carries its own `device`); card delete vectors supply `id`, `at`, `sig`, `deviceKey`, `now`; holder vectors supply `holder`, `cards`, `now`; the one `styleOwners` vector supplies `owners`, ryoku-hub's style owner table; upload vectors supply `header`, `method`, `path`, `sha`, `now`. Absent keys use the production public keys. `go test -run TestWriteVectors . -args -write-vectors` in `ryoku/hub/backend` rewrites `test/vectors.json`.

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

It returns `{ "v": 1, "badges": ["RYB-..."], "revoked": ["RYX-..."], "designs": ["RYD-..."] }`. It is cached for 60 seconds; listings paginate through all stored objects and cryptographically filter invalid entries. Consumers must verify the codes and apply revocations themselves. A design approval needs a current active badge of any role (tester, contributor, NixOS, core or founder) for its GitHub login, when it is posted and whenever it is listed; otherwise `POST /v1/designs` answers 400 `Design requires an active badge`, and a revoked badge takes its login's design out of the list. Newest approval day wins; equal days use accepted upload order. A design a holder wears privately on their own machine before approval never reaches the gate.

Each isolate memoizes the verified inventory for 30 seconds and shares any in-flight refresh. Card records and pictures are read fresh from R2 on every GET, missing ones included: a Cache API copy is only purged in the data centre that took the write, so friends elsewhere kept a superseded record for a minute and found its picture already deleted. The list's cache keys ignore query strings so they cannot bypass the cache. Writes first check the memo, and verification misses may refresh it at most once every five seconds per isolate, so a badge published through another isolate a moment ago can be refused once with `No active badge verifies this record`; Settings sends the record again six seconds later. Accepted badge, revocation and design uploads update the local memo immediately. Other isolates can take up to 30 seconds to observe a revocation, and an already-cached list can remain visible for its 60-second lifetime.

Workers rate-limit bindings allow approximately 120 reads and 20 writes per minute per `CF-Connecting-IP`, independently in each Cloudflare location. `POST /v1/cards/batch` counts as a read and also against `PULL_LIMITER`, 10 per minute, unless a badge holder spends an early refresh on it (see Live cards), which skips both. Exceeded limits return JSON 429 with `Retry-After: 60`; a refused batch also carries `next`, the server time to try again, and `holderLeft` when it carried a valid holder proof. Requests without that header share the `unknown` key, which also covers local development. The limiter is skipped when its binding is absent, allowing Node tests and runtimes without the binding to run. Namespace IDs `2026111501`, `2026111502` and `2026111603` must remain unique within the Cloudflare account unless sharing counters is intentional.

| Write | Exact Content-Type | Limit |
| --- | --- | --- |
| `POST /v1/badges`, `/v1/revoke`, `/v1/designs` | `text/plain` | 600 characters (2,400-byte streaming ceiling) |
| `PUT /v1/cards/<fp>` | `application/json` | 600 KiB JSON, 400 KiB decoded JPEG, 1120×780 maximum |
| `DELETE /v1/cards/<fp>` | `application/json` | 1 KiB |
| `POST /v1/cards/batch` | `application/json` | 16 KiB, 1 to 200 ids |
| `PUT /v1/pending/<sha>` | `application/zip` | 8 MiB, signed `X-Ryoku-Upload` header |

Every write needs an explicit `Content-Length`; chunked-only uploads are rejected before consuming their bodies. Card records must be within 600 seconds of server time and strictly newer than stored records. Upload authorization allows a 300-second clock difference and signs the method, exact path, time and body hash. ZIP structure/design-schema validation is performed by the signing `ryoku-hub` CLI before uploading; this gate authenticates issuers and pack digests, and never executes pack contents.

Read individual records at `/v1/cards/<fp>`, pictures at `/v1/cards/<fp>/<sha>.jpg`, and published legacy packs at `/v1/packs/<sha>.zip`. Review packs at `/v1/pending/<sha>.zip` require `X-Ryoku-Upload` signed for `GET`, the exact `.zip` path, current time and pack hash, using the existing `ryoku-upload-v1` domain. An unauthenticated pending read is refused. Explicit design withdrawal blocks direct pending/published pack URLs too. Fingerprints and SHA-256 hashes are lowercase hexadecimal, 16 and 64 characters respectively. Other paths return JSON 404 errors.

## Live cards

A classic (`"v":2`) record that carries `device`, the base64 Ed25519 public key of the machine, needs no badge: it is accepted when the key derives the card id and signs the record. A record without `device` (older clients) still needs an active badge for its id, and a device record may not name a key other than the one an active badge already holds for that id. Device records stay live when a badge is revoked; only team status goes away. Styled `"v":3` and worded `"v":4` records always need the badge, see below.

`DELETE /v1/cards/<fp>` takes `{"id","at","sig"}`, signed by the record's device (or the active badge's device) over `ryoku-card-delete-v1` NUL id | `at` u64, with `at` within 600 seconds. It removes the record, its picture and its index entry. The deleted record's timestamp is refused for an hour afterwards, so a copy someone fetched before the delete cannot put it back.

`POST /v1/cards/batch` takes `{"cards":{"<fp>":<known updated>}}` (0 for never seen) and returns `{"v":1,"now","next","records","gone","badges","designs"}`: records newer than what the binder holds, ids the server no longer has, active badge codes for the requested ids, and approved designs for the badge holders' logins among them. `next` is `now + PULL_SECONDS`. The response is `Cache-Control: no-store`. Signatures were checked when each record was stored and binders check them again, so a batch only checks what can change afterwards: a device record's key must still derive its id, and a record without `device` needs an active badge for its id, so it drops out once that badge is revoked.

### Badge holders' early refreshes

A batch may also carry `"holder":{"badge":"RYB-...","at":<unix s>,"sig":"<base64>"}`, signed by the badge's device over `ryoku-pull-v2` NUL badge id (8 bytes) | `at` u64 | SHA-256 of the batch's cards (ids in ascending order, each its 8 bytes then its known version as a u64). When the badge verifies, is still active and `at` is within 600 seconds, the batch asks for one of the badge's early refreshes: `HOLDER_REFRESHES` (10) per badge id per UTC day, counted in `holders/<badge id>.json` as `{"day","used","at"}`. The counter is read and written with an etag condition and retried, so parallel batches can never spend more than ten; one that keeps losing races gets none. A proof is taken once: its `at` must be newer than the last proof taken for the badge, so a copy of a proof replayed later, from any address, spends nothing and skips nothing, and because the proof signs the batch it cannot be carried to another request. An early refresh skips the cadence and the per-address limits (`READ_LIMITER` and `PULL_LIMITER`) and answers `next = now`. When the ten are used, or the proof was already taken, the batch is an ordinary one: it meets both per-address limits and answers the usual `next`. Every batch with a valid proof from an active badge answers `holderLeft`, the early refreshes left today; they come back at the next UTC midnight of the gate's clock, so neither the binder's clock, its time zone, a reinstall nor another address buys more. A broken, expired, foreign or revoked badge's proof is ignored, and the batch is treated like any other. The old `ryoku-pull-v1` proofs of earlier clients no longer verify, so those binders keep the ordinary schedule until they update. Cost: an early refresh adds one counter read and one conditional write.

`cards/index.json` maps every stored id to its record time. Card writes and deletes update it with etag-conditional writes, rebuilding each touched entry from the record read after the index, so racing writers cannot leave it behind the records. Batches read it once, memoised per isolate for 30 seconds and updated by writes in the same isolate. A missing index is rebuilt from the stored records on first use. `CARD_LIMIT` (20,000) caps the ids in the index: a new id over the cap gets 507 `Card service is full`, updates to existing ids still pass.

A cron at 03:17 UTC deletes records not updated for 90 days, with their pictures and index entries, up to 300 per run so a run stays inside the R2 call budget of a free invocation, then looks at the record of every badge's card id (one R2 read each) and deletes styled and worded records that no active badge holds any more, and styled ones whose active badges' logins no longer own the style. Friends' binders keep their copies of expired cards.

Cost: a binder makes one batch request per `PULL_SECONDS` (15 minutes) instead of one request per card, and a batch costs one index read (usually memoised) plus one record read per card that actually changed. Each accepted card write costs the record write, at most one picture write and one index write. Idle cards cost nothing and leave storage after 90 days, and `CARD_LIMIT` bounds the index and storage at roughly 20,000 records.

## Card styles belong to their owners

A built-in card style is one badge holder's own look, not a choice for every holder: each style belongs to GitHub logins in `CARD_STYLE_OWNERS` (`src/codec.js`, currently `{ "stand": ["neur0map"] }`, logins lowercase and matched without case). `ryoku-hub` keeps the same table (`cardStyleOwners` in `ryoku/hub/backend/cardstyles.go`) and writes it into `test/vectors.json` as the `styleOwners` vector; `test/vectors.test.js` fails when the two differ, so change both and regenerate the vectors. Every other badge holder makes their own look through a card design (`/card submit` and the owner's approval). Words (`tagline`, `standName`) are not owned: every active badge holder may send them.

A style rides only in a `"v":3` record (or a `"v":4` one with words): the v2 fields plus `style`, an id from the table (`CARD_STYLES` is its keys). Classic cards stay `"v":2` and never carry `style`; a v2 record with `style`, a v3 record without it, or any other key is refused with 400 `Invalid record fields`. A style off the table, or anything that is not 1–32 ASCII `[a-z0-9-]` (no case folding, look-alike letters, zero-width or control characters), gets 400 `Unknown card style`. The style is signed: the record signs `ryoku-card-v3` NUL, the v2 card payload, the style's UTF-8 byte length as a big-endian u16, then the style bytes.

The Worker, not the client, decides who may store one. `PUT /v1/cards/<fp>` accepts a v3 record only when an active (not revoked) badge exists for that card id and that badge's device key signed it, and a styled record only when an active badge for the id has a login that owns the style; when the record names `device`, it must be the badge's key. A machine's own key never vouches for a v3 record, so a self-signed device record without a badge cannot carry a style. With no active badge for the id the answer is 403 `Custom card styles need an active team badge on this machine`; with a badge but a signature from any other key, 403 `Invalid card signature`; with a valid signature from a badge whose login doesn't own the style, 403 `This card style belongs to another badge holder`. v2 records are unaffected.

Ownership is checked the same way on every read: GET, its picture and every batch return a styled record only while an active badge for its id has a login that owns the style. When the owner's badge is revoked and another login's badge for the same machine stays active, the styled record is withheld at once and deleted by the revocation (or the cron), while that machine may go on publishing words without the style.

Serving re-checks the badge every time: `GET /v1/cards/<fp>`, its picture and every batch return a v3 record only while an active badge still holds its id, so a revoked badge's styled card is withheld at once (within the 30-second inventory memo of other isolates). It is not downgraded to a classic card, because the classic signature does not exist on the server. `POST /v1/revoke` then deletes the styled record, its picture and index entry straight away, so binders see the id in `gone`, and the 03:17 cron deletes any styled record whose badges are all revoked, catching revocations that reached R2 without the route. A classic record under a revoked badge stays. The machine's next sync uploads its card as v2. A styled record whose badge file was removed from R2 outright, rather than revoked, is withheld the same way and leaves with the 90-day expiry.

## A badge holder's own words

A `"v":4` record carries its owner's words: the v2 fields, an optional `style` (as in v3), and at least one of `tagline` (the motto, 1–128 characters) and `standName` (the Stand's name, 1–64 characters). A field that is absent is not sent; an empty string, a v4 record with neither word, or any other key is refused with 400 (`Invalid card words` or `Invalid record fields`). It signs `ryoku-card-v4` NUL, the v2 card payload, then the style, the tagline and the Stand's name, each as its UTF-8 byte length in a big-endian u16 followed by its bytes, an absent field being length 0, so no word can be added, removed, swapped between fields or moved across a boundary without breaking the signature. A style alone stays `"v":3`.

Words must be well-formed Unicode in NFC with no space (`\p{Zs}`) at either end, and may hold no control (`Cc`) or format (`Cf`: bidi overrides and isolates, zero-width spaces and joiners, the soft hyphen, the BOM) characters, no line or paragraph separators, no private-use characters, surrogates or non-characters, no Hangul fillers (U+115F, U+1160, U+3164, U+FFA0) or U+FFFD, and no `<`, `>` or `&`; anything else is 400 `Invalid card words`. `ryoku-hub` applies the same rule before it signs (`backend/cardwords.go`) and binders check it again.

Only the badge decides, as for styles: `PUT /v1/cards/<fp>` stores a v4 record only for an active badge on that card id signed by that badge's device key, else 403 `Card words need an active team badge on this machine` (no badge) or 403 `Invalid card signature` (another key). Serving, revocation and the cron treat it as they treat a styled record: withheld at once once the badge is revoked, deleted by `POST /v1/revoke`, swept by the 03:17 cron. Deploy the Worker before clients that send v4: an older gate refuses v4 with 400 `Invalid record fields`. Binders from before v4 keep the last v2 or v3 copy they verified of a card that moved to v4.

The existing live-card routes still accept only still JPEG pictures from non-holders. Legacy design approvals remain issuer-signed and badge-bound. The private candidate below also admits verified Account users with active Creator, but does not add animated live-record or binder transport.

## Bot Fight Mode stays off

The ryoku.dev zone's Bot Fight Mode is off. It challenged Ryoku's own Go client (`Go-http-client`) from some networks with a browser page, so card sync and binder pulls failed; on the Free plan it can't be skipped for one hostname. The Worker does its own checking: signatures on every write, rate limiters, and size caps.

## Private inspection candidate

This source is not a deployment. Use a separate private bucket, Durable Object namespace and HTTPS staging hostname, never the production bucket for smoke tests. Existing card record v2/v3/v4 and legacy RYD v2 layouts are unchanged, including the owner-only Stand style.

### Required configuration

`INSPECTION_QUEUE` binds `InspectionQueue`, with the `private-inspection-v1` SQLite Durable Object migration. `BUCKET` must be private. Missing queue or Account authority returns 503, never in-memory or unauthenticated fallback.

| Setting | Meaning |
| --- | --- |
| `ACCOUNTS_URL` | HTTPS Account service origin, without path/query/userinfo |
| `CARD_GATE_TOKEN` | Separate Account bridge secret, at least 32 characters |
| `CARD_GATE_AUDIENCE` | Exact Account bridge audience sent as `X-Ryoku-Audience` |
| `SUBMISSION_AUDIENCE` | Exact signed manifest audience, normally the staging team origin |
| `ACCOUNT_WEB_ORIGINS` | JSON array of exact allowed HTTPS review app origins, empty denies browser origins |
| `INSPECTOR_WORKER_TOKEN` | Separate worker-only secret, at least 32 characters |
| `MIKA_REVIEW_TOKEN` | Separate restricted Mika review secret, never in the browser |
| `INSPECTOR_KEYS` | JSON map `keyId -> {publicKey,revoked:false}`, base64 raw Ed25519 public keys |
| `INSPECTOR_PIPELINE_VERSION` | Exact trusted pipeline version required in certificates |
| `INSPECTOR_POLICY_VERSION` | Exact trusted inspection/content policy version |
| `MIKA_REVIEW_MODELS` | Exact allowlist JSON array of operator-reviewed local model IDs |
| `DESIGN_APPROVAL_KEYS` | JSON map `keyId -> {publicKey,revoked:false,role:"owner"|"mika"}` |
| `DESIGN_SIGNER_URL`, `DESIGN_SIGNER_TOKEN` | Optional HTTPS restricted signer endpoint and separate bearer secret |

All service credentials must differ. No general owner/Mika issuer seed belongs in the gate, inspector, web app or reviewer. Restricted approval keys only authorize the new exact-hash approval format. An absent signer leaves `awaiting_signature`; it does not approve anything.

The Account bridge receives `POST /internal/cards/authorize` with `{token,action:"submit"|"own"|"review",cardId?}`. It verifies real account identity, linked device and current terms/attestation. Submit additionally requires active Creator or an independently verified active badge. The gate does not accept a caller-provided account id. Serving a new publication rechecks the badge or Account `/internal/cards/eligibility` for the linked account/card and `animated-share`.

### Submission and worker transport

`POST /v1/submissions` takes `{manifest:{payload,sig}}`, domain `ryoku-submit-v1`, and returns `{id,state:"uploading",uploadPath,expiresAt}`. The payload is the version 1 detached manifest from the shared product contract. It binds device/card, original ZIP digest, sorted per-file inventory, current terms/attestation and three explicit consent booleans, nonce, server-time window, audience and kind. Design archives contain `design.json` plus flat supported images, hero archives contain exactly one supported image. No video or executable content is admitted.

`PUT uploadPath` takes exact `application/zip`, explicit length, at most 8 MiB and the authenticated submitting account. Reads count actual streamed bytes before the immutable object is accepted, and SHA-256 must match the signed manifest. A successful upload returns 202 queued, not approval. GET `/v1/submissions/<id>` gives owner/reviewer status, DELETE cancels only an owned unclaimed upload/queued job.

Admission is serialized durably: at most 20 waiting jobs globally, one inspecting job globally, two waiting plus one running per account, 464 MiB charged original/output/reservation bytes plus 48 MiB crash headroom, and 2,000 retained job records, with a 100 KiB admission-index limit and 100 KiB per-job metadata limit, so published content still consumes real capacity rather than disappearing to make room

Each eligible account gets eight new reservations per rolling 24 hours, measured by server time, cancellation does not refund them and 429 gives the exact earliest admission retry in seconds, other capacity failures return 503 with `Retry-After: 60`

Cancelled, expired, failed, rejected, changes-requested and withdrawn jobs lose their private objects immediately, failed object deletion stays charged and the alarm retries it, their full metadata and audit stay private for 24 hours after the terminal transition and are then removed from durable storage and the index

Nonce markers last through the signed manifest's final accepted second, freshness is checked before replay lookup so an expired signed manifest can never reopen after cleanup, account admission records expire after their rolling window and exact-hash withdrawal/revocation tombstones remain indefinitely, approved publications and live review jobs are not garbage-collected

Service authentication and bounded body reads happen before the mutation queue, Account authorization also runs outside it with its existing ten-second outbound deadline, every request body has a ten-second total deadline that cancels a stalled reader, MIME and explicit byte-length limits remain unchanged and state, ownership and leases are checked again before mutation

Read-only requests run outside mutation serialization too, so a slow public eligibility lookup cannot block worker heartbeats or alarms, publication state and revocations are checked again after that lookup before anything is served

Worker-only routes:

| Route | Body/authentication |
| --- | --- |
| `POST /internal/inspection/claim` | `{}` -> `{jobId,lease,inputPath,manifest,expiresAt}`, 204 if no work |
| `GET /internal/inspection/<id>/input` | `X-Ryoku-Lease`, exact leased original only |
| `POST /internal/inspection/<id>/heartbeat` | `{lease}` -> `{expiresAt}`, 5-minute lease, 30-minute hard deadline |
| `PUT /internal/inspection/<id>/objects/<name>` | `X-Ryoku-Lease`, immutable bounded raw object |
| `POST /internal/inspection/<id>/result` | `{lease,certificate}` |
| `POST /internal/inspection/<id>/failure` | `{lease,reason}`, reason at most 256 characters |

Every worker route requires `INSPECTOR_WORKER_TOKEN`. Fixed output names are `canonical` (ZIP, 8 MiB), `poster` (PNG, 8 MiB), `preview` (GIF/WebP, 24 MiB) and `report` (JSON, 8 MiB). All four are required. Only SHA-256 of the secret lease is stored or signed as `leaseSha`, raw capabilities never enter certificates, reports, R2 keys or audit records.

The version 1 `ryoku-inspect-v1` certificate binds job, lease hash, manifest payload digest, original ZIP digest, exact output inventory/digests, pipeline/policy versions, inspection time, all four technical passes and moderation status. Missing/partial stages, revoked keys, wrong leases and changed bytes cannot enter review. The trusted certificate's moderation may remain `unreviewed`, never fabricate a clean classifier.

### Content review, decision and signing

Owner review uses Account-verified owner identity; Mika uses only the restricted review credential. List/search at `GET /v1/review/submissions?state=&q=&cursor=0&limit=25` returns `{items,nextCursor}`. Detail `/v1/review/submissions/<id>` includes manifest, certificate, payload digest `certificateSha`, report, content reviews, optional signing request and fixed authenticated asset paths. `/poster`, `/preview`, `/report`, `/canonical`, `/input` are authenticated object reads. ZIPs are explicit attachments, never auto-decoded by the review web page.

POST `/claim` with `{}` claims a job for that reviewer for 600 seconds, POST `/content-review` accepts `{verdict,reason,idempotencyKey,model,coverage:{frames,totalFrames,durationMs,composition?},policyVersion,reviewVersion,canonicalSha,previewSha,reportSha,certificateSha}`, verdict is `clear`, `flagged` or `uncertain` and each receipt binds the reviewer, model, actual coverage, policy and certified hashes without changing the technical certificate

Owner clear still requires explicit `model:"manual"` and every canonical frame, Mika clear requires `reviewVersion:"mika-local-card-review-v2"`, an allowed local model, every source-frame index from the signed report's complete review set of at most eight frames, and `coverage.composition:true`

The final signed report's `composition` binds the exact canonical ZIP, full rendered poster and preview hashes, its small composed PNG and all five canonical human-text fields, hero-only reports have no custom text, source and composed PNG hashes, dimensions and formats are checked before automatic clear

Flagged or uncertain receipts keep the actual visited source-frame indices, an uncertain escalation without inference uses `frames:[]` and `composition:false`, neither can approve and every non-clear receipt releases the claim immediately for the owner

Mika can append a non-clear escalation after its own clear, including while awaiting signature, this preserves the old receipt and approval decision in `decisionHistory`, returns the job to owner review and invalidates the old signing request, Mika cannot clear an existing hold and owner review appends a new receipt and decision

POST `/decision` accepts `{verdict:"approve"|"reject"|"changes",reason,idempotencyKey}`. It requires the current reviewer claim and valid exact certified bytes. Approval also requires that reviewer's current separate clear content review. Neither owner nor Mika can turn technical failure into pass. Decisions and content reviews are immutable and exact-idempotent; conflicting concurrent decisions return 409. Approve initially returns `awaiting_signature`.

New approvals use domain `ryoku-design-approval-v3`, not an extension to legacy binary RYD v2. The exact payload is `{v:3,keyId,jobId,accountId,cardId,canonicalSha,posterSha,previewSha,certificateSha,contentReviewSha,decisionId,reviewerRole,approvedAt}`. The public key must be trusted, not revoked and scoped to the decision role. `certificateSha` is SHA-256 of decoded canonical certificate payload bytes, not its envelope JSON. The content review digest covers `{v,jobId,actor,role,at,submitted}` before adding outer `sha` and `idempotencyKey`.

POST `/signature` accepts `{approval:{payload,sig}}` from an owner or Mika completing its own exact decision, it checks the genuine restricted signature, current technical authority and current clear-review predicate before entering `approved`, the same content predicate runs at approval decisions, signer invocation, signature completion and published serving so retained Mika-v1 or composition-omitting clear receipts cannot bypass the policy, an owner may POST `{}` to invoke the optional restricted signer and an unavailable signer never fabricates success

Only `approved` publications are served at `/v1/published-designs/<id>/{canonical|poster|preview}`. The exact certified objects stay private behind the gate; originals have no public path. POST `/v1/submissions/<id>/withdraw` lets the author withdraw, and owner reviewers use `/v1/review/submissions/<id>/withdraw`; both take `{reason,idempotencyKey}`. Owner-only `/v1/design-revocations` takes `{sha,reason,idempotencyKey}` for legacy exact-hash takedown too. Tombstones are checked on every direct read. Badge `RYX` remains a badge revocation, not a design withdrawal.

### Protocol fixtures and integrated checks

`test/inspection-vectors.json` contains stable synthetic RFC 8032 public-key fixtures for submission, inspection, approval v3 and Unicode canonical JSON. Each binds exact canonical hex, domain-separated message hex, payload SHA-256 and signature. Object keys are ASCII protocol keys, recursively sorted, compact JSON, arrays in original order, safe integer numbers only, no lone surrogates or duplicate/noncanonical fields. Envelopes use standard padded base64. These hooks do not claim parity until the integration owner runs them:

```sh
node --test release/team-badges/test/*.test.js
python3 release/team-badges/test/verify-inspection-vectors.py
go run release/team-badges/test/verify-inspection-vectors.go
```

The Python check needs `cryptography`, use the inspector's isolated Python environment if the host does not provide it. `test/inspection.test.js` uses synthetic signing keys and in-memory storage/Account fixtures only, never real accounts or R2. Staging proof additionally needs the real Account bridge, isolated inspector runtime, actual local review worker, registered restricted signer and private Cloudflare bindings. This queue alone does not finish animated friend sync.

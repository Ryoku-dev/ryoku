# Local card review

This client reviews certified source-frame PNGs, the real composed card poster and all canonical design text with Mika's existing local model. It never opens an uploaded ZIP, GIF, WebP or card script on Ally. The gate's distinct Mika review credential permits review actions; a separate Ed25519 seed signs only the restricted publication protocol, not badges.

The inspector certificate remains `unreviewed`. A successful model call creates a separate immutable content-review record, followed by a decision and an exact `ryoku-design-approval-v3` signature. Publication is complete only when the gate accepts that signature. Flags, uncertainty, incomplete coverage, unavailable inference and technical failures cannot become an automatic approval.

## Configure privately

Install `requirements.txt` in a separate virtual environment. Do not add dependencies to, restart or reconfigure the running Mika/model environment.

Provide:

- Exact HTTPS gate origin and a distinct Mika review token file.
- Independently provisioned inspector public-key registry, keyed by key ID with `publicKey` and explicit `revoked: false`. Keys supplied by a submission are not a trust source.
- A separate restricted approval seed, canonical base64 of exactly 32 bytes, and its registered review-only key ID. Never use the general owner/Mika badge issuer seed.
- The existing local model origin, model ID and optional local API credential file. The production model must be literal loopback HTTP. There is no cloud or paid fallback.

Private files must be regular, owned by the process user or root, and inaccessible to group/other. Symlinks are refused. Run as a dedicated, restricted reviewer identity with read-only code, narrow secrets, a serialized scheduler and bounded resources. Do not grant the conversational bot root, a Docker socket or arbitrary signing access.

```sh
/path/to/reviewer-venv/bin/python reviewer.py \
  --gate https://team.ryoku.dev \
  --token-file /etc/ryoku-card-reviewer/review-token \
  --inspector-keys /etc/ryoku-card-reviewer/inspector-public-keys.json \
  --approval-key /etc/ryoku-card-reviewer/review-seed \
  --approval-key-id REVIEW_KEY_ID \
  --model-url http://127.0.0.1:11435 \
  --model gemma-4-12b-local \
  --model-token-file /etc/ryoku-card-reviewer/local-model-token
```

One invocation handles one queued job or resumes its own pending signature. An idle queue does not call the model. Exit 75 means deferred; exit 1 means refused. Never translate either into a clean result. `--job-id` chooses one authorized job. `--allow-loopback-http` is only for an explicitly local gate test; it cannot authorize a non-loopback HTTP server or remote model.

The client refuses redirects, cross-origin requests and environment proxies, so a review credential cannot be redirected or accidentally sent through a plaintext local-model proxy. It verifies exact report bytes, certificate signatures, device-signed submission provenance, audience, full content-review scope and decision before signing. Resuming `awaiting_signature` performs fresh review and rejects certificate substitution; a cached clear assessment is not signing authority.

Review version `mika-local-card-review-v2` consumes every source frame when the entire timeline fits eight frames, then the real composed poster with every human-readable typed canonical design field. `report.composition` must bind the exact canonical/poster/preview hashes and final report. Text is explicitly supplied as untrusted JSON data, not instructions, even when small or hidden in the image. The resulting `coverage.composition` is true only after that additional actual model call. This is composed-poster context plus exact text, not exhaustive continuous composed-render coverage.

Each evidence image is stripped PNG at most 384 pixels per side. The client independently checks chunk CRCs, RGB/RGBA encoding, exact bounded inflate size, row filters and absence of metadata/trailers before passing it to the model. A larger source timeline, missing composed evidence, uncertainty or flags go to the owner; the client never invents full coverage. The Gate records partial/non-clear review with the actual consumed frame indices and withholds publication.

One invocation fairly pages actionable jobs, skipping previously held/uncertain/older-version jobs without starving later clean work. Cursor loops and excessive pagination fail closed. The local `/slots` endpoint must report idle before each image request. This is conservative admission, not a promise that a later user chat cannot arrive. A serialized scheduler must leave priority and memory headroom for Mika's normal work.

## Verification

```sh
python -m unittest discover -s release/team-badges/reviewer -p 'test_*.py' -v
```

The boundary suite exercises real Ed25519 signatures, source/certificate/decision substitution, fresh certificate checks on signing, incomplete/composed coverage, revocation, malformed PNGs and inflate bounds, canonical JSON, secret-file permissions, fair cursor pagination and real HTTP servers proving that redirects/proxies do not receive credentials.

A private smoke run verified the actual inspector certificate and report, then reached the existing authenticated local model for four source frames and the real composed poster with all five canonical text fields. All five assessments were valid `clear` results. No raw upload reached Ally, no Mika/model restart or cloud inference was used, and no approval signature was produced. That proves actual v2 inference on certified evidence, not the complete authenticated queue-to-publication workflow or high-volume capacity. Exact artifacts, measurements and remaining gates are in the owner's local handoff.

Model output is an assessment under the hosted-content policy, not legal advice or proof of age, copyright, consent, identity or legality. Missing authority and uncertain content remain held. Public activation and owner acceptance are separate rollout gates.

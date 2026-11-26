import test from 'node:test';
import assert from 'node:assert/strict';
import { InspectionQueue, inspectionRoute } from '../src/inspection-queue.js';
import { canonical, encoded, openEnvelope, submission, certificate, approval } from '../src/inspection-protocol.js';
import { base64, digest, message, unbase64 } from '../src/codec.js';
import { cardContentSHA } from '../src/card-safety.js';
import { setup, keys, now, code, call, cardBody, deviceRecord, wordedRecord } from './helpers.js';

class Storage {
  data = new Map();
  async get(key) { return structuredClone(this.data.get(key)); }
  async put(key, value) { this.data.set(key, structuredClone(value)); }
  async delete(key) { return this.data.delete(key); }
  async transaction(callback) {
    const saved = structuredClone(this.data);
    try { return await callback(this); } catch (error) { this.data = saved; throw error; }
  }
  async setAlarm(at) { this.alarmAt = at; }
  async getAlarm() { return this.alarmAt ?? null; }
}
async function envelope(key, domain, payload) {
  const raw = encoded(payload);
  return { payload: base64(raw), sig: base64(new Uint8Array(await crypto.subtle.sign('Ed25519', key.privateKey, message(domain, raw)))) };
}
async function fixture(t, overrides = {}) {
  const state = await setup(), inspector = await keys(), signer = await keys(), storage = new Storage();
  Object.assign(state.env, { ACCOUNTS_URL: 'https://accounts.invalid', CARD_GATE_TOKEN: 'a'.repeat(40), CARD_GATE_AUDIENCE: 'ryoku-card-gate-v1', SUBMISSION_AUDIENCE: 'https://team.ryoku.dev', INSPECTOR_WORKER_TOKEN: 'w'.repeat(40), MIKA_REVIEW_TOKEN: 'm'.repeat(40), INSPECTOR_KEYS: JSON.stringify({ scanner: { publicKey: base64(inspector.raw), revoked: false } }), DESIGN_APPROVAL_KEYS: JSON.stringify({ owner: { publicKey: base64(signer.raw), revoked: false, role: 'owner' } }), INSPECTOR_PIPELINE_VERSION: 'test-pipeline-v1', INSPECTOR_POLICY_VERSION: 'test-policy-v1', MIKA_REVIEW_MODELS: '["local-test-model"]', ...overrides });
  const identities = { owner: { accountId: 'owner-account', reviewer: 'owner' }, user: { accountId: 'test-account', cardId: state.id, device: base64(state.device.raw), termsVersion: 'terms-v1', attestationVersion: 'attest-v1', eligible: true }, other: { accountId: 'other-account', cardId: state.id, device: base64(state.device.raw), termsVersion: 'terms-v1', attestationVersion: 'attest-v1', eligible: true } };
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(new URL(url).hostname, 'accounts.invalid');
    assert.equal(options.headers['X-Ryoku-Audience'], 'ryoku-card-gate-v1');
    const body = JSON.parse(options.body);
    if (new URL(url).pathname.endsWith('/eligibility')) return Response.json({ accountId: 'test-account', cardId: state.id, active: true, until: Date.now() + 86400000, reason: 'creator' });
    const identity = identities[body.token];
    return identity ? Response.json(identity) : Response.json({ error: 'denied' }, { status: 403 });
  });
  const queue = new InspectionQueue({ storage }, state.env);
  async function request(method, path, value, auth = 'user', headers = {}) {
    const raw = value === undefined ? undefined : value instanceof Uint8Array ? value : new TextEncoder().encode(JSON.stringify(value));
    return queue.fetch(new Request(`https://team.ryoku.dev${path}`, { method, ...(raw ? { body: raw } : {}), headers: { ...(auth ? { Authorization: `Bearer ${auth}` } : {}), ...(raw ? { 'Content-Type': 'application/json', 'Content-Length': String(raw.length) } : {}), ...headers } }));
  }
  async function manifest(overrides = {}) {
    const archive = new Uint8Array([80, 75, 3, 4]);
    const payload = { v: 1, cardId: state.id, device: base64(state.device.raw), originalSha: await digest(archive), files: [{ name: 'design.json', sha: '01'.repeat(32), bytes: 10 }], termsVersion: 'terms-v1', attestationVersion: 'attest-v1', ownOrLicensed: true, policyAccepted: true, consentToReview: true, nonce: crypto.randomUUID().replaceAll('-', ''), submittedAt: now(), audience: 'https://team.ryoku.dev', kind: 'design', ...overrides };
    return { archive, payload, envelope: await envelope(state.device, 'ryoku-submit-v1', payload) };
  }
  async function reserve(overrides = {}) {
    const input = await manifest(overrides), response = await request('POST', '/v1/submissions', { manifest: input.envelope });
    assert.equal(response.status, 201, await response.clone().text());
    return { ...input, ...await response.json() };
  }
  async function inspected(moderation = 'unreviewed', composed = false) {
    const job = await reserve();
    assert.equal((await request('PUT', job.uploadPath, job.archive, 'user', { 'Content-Type': 'application/zip' })).status, 202);
    const lease = await (await request('POST', '/internal/inspection/claim', {}, state.env.INSPECTOR_WORKER_TOKEN)).json();
    const objects = { canonical: new Uint8Array([80, 75, 3, 4, 5]), poster: unbase64('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRusAAAAASUVORK5CYII='), preview: new Uint8Array([71, 73, 70]), report: new TextEncoder().encode(JSON.stringify({ v: 1, totalFrames: 1, durationMs: 0 })) };
    const types = { canonical: 'application/zip', poster: 'image/png', preview: 'image/gif', report: 'application/json' };
    if (composed) {
      const png = unbase64('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==');
      const image = { mime: 'image/png', sha256: await digest(png), base64: base64(png) };
      objects.report = new TextEncoder().encode(JSON.stringify({
        v: 1, totalFrames: 1, durationMs: 0,
        review: { complete: true, totalFrames: 1, images: [{ ...image, sourceAsset: 'asset.png', frameIndex: 0 }] },
        composition: { v: 1, canonicalSha: await digest(objects.canonical), posterSha: await digest(objects.poster), previewSha: await digest(objects.preview), coverage: 'poster-and-canonical-text', image,
          text: [{ field: 'name', value: 'Private card' }, { field: 'badge.title', value: 'Tester' }, { field: 'badge.kanji', value: '' }, { field: 'card.words.tagline', value: 'Untrusted design text' }, { field: 'card.words.epithet', value: '' }] },
      }));
    }
    const files = [];
    for (const name of Object.keys(objects).sort()) {
      assert.equal((await request('PUT', `/internal/inspection/${job.id}/objects/${name}`, objects[name], state.env.INSPECTOR_WORKER_TOKEN, { 'Content-Type': types[name], 'X-Ryoku-Lease': lease.lease })).status, 200);
      files.push({ name, sha: await digest(objects[name]), bytes: objects[name].length });
    }
    const cert = { v: 1, keyId: 'scanner', jobId: job.id, leaseSha: await digest(new TextEncoder().encode(lease.lease)), manifestSha: await digest(encoded(job.payload)), originalSha: job.payload.originalSha, canonicalSha: files.find(f => f.name === 'canonical').sha, files, posterSha: files.find(f => f.name === 'poster').sha, previewSha: files.find(f => f.name === 'preview').sha, reportSha: files.find(f => f.name === 'report').sha, pipelineVersion: 'test-pipeline-v1', policyVersion: 'test-policy-v1', inspectedAt: now(), technical: { archive: 'pass', decode: 'pass', canonical: 'pass', render: 'pass' }, moderation };
    const signed = await envelope(inspector, 'ryoku-inspect-v1', cert);
    assert.equal((await request('POST', `/internal/inspection/${job.id}/result`, { lease: lease.lease, certificate: signed }, state.env.INSPECTOR_WORKER_TOKEN)).status, 200);
    return { ...job, lease, cert, signed, objects };
  }
  return { ...state, queue, storage, inspector, signer, identities, request, manifest, reserve, inspected };
}

test('canonical protocol refuses ambiguity, unsafe integers, duplicate JSON and cross-domain signatures', async () => {
  assert.equal(canonical({ z: ['日本語', '🪨'], a: true }), '{"a":true,"z":["日本語","🪨"]}');
  for (const value of [NaN, Infinity, 1.5, 9007199254740992, -0, '\ud800', { 'é': 1 }]) assert.throws(() => canonical(value));
  const pair = await keys(), signed = await envelope(pair, 'ryoku-submit-v1', { v: 1 });
  await assert.rejects(openEnvelope(signed, 'ryoku-inspect-v1', base64(pair.raw)));
  const raw = new TextEncoder().encode('{"v":1,"v":1}');
  const duplicate = { payload: base64(raw), sig: base64(new Uint8Array(await crypto.subtle.sign('Ed25519', pair.privateKey, message('ryoku-submit-v1', raw)))) };
  await assert.rejects(openEnvelope(duplicate, 'ryoku-submit-v1', base64(pair.raw)), /Noncanonical/);
});

test('admission fails closed for auth, current consent, device, eligibility and missing durable binding', async t => {
  const f = await fixture(t), m = await f.manifest();
  assert.equal((await inspectionRoute(new Request('https://team.ryoku.dev/v1/submissions'), {})).status, 503);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: m.envelope }, null)).status, 401);
  f.identities.user.device = base64((await keys()).raw);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: m.envelope })).status, 403);
  f.identities.user.device = base64(f.device.raw); f.identities.user.termsVersion = 'old';
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: m.envelope })).status, 403);
  f.identities.user.termsVersion = 'terms-v1'; f.identities.user.eligible = false;
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: m.envelope })).status, 403);
  const badge = await code('RYB', f.owner, { device: f.device });
  await call(f.env, 'POST', '/v1/badges', badge);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: m.envelope })).status, 201);
});

test('owned immutable upload, nonce idempotency and serialized per-account admission survive racing requests', async t => {
  const f = await fixture(t), job = await f.reserve();
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: job.envelope })).status, 200);
  assert.equal((await f.request('GET', `/v1/submissions/${job.id}`, undefined, 'other')).status, 403);
  assert.equal((await f.request('PUT', job.uploadPath, new Uint8Array([1]), 'user', { 'Content-Type': 'application/zip' })).status, 400);
  assert.equal((await f.request('PUT', job.uploadPath, job.archive, 'user', { 'Content-Type': 'application/zip' })).status, 202);
  assert.equal((await f.request('PUT', job.uploadPath, job.archive, 'user', { 'Content-Type': 'application/zip' })).status, 409);
  const a = await f.manifest(), b = await f.manifest();
  const results = await Promise.all([a, b].map(m => f.request('POST', '/v1/submissions', { manifest: m.envelope })));
  assert.deepEqual(results.map(r => r.status).sort(), [201, 429]);
  const restarted = new InspectionQueue({ storage: f.storage }, f.env);
  assert.equal((await restarted.fetch(new Request(`https://team.ryoku.dev/v1/submissions/${job.id}`, { headers: { Authorization: 'Bearer user' } }))).status, 200);
});

test('worker identity, current secret lease, complete signed outputs, revoked keys and exact bytes are mandatory', async t => {
  const f = await fixture(t), job = await f.inspected();
  assert.equal((await f.request('POST', '/internal/inspection/claim', {}, f.env.MIKA_REVIEW_TOKEN)).status, 401);
  const stored = await f.storage.get(`job:${job.id}`);
  assert.equal(stored.lease, undefined);
  assert.equal(JSON.stringify(stored.certificate).includes(job.lease.lease), false);
  assert.equal((await f.request('POST', `/internal/inspection/${job.id}/result`, { lease: job.lease.lease, certificate: job.signed }, f.env.INSPECTOR_WORKER_TOKEN)).status, 200);
  const wrong = { ...job.cert, leaseSha: '00'.repeat(32) };
  await assert.rejects(certificate(await envelope(f.inspector, 'ryoku-inspect-v1', wrong), stored, f.env, now()));
  const partial = { ...job.cert, technical: { ...job.cert.technical, render: 'fail' } };
  await assert.rejects(certificate(await envelope(f.inspector, 'ryoku-inspect-v1', partial), stored, f.env, now()));
  f.env.INSPECTOR_KEYS = JSON.stringify({ scanner: { publicKey: base64(f.inspector.raw), revoked: true } });
  await assert.rejects(f.queue.validCertified(stored));
  f.env.INSPECTOR_KEYS = JSON.stringify({ scanner: { publicKey: base64(f.inspector.raw), revoked: false } });
  await f.env.BUCKET.put(stored.outputs.canonical.key, new Uint8Array([0]));
  await assert.rejects(f.queue.validCertified(stored), /changed/);
});

test('maximum 24-file detached manifest accepts a base64 envelope over 4KiB but rejects a 25th file', async t => {
  const f = await fixture(t);
  const files = Array.from({ length: 23 }, (_, i) => ({ name: `asset${String(i).padStart(2, '0')}_${'a'.repeat(20)}.png`, sha: '12'.repeat(32), bytes: 100 }));
  files.push({ name: 'design.json', sha: '34'.repeat(32), bytes: 100 });
  const manifest = await f.manifest({ files });
  assert.ok(manifest.envelope.payload.length > 4096);
  assert.equal((await submission(manifest.envelope, 'https://team.ryoku.dev', now())).payload.files.length, 24);
  const oversized = await f.manifest({ files: [...files, { name: 'z.png', sha: '56'.repeat(32), bytes: 100 }] });
  await assert.rejects(submission(oversized.envelope, 'https://team.ryoku.dev', now()), /inventory/);
});

test('review claims, moderation and immutable decisions never publish without a real restricted signature', async t => {
  const f = await fixture(t), job = await f.inspected();
  const base = `/v1/review/submissions/${job.id}`;
  assert.equal((await f.request('GET', base, undefined, 'user')).status, 403);
  assert.equal((await f.request('POST', `${base}/claim`, {}, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  const decision = { verdict: 'approve', reason: 'Reviewed complete preview', idempotencyKey: 'decision-one' };
  assert.equal((await f.request('POST', `${base}/decision`, decision, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  assert.equal((await f.request('POST', `${base}/claim`, {}, 'owner')).status, 409);
  let stored = await f.storage.get(`job:${job.id}`); stored.reviewClaim.expiresAt = now() - 1; await f.storage.put(`job:${job.id}`, stored);
  assert.equal((await f.request('POST', `${base}/claim`, {}, 'owner')).status, 200);
  const review = { verdict: 'clear', reason: 'Owner inspected all canonical frames', idempotencyKey: 'content-one', model: 'manual', coverage: { frames: [0], totalFrames: 1, durationMs: 0 }, policyVersion: 'test-policy-v1', reviewVersion: 'review-v1', canonicalSha: job.cert.canonicalSha, previewSha: job.cert.previewSha, reportSha: job.cert.reportSha, certificateSha: await digest(encoded(job.cert)) };
  assert.equal((await f.request('POST', `${base}/content-review`, review, 'owner')).status, 200);
  const [accepted, conflicting] = await Promise.all([
    f.request('POST', `${base}/decision`, decision, 'owner'),
    f.request('POST', `${base}/decision`, { ...decision, verdict: 'reject', idempotencyKey: 'racing-decision' }, 'owner'),
  ]);
  assert.equal(accepted.status, 200); assert.equal(conflicting.status, 409);
  assert.equal((await accepted.json()).state, 'awaiting_signature');
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}/canonical`, undefined, null)).status, 404);
  assert.equal((await f.request('POST', `${base}/signature`, {}, 'owner')).status, 200);
  stored = await f.storage.get(`job:${job.id}`); assert.equal(stored.state, 'awaiting_signature');
  const signed = await envelope(f.signer, 'ryoku-design-approval-v3', { ...stored.approvalPayload, keyId: 'owner' });
  assert.equal((await f.request('POST', `${base}/signature`, { approval: signed }, 'owner')).status, 200);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}/canonical`, undefined, null)).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { ...decision, verdict: 'reject' }, 'owner')).status, 409);
  assert.equal((await f.request('POST', `/v1/submissions/${job.id}/withdraw`, { reason: 'Author withdrawal', idempotencyKey: 'withdraw-one' })).status, 200);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}/canonical`, undefined, null)).status, 404);
});

test('expired worker lease fails without retry and removes all private source bytes', async t => {
  const f = await fixture(t), job = await f.reserve();
  await f.request('PUT', job.uploadPath, job.archive, 'user', { 'Content-Type': 'application/zip' });
  const claim = await (await f.request('POST', '/internal/inspection/claim', {}, f.env.INSPECTOR_WORKER_TOKEN)).json();
  const stored = await f.storage.get(`job:${job.id}`);
  assert.equal(stored.lease, undefined);
  assert.equal((await f.request('GET', claim.inputPath, undefined, f.env.INSPECTOR_WORKER_TOKEN, { 'X-Ryoku-Lease': 'wrong' })).status, 409);
  assert.equal((await f.request('GET', claim.inputPath, undefined, f.env.INSPECTOR_WORKER_TOKEN, { 'X-Ryoku-Lease': claim.lease })).status, 200);
  stored.expiresAt = now() - 1; await f.storage.put(`job:${job.id}`, stored);
  const status = await (await f.request('GET', `/v1/submissions/${job.id}`)).json();
  assert.equal(status.state, 'failed');
  assert.equal(await f.env.BUCKET.head(stored.inputKey), null);
  assert.equal((await f.request('POST', '/internal/inspection/claim', {}, f.env.INSPECTOR_WORKER_TOKEN)).status, 204);
  assert.equal((await f.storage.get('index')).bytes, 0);
});

test('global byte and retained-job backpressure is durable and carries Retry-After', async t => {
  const f = await fixture(t), input = await f.manifest();
  await f.storage.put('index', { ids: [], active: {}, bytes: 512 * 1024 * 1024 });
  const fullBytes = await f.request('POST', '/v1/submissions', { manifest: input.envelope });
  assert.equal(fullBytes.status, 503); assert.equal(fullBytes.headers.get('Retry-After'), '60');
  await f.storage.put('index', { ids: Array.from({ length: 2000 }, () => crypto.randomUUID()), active: {}, bytes: 0 });
  for (const id of (await f.storage.get('index')).ids) await f.storage.put(`job:${id}`, { id, state: 'approved' });
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: input.envelope })).status, 503);
});

test('browser origins are explicit and pending originals remain issuer-authenticated', async t => {
  const f = await fixture(t);
  const preflight = origin => new Request('https://team.ryoku.dev/v1/review/submissions', { method: 'OPTIONS', headers: { Origin: origin, 'Access-Control-Request-Headers': 'authorization,content-type' } });
  assert.equal((await inspectionRoute(preflight('https://accounts.invalid'), f.env)).status, 403);
  f.env.ACCOUNT_WEB_ORIGINS = '[\"https://accounts.invalid\"]';
  assert.equal((await inspectionRoute(preflight('https://accounts.invalid'), f.env)).status, 204);
  assert.equal((await inspectionRoute(preflight('https://evil.invalid'), f.env)).status, 403);
  const sha = '19'.repeat(32);
  await f.env.BUCKET.put(`packs/${sha}.zip`, new Uint8Array([80, 75]));
  assert.equal((await call(f.env, 'GET', `/v1/packs/${sha}.zip`)).status, 200);
  assert.equal((await call(f.env, 'GET', `/v1/pending/${sha}.zip`)).status, 400);
  await f.env.BUCKET.put(`design-revocations/${sha}.json`, JSON.stringify({ reason: 'Withdrawn' }));
  assert.equal((await call(f.env, 'GET', `/v1/packs/${sha}.zip`)).status, 404);
});

test('cancel churn is charged by server time, releases bytes and never permanently exhausts another account', async t => {
  const f = await fixture(t), first = await f.reserve();
  assert.equal((await f.request('DELETE', `/v1/submissions/${first.id}`)).status, 200);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: first.envelope })).status, 200);
  for (let i = 1; i < 8; i++) {
    const job = await f.reserve();
    assert.equal((await f.request('DELETE', `/v1/submissions/${job.id}`)).status, 200);
  }
  const denied = await f.request('POST', '/v1/submissions', { manifest: (await f.manifest()).envelope });
  assert.equal(denied.status, 429);
  const retry = Number(denied.headers.get('Retry-After'));
  assert.ok(retry >= 86390 && retry <= 86400);
  assert.equal((await f.storage.get('index')).bytes, 0);
  const other = await f.request('POST', '/v1/submissions', { manifest: (await f.manifest()).envelope }, 'other');
  assert.equal(other.status, 201);
  assert.equal((await f.request('DELETE', `/v1/submissions/${(await other.json()).id}`, undefined, 'other')).status, 200);
  const clock = Date.now();
  t.mock.method(Date, 'now', () => clock + 86401000);
  await f.queue.alarm();
  assert.equal(await f.storage.get(`job:${first.id}`), undefined);
  const index = await f.storage.get('index');
  assert.deepEqual(index.ids, []);
  assert.deepEqual(index.nonces, {});
  assert.equal([...f.storage.data.keys()].some(key => key.startsWith('nonce:')), false);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: first.envelope })).status, 400);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: (await f.manifest()).envelope })).status, 201);
});

test('held unauthenticated body cannot block heartbeat or alarm, and deadline cancels its reader', async t => {
  const f = await fixture(t), job = await f.inspected();
  // Keep an actual inspection lease alive while a separate request never sends bytes.
  const active = await f.reserve();
  await f.request('PUT', active.uploadPath, active.archive, 'user', { 'Content-Type': 'application/zip' });
  const lease = await (await f.request('POST', '/internal/inspection/claim', {}, f.env.INSPECTOR_WORKER_TOKEN)).json();
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let started, cancelled = false;
  const reading = new Promise(resolve => { started = resolve; });
  const stream = new ReadableStream({ pull() { started(); }, cancel() { cancelled = true; } });
  const stalled = f.queue.fetch(new Request('https://team.ryoku.dev/v1/submissions', { method: 'POST', body: stream, duplex: 'half', headers: { Authorization: 'Bearer not-a-user', 'Content-Type': 'application/json', 'Content-Length': '100' } }));
  await reading;
  assert.equal((await f.request('POST', `/internal/inspection/${active.id}/heartbeat`, { lease: lease.lease }, f.env.INSPECTOR_WORKER_TOKEN)).status, 200);
  assert.equal((await f.request('GET', `/v1/submissions/${job.id}`)).status, 200);
  await f.queue.alarm();
  t.mock.timers.tick(10001);
  assert.equal((await stalled).status, 408);
  assert.equal(cancelled, true);
});

test('Account verification awaits outside the mutation lock', async t => {
  const f = await fixture(t), job = await f.reserve(), original = globalThis.fetch;
  let entered, release;
  const waiting = new Promise(resolve => { entered = resolve; });
  const answer = new Promise(resolve => { release = resolve; });
  t.mock.method(globalThis, 'fetch', (url, options) => {
    if (JSON.parse(options.body).token === 'slow') { assert.ok(options.signal); entered(); return answer; }
    return original(url, options);
  });
  const slow = f.request('GET', `/v1/submissions/${job.id}`, undefined, 'slow');
  await waiting;
  assert.equal((await f.request('GET', `/v1/submissions/${job.id}`)).status, 200);
  await f.queue.alarm();
  release(Response.json({ error: 'denied' }, { status: 401 }));
  assert.equal((await slow).status, 401);
});

function mikaReview(job, overrides = {}) {
  return { verdict: 'clear', reason: 'Actual local review', idempotencyKey: 'mika-clear', model: 'local-test-model',
    coverage: { frames: [0], totalFrames: 1, durationMs: 0, composition: true }, policyVersion: 'test-policy-v1', reviewVersion: 'mika-local-card-review-v2',
    canonicalSha: job.cert.canonicalSha, previewSha: job.cert.previewSha, reportSha: job.cert.reportSha, certificateSha: undefined, ...overrides };
}
async function mikaClaim(f, job, overrides = {}) {
  const base = `/v1/review/submissions/${job.id}`;
  assert.equal((await f.request('POST', `${base}/claim`, {}, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  return { base, review: mikaReview(job, { certificateSha: await digest(encoded(job.cert)), ...overrides }) };
}

test('Mika cannot discard a submission or reclaim an escalated owner review', async t => {
  const f = await fixture(t), job = await f.inspected('unreviewed', true), { base, review } = await mikaClaim(f, job);
  assert.equal((await f.request('GET', `${base}/input`, undefined, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  for (const verdict of ['reject', 'changes']) {
    assert.equal((await f.request('POST', `${base}/decision`, { verdict, reason: 'Limited reviewer cannot terminate content', idempotencyKey: verdict }, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  }
  const held = { ...review, verdict: 'uncertain', coverage: { ...review.coverage, frames: [], composition: false } };
  assert.equal((await f.request('POST', `${base}/content-review`, held, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('POST', `${base}/claim`, {}, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  const stored = await f.storage.get(`job:${job.id}`);
  assert.equal(stored.state, 'needs_review');
  assert.equal(stored.contentReviews.at(-1).submitted.verdict, 'uncertain');
  assert.equal(await digest(new Uint8Array(await (await f.request('GET', `${base}/input`, undefined, 'owner')).arrayBuffer())), job.payload.originalSha);
  assert.equal(await digest(new Uint8Array(await (await f.request('GET', `${base}/report`, undefined, 'owner')).arrayBuffer())), job.cert.reportSha);
  assert.equal((await f.request('POST', `${base}/claim`, {}, 'owner')).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'reject', reason: 'Explicit owner decision', idempotencyKey: 'owner-reject' }, 'owner')).status, 200);
  assert.equal((await f.storage.get(`job:${job.id}`)).bytes, 0);
});

test('author withdrawal is idempotent and cannot revoke another identical publication', async t => {
  const f = await fixture(t), published = await f.inspected('unreviewed', true), { base, review } = await mikaClaim(f, published);
  f.env.DESIGN_APPROVAL_KEYS = JSON.stringify({ mika: { publicKey: base64(f.signer.raw), revoked: false, role: 'mika' } });
  assert.equal((await f.request('POST', `${base}/content-review`, review, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Exact reviewed bytes', idempotencyKey: 'publish-first' }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  const approved = await f.storage.get(`job:${published.id}`);
  const signed = await envelope(f.signer, 'ryoku-design-approval-v3', { ...approved.approvalPayload, keyId: 'mika' });
  assert.equal((await f.request('POST', `${base}/signature`, { approval: signed }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  const publicPath = `/v1/published-designs/${published.id}/canonical`;
  assert.equal((await f.request('GET', publicPath, undefined, null)).status, 200);
  const copy = await f.inspected('unreviewed', true);
  assert.equal(copy.cert.canonicalSha, published.cert.canonicalSha);
  const path = `/v1/submissions/${copy.id}/withdraw`, withdrawal = { reason: 'Withdraw only this submission', idempotencyKey: 'author-withdraw' };
  assert.equal((await f.request('POST', path, withdrawal)).status, 200);
  assert.equal((await f.request('GET', publicPath, undefined, null)).status, 200);
  assert.equal((await f.request('POST', path, withdrawal)).status, 200);
  assert.equal((await f.request('POST', path, { ...withdrawal, reason: 'Different action' })).status, 409);
  const removed = await f.storage.get(`job:${copy.id}`);
  assert.equal(removed.state, 'withdrawn');
  assert.equal(removed.bytes, 0);
  assert.equal(removed.withdrawal.actor, 'test-account');
  assert.equal(removed.withdrawal.idempotencyKey, withdrawal.idempotencyKey);
  assert.equal(await f.env.BUCKET.head(`design-revocations/${copy.cert.canonicalSha}.json`), null);
  assert.equal((await f.request('POST', '/v1/design-revocations', { sha: copy.cert.canonicalSha, reason: 'Ordinary author', idempotencyKey: 'blocked-global' })).status, 403);
  assert.equal((await f.request('POST', '/v1/design-revocations', { sha: copy.cert.canonicalSha, reason: 'Explicit global owner action', idempotencyKey: 'owner-global' }, 'owner')).status, 200);
  assert.ok([403, 404].includes((await f.request('GET', publicPath, undefined, null)).status));
});

test('flagged visited frames and zero-inference uncertainty persist immutably and cannot approve', async t => {
  const f = await fixture(t);
  for (const verdict of ['flagged', 'uncertain']) {
    const job = await f.inspected(), { base, review } = await mikaClaim(f, job, { verdict, coverage: { frames: verdict === 'flagged' ? [0] : [], totalFrames: 1, durationMs: 0, composition: false } });
    const result = await f.request('POST', `${base}/content-review`, review, f.env.MIKA_REVIEW_TOKEN);
    assert.equal(result.status, 200, await result.clone().text());
    const saved = await result.json();
    assert.equal((await f.request('POST', `${base}/content-review`, review, f.env.MIKA_REVIEW_TOKEN)).status, 200);
    assert.equal((await f.request('POST', `${base}/content-review`, { ...review, verdict: 'clear' }, f.env.MIKA_REVIEW_TOKEN)).status, 409);
    assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Must remain held', idempotencyKey: 'blocked' }, f.env.MIKA_REVIEW_TOKEN)).status, 409);
    assert.deepEqual((await f.storage.get(`job:${job.id}`)).contentReviews, [saved]);
  }
});

test('Mika v2 composed evidence finishes a restricted signature without relabeling technical moderation', async t => {
  const f = await fixture(t), job = await f.inspected('unreviewed', true), { base, review } = await mikaClaim(f, job);
  f.env.DESIGN_APPROVAL_KEYS = JSON.stringify({ mika: { publicKey: base64(f.signer.raw), revoked: false, role: 'mika' } });
  assert.equal((await f.request('POST', `${base}/content-review`, review, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Exact source and composed evidence reviewed', idempotencyKey: 'v2-approve' }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  const stored = await f.storage.get(`job:${job.id}`);
  const signed = await envelope(f.signer, 'ryoku-design-approval-v3', { ...stored.approvalPayload, keyId: 'mika' });
  assert.equal((await f.request('POST', `${base}/signature`, { approval: signed }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}`, undefined, null)).status, 200);
  assert.equal((await f.storage.get(`job:${job.id}`)).inspection.moderation, 'unreviewed');
  const original = globalThis.fetch;
  let entered, release;
  const waiting = new Promise(resolve => { entered = resolve; });
  const answer = new Promise(resolve => { release = resolve; });
  t.mock.method(globalThis, 'fetch', (url, options) => {
    if (new URL(url).pathname.endsWith('/eligibility')) { entered(); return answer; }
    return original(url, options);
  });
  const pendingRead = f.request('GET', `/v1/published-designs/${job.id}`, undefined, null);
  await waiting;
  assert.equal((await f.request('POST', '/internal/inspection/claim', {}, f.env.INSPECTOR_WORKER_TOKEN)).status, 204);
  await f.queue.alarm();
  assert.equal((await f.request('POST', `/v1/submissions/${job.id}/withdraw`, { reason: 'Withdraw exact bytes', idempotencyKey: 'v2-withdraw' })).status, 200);
  release(Response.json({ accountId: 'test-account', cardId: f.id, active: true, reason: 'creator' }));
  assert.equal((await pendingRead).status, 404);
  const clock = Date.now();
  t.mock.method(Date, 'now', () => clock + 86401000);
  await f.queue.alarm();
  assert.equal(await f.storage.get(`job:${job.id}`), undefined);
  assert.equal((await f.storage.get('index')).bytes, 0);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}`, undefined, null)).status, 404);
});

test('Mika legacy or composition-omitting clear cannot enter or cross any publication transition', async t => {
  const f = await fixture(t);
  for (const variant of ['legacy', 'omitted', 'binding']) {
    const job = await f.inspected('unreviewed', true), { base, review } = await mikaClaim(f, job);
    const unsafe = structuredClone(review);
    if (variant === 'legacy') unsafe.reviewVersion = 'mika-local-card-review-v1';
    if (variant === 'omitted') delete unsafe.coverage.composition;
    if (variant === 'binding') {
      const report = JSON.parse(new TextDecoder().decode(job.objects.report));
      report.composition.posterSha = '00'.repeat(32);
      const raw = new TextEncoder().encode(JSON.stringify(report)), stored = await f.storage.get(`job:${job.id}`);
      await f.env.BUCKET.put(stored.outputs.report.key, raw);
      // Recertify a technically signed report whose composed-object binding is wrong.
      stored.outputs.report.sha = await digest(raw); stored.outputs.report.bytes = raw.length;
      job.cert.reportSha = stored.outputs.report.sha;
      const descriptor = job.cert.files.find(file => file.name === 'report');
      descriptor.sha = stored.outputs.report.sha; descriptor.bytes = raw.length;
      stored.certificate = await envelope(f.inspector, 'ryoku-inspect-v1', job.cert);
      stored.certificateSha = await digest(encoded(job.cert)); stored.inspection = job.cert;
      await f.storage.put(`job:${job.id}`, stored);
      unsafe.reportSha = stored.outputs.report.sha; unsafe.certificateSha = stored.certificateSha;
    }
    const refused = await f.request('POST', `${base}/content-review`, unsafe, f.env.MIKA_REVIEW_TOKEN);
    assert.ok([400, 403].includes(refused.status));
    let stored = await f.storage.get(`job:${job.id}`);
    const attestation = { v: 1, jobId: job.id, actor: 'service:mika', role: 'mika', at: now(), submitted: unsafe };
    const record = { ...attestation, sha: await digest(encoded(attestation)), idempotencyKey: unsafe.idempotencyKey };
    // Emulate a genuinely retained pre-hardening immutable attestation, not a
    // broken signature: all subsequent authority boundaries must recheck policy.
    stored.contentReviews = [record]; await f.storage.put(`job:${job.id}`, stored);
    assert.ok([400, 403].includes((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Legacy must fail', idempotencyKey: 'legacy' }, f.env.MIKA_REVIEW_TOKEN)).status));
    stored = await f.storage.get(`job:${job.id}`);
    stored.state = 'awaiting_signature'; stored.decision = { id: 'legacy-decision', actor: 'service:mika', role: 'mika', verdict: 'approve', at: now() };
    stored.approvalPayload = { v: 3, jobId: job.id, accountId: stored.accountId, cardId: stored.manifest.cardId, canonicalSha: job.cert.canonicalSha, posterSha: job.cert.posterSha, previewSha: job.cert.previewSha, certificateSha: stored.certificateSha, contentReviewSha: record.sha, decisionId: 'legacy-decision', reviewerRole: 'mika', approvedAt: now() };
    f.env.DESIGN_APPROVAL_KEYS = JSON.stringify({ mika: { publicKey: base64(f.signer.raw), revoked: false, role: 'mika' } });
    const signed = await envelope(f.signer, 'ryoku-design-approval-v3', { ...stored.approvalPayload, keyId: 'mika' });
    await f.storage.put(`job:${job.id}`, stored);
    assert.ok([400, 403].includes((await f.request('POST', `${base}/signature`, { approval: signed }, f.env.MIKA_REVIEW_TOKEN)).status));
    assert.ok([400, 403].includes((await f.request('POST', `${base}/signature`, {}, 'owner')).status));
    stored.state = 'approved'; stored.approval = signed; await f.storage.put(`job:${job.id}`, stored);
    assert.ok([400, 403].includes((await f.request('GET', `/v1/published-designs/${job.id}`, undefined, null)).status));
    // Release a held slot without discarding the immutable test evidence.
    stored.state = 'withdrawn'; stored.terminalAt = now(); await f.storage.put(`job:${job.id}`, stored);
    const index = await f.storage.get('index'); delete index.active[job.id]; await f.storage.put('index', index);
  }
});

test('Mika may append non-clear escalation after its own clear but never clear an existing hold', async t => {
  const f = await fixture(t), job = await f.inspected('unreviewed', true), { base, review } = await mikaClaim(f, job);
  assert.equal((await f.request('POST', `${base}/content-review`, review, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Original clear receipt', idempotencyKey: 'original-clear' }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  const before = await f.storage.get(`job:${job.id}`);
  f.env.DESIGN_APPROVAL_KEYS = JSON.stringify({ mika: { publicKey: base64(f.signer.raw), revoked: false, role: 'mika' }, owner: { publicKey: base64(f.signer.raw), revoked: false, role: 'owner' } });
  const oldApproval = await envelope(f.signer, 'ryoku-design-approval-v3', { ...before.approvalPayload, keyId: 'mika' });
  const escalation = { ...review, verdict: 'uncertain', idempotencyKey: 'resume-escalation', coverage: { ...review.coverage, frames: [], composition: false } };
  assert.equal((await f.request('POST', `${base}/content-review`, escalation, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('POST', `${base}/signature`, { approval: oldApproval }, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  assert.equal((await f.request('POST', `${base}/content-review`, { ...review, idempotencyKey: 'cannot-clear-hold' }, f.env.MIKA_REVIEW_TOKEN)).status, 409);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Held', idempotencyKey: 'held' }, f.env.MIKA_REVIEW_TOKEN)).status, 409);
  let held = await f.storage.get(`job:${job.id}`);
  assert.deepEqual(held.decisionHistory, [{ decision: before.decision, approvalPayload: before.approvalPayload }]);
  assert.equal((await f.request('POST', `${base}/claim`, {}, 'owner')).status, 200);
  const manual = { ...review, model: 'manual', idempotencyKey: 'owner-manual', reviewVersion: 'manual-v1' };
  assert.equal((await f.request('POST', `${base}/content-review`, manual, 'owner')).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Independent owner review', idempotencyKey: 'owner-new' }, 'owner')).status, 200);
  assert.equal((await f.storage.get(`job:${job.id}`)).contentReviews.length, 3);
});

test('rejected and changes-requested originals and outputs are deleted while bounded audit remains', async t => {
  const f = await fixture(t);
  for (const verdict of ['reject', 'changes']) {
    const job = await f.inspected(), base = `/v1/review/submissions/${job.id}`;
    assert.equal((await f.request('POST', `${base}/claim`, {}, 'owner')).status, 200);
    assert.equal((await f.request('POST', `${base}/decision`, { verdict, reason: 'Terminal review', idempotencyKey: verdict }, 'owner')).status, 200);
    const stored = await f.storage.get(`job:${job.id}`);
    assert.equal(stored.bytes, 0);
    assert.ok(stored.audit.some(entry => entry.event === 'review_decision'));
    assert.ok(stored.terminalAt);
    assert.equal(await f.env.BUCKET.head(stored.inputKey), null);
    for (const output of Object.values(stored.outputs)) assert.equal(await f.env.BUCKET.head(output.key), null);
  }
  assert.equal((await f.storage.get('index')).bytes, 0);
});

async function publishedFixture(f) {
  const job = await f.inspected('unreviewed', true), { base, review } = await mikaClaim(f, job);
  f.env.DESIGN_APPROVAL_KEYS = JSON.stringify({ mika: { publicKey: base64(f.signer.raw), revoked: false, role: 'mika' } });
  assert.equal((await f.request('POST', `${base}/content-review`, review, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Exact fully reviewed fixture', idempotencyKey: 'publish' }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  const saved = await f.storage.get(`job:${job.id}`);
  const signed = await envelope(f.signer, 'ryoku-design-approval-v3', { ...saved.approvalPayload, keyId: 'mika' });
  assert.equal((await f.request('POST', `${base}/signature`, { approval: signed }, f.env.MIKA_REVIEW_TOKEN)).status, 200);
  return job;
}

test('reports enter private owner triage without takedown or Mika authority', async t => {
  const f = await fixture(t), job = await publishedFixture(f);
  const input = { cardId: f.id, publicationId: job.id, reason: 'Please inspect this exact hosted animation', idempotencyKey: 'report-one' };
  const created = await f.request('POST', '/v1/card-reports', input, 'other');
  assert.equal(created.status, 201, await created.clone().text());
  const item = await created.json();
  assert.equal((await f.request('POST', '/v1/card-reports', input, 'other')).status, 200);
  assert.equal((await f.request('POST', '/v1/card-reports', { ...input, reason: 'Changed payload' }, 'other')).status, 409);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}/canonical`, undefined, null)).status, 200);
  assert.deepEqual((await (await f.request('GET', '/v1/card-safety')).json()).items, []);
  assert.equal((await f.request('GET', '/v1/card-safety', undefined, 'other')).status, 200);
  assert.equal((await f.request('GET', '/v1/review/safety', undefined, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  const decision = { action: 'withdraw', reason: 'Owner confirmed the exact report', idempotencyKey: 'withdraw-report' };
  assert.equal((await f.request('POST', `/v1/review/safety/${item.id}/decision`, decision, f.env.MIKA_REVIEW_TOKEN)).status, 403);
  assert.equal((await f.request('POST', `/v1/review/safety/${item.id}/decision`, decision, 'owner')).status, 200);
  assert.equal((await f.request('POST', `/v1/review/safety/${item.id}/decision`, decision, 'owner')).status, 200);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}/canonical`, undefined, null)).status, 404);
  assert.equal((await (await f.request('GET', '/v1/card-safety', undefined, 'other')).json()).items[0].state, 'resolved');
});

test('an appeal can invite only a new inspected submission, never restore failed output', async t => {
  const f = await fixture(t), job = await f.inspected();
  const base = `/v1/review/submissions/${job.id}`;
  assert.equal((await f.request('POST', `${base}/claim`, {}, 'owner')).status, 200);
  assert.equal((await f.request('POST', `${base}/decision`, { verdict: 'changes', reason: 'Change the submitted artwork', idempotencyKey: 'changes' }, 'owner')).status, 200);
  const input = { reason: 'I can provide a corrected original ZIP', idempotencyKey: 'appeal' };
  assert.equal((await f.request('POST', `/v1/submissions/${job.id}/appeals`, input, 'other')).status, 403);
  const response = await f.request('POST', `/v1/submissions/${job.id}/appeals`, input);
  assert.equal(response.status, 201, await response.clone().text());
  const item = await response.json();
  const decision = { action: 'resubmit', reason: 'Submit a fresh ZIP through all ordinary checks', idempotencyKey: 'allow-new' };
  assert.equal((await f.request('POST', `/v1/review/safety/${item.id}/decision`, decision, 'owner')).status, 200);
  assert.equal((await f.storage.get(`job:${job.id}`)).state, 'changes_requested');
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}`, undefined, null)).status, 404);
  assert.equal((await f.request('POST', `${base}/signature`, {}, 'owner')).status, 409);
  assert.equal((await f.reserve()).state, 'uploading');
});

test('card publishing pauses preserve reads and stale unblock replay cannot lift a later pause', async t => {
  const f = await fixture(t), job = await publishedFixture(f);
  const pause = { accountId: 'test-account', blocked: true, reason: 'Pause new card submissions', idempotencyKey: 'pause-one' };
  assert.equal((await f.request('POST', '/v1/review/card-restrictions', pause)).status, 403);
  assert.equal((await f.request('POST', '/v1/review/card-restrictions', pause, 'owner')).status, 200);
  const candidate = await f.manifest();
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: candidate.envelope })).status, 403);
  assert.equal((await f.request('GET', `/v1/submissions/${job.id}`)).status, 200);
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}`, undefined, null)).status, 200);
  const lift = { ...pause, blocked: false, reason: 'Lift the card-only pause', idempotencyKey: 'lift' };
  assert.equal((await f.request('POST', '/v1/review/card-restrictions', lift, 'owner')).status, 200);
  assert.equal((await f.request('POST', '/v1/review/card-restrictions', { ...pause, idempotencyKey: 'pause-two' }, 'owner')).status, 200);
  const replay = await f.request('POST', '/v1/review/card-restrictions', lift, 'owner');
  assert.equal((await replay.json()).blocked, true);
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: candidate.envelope })).status, 403);
});

test('Account purge is audience-bound, resumable and cannot erase another tenant or later explicit still sharing', async t => {
  const f = await fixture(t), ids = [];
  for (let i = 0; i < 33; i++) {
    const id = crypto.randomUUID(); ids.push(id);
    await f.storage.put(`job:${id}`, { id, accountId: 'test-account', state: 'approved', createdAt: now() - Math.floor(i / 8) * 86400, updatedAt: now(), decision: { at: now() }, manifest: { cardId: f.id, kind: 'hero' }, outputs: {}, bytes: 0, audit: [] });
  }
  const other = crypto.randomUUID(); ids.push(other);
  await f.storage.put(`job:${other}`, { id: other, accountId: 'other-account', state: 'approved', createdAt: now(), updatedAt: now(), decision: { at: now() }, manifest: { cardId: f.id, kind: 'hero' }, outputs: {}, bytes: 0, audit: [] });
  await f.storage.put('index', { ids, active: {}, bytes: 0, nonces: {}, admissions: {} });
  const input = { accountId: 'test-account', cardIds: [f.id], idempotencyKey: 'permanent-close' }, path = '/internal/inspection/accounts/close';
  assert.equal((await f.request('POST', path, input, f.env.INSPECTOR_WORKER_TOKEN)).status, 401);
  assert.equal((await f.request('POST', path, input, f.env.CARD_GATE_TOKEN)).status, 403);
  const headers = { 'X-Ryoku-Audience': f.env.CARD_GATE_AUDIENCE };
  const oldStill = await deviceRecord(f.device, { updated: now() - 10 });
  assert.equal((await call(f.env, 'PUT', `/v1/cards/${f.id}`, JSON.stringify({ record: oldStill }), { 'Content-Type': 'application/json' })).status, 200);
  const beforePurge = Date.now();
  const first = await f.request('POST', path, input, f.env.CARD_GATE_TOKEN, headers);
  assert.equal(first.status, 202, await first.clone().text());
  assert.equal((await first.json()).closed, false);
  t.mock.method(Date, 'now', () => beforePurge + 1000);
  const replacement = await deviceRecord(f.device, { updated: now() + 1, body: base64(cardBody(f.id, ['new sharing', '', '', '', ''])) });
  assert.equal((await call(f.env, 'PUT', `/v1/cards/${f.id}`, JSON.stringify({ record: replacement }), { 'Content-Type': 'application/json' })).status, 200);
  const second = await f.request('POST', path, input, f.env.CARD_GATE_TOKEN, headers);
  assert.equal(second.status, 200, await second.clone().text());
  assert.equal((await second.json()).purged, true);
  const surviving = await call(f.env, 'GET', `/v1/cards/${f.id}`);
  assert.equal(surviving.status, 200);
  assert.equal((await surviving.json()).body, replacement.body);
  assert.equal((await f.storage.get(`job:${other}`)).state, 'approved');
  const still = { id: f.id, updated: now() + 1, body: 'explicit later device-only sharing' };
  await f.env.BUCKET.put(`cards/${f.id}.json`, JSON.stringify(still));
  assert.equal((await f.request('POST', path, input, f.env.CARD_GATE_TOKEN, headers)).status, 200);
  assert.deepEqual(JSON.parse(await (await f.env.BUCKET.get(`cards/${f.id}.json`)).text()), still);
  const candidate = await f.manifest();
  assert.equal((await f.request('POST', '/v1/submissions', { manifest: candidate.envelope })).status, 403);
});

test('queued and unpublished media expire without deleting unrelated stored objects', async t => {
  const f = await fixture(t), job = await f.inspected();
  await f.env.BUCKET.put('unrelated/keep.txt', 'not an inspection object');
  const before = Date.now();
  t.mock.method(Date, 'now', () => before + 7 * 86400_000 + 1000);
  await f.queue.alarm();
  assert.equal((await f.storage.get(`job:${job.id}`)).state, 'expired');
  assert.equal(await f.env.BUCKET.get(`private-inspection/${job.id}/original.zip`), null);
  for (const object of Object.values((await f.storage.get(`job:${job.id}`)).outputs)) assert.equal(await f.env.BUCKET.get(object.key), null);
  assert.equal((await f.storage.get('index')).bytes, 0);
  assert.equal(await (await f.env.BUCKET.get('unrelated/keep.txt')).text(), 'not an inspection object');
});

test('approved hosted media expires after ninety days and an old signature cannot restore it', async t => {
  const f = await fixture(t), job = await publishedFixture(f), saved = await f.storage.get(`job:${job.id}`);
  const before = Date.now();
  t.mock.method(Date, 'now', () => before + 90 * 86400_000 + 1000);
  await f.queue.alarm();
  assert.equal((await f.storage.get(`job:${job.id}`)).state, 'expired');
  assert.equal((await f.request('GET', `/v1/published-designs/${job.id}/canonical`, undefined, null)).status, 404);
  assert.equal((await f.request('POST', `/v1/review/submissions/${job.id}/signature`, { approval: saved.approval }, 'owner')).status, 409);
  assert.equal((await f.storage.get('index')).bytes, 0);
});

test('static safety identity binds every signed content field but not transport freshness', async t => {
  const f = await fixture(t);
  const original = await wordedRecord(f.device, { device: base64(f.device.raw), style: undefined, tagline: 'Original words', standName: 'Original title', picture: '12'.repeat(32) });
  const first = await cardContentSHA(original);
  for (const fields of [{ tagline: 'Changed words' }, { standName: 'Changed title' }, { style: 'stand' }, { body: base64(cardBody(f.id, ['different GPU', '', '', '', ''])) }, { picture: '34'.repeat(32) }]) {
    assert.notEqual(await cardContentSHA({ ...original, ...fields }), first);
  }
  const refreshed = await wordedRecord(f.device, { device: original.device, style: undefined, tagline: original.tagline, standName: original.standName, picture: original.picture, updated: original.updated + 1 });
  assert.equal(await cardContentSHA(refreshed), first);
});

test('withdrawn unchanged static cards are gone in binder batches, not silently current', async t => {
  const f = await fixture(t), record = await deviceRecord(f.device);
  assert.equal((await call(f.env, 'PUT', `/v1/cards/${f.id}`, JSON.stringify({ record }), { 'Content-Type': 'application/json' })).status, 200);
  const response = await f.request('POST', '/v1/card-reports', { cardId: f.id, reason: 'Review exact static content', idempotencyKey: 'static-report' }, 'other');
  assert.equal(response.status, 201);
  const item = await response.json();
  assert.equal((await f.request('POST', `/v1/review/safety/${item.id}/decision`, { action: 'withdraw', reason: 'Exact static content withdrawn', idempotencyKey: 'static-withdraw' }, 'owner')).status, 200);
  const batch = await call(f.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards: { [f.id]: record.updated } }), { 'Content-Type': 'application/json' });
  assert.equal(batch.status, 200);
  const pulled = await batch.json();
  assert.deepEqual(pulled.gone, [f.id]);
  assert.deepEqual(pulled.records, []);
});

test('Account purge CAS cannot delete a device publication racing its selected generation', async t => {
  const f = await fixture(t), original = await deviceRecord(f.device, { updated: now() - 10 });
  const path = `/v1/cards/${f.id}`, headers = { 'Content-Type': 'application/json' };
  assert.equal((await call(f.env, 'PUT', path, JSON.stringify({ record: original }), headers)).status, 200);
  const replacement = await deviceRecord(f.device, { updated: now() + 1, body: base64(cardBody(f.id, ['racing share', '', '', '', ''])) });
  const put = f.env.BUCKET.put.bind(f.env.BUCKET);
  let armed = true;
  t.mock.method(f.env.BUCKET, 'put', async (key, value, options = {}) => {
    if (armed && key === `cards/${f.id}.json` && typeof value === 'string' && JSON.parse(value).deleted === true) {
      armed = false;
      assert.equal((await call(f.env, 'PUT', path, JSON.stringify({ record: replacement }), headers)).status, 200);
    }
    return put(key, value, options);
  });
  const response = await f.request('POST', '/internal/inspection/accounts/close', { accountId: 'test-account', cardIds: [f.id], idempotencyKey: 'racing-close' }, f.env.CARD_GATE_TOKEN, { 'X-Ryoku-Audience': f.env.CARD_GATE_AUDIENCE });
  assert.equal(response.status, 200);
  const live = await call(f.env, 'GET', path);
  assert.equal(live.status, 200);
  assert.equal((await live.json()).body, replacement.body);
});

test('a late snapshot batch cannot claim independently published post-closure content', async t => {
  const f = await fixture(t);
  const ids = [...Array.from({ length: 32 }, (_, i) => (i + 1).toString(16).padStart(16, '0')), f.id];
  const input = { accountId: 'test-account', cardIds: ids, idempotencyKey: 'late-snapshot' };
  const headers = { 'X-Ryoku-Audience': f.env.CARD_GATE_AUDIENCE }, path = '/internal/inspection/accounts/close';
  const started = Date.now();
  assert.equal((await f.request('POST', path, input, f.env.CARD_GATE_TOKEN, headers)).status, 202);
  t.mock.method(Date, 'now', () => started + 1000);
  const record = await deviceRecord(f.device);
  assert.equal((await call(f.env, 'PUT', `/v1/cards/${f.id}`, JSON.stringify({ record }), { 'Content-Type': 'application/json' })).status, 200);
  let response;
  for (let i = 0; i < 4; i++) {
    response = await f.request('POST', path, input, f.env.CARD_GATE_TOKEN, headers);
    if (response.status !== 202) break;
  }
  assert.equal(response.status, 200);
  const live = await call(f.env, 'GET', `/v1/cards/${f.id}`);
  assert.equal(live.status, 200);
  assert.equal((await live.json()).sig, record.sig);
});

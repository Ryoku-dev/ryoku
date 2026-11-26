import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createPrivateKey, createPublicKey, sign as edSign } from 'node:crypto';
import worker from '../src/index.js';
import { InspectionQueue } from '../src/inspection-queue.js';
import { animatedPayload, verifyAnimatedRecord } from '../src/animated-record.js';
import { encoded } from '../src/inspection-protocol.js';
import { base64, digest, hex, message, unbase64, verifyCode } from '../src/codec.js';
import { setup, keys, now, sign, cardBody, code, deviceRecord } from './helpers.js';

class Storage {
  data = new Map();
  async get(key) { return structuredClone(this.data.get(key)); }
  async put(key, value) { this.data.set(key, structuredClone(value)); }
  async transaction(callback) {
    const saved = structuredClone(this.data);
    try { return await callback(this); } catch (error) { this.data = saved; throw error; }
  }
  async setAlarm(at) { this.alarmAt = at; }
}
async function envelope(key, domain, payload) {
  const raw = encoded(payload);
  return { payload: base64(raw), sig: base64(await sign(key, domain, raw)) };
}
async function fixture(t) {
  const f = await setup(), inspector = await keys(), signer = await keys(), storage = new Storage();
  Object.assign(f.env, { ACCOUNTS_URL: 'https://accounts.invalid', CARD_GATE_TOKEN: 'a'.repeat(40), CARD_GATE_AUDIENCE: 'ryoku-card-gate-v1', SUBMISSION_AUDIENCE: 'https://team.ryoku.dev', INSPECTOR_WORKER_TOKEN: 'w'.repeat(40), MIKA_REVIEW_TOKEN: 'm'.repeat(40), INSPECTOR_KEYS: JSON.stringify({ scanner: { publicKey: base64(inspector.raw), revoked: false } }), DESIGN_APPROVAL_KEYS: JSON.stringify({ owner: { publicKey: base64(signer.raw), revoked: false, role: 'owner' } }), INSPECTOR_PIPELINE_VERSION: 'test-pipeline-v1', INSPECTOR_POLICY_VERSION: 'test-policy-v1' });
  const identity = { accountId: 'account-one', cardId: f.id, device: base64(f.device.raw), termsVersion: 'terms-v1', attestationVersion: 'attest-v1', eligible: true };
  const eligibility = { accountId: identity.accountId, cardId: f.id, active: true, until: Date.now() + 86400000, reason: 'creator' };
  const authority = { bound: true, consent: true };
  t.mock.method(globalThis, 'fetch', async (url, options) => {
    assert.equal(new URL(url).origin, 'https://accounts.invalid');
    assert.equal(options.headers.Authorization, `Bearer ${f.env.CARD_GATE_TOKEN}`);
    assert.equal(options.headers['X-Ryoku-Audience'], f.env.CARD_GATE_AUDIENCE);
    const input = JSON.parse(options.body);
    if (new URL(url).pathname === '/internal/cards/eligibility') {
      assert.deepEqual(input, { cardId: f.id, action: 'animated-share' });
      if (!authority.bound) return Response.json({ error: 'Unbound' }, { status: 403 });
      return Response.json(authority.consent ? eligibility : { ...eligibility, active: false, reason: 'policy_required' });
    }
    assert.equal(new URL(url).pathname, '/internal/cards/authorize');
    if (input.token === 'owner' && input.action === 'review') return Response.json({ accountId: 'owner-account', reviewer: 'owner' });
    if (input.token !== 'user' || !authority.bound || (input.action === 'submit' && !authority.consent)) return Response.json({ error: 'Denied' }, { status: 403 });
    return Response.json(identity);
  });
  const queue = new InspectionQueue({ storage }, f.env);
  f.env.INSPECTION_QUEUE = { idFromName(name) { assert.equal(name, 'private-cards-v1'); return name; }, get() { return queue; } };
  async function request(method, path, value, auth = null, headers = {}) {
    const raw = value === undefined ? undefined : value instanceof Uint8Array ? value : new TextEncoder().encode(JSON.stringify(value));
    return worker.fetch(new Request(`https://team.ryoku.dev${path}`, { method, ...(raw ? { body: raw } : {}), headers: { ...(raw ? { 'Content-Type': 'application/json', 'Content-Length': String(raw.length) } : {}), ...(auth ? { Authorization: `Bearer ${auth}` } : {}), ...headers } }), f.env);
  }
  const archive = new Uint8Array([80, 75, 3, 4]);
  const manifest = { v: 1, cardId: f.id, device: base64(f.device.raw), originalSha: await digest(archive), files: [{ name: 'design.json', sha: '01'.repeat(32), bytes: 10 }], termsVersion: 'terms-v1', attestationVersion: 'attest-v1', ownOrLicensed: true, policyAccepted: true, consentToReview: true, nonce: crypto.randomUUID().replaceAll('-', ''), submittedAt: now(), audience: f.env.SUBMISSION_AUDIENCE, kind: 'design' };
  const reserved = await request('POST', '/v1/submissions', { manifest: await envelope(f.device, 'ryoku-submit-v1', manifest) }, 'user');
  assert.equal(reserved.status, 201, await reserved.clone().text());
  const job = await reserved.json();
  assert.equal((await request('PUT', job.uploadPath, archive, 'user', { 'Content-Type': 'application/zip' })).status, 202);
  const claim = await (await request('POST', '/internal/inspection/claim', {}, f.env.INSPECTOR_WORKER_TOKEN)).json();
  const objects = { canonical: new Uint8Array([80, 75, 3, 4, 5]), poster: unbase64('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRusAAAAASUVORK5CYII='), preview: new Uint8Array([71, 73, 70]), report: new TextEncoder().encode(JSON.stringify({ v: 1, totalFrames: 1, durationMs: 0 })) };
  const types = { canonical: 'application/zip', poster: 'image/png', preview: 'image/gif', report: 'application/json' }, files = [];
  for (const name of Object.keys(objects).sort()) {
    assert.equal((await request('PUT', `/internal/inspection/${job.id}/objects/${name}`, objects[name], f.env.INSPECTOR_WORKER_TOKEN, { 'Content-Type': types[name], 'X-Ryoku-Lease': claim.lease })).status, 200);
    files.push({ name, sha: await digest(objects[name]), bytes: objects[name].length });
  }
  const cert = { v: 1, keyId: 'scanner', jobId: job.id, leaseSha: await digest(new TextEncoder().encode(claim.lease)), manifestSha: await digest(encoded(manifest)), originalSha: manifest.originalSha, canonicalSha: files.find(f => f.name === 'canonical').sha, files, posterSha: files.find(f => f.name === 'poster').sha, previewSha: files.find(f => f.name === 'preview').sha, reportSha: files.find(f => f.name === 'report').sha, pipelineVersion: f.env.INSPECTOR_PIPELINE_VERSION, policyVersion: f.env.INSPECTOR_POLICY_VERSION, inspectedAt: now(), technical: { archive: 'pass', decode: 'pass', canonical: 'pass', render: 'pass' }, moderation: 'unreviewed' };
  const certificate = await envelope(inspector, 'ryoku-inspect-v1', cert);
  assert.equal((await request('POST', `/internal/inspection/${job.id}/result`, { lease: claim.lease, certificate }, f.env.INSPECTOR_WORKER_TOKEN)).status, 200);
  const base = `/v1/review/submissions/${job.id}`;
  assert.equal((await request('POST', `${base}/claim`, {}, 'owner')).status, 200);
  assert.equal((await request('POST', `${base}/content-review`, { verdict: 'clear', reason: 'All canonical frames inspected', idempotencyKey: 'content-one', model: 'manual', coverage: { frames: [0], totalFrames: 1, durationMs: 0 }, policyVersion: cert.policyVersion, reviewVersion: 'manual-v1', canonicalSha: cert.canonicalSha, previewSha: cert.previewSha, reportSha: cert.reportSha, certificateSha: await digest(encoded(cert)) }, 'owner')).status, 200);
  assert.equal((await request('POST', `${base}/decision`, { verdict: 'approve', reason: 'Approved exact canonical output', idempotencyKey: 'decision-one' }, 'owner')).status, 200);
  const stored = await storage.get(`job:${job.id}`);
  const approval = await envelope(signer, 'ryoku-design-approval-v3', { ...stored.approvalPayload, keyId: 'owner' });
  assert.equal((await request('POST', `${base}/signature`, { approval }, 'owner')).status, 200);
  async function signedRecord(fields = {}) {
    const record = { v: 5, id: f.id, updated: now(), body: base64(cardBody(f.id)), picture: cert.posterSha, sig: '', device: base64(f.device.raw), publication: job.id, canonicalSha: cert.canonicalSha, approvalSha: await digest(unbase64(approval.payload)), certificateSha: await digest(unbase64(certificate.payload)), ...fields };
    record.sig = base64(await sign(f.device, 'ryoku-card-v5', animatedPayload(record)));
    return record;
  }
  const record = await signedRecord();
  const put = (recordValue = record, auth = 'user', extra = {}) => request('PUT', `/v1/cards/${f.id}`, { record: recordValue, ...extra }, auth);
  const batch = (updated = 0) => request('POST', '/v1/cards/batch', { cards: { [f.id]: updated } });
  return { ...f, queue, storage, inspector, signer, identity, eligibility, authority, request, record, job, objects, cert, certificate, approval, signedRecord, put, batch };
}

test('shared Go/JS v5 vector covers exact binary frame and every authority field', async () => {
  const vector = JSON.parse(await readFile(new URL('./fixtures/card-v5.json', import.meta.url), 'utf8'));
  assert.equal(hex(animatedPayload(vector.record)), vector.payloadHex);
  assert.equal(hex(message(vector.domain, animatedPayload(vector.record))), vector.signingInputHex);
  assert.equal(vector.record.device, vector.publicKey);
  assert.equal(await verifyAnimatedRecord(vector.record), true);
  const key = createPrivateKey({ key: Buffer.concat([Buffer.from('302e020100300506032b657004220420', 'hex'), Buffer.from(vector.seed, 'base64')]), format: 'der', type: 'pkcs8' });
  assert.equal(createPublicKey(key).export({ format: 'der', type: 'spki' }).subarray(-32).toString('base64'), vector.publicKey);
  assert.equal(edSign(null, Buffer.from(vector.signingInputHex, 'hex'), key).toString('base64'), vector.record.sig);
  const wrongDomain = edSign(null, message('ryoku-card-v4', animatedPayload(vector.record)), key).toString('base64');
  assert.equal(await verifyAnimatedRecord({ ...vector.record, sig: wrongDomain }), false);
  for (const [field, value] of Object.entries({ id: '00'.repeat(8), updated: vector.record.updated + 1, body: vector.record.body.slice(0, -4) + 'AAAA', picture: '55'.repeat(32), device: base64(new Uint8Array(32)), publication: '11234567-89ab-4cde-8fab-0123456789ab', canonicalSha: '55'.repeat(32), approvalSha: '55'.repeat(32), certificateSha: '55'.repeat(32), sig: base64(new Uint8Array(64)) })) {
    let accepted = false;
    try { accepted = await verifyAnimatedRecord({ ...vector.record, [field]: value }); } catch { /* invalid framing is also refused */ }
    assert.equal(accepted, false, field);
  }
  for (const field of ['style', 'tagline', 'standName', 'accountId']) assert.throws(() => animatedPayload({ ...vector.record, [field]: 'extra' }));
  for (const field of Object.keys(vector.record)) { const record = { ...vector.record }; delete record[field]; assert.throws(() => animatedPayload(record), field); }
  assert.throws(() => animatedPayload({ ...vector.record, picture: '' }));
  assert.throws(() => animatedPayload({ ...vector.record, publication: 'x'.repeat(36) }));
  assert.throws(() => animatedPayload({ ...vector.record, canonicalSha: 'AA'.repeat(32) }));
});

test('genuine approved publication supports PUT, GET, batch, exact canonical and PNG without legacy aliases', async t => {
  const f = await fixture(t);
  assert.equal((await f.put()).status, 200);
  const response = await f.request('GET', `/v1/cards/${f.id}`);
  assert.equal(response.status, 200); assert.equal(response.headers.get('Cache-Control'), 'no-store');
  assert.deepEqual(await response.json(), f.record);
  assert.deepEqual((await (await f.batch()).json()).records, [f.record]);
  assert.deepEqual((await (await f.batch(f.record.updated)).json()).gone, []);
  for (const name of ['canonical', 'poster']) {
    const asset = await f.request('GET', `/v1/published-designs/${f.job.id}/${name}`);
    assert.equal(asset.status, 200); assert.match(asset.headers.get('Cache-Control'), /no-store/);
    assert.equal(asset.headers.get('Content-Type'), name === 'poster' ? 'image/png' : 'application/zip');
    assert.deepEqual(new Uint8Array(await asset.arrayBuffer()), f.objects[name]);
  }
  assert.deepEqual(new Uint8Array(await (await f.request('GET', `/v1/cards/${f.id}/${f.record.picture}.png`)).arrayBuffer()), f.objects.poster);
  assert.equal((await f.request('GET', `/v1/cards/${f.id}/${f.record.picture}.jpg`)).status, 404);
  await f.env.BUCKET.put(`packs/${f.record.canonicalSha}.zip`, f.objects.canonical);
  assert.equal((await f.request('GET', `/v1/packs/${f.record.canonicalSha}.zip`)).status, 404);
  assert.equal([...f.env.BUCKET.data.keys()].some(key => key.startsWith(`cards/${f.id}/`)), false);
});

test('PUT requires real Account bearer and exact device/account/publication/hash bindings', async t => {
  const f = await fixture(t);
  assert.equal((await f.put(f.record, null)).status, 401);
  assert.equal((await f.put(f.record, 'invalid')).status, 403);
  assert.equal((await f.put({ ...f.record, updated: f.record.updated + 1 })).status, 403);
  for (const field of ['picture', 'canonicalSha', 'approvalSha', 'certificateSha']) assert.equal((await f.put(await f.signedRecord({ [field]: 'aa'.repeat(32) }))).status, 403, field);
  assert.equal((await f.put(await f.signedRecord({ publication: crypto.randomUUID() }))).status, 404);
  assert.equal((await f.put(f.record, 'user', { picture: base64(f.objects.poster) })).status, 400);
  f.identity.accountId = 'another-account'; assert.equal((await f.put()).status, 403);
  f.identity.accountId = 'account-one'; f.identity.device = base64((await keys()).raw); assert.equal((await f.put()).status, 403);
  assert.equal((await f.request('POST', '/internal/inspection/records/authorize', { record: f.record }, 'user')).status, 401);
});

async function assertNoncurrent(f) {
  assert.notEqual((await f.request('GET', `/v1/cards/${f.id}`)).status, 200);
  for (const name of ['', '/canonical', '/poster', '/preview']) assert.notEqual((await f.request('GET', `/v1/published-designs/${f.job.id}${name}`)).status, 200);
  assert.notEqual((await f.request('GET', `/v1/cards/${f.id}/${f.record.picture}.png`)).status, 200);
  for (const updated of [0, f.record.updated, f.record.updated + 1]) {
    const batch = await (await f.batch(updated)).json();
    assert.deepEqual(batch.records, []); assert.deepEqual(batch.gone, [f.id]);
  }
  assert.ok(await f.env.BUCKET.head(`cards/${f.id}.json`));
}

test('lost Creator, missing binding, changed account and withdrawn consent stop unchanged reads', async t => {
  const f = await fixture(t); assert.equal((await f.put()).status, 200);
  f.eligibility.active = false; f.eligibility.reason = 'creator_required'; await assertNoncurrent(f);
  f.eligibility.active = true; f.eligibility.reason = 'creator'; f.authority.bound = false; await assertNoncurrent(f);
  f.authority.bound = true; f.eligibility.accountId = 'different-owner'; await assertNoncurrent(f);
  f.eligibility.accountId = 'account-one'; f.authority.consent = false; await assertNoncurrent(f);
});

test('a genuine active badge replaces Creator only; current binding/consent and revocation remain mandatory', async t => {
  const f = await fixture(t), badge = await code('RYB', f.owner, { device: f.device });
  assert.equal((await f.request('POST', '/v1/badges', new TextEncoder().encode(badge), null, { 'Content-Type': 'text/plain' })).status, 200);
  f.identity.eligible = false; f.eligibility.active = false; f.eligibility.reason = 'creator_required';
  assert.equal((await f.put()).status, 200);
  assert.equal((await f.request('GET', `/v1/cards/${f.id}`)).status, 200);
  f.authority.consent = false; assert.equal((await f.put()).status, 403); await assertNoncurrent(f);
  f.authority.consent = true; f.authority.bound = false; await assertNoncurrent(f);
  f.authority.bound = true;
  const badgeId = (await verifyCode('RYB', badge, f.env)).id;
  const revoked = await code('RYX', f.owner, { id: badgeId });
  // Write out of band after a successful read: no inventory memo may hide this.
  await f.env.BUCKET.put(`revoked/${badgeId}.txt`, revoked);
  await assertNoncurrent(f);
  await worker.scheduled({ scheduledTime: Date.now() }, f.env);
  assert.ok(await f.env.BUCKET.head(`cards/${f.id}.json`));
  f.eligibility.active = true; f.eligibility.reason = 'creator';
  assert.equal((await f.request('GET', `/v1/cards/${f.id}`)).status, 200);
});

test('withdrawal, tombstones, revoked independent signing keys and altered certified bytes cannot bypass reads', async t => {
  const f = await fixture(t); assert.equal((await f.put()).status, 200);
  const originalKeys = f.env.INSPECTOR_KEYS;
  f.env.INSPECTOR_KEYS = JSON.stringify({ scanner: { publicKey: base64(f.inspector.raw), revoked: true } }); await assertNoncurrent(f);
  f.env.INSPECTOR_KEYS = originalKeys;
  const approvals = f.env.DESIGN_APPROVAL_KEYS;
  f.env.DESIGN_APPROVAL_KEYS = JSON.stringify({ owner: { publicKey: base64(f.signer.raw), revoked: true, role: 'owner' } }); await assertNoncurrent(f);
  f.env.DESIGN_APPROVAL_KEYS = approvals;
  const stored = await f.storage.get(`job:${f.job.id}`);
  await f.env.BUCKET.put(stored.outputs.poster.key, new Uint8Array([255, 216, 255, 217])); await assertNoncurrent(f);
  await f.env.BUCKET.put(stored.outputs.poster.key, f.objects.poster);
  assert.equal((await f.request('POST', `/v1/submissions/${f.job.id}/withdraw`, { reason: 'Author withdrew exact bytes', idempotencyKey: 'withdraw-one' }, 'user')).status, 200);
  await assertNoncurrent(f);
  stored.state = 'approved'; await f.storage.put(`job:${f.job.id}`, stored);
  await assertNoncurrent(f); // Persistent canonical tombstone still wins over state.
});

test('ordinary static device records remain independent of Account configuration and consent', async t => {
  const f = await setup(), record = await deviceRecord(f.device);
  t.mock.method(globalThis, 'fetch', () => { throw new Error('Static sync must not call Account'); });
  const raw = JSON.stringify({ record });
  const response = await worker.fetch(new Request(`https://team.ryoku.dev/v1/cards/${f.id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json', 'Content-Length': String(new TextEncoder().encode(raw).length) }, body: raw }), f.env);
  assert.equal(response.status, 200);
  assert.equal((await worker.fetch(new Request(`https://team.ryoku.dev/v1/cards/${f.id}`), f.env)).status, 200);
});

test('even an inspector-signed JPEG cannot stand in for the certified PNG poster', async t => {
  const f = await fixture(t), job = await f.storage.get(`job:${f.job.id}`);
  const jpeg = new Uint8Array([255, 216, 255, 217]), sha = await digest(jpeg);
  await f.env.BUCKET.put(job.outputs.poster.key, jpeg);
  job.outputs.poster = { ...job.outputs.poster, sha, bytes: jpeg.length };
  const cert = { ...f.cert, posterSha: sha, files: f.cert.files.map(file => file.name === 'poster' ? { ...file, sha, bytes: jpeg.length } : file) };
  job.certificate = await envelope(f.inspector, 'ryoku-inspect-v1', cert);
  await assert.rejects(f.queue.validCertified(job), /poster must be PNG/);
});

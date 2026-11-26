import { InputError, base64, unbase64, digest, verifyCode } from './codec.js';
import { objects } from './storage.js';
import { LIMITS, fail, fields, text, hex, submission, certificate, approval, canonical } from './inspection-protocol.js';
import { verifyAnimatedRecord } from './animated-record.js';
import { handleCardSafety, expireCardSafety, assertCardWriteAllowed, cardAccountClosed } from './card-safety.js';

const ARCHIVE_CAP = 8 * 1024 * 1024, BYTE_BUDGET = (512 - 48) * 1024 * 1024, JOB_CAP = 2000;
// leave room for one worker's R2 objects if it dies before durable metadata lands
const TERMINAL_RETENTION = 86400, ADMISSION_WINDOW = 86400, ACCOUNT_ADMISSIONS = 8, BODY_DEADLINE_MS = 10000;
const terminal = state => ['cancelled', 'expired', 'failed', 'rejected', 'changes_requested', 'withdrawn'].includes(state);
const prepared = new WeakMap(), authenticated = new WeakMap();
function retry(message, status, seconds) {
  const error = new InputError(message, status); error.retryAfter = Math.max(1, Math.ceil(seconds)); throw error;
}
const now = () => Math.floor(Date.now() / 1000);
const json = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store', ...(status === 429 || status === 503 ? { 'Retry-After': '60' } : {}) } });
const token = request => /^Bearer ([^\s]+)$/.exec(request.headers.get('Authorization') || '')?.[1];
function configuredToken(env, name, request) {
  const secret = env[name], supplied = token(request);
  if (typeof secret !== 'string' || secret.length < 32) fail('Service authority unavailable', 503);
  if ([env.INSPECTOR_WORKER_TOKEN, env.MIKA_REVIEW_TOKEN, env.DESIGN_SIGNER_TOKEN, env.CARD_GATE_TOKEN].filter(Boolean).filter(t => t === secret).length !== 1) fail('Service credentials must be distinct', 503);
  if (!supplied || supplied !== secret) fail('Service authentication required', 401);
}
function inspectorConfigured(env) {
  let keys;
  try { keys = JSON.parse(env.INSPECTOR_KEYS); } catch { fail('Inspector authority unavailable', 503); }
  if (!keys || typeof keys !== 'object' || Array.isArray(keys) || !Object.values(keys).some(key => key?.revoked === false && typeof key.publicKey === 'string' && key.publicKey.length === 44) || !env.INSPECTOR_PIPELINE_VERSION || !env.INSPECTOR_POLICY_VERSION) fail('Inspector policy or authority unavailable', 503);
}
async function bytes(request, types, cap) {
  if (prepared.has(request)) return prepared.get(request);
  if (!types.includes(request.headers.get('Content-Type'))) fail('Unsupported content type', 415);
  const value = request.headers.get('Content-Length');
  if (!value || !/^[1-9][0-9]*$/.test(value)) fail('Explicit Content-Length required');
  const expected = Number(value);
  if (!Number.isSafeInteger(expected) || expected > cap) fail('Body too large', 413);
  const reader = request.body?.getReader();
  if (!reader) fail('Missing body');
  const chunks = []; let length = 0, timer;
  const deadline = new Promise((_, reject) => { timer = setTimeout(() => {
    reject(new InputError('Request body deadline exceeded', 408));
    // cancellation itself can hang too, so don't wait for it
    void reader.cancel().catch(() => {});
  }, BODY_DEADLINE_MS); });
  try {
    for (;;) {
      const part = await Promise.race([reader.read(), deadline]); if (part.done) break;
      length += part.value.length;
      if (length > expected || length > cap) { void reader.cancel().catch(() => {}); fail('Body length exceeded', 413); }
      chunks.push(part.value);
    }
    if (length !== expected) fail('Body length mismatch');
    const output = new Uint8Array(length); let at = 0;
    for (const chunk of chunks) { output.set(chunk, at); at += chunk.length; }
    return output;
  } finally { clearTimeout(timer); }
}
async function body(request, cap = 65536) {
  const raw = await bytes(request, ['application/json'], cap);
  try { return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(raw)); } catch { fail('Invalid JSON'); }
}
function summary(job) {
  return { id: job.id, state: job.state, kind: job.manifest.kind, cardId: job.manifest.cardId, accountId: job.accountId, createdAt: job.createdAt, updatedAt: job.updatedAt, expiresAt: job.expiresAt, moderation: job.inspection?.moderation ?? null, decision: job.decision ?? null };
}
export function inspectionPath(path) {
  return /^\/v1\/(submissions(?:\/|$)|review\/(?:submissions|safety|card-restrictions)(?:\/|$)|published-designs\/|design-revocations$|card-reports$|card-safety$)/.test(path) || /^\/internal\/inspection(?:\/|$)/.test(path);
}
export async function inspectionRoute(request, env) {
  const origin = request.headers.get('Origin');
  let allowed = [];
  try { allowed = JSON.parse(env.ACCOUNT_WEB_ORIGINS || '[]'); } catch { return json({ error: 'Review origins unavailable' }, 503); }
  if (!Array.isArray(allowed) || allowed.some(value => { try { return new URL(value).origin !== value || !value.startsWith('https://'); } catch { return true; } })) return json({ error: 'Invalid review origins' }, 503);
  if (origin && !allowed.includes(origin)) return json({ error: 'Origin not allowed' }, 403);
  if (request.method === 'OPTIONS') {
    if (!origin || !allowed.includes(origin)) return json({ error: 'Origin not allowed' }, 403);
    const requested = (request.headers.get('Access-Control-Request-Headers') || '').toLowerCase().split(',').map(h => h.trim()).filter(Boolean);
    if (requested.some(h => !['authorization', 'content-type', 'x-ryoku-lease'].includes(h))) return json({ error: 'Header not allowed' }, 403);
    return new Response(null, { status: 204, headers: { 'Access-Control-Allow-Origin': origin, 'Access-Control-Allow-Methods': 'GET,POST,PUT,DELETE,OPTIONS', 'Access-Control-Allow-Headers': 'Authorization,Content-Type,X-Ryoku-Lease', Vary: 'Origin' } });
  }
  if (!env.INSPECTION_QUEUE || !env.BUCKET) return json({ error: 'Private inspection is not configured' }, 503);
  const response = await env.INSPECTION_QUEUE.get(env.INSPECTION_QUEUE.idFromName('private-cards-v1')).fetch(request);
  if (!origin) return response;
  const headers = new Headers(response.headers); headers.set('Access-Control-Allow-Origin', origin); headers.set('Access-Control-Expose-Headers', 'Retry-After'); headers.set('Vary', 'Origin');
  return new Response(response.body, { status: response.status, headers });
}

// Authentication and bounded body reads happen before joining the mutation chain.
// State/lease/claim checks are repeated inside it; cached identity grants no state.
export class InspectionQueue {
  constructor(ctx, env) { this.ctx = ctx; this.env = env; this.tail = Promise.resolve(); }
  async prepare(request) {
    const path = new URL(request.url).pathname;
    if (new URL(request.url).protocol !== 'https:') fail('HTTPS required');
    if (['/internal/inspection/records/authorize', '/internal/inspection/accounts/close'].includes(path)) configuredToken(this.env, 'CARD_GATE_TOKEN', request);
    else if (path.startsWith('/internal/inspection/')) configuredToken(this.env, 'INSPECTOR_WORKER_TOKEN', request);
    else if (path.startsWith('/v1/review/') || path === '/v1/design-revocations') await this.reviewer(request);
    else if (path === '/v1/submissions' && request.method === 'POST') {
      if (!token(request)) fail('Account authentication required', 401);
    } else if (path === '/v1/card-reports' || path === '/v1/card-safety') {
      await this.account(request, 'own');
    } else if (path.startsWith('/v1/submissions/')) {
      const id = /^\/v1\/submissions\/([0-9a-f-]{36})(?:\/|$)/.exec(path)?.[1];
      if (id) {
        const job = await this.job(id);
        try { await this.own(request, job); }
        catch (error) { if (request.method !== 'GET' || error.status !== 403) throw error; await this.reviewer(request); }
      }
    }
    if (['POST', 'PUT'].includes(request.method)) {
      const name = /\/objects\/(canonical|poster|preview|report)$/.exec(path)?.[1];
      const types = { canonical: ['application/zip'], poster: ['image/png'], preview: ['image/gif', 'image/webp'], report: ['application/json'] };
      prepared.set(request, await bytes(request, name ? types[name] : request.method === 'PUT' && path.endsWith('/archive') ? ['application/zip'] : ['application/json'], name ? LIMITS[name] : request.method === 'PUT' && path.endsWith('/archive') ? ARCHIVE_CAP : 65536));
      if (path === '/v1/submissions') {
        const value = await body(request);
        const opened = await submission(value.manifest, this.env.SUBMISSION_AUDIENCE, now(), false);
        await this.account(request, 'submit', opened.payload.cardId);
      }
      if (path === '/internal/inspection/records/authorize') {
        const value = await body(request);
        if (Object.hasOwn(value, 'token')) {
          const credential = new Request(request.url, { headers: { Authorization: `Bearer ${value.token}` } });
          const identity = await this.account(credential, 'submit', value.record?.id);
          authenticated.set(request, new Map([['recordIdentity', identity]]));
        }
      }
    }
  }
  async fetch(request) {
    try {
      await this.prepare(request);
      const readOnly = request.method === 'GET';
      const operation = this.tail.then(async () => { await this.expire(); return readOnly ? null : this.route(request); });
      this.tail = operation.then(() => {}, () => {});
      const response = await operation;
      return readOnly ? await this.route(request) : response;
    } catch (error) {
      const response = json({ error: error instanceof InputError ? error.message : 'Inspection request failed' }, error instanceof InputError ? error.status : 503);
      if (error.retryAfter) response.headers.set('Retry-After', String(error.retryAfter));
      return response;
    }
  }
  alarm() {
    const operation = this.tail.then(() => this.expire());
    this.tail = operation.then(() => {}, () => {});
    return operation;
  }
  async index() {
    const index = await this.ctx.storage.get('index') ?? { ids: [], active: {}, bytes: 0 };
    index.nonces ??= {}; index.admissions ??= {};
    return index;
  }
  async job(id) {
    if (!/^[0-9a-f-]{36}$/.test(id)) fail('Not found', 404);
    const job = await this.ctx.storage.get(`job:${id}`); if (!job) fail('Not found', 404); return job;
  }
  async save(job, event, index = null) {
    index ??= await this.index();
    job.updatedAt = now();
    job.audit ??= [];
    job.audit.push({ at: job.updatedAt, ...event });
    if (job.audit.length > 64) fail('Audit capacity reached', 409);
    if (new TextEncoder().encode(JSON.stringify(job)).length > 100 * 1024) fail('Retained job metadata capacity reached', 409);
    if (terminal(job.state)) job.terminalAt ??= job.updatedAt;
    if (['uploading', 'queued', 'inspecting', 'needs_review', 'awaiting_signature'].includes(job.state)) index.active[job.id] = { accountId: job.accountId, state: job.state };
    else delete index.active[job.id];
    await this.ctx.storage.transaction(async txn => { await txn.put(`job:${job.id}`, job); await txn.put('index', index); });
    if (index.ids.length || Object.keys(index.nonces).length || Object.keys(index.admissions).length) await this.ctx.storage.setAlarm(Date.now() + 60000);
  }
  async discard(job, state, reason) {
    // Commit denial before object deletion. A failed deletion remains charged and
    // is retried by the alarm without growing the retained audit on every retry.
    if (job.state !== state || !job.terminalAt) {
      job.state = state; job.reason = reason;
      await this.save(job, { event: state, reason });
    }
    if (!job.bytes) return;
    const keys = [`private-inspection/${job.id}/original.zip`, ...Object.values(job.outputs).map(o => o.key)];
    if (job.leaseSha) keys.push(...Object.keys(LIMITS).map(name => `private-inspection/${job.id}/${job.leaseSha}/${name}`));
    await this.env.BUCKET.delete([...new Set(keys)]);
    const index = await this.index(); index.bytes -= job.bytes; job.bytes = 0;
    await this.save(job, { event: 'objects_deleted' }, index);
  }
  async expire() {
    const index = await this.index(), time = now();
    await expireCardSafety(this);
    for (const [id, entry] of Object.entries(index.active)) {
      const job = await this.job(id);
      if (entry.state === 'uploading' && job.expiresAt <= time) await this.discard(job, 'expired', 'Upload reservation expired');
      else if (entry.state === 'inspecting' && job.expiresAt <= time) await this.discard(job, 'failed', 'Inspection lease expired; automatic retry disabled');
      else if (entry.state === 'queued' && job.createdAt + 86400 <= time) await this.discard(job, 'expired', 'Inspection queue retention expired');
      else if (['needs_review', 'awaiting_signature'].includes(entry.state) && job.createdAt + 7 * 86400 <= time) await this.discard(job, 'expired', 'Unpublished review retention expired');
    }
    const current = await this.index();
    for (const id of [...current.ids]) {
      let job = await this.job(id);
      if (job.state === 'approved' && job.decision.at + 90 * 86400 <= time) {
        await this.discard(job, 'expired', 'Hosted publication retention expired');
        job = await this.job(id);
      }
      if (!terminal(job.state)) continue;
      if (job.bytes) { await this.discard(job, job.state, job.reason ?? job.decision?.reason ?? 'Terminal retention cleanup'); job = await this.job(id); }
      if ((job.terminalAt ?? job.updatedAt) + TERMINAL_RETENTION <= time && !job.bytes) {
        current.ids = current.ids.filter(value => value !== id);
        delete current.active[id];
        current.bytes = (await this.index()).bytes;
        await this.ctx.storage.transaction(async txn => { await txn.delete(`job:${id}`); await txn.put('index', current); });
      }
    }
    // Expired signed manifests are refused before nonce lookup. Removing a compact
    // nonce marker therefore cannot reopen that manifest, even after job GC.
    for (const [key, until] of Object.entries(current.nonces)) if (until <= time) { await this.ctx.storage.delete(key); delete current.nonces[key]; }
    for (const [account, times] of Object.entries(current.admissions)) {
      current.admissions[account] = times.filter(at => at + ADMISSION_WINDOW > time);
      if (!current.admissions[account].length) delete current.admissions[account];
    }
    // discard() updates the byte counter independently during terminal retries.
    current.bytes = (await this.index()).bytes;
    await this.ctx.storage.put('index', current);
    if (current.ids.length || Object.keys(current.nonces).length || Object.keys(current.admissions).length) await this.ctx.storage.setAlarm(Date.now() + 60000);
  }
  async account(request, action, cardId) {
    const cache = authenticated.get(request) ?? new Map(), cacheKey = `${action}:${cardId ?? ''}`;
    if (cache.has(cacheKey)) return cache.get(cacheKey);
    const env = this.env, userToken = token(request);
    if (!userToken) fail('Account authentication required', 401);
    if (!env.ACCOUNTS_URL || !env.CARD_GATE_AUDIENCE || !env.CARD_GATE_TOKEN || env.CARD_GATE_TOKEN.length < 32) fail('Account authority unavailable', 503);
    if ([env.INSPECTOR_WORKER_TOKEN, env.MIKA_REVIEW_TOKEN, env.DESIGN_SIGNER_TOKEN].includes(env.CARD_GATE_TOKEN)) fail('Account service credential must be distinct', 503);
    const origin = new URL(env.ACCOUNTS_URL);
    if (origin.protocol !== 'https:' || origin.pathname !== '/' || origin.search || origin.username || origin.password) fail('Invalid account authority configuration', 503);
    const response = await fetch(new URL('/internal/cards/authorize', origin), { method: 'POST', redirect: 'manual', signal: AbortSignal.timeout(10000), headers: { Authorization: `Bearer ${env.CARD_GATE_TOKEN}`, 'X-Ryoku-Audience': env.CARD_GATE_AUDIENCE, 'Content-Type': 'application/json' }, body: JSON.stringify({ token: userToken, action, ...(cardId ? { cardId } : {}) }) });
    if (!response.ok) fail('Account authority refused request', response.status === 401 || response.status === 403 ? response.status : 503);
    const raw = await response.text(); if (raw.length > 4096) fail('Invalid account response', 503);
    let identity; try { identity = JSON.parse(raw); } catch { fail('Invalid account response', 503); }
    text(identity.accountId, 128);
    if (cardId && identity.cardId !== cardId) fail('Account device mismatch', 403);
    cache.set(cacheKey, identity); authenticated.set(request, cache);
    return identity;
  }
  async reviewer(request) {
    if (this.env.MIKA_REVIEW_TOKEN && token(request) === this.env.MIKA_REVIEW_TOKEN) { configuredToken(this.env, 'MIKA_REVIEW_TOKEN', request); return { actor: 'service:mika', role: 'mika' }; }
    const account = await this.account(request, 'review');
    if (account.reviewer !== 'owner') fail('Owner review authority required', 403);
    return { actor: account.accountId, role: 'owner' };
  }
  async own(request, job) {
    const identity = await this.account(request, 'own', job.manifest.cardId);
    if (identity.accountId !== job.accountId) fail('Submission ownership mismatch', 403);
    return identity;
  }
  async badge(manifest) {
    for await (const { key } of objects(this.env.BUCKET, 'badges/')) {
      if (!/^badges\/[0-9a-f]{16}\.txt$/.test(key)) continue;
      const object = await this.env.BUCKET.get(key); let badge;
      try { badge = await verifyCode('RYB', await object.text(), this.env); } catch { continue; }
      if (badge.cardId !== manifest.cardId || base64(badge.device) !== manifest.device) continue;
      const revoked = await this.env.BUCKET.get(`revoked/${badge.id}.txt`);
      if (revoked) {
        try { const revoke = await verifyCode('RYX', await revoked.text(), this.env); if (revoke.id === badge.id && (revoke.issuer === 1 || badge.issuer === 2)) continue; } catch { fail('Badge revocation could not be verified', 403); }
      }
      return true;
    }
    return false;
  }
  async lease(request, job, supplied) {
    configuredToken(this.env, 'INSPECTOR_WORKER_TOKEN', request);
    if (job.state !== 'inspecting' || typeof supplied !== 'string' || supplied.length > 128 || await digest(new TextEncoder().encode(supplied)) !== job.leaseSha || job.expiresAt <= now()) fail('Stale or mismatched inspection lease', 409);
  }
  async putImmutable(key, raw, sha, type) {
    if (await this.env.BUCKET.put(key, raw, { onlyIf: { etagDoesNotMatch: '*' }, httpMetadata: { contentType: type } })) return;
    // an R2 write can finish before the durable metadata transaction does
    const existing = await this.env.BUCKET.get(key);
    if (!existing) fail('Immutable object is missing', 409);
    const previous = new Uint8Array(await new Response(existing.body).arrayBuffer());
    if (previous.length !== raw.length || await digest(previous) !== sha) fail('Immutable object conflicts', 409);
  }
  async readObject(job, name) {
    const descriptor = name === 'input' ? { key: job.inputKey, sha: job.manifest.originalSha, type: 'application/zip' } : job.outputs[name];
    if (!descriptor?.key) fail('Object not found', 404);
    const object = await this.env.BUCKET.get(descriptor.key);
    if (!object) fail('Object not found', 404);
    const raw = new Uint8Array(await new Response(object.body).arrayBuffer());
    if ((descriptor.bytes !== undefined && raw.length !== descriptor.bytes) || await digest(raw) !== descriptor.sha) fail('Certified bytes changed', 409);
    return new Response(raw, { headers: { 'Content-Type': descriptor.type, 'Cache-Control': 'private, no-store', 'X-Content-Type-Options': 'nosniff', 'Content-Disposition': name === 'input' || name === 'canonical' ? `attachment; filename="${name}.zip"` : `inline; filename="${name}"`, 'Content-Security-Policy': "default-src 'none'; sandbox" } });
  }
  async validCertified(job) {
    if (!job.certificate) fail('Technical inspection required', 409);
    // Revalidation includes current key revocation and immutable object hashes, not
    // merely the state recorded when the worker submitted the certificate.
    const certified = await certificate(job.certificate, job, this.env, now());
    for (const descriptor of Object.values(job.outputs)) {
      const object = await this.env.BUCKET.get(descriptor.key);
      if (!object) fail('Certified output missing', 409);
      const raw = new Uint8Array(await new Response(object.body).arrayBuffer());
      if (raw.length !== descriptor.bytes || await digest(raw) !== descriptor.sha) fail('Certified bytes changed', 409);
      if (descriptor === job.outputs.poster && (descriptor.type !== 'image/png' || raw.length < 8 || ![137, 80, 78, 71, 13, 10, 26, 10].every((byte, i) => raw[i] === byte))) fail('Certified poster must be PNG', 409);
    }
    return certified;
  }
  async reviewEvidence(job, value, role) {
    if (value.canonicalSha !== job.outputs.canonical.sha || value.previewSha !== job.outputs.preview.sha || value.reportSha !== job.outputs.report.sha || value.certificateSha !== job.certificateSha || value.policyVersion !== this.env.INSPECTOR_POLICY_VERSION) fail('Content review provenance mismatch', 403);
    const object = await this.env.BUCKET.get(job.outputs.report.key);
    if (!object) fail('Certified review report missing', 409);
    const raw = new Uint8Array(await new Response(object.body).arrayBuffer());
    if (raw.length !== job.outputs.report.bytes || await digest(raw) !== value.reportSha) fail('Certified review report changed', 403);
    const report = JSON.parse(new TextDecoder().decode(raw)), coverage = value.coverage;
    fields(coverage, ['frames', 'totalFrames', 'durationMs'], ['composition']);
    const total = role === 'mika' ? report.review?.totalFrames ?? report.totalFrames : report.totalFrames;
    if (!Number.isSafeInteger(coverage.totalFrames) || coverage.totalFrames < 1 || coverage.totalFrames > 4096 || coverage.totalFrames !== total || !Number.isSafeInteger(coverage.durationMs) || coverage.durationMs < 0 || coverage.durationMs !== report.durationMs || !Array.isArray(coverage.frames) || coverage.frames.length > total || coverage.frames.some((frame, i) => !Number.isSafeInteger(frame) || frame < 0 || frame >= total || (i > 0 && frame <= coverage.frames[i - 1])) || (Object.hasOwn(coverage, 'composition') && typeof coverage.composition !== 'boolean')) fail('Content coverage does not match certified report');
    if (!coverage.frames.length && (value.verdict !== 'uncertain' || coverage.composition !== false)) fail('Empty inference coverage requires explicit uncertain escalation');
    if (value.verdict !== 'clear') return;
    if (coverage.frames.length !== total || coverage.frames.some((frame, i) => frame !== i)) fail('Clear requires actual full frame coverage', 403);
    if (role === 'owner') { if (value.model !== 'manual') fail('Owner review must be manual', 403); return; }
    let models; try { models = JSON.parse(this.env.MIKA_REVIEW_MODELS); } catch { fail('Local review models unavailable', 503); }
    if (role !== 'mika' || value.reviewVersion !== 'mika-local-card-review-v2' || coverage.composition !== true || !Array.isArray(models) || !models.includes(value.model)) fail('Current composed-content Mika review required', 403);
    if (report.review?.complete !== true || total > 8 || !Array.isArray(report.review.images) || report.review.images.length !== total) fail('Incomplete original-frame coverage requires owner review', 403);
    for (const image of report.review.images) await this.reviewImage(image);
    const composition = report.composition;
    fields(composition, ['v', 'canonicalSha', 'posterSha', 'previewSha', 'text', 'image', 'coverage']);
    if (composition.v !== 1 || composition.canonicalSha !== job.outputs.canonical.sha || composition.posterSha !== job.outputs.poster.sha || composition.previewSha !== job.outputs.preview.sha || composition.coverage !== 'poster-and-canonical-text' || !Array.isArray(composition.text)) fail('Composed report bindings mismatch', 403);
    const labels = job.manifest.kind === 'hero' ? [] : [['name', 24], ['badge.title', 12], ['badge.kanji', 4], ['card.words.tagline', 36], ['card.words.epithet', 12]];
    if (composition.text.length !== labels.length) fail('Incomplete canonical human text', 403);
    for (let i = 0; i < labels.length; i++) {
      const entry = composition.text[i]; fields(entry, ['field', 'value']);
      if (entry.field !== labels[i][0] || typeof entry.value !== 'string' || [...entry.value].length > labels[i][1] || /[\u0000-\u001f\u007f\ud800-\udfff]/u.test(entry.value) || (i < 2 && !entry.value.length)) fail('Invalid canonical human text', 403);
    }
    await this.reviewImage(composition.image);
  }
  async reviewImage(image) {
    if (!image || image.mime !== 'image/png' || !hex(image.sha256) || typeof image.base64 !== 'string' || image.base64.length > 1048576) fail('Invalid review PNG', 403);
    let raw; try { raw = unbase64(image.base64); } catch { fail('Invalid review PNG', 403); }
    if (base64(raw) !== image.base64 || raw.length < 33 || ![137, 80, 78, 71, 13, 10, 26, 10].every((b, i) => raw[i] === b) || await digest(raw) !== image.sha256) fail('Invalid review PNG hash', 403);
    const view = new DataView(raw.buffer, raw.byteOffset, raw.byteLength);
    if (view.getUint32(8) !== 13 || view.getUint32(12) !== 0x49484452 || !view.getUint32(16) || view.getUint32(16) > 384 || !view.getUint32(20) || view.getUint32(20) > 384 || raw[24] !== 8 || ![2, 6].includes(raw[25])) fail('Invalid review PNG dimensions or format', 403);
  }
  async clearReview(job) {
    const record = job.contentReviews?.at(-1), value = record?.submitted;
    if (!record || value?.verdict !== 'clear') fail('Actual current clear content review required; escalate uncertainty to owner', 403);
    const attestation = { v: record.v, jobId: record.jobId, actor: record.actor, role: record.role, at: record.at, submitted: value };
    if (record.jobId !== job.id || await digest(new TextEncoder().encode(canonical(attestation))) !== record.sha) fail('Content review integrity mismatch', 403);
    await this.reviewEvidence(job, value, record.role);
    if (job.decision && (job.decision.verdict !== 'approve' || job.decision.actor !== record.actor || job.decision.role !== record.role || job.approvalPayload?.contentReviewSha !== record.sha || job.approvalPayload?.decisionId !== job.decision.id || job.approvalPayload?.reviewerRole !== record.role)) fail('Decision content review mismatch', 403);
    return record;
  }
  async acceptSignature(job, envelope) {
    if (job.state !== 'awaiting_signature') fail('Design is not awaiting signature', 409);
    await assertCardWriteAllowed(this.env, job.manifest.cardId, job.accountId);
    await this.validCertified(job);
    await this.clearReview(job);
    await approval(envelope, job.approvalPayload, this.env);
    if (await this.env.BUCKET.head(`design-revocations/${job.outputs.canonical.sha}.json`)) fail('Design is revoked', 409);
    // Even identical bytes in the old namespace must not bypass publication checks.
    await this.env.BUCKET.put(`published-canonical/${job.outputs.canonical.sha}.json`, JSON.stringify({ publication: job.id }), { httpMetadata: { contentType: 'application/json' } });
    job.approval = envelope; job.state = 'approved';
    await this.save(job, { event: 'restricted_signature_verified', actor: job.decision.actor });
  }
  async sign(job) {
    if (job.state !== 'awaiting_signature') fail('Design is not awaiting signature', 409);
    await this.validCertified(job);
    await this.clearReview(job);
    if (!this.env.DESIGN_SIGNER_URL || !this.env.DESIGN_SIGNER_TOKEN || this.env.DESIGN_SIGNER_TOKEN.length < 32) return;
    if ([this.env.INSPECTOR_WORKER_TOKEN, this.env.MIKA_REVIEW_TOKEN, this.env.CARD_GATE_TOKEN].includes(this.env.DESIGN_SIGNER_TOKEN)) fail('Signer credential must be distinct', 503);
    const url = new URL(this.env.DESIGN_SIGNER_URL); if (url.protocol !== 'https:' || url.username || url.password) fail('Invalid signer URL', 503);
    const response = await fetch(url, { method: 'POST', redirect: 'manual', signal: AbortSignal.timeout(10000), headers: { Authorization: `Bearer ${this.env.DESIGN_SIGNER_TOKEN}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ payload: job.approvalPayload, manifest: job.envelope, certificate: job.certificate, decision: job.decision, contentReview: job.contentReviews.at(-1) }) });
    if (!response.ok) return;
    const raw = await response.text(); if (raw.length > 8192) fail('Oversized signer response', 503);
    let envelope; try { envelope = JSON.parse(raw); } catch { fail('Invalid signer response', 503); }
    await this.acceptSignature(job, envelope);
  }
  async currentPublication(job) {
    if (job.state !== 'approved' || !job.outputs.canonical || await this.env.BUCKET.head(`design-revocations/${job.outputs.canonical.sha}.json`)) fail('Not found', 404);
    if (await cardAccountClosed(this.env, job.accountId)) fail('Submitting Account is closed', 403);
    const opened = await submission(job.envelope, this.env.SUBMISSION_AUDIENCE, now(), false);
    if (opened.sha !== job.manifestSha || canonical(opened.payload) !== canonical(job.manifest)) fail('Publication manifest mismatch', 403);
    const certified = await this.validCertified(job);
    await this.clearReview(job);
    const approved = await approval(job.approval, job.approvalPayload, this.env);
    const p = approved.payload;
    if (p.v !== 3 || p.jobId !== job.id || p.accountId !== job.accountId || p.cardId !== job.manifest.cardId || p.canonicalSha !== job.outputs.canonical.sha || p.posterSha !== job.outputs.poster.sha || p.previewSha !== job.outputs.preview.sha || p.certificateSha !== certified.sha || job.certificateSha !== certified.sha || job.outputs.poster.type !== 'image/png') fail('Publication provenance mismatch', 403);
    // Badge entitlement replaces only Creator billing, never the Account binding
    // or current consent. In particular, policy_required must fail for badge holders.
    const env = this.env;
    if (!env.ACCOUNTS_URL || !env.CARD_GATE_TOKEN || env.CARD_GATE_TOKEN.length < 32 || !env.CARD_GATE_AUDIENCE || [env.INSPECTOR_WORKER_TOKEN, env.MIKA_REVIEW_TOKEN, env.DESIGN_SIGNER_TOKEN].includes(env.CARD_GATE_TOKEN)) fail('Eligibility authority unavailable', 503);
    const origin = new URL(env.ACCOUNTS_URL);
    if (origin.protocol !== 'https:' || origin.pathname !== '/' || origin.search || origin.username || origin.password) fail('Invalid eligibility authority', 503);
    const response = await fetch(new URL('/internal/cards/eligibility', origin), { method: 'POST', redirect: 'manual', signal: AbortSignal.timeout(10000), headers: { Authorization: `Bearer ${env.CARD_GATE_TOKEN}`, 'X-Ryoku-Audience': env.CARD_GATE_AUDIENCE, 'Content-Type': 'application/json' }, body: JSON.stringify({ cardId: job.manifest.cardId, action: 'animated-share' }) });
    if (!response.ok) fail('Current Account binding required', response.status === 403 ? 403 : 503);
    const raw = await response.text(); if (raw.length > 4096) fail('Invalid eligibility response', 503);
    let identity; try { identity = JSON.parse(raw); } catch { fail('Invalid eligibility response', 503); }
    if (identity.accountId !== job.accountId || identity.cardId !== job.manifest.cardId || !['creator', 'creator_required'].includes(identity.reason) || typeof identity.active !== 'boolean' || (identity.active && identity.reason !== 'creator')) fail('Current Account binding and consent required', 403);
    if (!identity.active && !await this.badge(job.manifest)) fail('Active Creator or verified badge required', 403);
    // a withdrawal may finish while a public read waits for the Account service
    const current = await this.job(job.id);
    if (current.state !== 'approved' || current.approval?.sig !== job.approval?.sig || await this.env.BUCKET.head(`design-revocations/${job.outputs.canonical.sha}.json`) || await cardAccountClosed(this.env, job.accountId)) fail('Not found', 404);
    return { approvalSha: approved.sha, certificateSha: certified.sha };
  }
  async authorizeRecord(request) {
    configuredToken(this.env, 'CARD_GATE_TOKEN', request);
    const value = await body(request);
    fields(value, ['record'], ['token']);
    const record = value.record;
    if (!await verifyAnimatedRecord(record)) fail('Invalid animated card signature', 403);
    const job = await this.job(record.publication);
    if (record.id !== job.manifest.cardId || record.device !== job.manifest.device) fail('Publication device mismatch', 403);
    const provenance = await this.currentPublication(job);
    if (record.canonicalSha !== job.outputs.canonical.sha || record.picture !== job.outputs.poster.sha || record.approvalSha !== provenance.approvalSha || record.certificateSha !== provenance.certificateSha) fail('Animated publication hash mismatch', 403);
    if (Object.hasOwn(value, 'token')) {
      text(value.token, 4096);
      const identity = authenticated.get(request)?.get('recordIdentity');
      if (identity.accountId !== job.accountId || identity.device !== record.device) fail('Account publication binding mismatch', 403);
      await assertCardWriteAllowed(this.env, record.id, identity.accountId);
      // account(submit) enforces the current policy; versions need not match the
      // historical submission after the account accepts a newer policy.
      text(identity.termsVersion, 80); text(identity.attestationVersion, 80);
      if (identity.eligible !== true && !await this.badge(job.manifest)) fail('Active Creator or verified badge required', 403);
    }
    return json({ current: true });
  }
  async route(request) {
    const url = new URL(request.url), path = url.pathname, method = request.method;
    if (url.protocol !== 'https:') fail('HTTPS required');
    const safety = await handleCardSafety(this, request, body);
    if (safety) return safety;
    if (path === '/internal/inspection/records/authorize' && method === 'POST') return this.authorizeRecord(request);
    if (method === 'POST' && path === '/v1/submissions') {
      const value = await body(request); fields(value, ['manifest']);
      if (!this.env.SUBMISSION_AUDIENCE) fail('Submission audience unavailable', 503);
      inspectorConfigured(this.env);
      const opened = await submission(value.manifest, this.env.SUBMISSION_AUDIENCE, now(), false), manifest = opened.payload;
      const identity = await this.account(request, 'submit', manifest.cardId);
      if (identity.device !== manifest.device || identity.termsVersion !== manifest.termsVersion || identity.attestationVersion !== manifest.attestationVersion) fail('Device or current consent mismatch', 403);
      await assertCardWriteAllowed(this.env, manifest.cardId, identity.accountId);
      if (identity.eligible !== true && !await this.badge(manifest)) fail('Active Creator or verified badge required', 403);
      if (Math.abs(now() - manifest.submittedAt) > 600) fail('Submission manifest expired');
      const nonceKey = `nonce:${await digest(new TextEncoder().encode(`${identity.accountId}\n${manifest.nonce}`))}`;
      const previous = await this.ctx.storage.get(nonceKey);
      if (previous) {
        const existing = await this.job(previous);
        if (existing.manifestSha !== opened.sha) fail('Submission nonce reused with different bytes', 409);
        return json({ id: existing.id, state: existing.state, uploadPath: `/v1/submissions/${existing.id}/archive`, expiresAt: existing.expiresAt });
      }
      const index = await this.index(), active = Object.values(index.active), own = active.filter(e => e.accountId === identity.accountId);
      if (index.ids.length >= JOB_CAP || index.bytes + ARCHIVE_CAP > BYTE_BUDGET || active.filter(e => e.state !== 'inspecting').length >= 20) fail('Inspection queue is full', 503);
      if (own.filter(e => e.state !== 'inspecting').length >= 2) fail('Account waiting limit reached', 429);
      const admissions = index.admissions[identity.accountId] ?? [];
      if (admissions.length >= ACCOUNT_ADMISSIONS) retry('Account admission budget reached', 429, admissions[0] + ADMISSION_WINDOW - now());
      if (!Object.hasOwn(index.admissions, identity.accountId) && Object.keys(index.admissions).length >= JOB_CAP) retry('Admission accounting capacity reached', 503, Math.min(...Object.values(index.admissions).map(times => times.at(-1) + ADMISSION_WINDOW)) - now());
      const id = crypto.randomUUID(), time = now();
      const job = { id, accountId: identity.accountId, state: 'uploading', envelope: value.manifest, manifest, manifestSha: opened.sha, createdAt: time, updatedAt: time, expiresAt: time + 900, outputs: {}, bytes: ARCHIVE_CAP, audit: [] };
      index.ids.push(id); index.bytes += ARCHIVE_CAP;
      index.nonces[nonceKey] = manifest.submittedAt + 601;
      index.admissions[identity.accountId] = [...admissions, time];
      // Nonce is written in the same transaction as reservation and admission index.
      index.active[id] = { accountId: identity.accountId, state: job.state };
      job.audit.push({ at: time, event: 'reserved', actor: identity.accountId });
      if (new TextEncoder().encode(JSON.stringify(index)).length > 100 * 1024) fail('Retained index metadata capacity reached', 503);
      await this.ctx.storage.transaction(async txn => { await txn.put(`job:${id}`, job); await txn.put('index', index); await txn.put(nonceKey, id); });
      await this.ctx.storage.setAlarm(Date.now() + 60000);
      return json({ id, state: job.state, uploadPath: `/v1/submissions/${id}/archive`, expiresAt: job.expiresAt }, 201);
    }
    const ownerPath = /^\/v1\/submissions\/([0-9a-f-]{36})(\/archive)?$/.exec(path);
    if (ownerPath) {
      const job = await this.job(ownerPath[1]);
      if (method === 'GET' && !ownerPath[2]) {
        try { await this.own(request, job); }
        catch (error) { if (!(error instanceof InputError) || error.status !== 403) throw error; await this.reviewer(request); }
        return json(summary(job));
      }
      await this.own(request, job);
      if (method === 'DELETE' && !ownerPath[2]) {
        if (!['uploading', 'queued'].includes(job.state)) fail('Claimed submission cannot be cancelled', 409);
        await this.discard(job, 'cancelled', 'Cancelled by submitting account'); return json(summary(job));
      }
      if (method === 'PUT' && ownerPath[2]) {
        if (job.state !== 'uploading' || job.expiresAt <= now()) fail('Upload reservation is not writable', 409);
        const raw = await bytes(request, ['application/zip'], ARCHIVE_CAP);
        if (await digest(raw) !== job.manifest.originalSha) fail('Original archive hash mismatch');
        const key = `private-inspection/${job.id}/original.zip`;
        await this.putImmutable(key, raw, job.manifest.originalSha, 'application/zip');
        const index = await this.index(); index.bytes += raw.length - job.bytes; job.bytes = raw.length;
        job.inputKey = key; job.state = 'queued'; job.expiresAt = 0;
        await this.save(job, { event: 'original_uploaded', actor: job.accountId }, index);
        return json(summary(job), 202);
      }
      fail('Method not allowed', 405);
    }
    if (path.startsWith('/internal/inspection/')) {
      configuredToken(this.env, 'INSPECTOR_WORKER_TOKEN', request);
      inspectorConfigured(this.env);
      if (path === '/internal/inspection/claim' && method === 'POST') {
        const value = await body(request); fields(value, []);
        const index = await this.index();
        if (Object.values(index.active).some(e => e.state === 'inspecting')) return new Response(null, { status: 204 });
        for (const id of index.ids) {
          if (index.active[id]?.state !== 'queued') continue;
          const job = await this.job(id);
          const lease = `${crypto.randomUUID()}${crypto.randomUUID()}`;
          job.state = 'inspecting'; job.claimedAt = now(); job.expiresAt = now() + 300; job.deadline = now() + 1800;
          job.leaseSha = await digest(new TextEncoder().encode(lease));
          await this.save(job, { event: 'worker_claimed', actor: 'service:inspector' });
          return json({ jobId: id, lease, inputPath: `/internal/inspection/${id}/input`, manifest: job.envelope, expiresAt: job.expiresAt });
        }
        return new Response(null, { status: 204 });
      }
      const match = /^\/internal\/inspection\/([0-9a-f-]{36})\/(input|heartbeat|result|failure|objects\/(canonical|poster|preview|report))$/.exec(path);
      if (!match) fail('Not found', 404);
      const job = await this.job(match[1]), action = match[2];
      if (action === 'input' && method === 'GET') { await this.lease(request, job, request.headers.get('X-Ryoku-Lease')); return this.readObject(job, 'input'); }
      if (match[3] && method === 'PUT') {
        await this.lease(request, job, request.headers.get('X-Ryoku-Lease'));
        const name = match[3];
        const types = { canonical: ['application/zip'], poster: ['image/png'], preview: ['image/gif', 'image/webp'], report: ['application/json'] };
        const raw = await bytes(request, types[name], LIMITS[name]), sha = await digest(raw), existing = job.outputs[name];
        if (existing) { if (existing.sha !== sha || existing.bytes !== raw.length) fail('Output is immutable', 409); return json({ sha, bytes: raw.length }); }
        const index = await this.index(); if (index.bytes + raw.length > BYTE_BUDGET) fail('Output storage budget reached', 503);
        const key = `private-inspection/${job.id}/${job.leaseSha}/${name}`;
        await this.putImmutable(key, raw, sha, request.headers.get('Content-Type'));
        job.outputs[name] = { key, sha, bytes: raw.length, type: request.headers.get('Content-Type') }; job.bytes += raw.length; index.bytes += raw.length;
        await this.save(job, { event: 'output_uploaded', name, sha }, index); return json({ sha, bytes: raw.length });
      }
      if (method !== 'POST') fail('Method not allowed', 405);
      const value = await body(request);
      if (action === 'result' && job.certificate && typeof value.lease === 'string' && value.lease.length <= 128 && await digest(new TextEncoder().encode(value.lease)) === job.leaseSha && value.certificate?.payload === job.certificate.payload && value.certificate?.sig === job.certificate.sig) {
        fields(value, ['lease', 'certificate']); fields(value.certificate, ['payload', 'sig']);
        await this.validCertified(job);
        return json(summary(job));
      }
      await this.lease(request, job, value.lease);
      if (action === 'heartbeat') {
        fields(value, ['lease']); job.expiresAt = Math.min(now() + 300, job.deadline);
        if (job.expiresAt <= now()) fail('Inspection deadline exceeded', 409);
        // Heartbeats do not grow the audit log; the claim and final result are durable.
        await this.ctx.storage.put(`job:${job.id}`, job); return json({ expiresAt: job.expiresAt });
      }
      if (action === 'failure') { fields(value, ['lease', 'reason']); text(value.reason, 256); await this.discard(job, 'failed', value.reason); return json(summary(job)); }
      if (action === 'result') {
        fields(value, ['lease', 'certificate']);
        const opened = await certificate(value.certificate, job, this.env, now());
        job.certificate = value.certificate; job.inspection = opened.payload; job.certificateSha = opened.sha;
        await this.validCertified(job);
        job.state = 'needs_review'; job.expiresAt = 0;
        await this.save(job, { event: 'technical_pass', moderation: opened.payload.moderation, actor: 'service:inspector' }); return json(summary(job));
      }
      fail('Not found', 404);
    }
    if (path === '/v1/review/submissions' && method === 'GET') {
      await this.reviewer(request);
      const limitText = url.searchParams.get('limit') ?? '25', cursor = url.searchParams.get('cursor') ?? '0';
      if (!/^[0-9]{1,4}$/.test(cursor) || !/^[0-9]{1,2}$/.test(limitText)) fail('Invalid pagination');
      const start = Number(cursor), limit = Number(limitText); if (!limit || limit > 50) fail('Invalid page limit');
      const index = await this.index(), filter = url.searchParams.get('state'), search = url.searchParams.get('q')?.toLowerCase();
      if (search && search.length > 128) fail('Invalid search');
      const ids = [...index.ids].reverse(), items = []; let i = start;
      for (; i < ids.length && items.length < limit; i++) { const job = await this.job(ids[i]); if ((!filter || job.state === filter) && (!search || `${job.id} ${job.accountId} ${job.manifest.cardId}`.toLowerCase().includes(search))) items.push(summary(job)); }
      return json({ items, nextCursor: i < ids.length ? String(i) : null });
    }
    const reviewPath = /^\/v1\/review\/submissions\/([0-9a-f-]{36})(?:\/(claim|content-review|decision|signature|withdraw|poster|preview|report|canonical|input))?$/.exec(path);
    if (reviewPath) {
      const job = await this.job(reviewPath[1]), action = reviewPath[2], reviewer = await this.reviewer(request);
      if (action === 'input' && reviewer.role !== 'owner') fail('Original archives require owner authority', 403);
      if (method === 'GET' && !action) {
        let report = null; if (job.outputs.report) { const object = await this.env.BUCKET.get(job.outputs.report.key); if (object) { try { report = JSON.parse(await object.text()); } catch { fail('Invalid inspection report', 409); } } }
        const base = `/v1/review/submissions/${job.id}`;
        return json({ ...summary(job), manifest: job.manifest, certificate: job.certificate ?? null, certificateSha: job.certificateSha ?? null, contentReviews: job.contentReviews ?? [], decisionHistory: job.decisionHistory ?? [], signingRequest: job.approvalPayload ? { payload: job.approvalPayload, manifest: job.envelope, certificate: job.certificate, decision: job.decision, contentReview: job.contentReviews.at(-1) } : null, report, outputPaths: { poster: `${base}/poster`, preview: `${base}/preview`, canonical: `${base}/canonical`, input: `${base}/input` } });
      }
      if (method === 'GET' && ['poster', 'preview', 'report', 'canonical', 'input'].includes(action)) return this.readObject(job, action);
      if (method !== 'POST') fail('Method not allowed', 405);
      const value = await body(request);
      if (action === 'claim') {
        fields(value, []);
        if (!['needs_review', 'awaiting_signature'].includes(job.state)) fail('Submission is not reviewable', 409);
        if (reviewer.role === 'mika' && job.contentReviews?.some(review => review.role !== 'mika' || review.submitted.verdict !== 'clear')) fail('Escalated submissions require owner review', 403);
        if (job.reviewClaim?.expiresAt > now() && job.reviewClaim.actor !== reviewer.actor) fail('Submission claimed by another reviewer', 409);
        if (job.reviewClaim?.actor === reviewer.actor && job.reviewClaim.expiresAt > now()) {
          job.reviewClaim.expiresAt = now() + 600;
          await this.ctx.storage.put(`job:${job.id}`, job);
          return json({ expiresAt: job.reviewClaim.expiresAt });
        }
        job.reviewClaim = { ...reviewer, expiresAt: now() + 600 };
        await this.save(job, { event: 'review_claimed', ...reviewer }); return json({ expiresAt: job.reviewClaim.expiresAt });
      }
      if (action === 'content-review') {
        fields(value, ['verdict', 'reason', 'idempotencyKey', 'model', 'coverage', 'policyVersion', 'reviewVersion', 'canonicalSha', 'previewSha', 'reportSha', 'certificateSha']);
        fields(value.coverage, ['frames', 'totalFrames', 'durationMs'], ['composition']);
        text(value.reason, 1024); text(value.idempotencyKey, 128); text(value.model, 128); text(value.reviewVersion, 80);
        if (!['clear', 'flagged', 'uncertain'].includes(value.verdict)) fail('Invalid content review verdict');
        const previous = job.contentReviews?.find(r => r.idempotencyKey === value.idempotencyKey);
        if (previous) {
          if (previous.actor !== reviewer.actor || canonical(previous.submitted) !== canonical(value)) fail('Review idempotency conflict', 409);
          return json(previous);
        }
        const signatureEscalation = job.state === 'awaiting_signature' && value.verdict !== 'clear' && reviewer.role === 'mika' && job.decision?.actor === reviewer.actor && job.decision?.role === 'mika';
        if ((!signatureEscalation && job.state !== 'needs_review') || job.reviewClaim?.actor !== reviewer.actor || job.reviewClaim.expiresAt <= now()) fail('Current review claim required', 409);
        await this.validCertified(job);
        if (value.canonicalSha !== job.outputs.canonical.sha || value.previewSha !== job.outputs.preview.sha || value.reportSha !== job.outputs.report.sha || value.certificateSha !== job.certificateSha || value.policyVersion !== this.env.INSPECTOR_POLICY_VERSION) fail('Content review provenance mismatch', 403);
        if ((job.contentReviews?.length ?? 0) >= 4) fail('Content review capacity reached', 409);
        const latest = job.contentReviews?.at(-1);
        if (latest && reviewer.role !== 'owner' && !(latest.actor === reviewer.actor && latest.role === 'mika' && latest.submitted.verdict === 'clear' && value.verdict !== 'clear')) fail('Further content review requires owner escalation', 403);
        if (reviewer.role === 'owner') {
          if (value.model !== 'manual') fail('Owner review must be explicitly manual');
        } else {
          let models; try { models = JSON.parse(this.env.MIKA_REVIEW_MODELS); } catch { fail('Local review models unavailable', 503); }
          if (!Array.isArray(models) || !models.includes(value.model)) fail('Local review model is not authorized', 403);
        }
        await this.reviewEvidence(job, value, reviewer.role);
        const attestation = { v: 1, jobId: job.id, actor: reviewer.actor, role: reviewer.role, at: now(), submitted: value };
        const sha = await digest(new TextEncoder().encode(canonical(attestation)));
        const record = { ...attestation, sha, idempotencyKey: value.idempotencyKey };
        job.contentReviews ??= []; job.contentReviews.push(record);
        if (value.verdict !== 'clear') {
          if (signatureEscalation) {
            job.decisionHistory ??= [];
            job.decisionHistory.push({ decision: job.decision, approvalPayload: job.approvalPayload });
            delete job.decision; delete job.approvalPayload;
            job.state = 'needs_review';
          }
          delete job.reviewClaim;
        }
        await this.save(job, { event: 'content_review', actor: reviewer.actor, verdict: value.verdict, sha });
        return json(record);
      }
      if (action === 'decision') {
        fields(value, ['verdict', 'reason', 'idempotencyKey']); text(value.reason, 1024); text(value.idempotencyKey, 128);
        if (!['approve', 'reject', 'changes'].includes(value.verdict)) fail('Invalid review verdict');
        if (reviewer.role === 'mika' && value.verdict !== 'approve') fail('Only the owner can reject or request changes', 403);
        if (job.decision) {
          if (job.decision.idempotencyKey !== value.idempotencyKey || job.decision.actor !== reviewer.actor || job.decision.verdict !== value.verdict || job.decision.reason !== value.reason) fail('Submission already decided', 409);
          return json({ id: job.id, state: job.state, decision: job.decision });
        }
        if (job.state !== 'needs_review' || job.reviewClaim?.actor !== reviewer.actor || job.reviewClaim.expiresAt <= now()) fail('Current review claim required', 409);
        await this.validCertified(job);
        const contentReview = job.contentReviews?.at(-1);
        if (value.verdict === 'approve') {
          if (contentReview?.actor !== reviewer.actor) fail('Current reviewer must own clear attestation', 403);
          await this.clearReview(job);
        }
        const decision = { id: crypto.randomUUID(), verdict: value.verdict, reason: value.reason, actor: reviewer.actor, role: reviewer.role, at: now(), idempotencyKey: value.idempotencyKey };
        job.decision = decision; job.state = value.verdict === 'approve' ? 'awaiting_signature' : value.verdict === 'reject' ? 'rejected' : 'changes_requested';
        if (value.verdict === 'approve') job.approvalPayload = { v: 3, jobId: job.id, accountId: job.accountId, cardId: job.manifest.cardId, canonicalSha: job.outputs.canonical.sha, posterSha: job.outputs.poster.sha, previewSha: job.outputs.preview.sha, certificateSha: job.certificateSha, contentReviewSha: contentReview.sha, decisionId: decision.id, reviewerRole: reviewer.role, approvedAt: decision.at };
        await this.save(job, { event: 'review_decision', ...decision });
        if (value.verdict !== 'approve') await this.discard(job, job.state, value.reason);
        // A separate explicit signature request is retryable and cannot overwrite the decision.
        return json({ id: job.id, state: job.state, decision });
      }
      if (action === 'signature') {
        fields(value, [], ['approval']);
        if (reviewer.role !== 'owner' && (!value.approval || job.decision?.actor !== reviewer.actor || job.decision?.role !== 'mika')) fail('Signature completion is outside reviewer scope', 403);
        if (job.state === 'approved') { await this.validCertified(job); await this.clearReview(job); return json({ id: job.id, state: job.state, decision: job.decision }); }
        if (value.approval) await this.acceptSignature(job, value.approval); else await this.sign(job);
        return json({ id: job.id, state: job.state, decision: job.decision });
      }
      if (action === 'withdraw') {
        fields(value, ['reason', 'idempotencyKey']); text(value.reason, 1024); text(value.idempotencyKey, 128);
        if (reviewer.role !== 'owner') fail('Owner authority required', 403);
        return this.withdraw(job, value, reviewer.actor);
      }
      fail('Not found', 404);
    }
    const withdraw = /^\/v1\/submissions\/([0-9a-f-]{36})\/withdraw$/.exec(path);
    if (withdraw && method === 'POST') {
      const job = await this.job(withdraw[1]); await this.own(request, job);
      const value = await body(request); fields(value, ['reason', 'idempotencyKey']); text(value.reason, 1024); text(value.idempotencyKey, 128);
      return this.withdraw(job, value, job.accountId);
    }
    if (path === '/v1/design-revocations' && method === 'POST') {
      const reviewer = await this.reviewer(request); if (reviewer.role !== 'owner') fail('Owner authority required', 403);
      const value = await body(request); fields(value, ['sha', 'reason', 'idempotencyKey']); if (!hex(value.sha)) fail('Invalid design hash'); text(value.reason, 1024); text(value.idempotencyKey, 128);
      await this.tombstone(value.sha, { ...value, actor: reviewer.actor, at: now() }); return json({ sha: value.sha, state: 'revoked' });
    }
    const published = /^\/v1\/published-designs\/([0-9a-f-]{36})(?:\/(canonical|poster|preview))?$/.exec(path);
    if (published && method === 'GET') {
      const job = await this.job(published[1]);
      await this.currentPublication(job);
      if (!published[2]) return json({ id: job.id, state: job.state, approval: job.approval, certificate: job.certificate });
      return this.readObject(job, published[2]);
    }
    fail('Not found', 404);
  }
  async tombstone(sha, value) {
    const key = `design-revocations/${sha}.json`, previous = await this.env.BUCKET.get(key);
    if (previous) { const stored = JSON.parse(await previous.text()); if (stored.idempotencyKey !== value.idempotencyKey || stored.actor !== value.actor || stored.reason !== value.reason) fail('Design already revoked', 409); return; }
    if (!await this.env.BUCKET.put(key, JSON.stringify(value), { onlyIf: { etagDoesNotMatch: '*' }, httpMetadata: { contentType: 'application/json' } })) fail('Revocation changed concurrently', 409);
  }
  async withdraw(job, value, actor) {
    if (!job.outputs.canonical || !['approved', 'awaiting_signature', 'needs_review', 'withdrawn'].includes(job.state)) fail('Design is not withdrawable', 409);
    if (job.withdrawal) {
      if (job.withdrawal.idempotencyKey !== value.idempotencyKey || job.withdrawal.actor !== actor || job.withdrawal.reason !== value.reason) fail('Submission already withdrawn', 409);
    } else {
      job.withdrawal = { ...value, actor, at: now() };
      job.state = 'withdrawn'; job.reason = value.reason;
      await this.save(job, { event: 'withdrawn', ...job.withdrawal });
    }
    if (job.bytes) await this.discard(job, 'withdrawn', value.reason);
    return json({ id: job.id, state: job.state });
  }
}

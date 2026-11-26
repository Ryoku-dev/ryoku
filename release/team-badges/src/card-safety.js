import { CARD_ID, InputError, digest } from './codec.js';
import { canonical, fail, fields, hex, text } from './inspection-protocol.js';
import { reconcileIndex, storedRecord } from './storage.js';

const DAY = 86400, RETENTION = 90 * DAY, CASE_CAP = 256, OWN_CAP = 32, DAILY_CAP = 8;
const uuid = value => typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value);
const seconds = () => Math.floor(Date.now() / 1000);
const json = (value, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', 'Cache-Control': 'no-store', ...(status === 202 ? { 'Retry-After': '30' } : {}) } });
const encode = value => new TextEncoder().encode(value);
const accountID = value => typeof value === 'string' && /^[A-Za-z0-9_-]{1,128}$/.test(value);
const reason = value => {
  if (typeof value !== 'string' || !value.trim() || [...value].length > 1024 || /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f\ud800-\udfff]/u.test(value)) fail('Invalid safety reason');
  return value;
};
const restrictionKey = (kind, id) => `card-restrictions/${kind}/${id}.json`;
const closureKey = id => `card-account-closures/${id}.json`;
const contentKey = (id, sha) => `card-content-withdrawals/${id}/${sha}.json`;

export async function cardContentSHA(record) {
  if (typeof record?.body !== 'string' || !record.body.length || !(record.picture === '' || hex(record.picture))) fail('Stored card content unavailable', 409);
  const content = { v: record.v, body: record.body, picture: record.picture, style: record.style ?? '', tagline: record.tagline ?? '', standName: record.standName ?? '' };
  if (record.v === 5) { content.canonicalSha = record.canonicalSha; content.publication = record.publication; }
  return digest(encode(canonical(content)));
}
export async function cardContentWithdrawn(env, record) {
  return !!await env.BUCKET.head(contentKey(record.id, await cardContentSHA(record)));
}
export async function cardAccountClosed(env, id) {
  return accountID(id) && !!await env.BUCKET.head(closureKey(id));
}
export async function assertCardWriteAllowed(env, id, accountId) {
  if (!CARD_ID.test(id)) fail('Invalid card id');
  if (accountId && await cardAccountClosed(env, accountId)) fail('Card account is closed', 403);
  for (const [kind, target] of [['card', id], ...(accountId ? [['account', accountId]] : [])]) {
    const object = await env.BUCKET.get(restrictionKey(kind, target));
    if (!object) continue;
    let value; try { value = JSON.parse(await object.text()); } catch { fail('Card restriction unavailable', 503); }
    if (typeof value?.blocked !== 'boolean') fail('Card restriction unavailable', 503);
    if (value.blocked) fail('New card submissions and publication are paused for this target', 403);
  }
}
async function index(queue) {
  return await queue.ctx.storage.get('safety-index') ?? { items: [], admissions: {}, restrictions: [] };
}
function boundedIndex(value) {
  if (encode(JSON.stringify(value)).length > 100 * 1024) fail('Safety metadata capacity reached', 503);
}
async function saveIndex(queue, value) {
  boundedIndex(value);
  await queue.ctx.storage.put('safety-index', value);
}
async function schedule(queue) {
  const time = Date.now();
  let at = time + DAY * 1000;
  const existing = await queue.ctx.storage.getAlarm?.();
  if (typeof existing === 'number') at = Math.min(at, existing);
  if ((await queue.index()).ids.length) at = Math.min(at, time + 60000);
  await queue.ctx.storage.setAlarm(Math.max(time + 1000, at));
}
function visible(item, owner = false) {
  return { id: item.id, type: item.type, state: item.state, createdAt: item.createdAt, updatedAt: item.updatedAt, reporterAccountId: item.reporterAccountId, target: item.target, reason: item.reason, ...(item.decision ? { decision: { action: item.decision.action, reason: item.decision.reason, actor: item.decision.actor, at: item.decision.at } } : {}), ...(owner && item.targetAccountId ? { targetAccountId: item.targetAccountId } : {}) };
}
async function owner(queue, request) {
  const identity = await queue.reviewer(request);
  if (identity.role !== 'owner') fail('Owner authority required', 403);
  return identity;
}
async function target(queue, value) {
  if (!CARD_ID.test(value.cardId)) fail('Invalid card id');
  if (value.publicationId !== undefined && !uuid(value.publicationId)) fail('Invalid publication id');
  let publicationId = value.publicationId, record;
  if (!publicationId) {
    record = await storedRecord(await queue.env.BUCKET.get(`cards/${value.cardId}.json`));
    if (!record || record.id !== value.cardId) fail('Hosted card not found', 404);
    if (record.v === 5) publicationId = record.publication;
  }
  if (publicationId) {
    const job = await queue.job(publicationId);
    if (job.manifest.cardId !== value.cardId || job.decision?.verdict !== 'approve' || !job.approval) fail('Hosted publication not found', 404);
    return { target: { cardId: value.cardId, publicationId }, targetAccountId: job.accountId, binding: { contentSha: job.approvalPayload.canonicalSha, certificateSha: job.certificateSha } };
  }
  return { target: { cardId: value.cardId }, binding: { contentSha: await cardContentSHA(record) } };
}
async function recordCase(queue, identity, type, input, resolveTarget) {
  reason(input.reason); text(input.idempotencyKey, 128);
  const state = await index(queue), time = seconds();
  const submitted = canonical(input), nonce = await digest(encode(`${identity.accountId}\0${input.idempotencyKey}`));
  const replay = state.items.find(item => item.nonce === nonce);
  if (replay) {
    const item = await queue.ctx.storage.get(`safety:${replay.id}`);
    if (!item || item.type !== type || item.submitted !== submitted) fail('Safety request idempotency conflict', 409);
    return json(visible(item));
  }
  const bound = await resolveTarget();
  const targetSHA = await digest(encode(canonical({ type, target: bound.target, binding: bound.binding })));
  if (state.items.some(item => item.reporterAccountId === identity.accountId && item.state === 'open' && item.targetSHA === targetSHA)) fail('An open safety case already covers this exact content', 409);
  if (state.items.length >= CASE_CAP) fail('Owner safety inbox is full', 503);
  if (state.items.filter(item => item.reporterAccountId === identity.accountId).length >= OWN_CAP) fail('Account safety history capacity reached', 429);
  const admissionKey = `account:${identity.accountId}`, times = (state.admissions[admissionKey] ?? []).filter(at => at + DAY > time);
  if (times.length >= DAILY_CAP) {
    const error = new InputError('Safety request budget reached', 429);
    error.retryAfter = Math.max(1, times[0] + DAY - time); throw error;
  }
  const id = crypto.randomUUID();
  const item = { id, type, state: 'open', createdAt: time, updatedAt: time, reporterAccountId: identity.accountId, reason: input.reason, submitted, ...bound };
  state.items.push({ id, type, state: 'open', createdAt: time, reporterAccountId: identity.accountId, nonce, targetSHA });
  state.admissions[admissionKey] = [...times, time]; boundedIndex(state);
  await queue.ctx.storage.transaction(async transaction => { await transaction.put(`safety:${id}`, item); await transaction.put('safety-index', state); });
  await schedule(queue);
  return json(visible(item), 201);
}
async function decide(queue, request, id, input) {
  const identity = await owner(queue, request);
  fields(input, ['action', 'reason', 'idempotencyKey']); reason(input.reason); text(input.idempotencyKey, 128);
  const item = await queue.ctx.storage.get(`safety:${id}`); if (!item) fail('Safety case not found', 404);
  if (!(item.type === 'report' ? ['dismiss', 'withdraw'] : ['dismiss', 'resubmit']).includes(input.action)) fail('Invalid safety decision');
  if (item.decision) {
    if (item.decision.actor !== identity.actor || canonical(item.decision.submitted) !== canonical(input)) fail('Safety case already decided', 409);
    if (item.state === 'resolved') return json(visible(item, true));
  } else {
    item.decision = { action: input.action, reason: input.reason, actor: identity.actor, at: seconds(), submitted: input };
    await queue.ctx.storage.put(`safety:${id}`, item);
  }
  if (input.action === 'withdraw') {
    if (item.target.publicationId) {
      const job = await queue.ctx.storage.get(`job:${item.target.publicationId}`);
      if (job) {
        if (job.accountId !== item.targetAccountId || job.approvalPayload?.canonicalSha !== item.binding.contentSha || job.certificateSha !== item.binding.certificateSha) fail('Reported publication changed', 409);
        if (['approved', 'awaiting_signature', 'needs_review'].includes(job.state)) await queue.withdraw(job, { reason: input.reason, idempotencyKey: `safety:${id}` }, identity.actor);
      }
    } else {
      const key = contentKey(item.target.cardId, item.binding.contentSha);
      await queue.env.BUCKET.put(key, JSON.stringify({ caseId: id, actor: identity.actor, reason: input.reason, at: item.decision.at }), { httpMetadata: { contentType: 'application/json' } });
    }
  }
  item.state = 'resolved'; item.updatedAt = seconds();
  const state = await index(queue), entry = state.items.find(entry => entry.id === id); if (entry) entry.state = item.state;
  await queue.ctx.storage.transaction(async transaction => { await transaction.put(`safety:${id}`, item); await transaction.put('safety-index', state); });
  return json(visible(item, true));
}
async function restrict(queue, request, input) {
  const identity = await owner(queue, request);
  fields(input, ['blocked', 'reason', 'idempotencyKey'], ['accountId', 'cardId']);
  if (Object.hasOwn(input, 'accountId') === Object.hasOwn(input, 'cardId') || typeof input.blocked !== 'boolean') fail('Exactly one card restriction target required');
  const kind = input.accountId !== undefined ? 'account' : 'card', id = input.accountId ?? input.cardId;
  if (kind === 'account' ? !accountID(id) : !CARD_ID.test(id)) fail('Invalid restriction target');
  reason(input.reason); text(input.idempotencyKey, 128);
  const state = await index(queue), name = `${kind}:${id}`, key = `safety-restriction:${name}`;
  const history = await queue.ctx.storage.get(key) ?? [];
  const submitted = canonical(input), old = history.find(item => item.key === input.idempotencyKey && item.actor === identity.actor);
  if (old && old.submitted !== submitted) fail('Restriction idempotency conflict', 409);
  if (history.at(-1)?.pending && history.at(-1) !== old) fail('Retry the unfinished restriction update first', 409);
  if (old && !old.pending) return json({ target: { [kind === 'account' ? 'accountId' : 'cardId']: id }, blocked: history.at(-1).blocked, replay: true });
  if (!old) {
    if (history.length >= 16 || (!state.restrictions.includes(name) && state.restrictions.length >= CASE_CAP)) fail('Restriction audit capacity reached', 409);
    history.push({ key: input.idempotencyKey, submitted, actor: identity.actor, blocked: input.blocked, at: seconds(), pending: true });
    if (!state.restrictions.includes(name)) state.restrictions.push(name);
    boundedIndex(state);
    await queue.ctx.storage.transaction(async transaction => { await transaction.put(key, history); await transaction.put('safety-index', state); });
  }
  const current = history.at(-1);
  await queue.env.BUCKET.put(restrictionKey(kind, id), JSON.stringify({ blocked: current.blocked, reason: input.reason, actor: identity.actor, at: current.at }), { httpMetadata: { contentType: 'application/json' } });
  current.pending = false; await queue.ctx.storage.put(key, history);
  return json({ target: { [kind === 'account' ? 'accountId' : 'cardId']: id }, blocked: current.blocked });
}
async function retireSnapshot(queue, id, snapshot, generation) {
  const key = `cards/${id}.json`;
  // R2 cannot conditionally delete: a CAS tombstone retires only the selected
  // generation, without a delete racing a later independent device publication.
  const retired = { id, updated: snapshot.updated, deleted: true, closure: generation };
  const replaced = await queue.env.BUCKET.put(key, JSON.stringify(retired), { onlyIf: { etagMatches: snapshot.etag }, httpMetadata: { contentType: 'application/json' } });
  if (!replaced) {
    const current = await queue.env.BUCKET.get(key);
    if (current) {
      let value; try { value = JSON.parse(await current.text()); } catch { return; }
      if (value.deleted !== true || value.closure !== generation) return;
    }
  }
  if (snapshot.pictureKey) await queue.env.BUCKET.delete(snapshot.pictureKey);
  await reconcileIndex(queue.env, [id], seconds());
}

async function closeAccount(queue, request, input) {
  if (request.headers.get('X-Ryoku-Audience') !== queue.env.CARD_GATE_AUDIENCE) fail('Wrong card closure audience', 403);
  fields(input, ['accountId', 'cardIds', 'idempotencyKey']);
  if (!accountID(input.accountId) || !Array.isArray(input.cardIds) || input.cardIds.length > 2048 || input.cardIds.some(id => !CARD_ID.test(id)) || new Set(input.cardIds).size !== input.cardIds.length) fail('Invalid account closure binding');
  text(input.idempotencyKey, 128);
  const key = `card-account-close:${input.accountId}`, submitted = await digest(encode(canonical(input)));
  let closure = await queue.ctx.storage.get(key);
  if (closure && closure.submitted !== submitted) fail('Account closure idempotency conflict', 409);
  if (closure?.complete) return json({ closed: true, purged: true, accountId: input.accountId });
  if (!closure) {
    const jobs = [], state = await queue.index();
    for (const id of state.ids) { const job = await queue.job(id); if (job.accountId === input.accountId) jobs.push(id); }
    closure = { submitted, jobIds: jobs, cardIds: input.cardIds, snapshotCursor: 0, jobCursor: 0, cardCursor: 0, complete: false, startedAt: Date.now(), at: seconds() };
    await queue.ctx.storage.put(key, closure);
  }
  await queue.env.BUCKET.put(closureKey(input.accountId), JSON.stringify({ accountId: input.accountId, at: closure.at }), { httpMetadata: { contentType: 'application/json' } });
  let remaining = 32;
  while (remaining && closure.snapshotCursor < closure.cardIds.length) {
    const cursor = closure.snapshotCursor, id = closure.cardIds[cursor], object = await queue.env.BUCKET.get(`cards/${id}.json`);
    let snapshot = { skip: true };
    if (object) {
      const uploaded = object.uploaded?.getTime();
      if (!Number.isFinite(uploaded)) fail('Card generation time unavailable', 503);
      if (uploaded <= closure.startedAt) {
        const record = await storedRecord(object);
        snapshot = { etag: object.etag, updated: record?.updated ?? Math.floor(uploaded / 1000), pictureKey: object.customMetadata?.pictureKey ?? '' };
      }
    }
    closure.snapshotCursor++; remaining--;
    await queue.ctx.storage.transaction(async transaction => {
      await transaction.put(`${key}:card:${cursor}`, snapshot);
      await transaction.put(key, closure);
    });
  }
  if (closure.snapshotCursor < closure.cardIds.length) return json({ closed: false, cleanupPending: true, retryable: true, accountId: input.accountId }, 202);
  while (remaining && closure.jobCursor < closure.jobIds.length) {
    const id = closure.jobIds[closure.jobCursor], job = await queue.ctx.storage.get(`job:${id}`);
    if (job) { if (job.accountId !== input.accountId) fail('Account closure job binding changed', 409); await queue.discard(job, 'withdrawn', 'Submitting Ryoku Account closed'); }
    closure.jobCursor++; remaining--;
    await queue.ctx.storage.put(key, closure);
  }
  while (remaining && closure.cardCursor < closure.cardIds.length) {
    const cursor = closure.cardCursor, id = closure.cardIds[cursor], snapshotKey = `${key}:card:${cursor}`;
    const snapshot = await queue.ctx.storage.get(snapshotKey);
    if (!snapshot) fail('Card purge snapshot unavailable', 503);
    if (!snapshot.skip) await retireSnapshot(queue, id, snapshot, closure.submitted);
    closure.cardCursor++; remaining--;
    await queue.ctx.storage.transaction(async transaction => {
      await transaction.delete(snapshotKey);
      await transaction.put(key, closure);
    });
  }
  closure.complete = closure.jobCursor === closure.jobIds.length && closure.cardCursor === closure.cardIds.length;
  await queue.ctx.storage.put(key, closure);
  return json({ closed: closure.complete, ...(closure.complete ? { purged: true } : { cleanupPending: true, retryable: true }), accountId: input.accountId }, closure.complete ? 200 : 202);
}
export async function expireCardSafety(queue) {
  const state = await index(queue), time = seconds();
  const expired = state.items.filter(item => item.createdAt + RETENTION <= time);
  let changed = expired.length > 0;
  if (expired.length) {
    state.items = state.items.filter(item => item.createdAt + RETENTION > time);
    for (const item of expired) await queue.ctx.storage.delete(`safety:${item.id}`);
  }
  for (const [id, times] of Object.entries(state.admissions)) {
    state.admissions[id] = times.filter(at => at + DAY > time);
    if (state.admissions[id].length !== times.length) changed = true;
    if (!state.admissions[id].length) delete state.admissions[id];
  }
  if (changed) await saveIndex(queue, state);
  if (state.items.length) await schedule(queue);
}
export async function handleCardSafety(queue, request, readBody) {
  const path = new URL(request.url).pathname, method = request.method;
  if (path === '/internal/inspection/accounts/close' && method === 'POST') return closeAccount(queue, request, await readBody(request));
  if (path === '/v1/card-reports' && method === 'POST') {
    const identity = await queue.account(request, 'own'), input = await readBody(request);
    fields(input, ['cardId', 'reason', 'idempotencyKey'], ['publicationId']);
    return recordCase(queue, identity, 'report', input, () => target(queue, input));
  }
  const appeal = /^\/v1\/submissions\/([0-9a-f-]{36})\/appeals$/.exec(path);
  if (appeal && method === 'POST') {
    const job = await queue.job(appeal[1]); await queue.own(request, job);
    const identity = await queue.account(request, 'own'), input = await readBody(request); fields(input, ['reason', 'idempotencyKey']);
    return recordCase(queue, identity, 'appeal', { ...input, submissionId: job.id }, async () => {
      if (!['rejected', 'changes_requested', 'failed', 'expired'].includes(job.state)) fail('This submission has no appealable decision', 409);
      return { target: { cardId: job.manifest.cardId, submissionId: job.id }, targetAccountId: job.accountId, binding: { state: job.state, decisionId: job.decision?.id ?? null } };
    });
  }
  if ((path === '/v1/card-safety' || path === '/v1/review/safety') && method === 'GET') {
    const reviewer = path.startsWith('/v1/review/'), identity = reviewer ? await owner(queue, request) : await queue.account(request, 'own');
    const state = await index(queue), items = [];
    for (const entry of [...state.items].reverse()) {
      if (!reviewer && entry.reporterAccountId !== identity.accountId) continue;
      const item = await queue.ctx.storage.get(`safety:${entry.id}`); if (item) items.push(visible(item, reviewer));
    }
    return json({ items });
  }
  const decision = /^\/v1\/review\/safety\/([0-9a-f-]{36})\/decision$/.exec(path);
  if (decision && method === 'POST') return decide(queue, request, decision[1], await readBody(request));
  if (path === '/v1/review/card-restrictions' && method === 'POST') return restrict(queue, request, await readBody(request));
  return null;
}

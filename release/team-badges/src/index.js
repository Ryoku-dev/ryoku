import { CARD_ID, InputError, base64, cardId, cardPayload, digest, jpegDimensions, unbase64, verifyCard, verifyCode, verifyDelete, verifyDeviceCard, verifyHolder, verifyUpload } from './codec.js';
import { admitRecord, cardIndex, claimHolderRefresh, deleteRecord, dropStyledRecords, expireCards, objects, reconcileIndex, reserveBadge, storeDesign, storeRecord, storedRecord } from './storage.js';

const TEXT_CAP = 2400; // At most 600 characters, including Unicode whitespace.
const CARD_CAP = 600 * 1024;
const PICTURE_CAP = 400 * 1024;
const PACK_CAP = 8 * 1024 * 1024;
const DELETE_CAP = 1024;
const BATCH_CAP = 16 * 1024;
const BATCH_IDS = 200;
const json = (value, status = 200, headers = {}) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', ...headers } });
const fail = (message, status = 400) => { throw new InputError(message, status); };
const seconds = () => Math.floor(Date.now() / 1000);
const setting = (value, fallback) => { const number = Number(value); return Number.isSafeInteger(number) && number > 0 ? number : fallback; };
const isMap = value => value && typeof value === 'object' && !Array.isArray(value);

const inventoryMemos = new WeakMap();
function inventoryMemo(env) {
  let memo = inventoryMemos.get(env.BUCKET);
  if (!memo || memo.owner !== env.OWNER_KEY || memo.mika !== env.MIKA_KEY) {
    memo = { owner: env.OWNER_KEY, mika: env.MIKA_KEY, data: null, loaded: 0, attempted: 0, pending: null };
    inventoryMemos.set(env.BUCKET, memo);
  }
  return memo;
}
async function inventory(env, refreshOnMiss = false) {
  const memo = inventoryMemo(env), now = Date.now();
  if (memo.pending) return memo.pending;
  if (memo.data && now - memo.loaded < 30_000 && (!refreshOnMiss || now - memo.attempted < 5_000)) return memo.data;
  memo.attempted = now;
  memo.pending = loadInventory(env).then(data => {
    memo.data = data;
    memo.loaded = Date.now();
    return data;
  }).finally(() => { memo.pending = null; });
  return memo.pending;
}
async function refreshInventory(env, request) {
  const memo = inventoryMemo(env);
  if (memo.pending) await memo.pending;
  memo.data = null;
  memo.loaded = 0;
  memo.attempted = 0;
  await inventory(env);
  await deleteReadCache(request, ['/v1/list']);
}
async function authorizedInventory(env, accepts) {
  let data = await inventory(env);
  if (await accepts(data.active)) return true;
  const refreshed = await inventory(env, true);
  if (refreshed === data) return false;
  data = refreshed;
  return accepts(data.active);
}
async function cachedResponse(request, ctx, produce) {
  const url = new URL(request.url);
  const cache = globalThis.caches?.default;
  const key = new Request(`${url.origin}${url.pathname}`, { method: 'GET' });
  if (cache) { const hit = await cache.match(key); if (hit) return hit; }
  const response = await produce();
  if (response.status !== 200 && response.status !== 404) return response;
  response.headers.set('Cache-Control', 'public, max-age=60');
  if (cache) {
    const pending = cache.put(key, response.clone());
    if (ctx?.waitUntil) ctx.waitUntil(pending); else await pending;
  }
  return response;
}
async function deleteReadCache(request, paths) {
  const cache = globalThis.caches?.default;
  if (typeof cache?.delete !== 'function') return;
  const origin = new URL(request.url).origin;
  await Promise.all(paths.map(path => cache.delete(new Request(`${origin}${path}`, { method: 'GET' }))));
}

async function bodyBytes(request, type, cap) {
  if (request.headers.get('Content-Type') !== type) fail(`Expected ${type}`, 415);
  const length = request.headers.get('Content-Length');
  if (length === null || !/^(0|[1-9][0-9]*)$/.test(length)) fail('Missing or invalid Content-Length');
  const expected = Number(length);
  if (!Number.isSafeInteger(expected) || expected > cap) fail('Body too large', 413);
  const reader = request.body?.getReader();
  if (!reader) { if (expected) fail('Missing body'); return new Uint8Array(); }
  const chunks = [];
  let total = 0;
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    total += value.length;
    if (total > cap) { await reader.cancel(); fail('Body too large', 413); }
    if (total > expected) { await reader.cancel(); fail('Content-Length mismatch'); }
    chunks.push(value);
  }
  if (total !== expected) fail('Content-Length mismatch');
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.length; }
  return bytes;
}
async function textBody(request) {
  const bytes = await bodyBytes(request, 'text/plain', TEXT_CAP);
  try { return new TextDecoder('utf-8', { fatal: true }).decode(bytes); } catch { fail('Invalid UTF-8'); }
}
async function parseStoredCode(bucket, key, kind, env) {
  const object = await bucket.get(key);
  if (!object) return null;
  try { return await verifyCode(kind, await object.text(), env); } catch { return null; }
}
async function loadInventory(env) {
  const badges = [], revoked = [], designs = [];
  for await (const object of objects(env.BUCKET, 'badges/')) {
    if (!/^badges\/[0-9a-f]{16}\.txt$/.test(object.key)) continue;
    const badge = await parseStoredCode(env.BUCKET, object.key, 'RYB', env);
    if (badge && object.key === `badges/${badge.id}.txt`) badges.push(badge);
  }
  const badgeById = new Map(badges.map(b => [b.id, b]));
  for await (const object of objects(env.BUCKET, 'revoked/')) {
    if (!/^revoked\/[0-9a-f]{16}\.txt$/.test(object.key)) continue;
    const revocation = await parseStoredCode(env.BUCKET, object.key, 'RYX', env);
    if (revocation && object.key === `revoked/${revocation.id}.txt` && (revocation.issuer === 1 || badgeById.get(revocation.id)?.issuer === 2)) revoked.push(revocation);
  }
  const revokedIds = new Set(revoked.map(r => r.id));
  const active = badges.filter(b => !revokedIds.has(b.id));
  for await (const object of objects(env.BUCKET, 'designs/')) {
    const design = await parseStoredCode(env.BUCKET, object.key, 'RYD', env);
    if (design && object.key === `designs/${design.login.toLowerCase()}.txt` && active.some(b => b.role >= 3 && b.login.toLowerCase() === design.login.toLowerCase()) && await env.BUCKET.head(`packs/${design.sha}.zip`)) designs.push(design);
  }
  return { badges, revoked, active, designs };
}
const STYLE_REFUSED = 'Custom card styles need an active team badge on this machine';
const WORDS_REFUSED = 'Card words need an active team badge on this machine';
// a styled v3 or worded v4 record needs an active badge for its id whose device key signed it,
// the record's own device key never vouches for one
async function styledBy(record, active) {
  const badges = active.filter(b => b.cardId === record.id);
  if (!badges.length || (record.device !== undefined && !badges.every(b => base64(b.device) === record.device))) return false;
  for (const badge of badges) if (await verifyCard(record, badge.device)) return true;
  return false;
}
// a v2 device record speaks for itself but never against the key an active badge holds for the id
async function trustedBy(record, active) {
  if (record.v >= 3) return styledBy(record, active);
  const badges = active.filter(b => b.cardId === record.id);
  if (record.device !== undefined) return badges.every(b => base64(b.device) === record.device) && verifyDeviceCard(record);
  for (const badge of badges) if (await verifyCard(record, badge.device)) return true;
  return false;
}
// PUT verified these and binders verify again, so a batch only checks what can change after storing:
// a revoked badge takes its styled or worded record out of every batch at once
async function listedBy(record, active) {
  const badges = active.filter(b => b.cardId === record.id);
  if (record.device === undefined || (record.v >= 3 && !badges.length)) return badges.length > 0;
  return badges.every(b => base64(b.device) === record.device) && await cardId(unbase64(record.device)) === record.id;
}
// ids without an active badge lose their styled record; revocation calls this at once, the cron daily
async function withdrawStyles(env, data, ids, now) {
  const held = new Set(data.active.map(b => b.cardId));
  await dropStyledRecords(env, [...new Set(ids)].filter(id => !held.has(id)), now);
}
async function trustedRecord(env, id, active, trusts = trustedBy) {
  const stored = await env.BUCKET.get(`cards/${id}.json`), record = await storedRecord(stored);
  try { if (record?.id === id && await trusts(record, active)) return { record, pictureKey: stored.customMetadata?.pictureKey }; }
  catch { /* Invalid persisted records are never served as trusted data. */ }
  return null;
}
async function jsonBody(request, cap) {
  const bytes = await bodyBytes(request, 'application/json', cap);
  try { return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)); } catch { fail('Invalid JSON'); }
}
async function putCard(request, env, id) {
  const input = await jsonBody(request, CARD_CAP);
  if (!isMap(input) || Object.keys(input).some(k => !['record', 'picture'].includes(k))) fail('Invalid card upload');
  const record = input.record, now = seconds();
  cardPayload(record);
  if (record.id !== id || Math.abs(now - record.updated) > 600) fail('Invalid card id or timestamp');
  if (record.v >= 3) {
    if (!await authorizedInventory(env, active => styledBy(record, active))) {
      fail((await inventory(env)).active.some(b => b.cardId === id) ? 'Invalid card signature' : record.v === 4 ? WORDS_REFUSED : STYLE_REFUSED, 403);
    }
  } else if (record.device !== undefined) {
    if (!await trustedBy(record, (await inventory(env)).active)) fail('Invalid card signature');
  } else if (!await authorizedInventory(env, active => trustedBy(record, active))) fail('No active badge verifies this record');
  let picture;
  if (input.picture !== undefined) {
    picture = unbase64(input.picture);
    if (picture.length > PICTURE_CAP) fail('Picture too large', 413);
    jpegDimensions(picture);
    if (!record.picture || await digest(picture) !== record.picture) fail('Picture hash mismatch');
  }
  const loaded = await admitRecord(env, record, setting(env.CARD_LIMIT, 20000));
  await storeRecord(env, record, picture);
  await reconcileIndex(env, [id], now, { loaded });
  return json({ id });
}
async function deleteCard(request, env, id) {
  const input = await jsonBody(request, DELETE_CAP), now = seconds();
  if (input?.id !== id) fail('Invalid card id or timestamp');
  const object = await env.BUCKET.get(`cards/${id}.json`), record = await storedRecord(object);
  if (!object) return json({ error: 'Not found' }, 404);
  const devices = (await inventory(env)).active.filter(b => b.cardId === id).map(b => b.device);
  try { if (record?.device !== undefined) devices.push(unbase64(record.device)); } catch { /* a malformed stored key verifies nothing */ }
  if (!await verifyDelete(input, devices, now)) fail('Invalid delete signature');
  await deleteRecord(env, id, object, record, now);
  return json({ id });
}
async function limited(request, limiter) {
  if (typeof limiter?.limit !== 'function') return false;
  return !(await limiter.limit({ key: request.headers.get('CF-Connecting-IP') || 'unknown' })).success;
}
// binders plan their next pull from next, so a refused pull says when to come back
const refused = (pull, early) => json({ error: 'Rate limit exceeded', ...(pull ? { next: seconds() + 60 } : {}), ...(early ? { holderLeft: early.left } : {}) }, 429, { 'Retry-After': '60' });
// a valid proof from an active badge spends one of its early refreshes for the day; anything else,
// a broken or replayed proof included, just means a normal binder, never an error
async function earlyRefresh(holder, cards, env) {
  const now = seconds();
  let badge;
  try { badge = await verifyHolder(holder, cards, env, now); } catch { return null; }
  if (!(await inventory(env)).active.some(b => b.id === badge.id)) return null;
  return claimHolderRefresh(env, badge.id, holder.at, now, setting(env.HOLDER_REFRESHES, 10));
}
async function pullCards(request, env) {
  const input = await jsonBody(request, BATCH_CAP), known = input?.cards;
  if (!isMap(input) || Object.keys(input).some(k => !['cards', 'holder'].includes(k)) || !isMap(known)) fail('Invalid card batch');
  const ids = Object.keys(known);
  if (ids.length < 1 || ids.length > BATCH_IDS || !ids.every(id => CARD_ID.test(id) && Number.isSafeInteger(known[id]) && known[id] >= 0)) fail('Invalid card batch');
  const early = input.holder === undefined ? null : await earlyRefresh(input.holder, known, env);
  // an early refresh skips the cadence and the per-address limits; without one a batch is a read
  // and a pull like any other
  if (!early?.granted && (await limited(request, env.READ_LIMITER) || await limited(request, env.PULL_LIMITER))) return refused(true, early);
  const { active, designs } = await inventory(env);
  const index = await cardIndex(env);
  const changed = ids.filter(id => Object.hasOwn(index.cards, id) && index.cards[id] > known[id]);
  const records = (await Promise.all(changed.map(id => trustedRecord(env, id, active, listedBy)))).filter(t => t && t.record.updated > known[t.record.id]).map(t => t.record);
  const badges = active.filter(b => Object.hasOwn(known, b.cardId));
  const team = new Set(badges.filter(b => b.role >= 3).map(b => b.login.toLowerCase()));
  const now = seconds();
  return json({
    v: 1, now, next: early?.granted ? now : now + setting(env.PULL_SECONDS, 900), records,
    gone: ids.filter(id => !Object.hasOwn(index.cards, id)),
    badges: badges.map(b => b.code),
    designs: designs.filter(d => team.has(d.login.toLowerCase())).map(d => d.code),
    ...(early ? { holderLeft: early.left } : {}),
  }, 200, { 'Cache-Control': 'no-store' });
}
async function serveObject(env, key, type) {
  const object = await env.BUCKET.get(key);
  return object ? new Response(object.body, { headers: { 'Content-Type': type, 'X-Content-Type-Options': 'nosniff' } }) : json({ error: 'Not found' }, 404);
}
async function route(request, env, ctx) {
  const url = new URL(request.url), path = url.pathname, method = request.method;
  if (url.protocol !== 'https:') fail('HTTPS required');
  const batch = method === 'POST' && path === '/v1/cards/batch';
  // a batch takes its read limit once it knows whether a holder spent an early refresh on it
  if (!batch && await limited(request, method === 'GET' || method === 'HEAD' ? env.READ_LIMITER : env.WRITE_LIMITER)) return refused(false);
  if (batch) return pullCards(request, env);
  if (method === 'GET' && path === '/v1/list') return cachedResponse(request, ctx, async () => {
    const data = await inventory(env);
    return json({ v: 1, badges: data.badges.map(b => b.code), revoked: data.revoked.map(r => r.code), designs: data.designs.map(d => d.code) });
  });
  if (method === 'POST' && path === '/v1/badges') {
    const badge = await verifyCode('RYB', await textBody(request), env);
    if (!await env.BUCKET.head(`badges/${badge.id}.txt`)) await reserveBadge(env, badge, (await inventory(env)).active);
    await env.BUCKET.put(`badges/${badge.id}.txt`, badge.code);
    await refreshInventory(env, request);
    return json({ id: badge.id });
  }
  if (method === 'POST' && path === '/v1/revoke') {
    const revoke = await verifyCode('RYX', await textBody(request), env);
    const badge = await parseStoredCode(env.BUCKET, `badges/${revoke.id}.txt`, 'RYB', env);
    if (revoke.issuer === 2 && (!badge || badge.issuer !== 2)) fail('Mika may only revoke Mika badges');
    const existing = await parseStoredCode(env.BUCKET, `revoked/${revoke.id}.txt`, 'RYX', env);
    if (!existing || (existing.issuer !== 1 && revoke.issuer === 1) || (existing.issuer === revoke.issuer && revoke.day >= existing.day)) {
      await env.BUCKET.put(`revoked/${revoke.id}.txt`, revoke.code);
    }
    await refreshInventory(env, request);
    if (badge) await withdrawStyles(env, await inventory(env), [badge.cardId], seconds());
    return json({ id: revoke.id });
  }
  // Card records and pictures are read fresh: an edge copy outlived a change in every data
  // centre but the writer's, and a binder holding a superseded record found its picture gone.
  const card = /^\/v1\/cards\/([0-9a-f]{16})$/.exec(path);
  if (card && ['GET', 'PUT', 'DELETE'].includes(method)) {
    const id = card[1];
    if (!CARD_ID.test(id)) fail('Invalid card id');
    if (method === 'PUT') return putCard(request, env, id);
    if (method === 'DELETE') return deleteCard(request, env, id);
    const { active } = await inventory(env);
    const trusted = await trustedRecord(env, id, active);
    return trusted ? json(trusted.record) : json({ error: 'Not found' }, 404);
  }
  const picture = /^\/v1\/cards\/([0-9a-f]{16})\/([0-9a-f]{64})\.jpg$/.exec(path);
  if (method === 'GET' && picture) {
    const { active } = await inventory(env);
    const trusted = await trustedRecord(env, picture[1], active);
    if (!trusted || trusted.record.picture !== picture[2] || !trusted.pictureKey) return json({ error: 'Not found' }, 404);
    return serveObject(env, trusted.pictureKey, 'image/jpeg');
  }
  const pending = /^\/v1\/pending\/([0-9a-f]{64})$/.exec(path);
  if (method === 'PUT' && pending) {
    // Check the header window before reading the potentially large pack; authenticate its digest afterwards.
    await verifyUpload(request.headers.get('X-Ryoku-Upload'), method, path, pending[1], env, Math.floor(Date.now() / 1000));
    const bytes = await bodyBytes(request, 'application/zip', PACK_CAP);
    if (await digest(bytes) !== pending[1]) fail('Pack hash mismatch');
    await env.BUCKET.put(`pending/${pending[1]}.zip`, bytes, { httpMetadata: { contentType: 'application/zip' } });
    return json({ sha: pending[1] });
  }
  const pack = /^\/v1\/(pending|packs)\/([0-9a-f]{64})\.zip$/.exec(path);
  if (method === 'GET' && pack) return serveObject(env, `${pack[1]}/${pack[2]}.zip`, 'application/zip');
  if (method === 'POST' && path === '/v1/designs') {
    const design = await verifyCode('RYD', await textBody(request), env);
    if (!await authorizedInventory(env, active => active.some(b => b.role >= 3 && b.login.toLowerCase() === design.login.toLowerCase()))) fail('Design requires an active team badge');
    await storeDesign(env, design);
    await refreshInventory(env, request);
    return json({ sha: design.sha });
  }
  return json({ error: 'Not found' }, 404);
}
export default {
  async fetch(request, env, ctx) {
    try { return await route(request, env, ctx); }
    catch (error) { return json({ error: error instanceof InputError ? error.message : 'Request could not be processed' }, error instanceof InputError ? error.status : 400); }
  },
  async scheduled(controller, env) {
    const now = Math.floor(controller.scheduledTime / 1000);
    await expireCards(env, now);
    // read fresh: a revocation stored without the route still takes its styled record down
    const data = await loadInventory(env);
    await withdrawStyles(env, data, data.badges.map(b => b.cardId), now);
  },
};

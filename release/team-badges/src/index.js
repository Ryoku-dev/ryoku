import { CARD_ID, SHA, InputError, cardPayload, digest, jpegDimensions, unbase64, verifyCard, verifyCode, verifyUpload } from './codec.js';
import { reserveBadge, storeDesign, storeRecord } from './storage.js';

const TEXT_CAP = 2400; // At most 600 characters, including Unicode whitespace.
const CARD_CAP = 600 * 1024;
const PICTURE_CAP = 400 * 1024;
const PACK_CAP = 8 * 1024 * 1024;
const json = (value, status = 200, headers = {}) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json', ...headers } });
const fail = (message, status = 400) => { throw new InputError(message, status); };

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
async function* objects(bucket, prefix) {
  let cursor;
  do {
    const page = await bucket.list({ prefix, ...(cursor ? { cursor } : {}) });
    for (const object of page.objects) yield object;
    cursor = page.truncated ? page.cursor : undefined;
    if (page.truncated && !cursor) throw new Error('Invalid storage pagination');
  } while (cursor);
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
async function trustedRecord(env, id, active) {
  const stored = await env.BUCKET.get(`cards/${id}.json`);
  if (!stored) return null;
  try {
    const record = JSON.parse(await stored.text());
    if (record.id !== id) return null;
    for (const badge of active.filter(b => b.cardId === id)) if (await verifyCard(record, badge.device)) return { record, pictureKey: stored.customMetadata?.pictureKey };
  } catch { /* Invalid persisted records are never served as trusted data. */ }
  return null;
}
async function serveObject(env, key, type) {
  const object = await env.BUCKET.get(key);
  return object ? new Response(object.body, { headers: { 'Content-Type': type, 'X-Content-Type-Options': 'nosniff' } }) : json({ error: 'Not found' }, 404);
}
async function route(request, env, ctx) {
  const url = new URL(request.url), path = url.pathname, method = request.method;
  if (url.protocol !== 'https:') fail('HTTPS required');
  const limiter = method === 'GET' || method === 'HEAD' ? env.READ_LIMITER : env.WRITE_LIMITER;
  if (typeof limiter?.limit === 'function') {
    const { success } = await limiter.limit({ key: request.headers.get('CF-Connecting-IP') || 'unknown' });
    if (!success) return json({ error: 'Rate limit exceeded' }, 429, { 'Retry-After': '60' });
  }
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
    if (badge) {
      const cardPath = `/v1/cards/${badge.cardId}`;
      const paths = [cardPath];
      const stored = await env.BUCKET.get(`cards/${badge.cardId}.json`);
      if (stored) {
        try {
          const record = JSON.parse(await stored.text());
          if (typeof record.picture === 'string' && SHA.test(record.picture)) paths.push(`${cardPath}/${record.picture}.jpg`);
        } catch { /* A corrupt stored record cannot identify a trusted picture. */ }
      }
      await deleteReadCache(request, paths);
    }
    return json({ id: revoke.id });
  }
  const card = /^\/v1\/cards\/([0-9a-f]{16})$/.exec(path);
  if (card && (method === 'GET' || method === 'PUT')) {
    const id = card[1];
    if (!CARD_ID.test(id)) fail('Invalid card id');
    if (method === 'GET') return cachedResponse(request, ctx, async () => {
      const { active } = await inventory(env);
      const trusted = await trustedRecord(env, id, active);
      return trusted ? json(trusted.record) : json({ error: 'Not found' }, 404);
    });
    const bytes = await bodyBytes(request, 'application/json', CARD_CAP);
    let input;
    try { input = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)); } catch { fail('Invalid JSON'); }
    if (!input || typeof input !== 'object' || Array.isArray(input) || Object.keys(input).some(k => !['record', 'picture'].includes(k))) fail('Invalid card upload');
    const record = input.record;
    cardPayload(record);
    if (record.id !== id || Math.abs(Math.floor(Date.now() / 1000) - record.updated) > 600) fail('Invalid card id or timestamp');
    const verified = await authorizedInventory(env, async active => {
      for (const badge of active.filter(b => b.cardId === id)) if (await verifyCard(record, badge.device)) return true;
      return false;
    });
    if (!verified) fail('No active badge verifies this record');
    let picture;
    if (input.picture !== undefined) {
      picture = unbase64(input.picture);
      if (picture.length > PICTURE_CAP) fail('Picture too large', 413);
      jpegDimensions(picture);
      if (!record.picture || await digest(picture) !== record.picture) fail('Picture hash mismatch');
    }
    const old = await storeRecord(env, record, picture);
    await deleteReadCache(request, [path, ...(record.picture ? [`${path}/${record.picture}.jpg`] : []), ...(old?.picture && old.picture !== record.picture ? [`${path}/${old.picture}.jpg`] : [])]);
    return json({ id });
  }
  const picture = /^\/v1\/cards\/([0-9a-f]{16})\/([0-9a-f]{64})\.jpg$/.exec(path);
  if (method === 'GET' && picture) return cachedResponse(request, ctx, async () => {
    const { active } = await inventory(env);
    const trusted = await trustedRecord(env, picture[1], active);
    if (!trusted || trusted.record.picture !== picture[2] || !trusted.pictureKey) return json({ error: 'Not found' }, 404);
    return serveObject(env, trusted.pictureKey, 'image/jpeg');
  });
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
};

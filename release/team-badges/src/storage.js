import { InputError, verifyCode } from './codec.js';

const ATTEMPTS = 8;
const condition = object => object ? { etagMatches: object.etag } : { etagDoesNotMatch: '*' };
const conflict = message => { throw new InputError(message, 409); };
const same = (a, b) => Object.keys(a).length === Object.keys(b).length && Object.keys(a).every(key => Object.hasOwn(b, key) && b[key] === a[key]);
const isMap = value => value && typeof value === 'object' && !Array.isArray(value);

const INDEX = 'cards/index.json';
const RECORD_KEY = /^cards\/([0-9a-f]{16})\.json$/;
const EXPIRY_SECONDS = 90 * 86400;
// three R2 calls per card, 300 keeps a run under the 1,000 calls a free invocation gets
const EXPIRY_BATCH = 300;
// a copy fetched before a delete still fits the 600 s upload window for a while, so keep refusing it
const TOMBSTONE_SECONDS = 3600;
const indexMemos = new WeakMap();

export async function* objects(bucket, prefix) {
  let cursor;
  do {
    const page = await bucket.list({ prefix, ...(cursor ? { cursor } : {}) });
    for (const object of page.objects) yield object;
    cursor = page.truncated ? page.cursor : undefined;
    if (page.truncated && !cursor) throw new Error('Invalid storage pagination');
  } while (cursor);
}
export async function storedRecord(object) {
  if (!object) return null;
  try {
    const record = JSON.parse(await object.text());
    return isMap(record) && Number.isSafeInteger(record.updated) ? record : null;
  } catch { return null; }
}
async function removeRecord(env, id, object) {
  const pictureKey = object.customMetadata?.pictureKey;
  await env.BUCKET.delete([`cards/${id}.json`, ...(pictureKey ? [pictureKey] : [])]);
}

function indexMemo(env) {
  let memo = indexMemos.get(env.BUCKET);
  if (!memo) indexMemos.set(env.BUCKET, memo = { data: null, loaded: 0, pending: null, generation: 0 });
  return memo;
}
// a read that started before a write here must not overwrite what the write remembered
function remember(env, data, generation) {
  const memo = indexMemo(env);
  if (generation !== undefined && generation !== memo.generation) return;
  memo.generation++;
  memo.data = data;
  memo.loaded = Date.now();
}
// records from before the index existed join it when it's first created
async function seedCards(env) {
  const cards = {};
  for await (const { key } of objects(env.BUCKET, 'cards/')) {
    const id = RECORD_KEY.exec(key)?.[1];
    const record = id ? await storedRecord(await env.BUCKET.get(key)) : null;
    if (record) cards[id] = record.updated;
  }
  return cards;
}
async function loadIndex(env) {
  const generation = indexMemo(env).generation;
  const stored = await env.BUCKET.get(INDEX);
  let data;
  if (stored) {
    const value = JSON.parse(await stored.text());
    if (value?.v !== 1 || !isMap(value.cards) || !isMap(value.deleted)) throw new Error('Invalid card index');
    data = { cards: value.cards, deleted: value.deleted };
  } else data = { cards: await seedCards(env), deleted: {} };
  remember(env, data, generation);
  return { stored, ...data };
}
export async function cardIndex(env) {
  const memo = indexMemo(env);
  if (memo.data && Date.now() - memo.loaded < 30_000) return memo.data;
  memo.pending ??= loadIndex(env).finally(() => { memo.pending = null; });
  await memo.pending;
  return memo.data;
}
export async function admitRecord(env, record, limit) {
  const index = await loadIndex(env);
  if (Object.hasOwn(index.deleted, record.id) && record.updated <= index.deleted[record.id]) throw new InputError('Card record is not newer');
  if (!Object.hasOwn(index.cards, record.id) && Object.keys(index.cards).length >= limit) throw new InputError('Card service is full', 507);
  return index;
}
// each entry comes from the record read after the index, so whichever conditional write lands last saw every record change before it
export async function reconcileIndex(env, ids, now, { tombstones = {}, loaded } = {}) {
  for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
    const { stored, cards, deleted } = attempt === 0 && loaded ? loaded : await loadIndex(env);
    const next = { cards: { ...cards }, deleted: {} };
    for (const [id, at] of [...Object.entries(deleted), ...Object.entries(tombstones)]) {
      if (at > now - TOMBSTONE_SECONDS && !(next.deleted[id] >= at)) next.deleted[id] = at;
    }
    for (const id of ids) {
      const object = await env.BUCKET.get(`cards/${id}.json`);
      let record = await storedRecord(object);
      // an old copy uploaded beside a delete must not bring the card back
      if (record && Object.hasOwn(next.deleted, id) && record.updated <= next.deleted[id]) { await removeRecord(env, id, object); record = null; }
      if (record) next.cards[id] = record.updated; else delete next.cards[id];
    }
    // skipping is only safe when no record was read, a reconcile that saw a newer record has to move the etag
    if (stored && !ids.length && same(next.cards, cards) && same(next.deleted, deleted)) return;
    if (await env.BUCKET.put(INDEX, JSON.stringify({ v: 1, ...next }), { onlyIf: condition(stored), httpMetadata: { contentType: 'application/json' } })) {
      remember(env, next);
      return;
    }
  }
  conflict('Card index changed concurrently');
}
export async function deleteRecord(env, id, object, record, now) {
  await removeRecord(env, id, object);
  await reconcileIndex(env, [id], now, { tombstones: record ? { [id]: record.updated } : {} });
}
export async function expireCards(env, now) {
  const cutoff = now - EXPIRY_SECONDS, loaded = await loadIndex(env);
  const stale = Object.keys(loaded.cards).filter(id => loaded.cards[id] < cutoff).slice(0, EXPIRY_BATCH);
  for (const id of stale) {
    const object = await env.BUCKET.get(`cards/${id}.json`), record = await storedRecord(object);
    if (object && !(record?.updated >= cutoff)) await removeRecord(env, id, object);
  }
  await reconcileIndex(env, stale, now, { loaded });
}
// styled (v3) records only stand while an active badge vouches for them; the caller passes ids that
// lost theirs, and only a stored v3 record goes, a classic record under the same id stays
export async function dropStyledRecords(env, ids, now) {
  if (!ids.length) return;
  const loaded = await loadIndex(env), dropped = [];
  for (const id of ids) {
    if (!Object.hasOwn(loaded.cards, id)) continue;
    const object = await env.BUCKET.get(`cards/${id}.json`);
    if (object && (await storedRecord(object))?.v === 3) { await removeRecord(env, id, object); dropped.push(id); }
  }
  if (dropped.length) await reconcileIndex(env, dropped, now, { loaded });
}

export async function reserveBadge(env, badge, initialActive) {
  const key = `logins/${badge.login.toLowerCase()}.json`;
  for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
    const stored = await env.BUCKET.get(key);
    const ids = stored ? JSON.parse(await stored.text()) : initialActive.filter(b => b.login.toLowerCase() === badge.login.toLowerCase()).map(b => b.id);
    if (ids.includes(badge.id)) return;
    const active = [];
    for (const id of ids) {
      const revoked = await env.BUCKET.get(`revoked/${id}.txt`);
      let valid = false;
      if (revoked) {
        try {
          const revocation = await verifyCode('RYX', await revoked.text(), env);
          const original = await env.BUCKET.get(`badges/${id}.txt`);
          const issuer = original ? (await verifyCode('RYB', await original.text(), env)).issuer : null;
          valid = revocation.id === id && (revocation.issuer === 1 || issuer === 2);
        } catch { /* Corrupt revocations must not release a badge slot. */ }
      }
      if (!valid) active.push(id);
    }
    if (active.length >= 8) conflict('Login already has eight active badges');
    if (await env.BUCKET.put(key, JSON.stringify([...active, badge.id]), { onlyIf: condition(stored) })) return;
  }
  conflict('Badge registration changed concurrently');
}

export async function storeDesign(env, design) {
  const key = `designs/${design.login.toLowerCase()}.txt`;
  for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
    const stored = await env.BUCKET.get(key);
    if (stored) {
      const existing = await verifyCode('RYD', await stored.text(), env);
      if (existing.code === design.code) return false;
      if (design.approvedAt <= existing.approvedAt) conflict('Design approval is not newer');
    }
    if (!await env.BUCKET.head(`packs/${design.sha}.zip`)) {
      const pack = await env.BUCKET.get(`pending/${design.sha}.zip`);
      if (!pack) throw new InputError('Design pack not found', 404);
      await env.BUCKET.put(`packs/${design.sha}.zip`, pack.body, { httpMetadata: { contentType: 'application/zip' } });
    }
    if (await env.BUCKET.put(key, design.code, { onlyIf: condition(stored) })) return true;
  }
  conflict('Design approval changed concurrently');
}

export async function storeRecord(env, record, picture) {
  const key = `cards/${record.id}.json`;
  // Each candidate owns its picture object. Losing writes can delete theirs without
  // deleting a concurrent winner's identical hash or a later reuse of an old image.
  const pictureKey = record.picture ? `cards/${record.id}/${crypto.randomUUID()}/${record.picture}.jpg` : '';
  let published = false;
  try {
    for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
      const stored = await env.BUCKET.get(key);
      const old = stored ? JSON.parse(await stored.text()) : null;
      if (old && (!Number.isSafeInteger(old.updated) || record.updated <= old.updated)) throw new InputError('Card record is not newer');
      if (record.picture && !await env.BUCKET.head(pictureKey)) {
        let bytes = picture;
        if (!bytes && old?.picture === record.picture && stored.customMetadata?.pictureKey) bytes = (await env.BUCKET.get(stored.customMetadata.pictureKey))?.body;
        if (!bytes) throw new InputError('Missing picture');
        await env.BUCKET.put(pictureKey, bytes, { httpMetadata: { contentType: 'image/jpeg' } });
      }
      if (!await env.BUCKET.put(key, JSON.stringify(record), {
        onlyIf: condition(stored), httpMetadata: { contentType: 'application/json' }, customMetadata: { pictureKey },
      })) continue;
      published = true;
      if (stored?.customMetadata?.pictureKey) await env.BUCKET.delete(stored.customMetadata.pictureKey);
      return;
    }
    conflict('Card record changed concurrently');
  } finally {
    if (!published && pictureKey) await env.BUCKET.delete(pictureKey);
  }
}

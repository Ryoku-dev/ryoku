import test from 'node:test';
import assert from 'node:assert/strict';
import { base64, digest, verifyCode } from '../src/codec.js';
import { call, cardBody, code, jpeg, keys, record, setup } from './helpers.js';

// list counts inventory listings, creating the card index lists cards/ once on its own
function countReads(bucket) {
  const counts = { list: 0, badges: 0, cards: 0, other: 0 };
  const list = bucket.list.bind(bucket), get = bucket.get.bind(bucket);
  bucket.list = async options => { if (!options.prefix.startsWith('cards/')) counts.list++; return list(options); };
  bucket.get = async key => {
    counts[key.startsWith('badges/') ? 'badges' : key.startsWith('cards/') ? 'cards' : 'other']++;
    return get(key);
  };
  return counts;
}
async function publish(state) {
  const text = await code('RYB', state.owner, { device: state.device });
  assert.equal((await call(state.env, 'POST', '/v1/badges', text)).status, 200);
  return text;
}
async function putRecord(state, value, picture) {
  return call(state.env, 'PUT', `/v1/cards/${state.id}`, JSON.stringify({ record: value, ...(picture ? { picture: base64(picture) } : {}) }), { 'Content-Type': 'application/json' });
}

test('inventory memo avoids repeated R2 listings and badge reads for records and rejected writes', async () => {
  const state = await setup(); await publish(state);
  const counts = countReads(state.env.BUCKET);
  assert.equal((await putRecord(state, await record(state.device))).status, 200);
  const list = counts.list, badges = counts.badges;
  for (let i = 0; i < 8; i++) assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}`)).status, 200);
  const unsigned = await record(await keys());
  for (let i = 0; i < 8; i++) assert.equal((await putRecord(state, unsigned)).status, 400);
  assert.equal(counts.list, list);
  assert.equal(counts.badges, badges);
});

test('initial concurrent reads share one in-flight inventory load', async () => {
  const state = await setup(), counts = countReads(state.env.BUCKET);
  const responses = await Promise.all(Array.from({ length: 12 }, () => call(state.env, 'GET', `/v1/cards/${state.id}`)));
  assert.ok(responses.every(response => response.status === 404));
  assert.equal(counts.list, 3);
});

test('a newly posted badge immediately authorizes its card with an already-warm memo', async () => {
  const state = await setup(), counts = countReads(state.env.BUCKET);
  assert.equal((await call(state.env, 'GET', '/v1/list')).status, 200);
  const lists = counts.list;
  await publish(state);
  assert.equal((await putRecord(state, await record(state.device))).status, 200);
  assert.equal(counts.list, lists + 3);
});

test('miss refreshes are throttled to five seconds and memo expires after thirty seconds', async () => {
  const state = await setup(), counts = countReads(state.env.BUCKET), realNow = Date.now;
  let clock = realNow(); Date.now = () => clock;
  try {
    await call(state.env, 'GET', '/v1/list');
    const initial = counts.list;
    const text = await code('RYB', state.owner, { device: state.device });
    const badge = await verifyCode('RYB', text, state.env);
    await state.env.BUCKET.put(`badges/${badge.id}.txt`, text);
    const value = await record(state.device);
    assert.equal((await putRecord(state, value)).status, 400);
    assert.equal(counts.list, initial);
    clock += 5_001;
    assert.equal((await putRecord(state, value)).status, 200);
    const refreshed = counts.list;
    assert.equal(refreshed, initial + 3);
    const invalid = await record(await keys(), { id: state.id, body: base64(cardBody(state.id)) });
    for (let i = 0; i < 8; i++) assert.equal((await putRecord(state, invalid)).status, 400);
    assert.equal(counts.list, refreshed);
    clock += 5_001;
    assert.equal((await putRecord(state, invalid)).status, 400);
    assert.equal(counts.list, refreshed + 3);
    clock += 30_001;
    assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}`)).status, 200);
    assert.equal(counts.list, refreshed + 6);
  } finally { Date.now = realNow; }
});

test('card records and pictures are read fresh past any edge copy, so a change reaches every binder at once', async () => {
  const state = await setup(); await publish(state);
  const original = globalThis.caches, saved = new Map();
  globalThis.caches = { default: {
    async match(request) { return saved.get(request.url)?.clone(); },
    async put(request, response) { saved.set(request.url, response); },
    async delete(request) { return saved.delete(request.url); },
  } };
  try {
    const cardPath = `/v1/cards/${state.id}`;
    assert.equal((await call(state.env, 'GET', cardPath)).status, 404);
    const first = jpeg(24, 32), firstSHA = await digest(first);
    const older = await record(state.device, { picture: firstSHA });
    assert.equal((await putRecord(state, older, first)).status, 200);
    assert.deepEqual(await (await call(state.env, 'GET', cardPath)).json(), older);
    assert.equal((await call(state.env, 'GET', `${cardPath}/${firstSHA}.jpg`)).status, 200);
    const second = jpeg(32, 24), secondSHA = await digest(second);
    const newer = await record(state.device, { picture: secondSHA, updated: older.updated + 1 });
    assert.equal((await putRecord(state, newer, second)).status, 200);
    assert.deepEqual(await (await call(state.env, 'GET', cardPath)).json(), newer);
    assert.deepEqual(new Uint8Array(await (await call(state.env, 'GET', `${cardPath}/${secondSHA}.jpg`)).arrayBuffer()), second);
    assert.equal((await call(state.env, 'GET', `${cardPath}/${firstSHA}.jpg`)).status, 404);
    assert.ok([...saved.keys()].every(url => !url.includes('/v1/cards/')), 'card responses were cached');
  } finally { if (original === undefined) delete globalThis.caches; else globalThis.caches = original; }
});

test('per-client rate limits use separate read and write bindings and return JSON 429', async () => {
  const state = await setup(), seen = [];
  function limiter(limit, name) {
    const counts = new Map();
    return { async limit({ key }) {
      seen.push([name, key]);
      const count = (counts.get(key) || 0) + 1;
      counts.set(key, count);
      return { success: count <= limit };
    } };
  }
  state.env.READ_LIMITER = limiter(120, 'read');
  state.env.WRITE_LIMITER = limiter(20, 'write');
  const headers = { 'CF-Connecting-IP': '192.0.2.7' };
  for (let i = 0; i < 120; i++) assert.equal((await call(state.env, 'GET', '/missing', undefined, headers)).status, 404);
  const blocked = await call(state.env, 'GET', '/missing', undefined, headers);
  assert.equal(blocked.status, 429);
  assert.deepEqual(await blocked.json(), { error: 'Rate limit exceeded' });
  assert.equal(blocked.headers.get('Retry-After'), '60');
  for (let i = 0; i < 20; i++) assert.equal((await call(state.env, 'POST', '/missing', undefined, headers)).status, 404);
  assert.equal((await call(state.env, 'POST', '/missing', undefined, headers)).status, 429);
  assert.equal((await call(state.env, 'GET', '/missing', undefined, { 'CF-Connecting-IP': '192.0.2.8' })).status, 404);
  assert.equal((await call(state.env, 'GET', '/missing')).status, 404);
  assert.deepEqual(seen.at(-1), ['read', 'unknown']);
  assert.ok(seen.some(([name, key]) => name === 'write' && key === '192.0.2.7'));
});

test('accepted badge, approval and revocation writes refresh inventory and invalidate public caches', async () => {
  const state = await setup(), original = globalThis.caches, saved = new Map();
  globalThis.caches = { default: {
    async match(request) { return saved.get(request.url)?.clone(); },
    async put(request, response) { saved.set(request.url, response); },
    async delete(request) { return saved.delete(request.url); },
  } };
  const list = async () => (await call(state.env, 'GET', '/v1/list?fresh=1')).json();
  try {
    assert.deepEqual((await list()).badges, []);
    const badgeCode = await publish(state);
    const badge = await verifyCode('RYB', badgeCode, state.env);
    assert.deepEqual((await list()).badges, [badgeCode]);
    const sha = 'ab'.repeat(32);
    await state.env.BUCKET.put(`pending/${sha}.zip`, new Uint8Array([80, 75]));
    const approval = await code('RYD', state.owner, { sha });
    assert.deepEqual((await list()).designs, []);
    assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 200);
    assert.deepEqual((await list()).designs, [approval]);
    saved.set('https://team.ryoku.dev/v1/list', new Response(JSON.stringify({ v: 1, badges: [], revoked: [], designs: [] })));
    assert.equal((await call(state.env, 'POST', '/v1/badges', badgeCode)).status, 200);
    assert.deepEqual((await list()).badges, [badgeCode]);
    saved.set('https://team.ryoku.dev/v1/list', new Response(JSON.stringify({ v: 1, badges: [], revoked: [], designs: [] })));
    assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 200);
    assert.deepEqual((await list()).designs, [approval]);
    const image = jpeg(24, 32), imageSHA = await digest(image);
    assert.equal((await putRecord(state, await record(state.device, { picture: imageSHA }), image)).status, 200);
    const cardPath = `/v1/cards/${state.id}`, picturePath = `${cardPath}/${imageSHA}.jpg`;
    assert.equal((await call(state.env, 'GET', cardPath + '?cache=1')).status, 200);
    assert.equal((await call(state.env, 'GET', picturePath + '?cache=1')).status, 200);
    const revocation = await code('RYX', state.owner, { id: badge.id });
    assert.equal((await call(state.env, 'POST', '/v1/revoke', revocation)).status, 200);
    const revokedList = await list();
    assert.deepEqual(revokedList.revoked, [revocation]);
    assert.deepEqual(revokedList.designs, []);
    assert.equal((await call(state.env, 'GET', cardPath)).status, 404);
    assert.equal((await call(state.env, 'GET', picturePath)).status, 404);
    saved.set('https://team.ryoku.dev/v1/list', new Response(JSON.stringify({ v: 1, badges: [], revoked: [], designs: [] })));
    assert.equal((await call(state.env, 'POST', '/v1/revoke', revocation)).status, 200);
    assert.deepEqual((await list()).revoked, [revocation]);
  } finally { if (original === undefined) delete globalThis.caches; else globalThis.caches = original; }
});

import test from 'node:test';
import assert from 'node:assert/strict';
import worker from '../src/index.js';
import { base64, digest } from '../src/codec.js';
import { call, code, jpeg, keys, now, record, setup, uploadHeader } from './helpers.js';

async function publish(state, fields = {}) {
  const text = await code('RYB', fields.issuer === 2 ? state.mika : state.owner, { device: state.device, ...fields });
  const response = await call(state.env, 'POST', '/v1/badges', text);
  assert.equal(response.status, 200);
  return { text, ...(await response.json()) };
}
async function putRecord(state, value, picture) {
  return call(state.env, 'PUT', `/v1/cards/${state.id}`, JSON.stringify({ record: value, ...(picture ? { picture: base64(picture) } : {}) }), { 'Content-Type': 'application/json' });
}
async function pending(state, bytes) {
  const sha = await digest(bytes), path = `/v1/pending/${sha}`;
  const response = await call(state.env, 'PUT', path, bytes, { 'Content-Type': 'application/zip', 'X-Ryoku-Upload': await uploadHeader(state.owner, 1, path, bytes) });
  assert.equal(response.status, 200);
  return sha;
}

test('badge POST is idempotent, list paginates and filters corrupt or forged storage', async () => {
  const state = await setup(), expected = [];
  for (let i = 0; i < 5; i++) expected.push((await publish(state, { login: `member${i}`, role: 1 })).text);
  const first = await call(state.env, 'POST', '/v1/badges', expected[0]);
  assert.equal(first.status, 200);
  await state.env.BUCKET.put('badges/0000000000000000.txt', 'invalid');
  await state.env.BUCKET.put('badges/misnamed.txt', expected[0]);
  const forged = await code('RYB', await keys(), { device: state.device });
  assert.equal((await call(state.env, 'POST', '/v1/badges', forged)).status, 400);
  const response = await call(state.env, 'GET', '/v1/list');
  assert.equal(response.headers.get('Cache-Control'), 'public, max-age=60');
  const list = await response.json();
  assert.deepEqual(list.badges.sort(), expected.sort());
  assert.deepEqual(list.revoked, []);
  assert.deepEqual(list.designs, []);
});

test('records require an active device-bound badge and reject replay or stale data', async () => {
  const state = await setup();
  const value = await record(state.device);
  assert.equal((await putRecord(state, value)).status, 400);
  const badge = await publish(state);
  assert.equal((await putRecord(state, await record(await keys()))).status, 400);
  assert.equal((await putRecord(state, await record(state.device, { updated: now() - 601 }))).status, 400);
  assert.equal((await putRecord(state, await record(state.device, { updated: now() + 601 }))).status, 400);
  assert.equal((await putRecord(state, value)).status, 200);
  assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${state.id}`)).json(), value);
  assert.equal((await putRecord(state, value)).status, 400);
  assert.equal((await putRecord(state, await record(state.device, { updated: value.updated - 1 }))).status, 400);
  const revoke = await code('RYX', state.owner, { id: badge.id });
  assert.equal((await call(state.env, 'POST', '/v1/revoke', revoke)).status, 200);
  assert.equal((await putRecord(state, await record(state.device, { updated: value.updated + 1 }))).status, 400);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}`)).status, 404);
  const list = await (await call(state.env, 'GET', '/v1/list')).json();
  assert.deepEqual(list.revoked, [revoke]);
  assert.deepEqual(list.badges, [badge.text]);
});

test('Mika cannot revoke owner badges or unknown ids, but can revoke Mika badges', async () => {
  const state = await setup(), ownerBadge = await publish(state), mikaBadge = await publish(state, { issuer: 2, role: 2 });
  for (const id of [ownerBadge.id, '00'.repeat(8)]) assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.mika, { issuer: 2, id }))).status, 400);
  const revoke = await code('RYX', state.mika, { issuer: 2, id: mikaBadge.id });
  assert.equal((await call(state.env, 'POST', '/v1/revoke', revoke)).status, 200);
  assert.deepEqual((await (await call(state.env, 'GET', '/v1/list')).json()).revoked, [revoke]);
  await state.env.BUCKET.put(`revoked/${ownerBadge.id}.txt`, await code('RYX', state.mika, { issuer: 2, id: ownerBadge.id }));
  assert.deepEqual((await (await call(state.env, 'GET', '/v1/list')).json()).revoked, [revoke]);
});

test('picture hash, size and dimensions are checked and replaced pictures are deleted', async () => {
  const state = await setup(); await publish(state);
  let updated = now();
  const first = jpeg(20, 30), firstSha = await digest(first);
  let value = await record(state.device, { updated, picture: firstSha });
  assert.equal((await putRecord(state, value, first)).status, 200);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}/${firstSha}.jpg`)).headers.get('Content-Type'), 'image/jpeg');
  const oversized = jpeg(1121, 780);
  value = await record(state.device, { updated: ++updated, picture: await digest(oversized) });
  assert.equal((await putRecord(state, value, oversized)).status, 400);
  const tooManyBytes = new Uint8Array(400 * 1024 + 1); tooManyBytes.set(first);
  value = await record(state.device, { updated: ++updated, picture: await digest(tooManyBytes) });
  assert.equal((await putRecord(state, value, tooManyBytes)).status, 413);
  const second = jpeg(40, 50), secondSha = await digest(second);
  value = await record(state.device, { updated: ++updated, picture: secondSha });
  assert.equal((await putRecord(state, value, first)).status, 400);
  assert.equal((await putRecord(state, value)).status, 400);
  assert.equal((await putRecord(state, value, second)).status, 200);
  assert.ok(![...state.env.BUCKET.data.keys()].some(key => key.endsWith(`/${firstSha}.jpg`)));
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}/${firstSha}.jpg`)).status, 404);
  value = await record(state.device, { updated: ++updated });
  assert.equal((await putRecord(state, value)).status, 200);
  assert.ok(![...state.env.BUCKET.data.keys()].some(key => key.endsWith(`/${secondSha}.jpg`)));
});

test('approved designs require team membership and packs, honor ordering and disappear on revocation', async () => {
  const state = await setup();
  const packBytes = new Uint8Array([80, 75, 3, 4]), sha = await pending(state, packBytes);
  const approval = await code('RYD', state.mika, { issuer: 2, sha, login: 'Neur0map' });
  assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 400);
  await publish(state, { role: 2 });
  assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 400);
  const badge = await publish(state);
  const missing = await code('RYD', state.owner, { sha: 'ab'.repeat(32) });
  assert.equal((await call(state.env, 'POST', '/v1/designs', missing)).status, 404);
  assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 200);
  assert.ok(await state.env.BUCKET.head(`designs/neur0map.txt`));
  assert.deepEqual(new Uint8Array(await (await call(state.env, 'GET', `/v1/packs/${sha}.zip`)).arrayBuffer()), packBytes);
  assert.equal((await call(state.env, 'GET', `/v1/pending/${sha}.zip`)).status, 200);
  assert.deepEqual((await (await call(state.env, 'GET', '/v1/list')).json()).designs, [approval]);
  assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 200);
  const older = await code('RYD', state.owner, { sha, approvedAt: 1794697199 });
  assert.equal((await call(state.env, 'POST', '/v1/designs', older)).status, 409);
  const sameTime = await code('RYD', state.owner, { sha });
  assert.equal((await call(state.env, 'POST', '/v1/designs', sameTime)).status, 409);
  const newer = await code('RYD', state.owner, { sha, approvedAt: 1794697201 });
  assert.equal((await call(state.env, 'POST', '/v1/designs', newer)).status, 200);
  assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 409);
  assert.deepEqual((await (await call(state.env, 'GET', '/v1/list')).json()).designs, [newer]);
  await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id: badge.id }));
  assert.deepEqual((await (await call(state.env, 'GET', '/v1/list')).json()).designs, []);
});

test('pending uploads require correct signed authorization, hash and clock window', async () => {
  const state = await setup(), bytes = new Uint8Array([80, 75, 3, 4]), sha = await digest(bytes), path = `/v1/pending/${sha}`;
  assert.equal((await call(state.env, 'PUT', path, bytes, { 'Content-Type': 'application/zip' })).status, 400);
  for (const seconds of [now() - 301, now() + 301]) assert.equal((await call(state.env, 'PUT', path, bytes, { 'Content-Type': 'application/zip', 'X-Ryoku-Upload': await uploadHeader(state.owner, 1, path, bytes, seconds) })).status, 400);
  const badPath = `/v1/pending/${'ab'.repeat(32)}`;
  assert.equal((await call(state.env, 'PUT', badPath, bytes, { 'Content-Type': 'application/zip', 'X-Ryoku-Upload': await uploadHeader(state.owner, 1, badPath, bytes) })).status, 400);
  assert.equal(await pending(state, bytes), sha);
});

test('rejects missing or oversize length before reading and caps dishonest streams', async () => {
  const state = await setup();
  for (const [path, type, cap] of [['/v1/badges', 'text/plain', 2400], [`/v1/cards/${state.id}`, 'application/json', 600 * 1024]]) {
    for (const length of [undefined, String(cap + 1)]) {
      let read = false;
      const body = new ReadableStream({ pull() { read = true; throw new Error('Must not read'); } }, { highWaterMark: 0 });
      const response = await worker.fetch(new Request(`https://team.ryoku.dev${path}`, { method: path.includes('badges') ? 'POST' : 'PUT', body, duplex: 'half', headers: { 'Content-Type': type, ...(length ? { 'Content-Length': length } : {}) } }), state.env);
      assert.equal(response.status, length ? 413 : 400);
      assert.equal(read, false);
    }
  }
  let cancelled = false;
  const body = new ReadableStream({ start(controller) { controller.enqueue(new Uint8Array(2401)); }, cancel() { cancelled = true; } });
  const response = await worker.fetch(new Request('https://team.ryoku.dev/v1/badges', { method: 'POST', body, duplex: 'half', headers: { 'Content-Type': 'text/plain', 'Content-Length': '1' } }), state.env);
  assert.equal(response.status, 413);
  assert.equal(cancelled, true);
  const bytes = new Uint8Array([80, 75]), sha = await digest(bytes), path = `/v1/pending/${sha}`;
  assert.equal((await call(state.env, 'PUT', path, bytes, { 'Content-Type': 'application/zip', 'Content-Length': String(8 * 1024 * 1024 + 1), 'X-Ryoku-Upload': await uploadHeader(state.owner, 1, path, bytes) })).status, 413);
  assert.equal((await call(state.env, 'POST', '/v1/badges', 'x', { 'Content-Type': 'text/plain; charset=utf-8' })).status, 415);
  assert.equal((await call(state.env, 'PUT', `/v1/cards/${state.id}`, '{', { 'Content-Type': 'application/json' })).status, 400);
});

test('unknown paths, malformed identifiers, absent objects and HTTP are refused', async () => {
  const state = await setup();
  for (const path of ['/v1/nope', '/v1/cards/abcdef0', '/v1/cards/ABCDEF', `/v1/packs/${'ab'.repeat(31)}.zip`, '/v1/cards/abcdef', `/v1/packs/${'ab'.repeat(32)}.zip`]) assert.equal((await call(state.env, 'GET', path)).status, 404);
  assert.equal((await worker.fetch(new Request('http://team.ryoku.dev/v1/list'), state.env)).status, 400);
});

test('Cache API receives list responses with a 60-second lifetime', async () => {
  const state = await setup(); await publish(state);
  const saved = new Map();
  const original = globalThis.caches;
  globalThis.caches = { default: { async match(request) { return saved.get(request.url)?.clone(); }, async put(request, response) { saved.set(request.url, response); } } };
  try {
    assert.equal((await call(state.env, 'GET', '/v1/list')).status, 200);
    assert.equal(saved.size, 1);
    state.env.BUCKET.list = () => { throw new Error('cache should be hit'); };
    assert.equal((await call(state.env, 'GET', '/v1/list')).headers.get('Cache-Control'), 'public, max-age=60');
  } finally { if (original === undefined) delete globalThis.caches; else globalThis.caches = original; }
});

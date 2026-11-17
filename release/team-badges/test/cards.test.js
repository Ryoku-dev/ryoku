import test from 'node:test';
import assert from 'node:assert/strict';
import worker from '../src/index.js';
import { base64, cardId, digest, verifyCode } from '../src/codec.js';
import { Bucket, call, cardBody, code, deleteRequest, deviceRecord, jpeg, keys, now, record, setup } from './helpers.js';

const DAY = 86400;
const JSON_TYPE = { 'Content-Type': 'application/json' };
const putCard = (state, value, picture) => call(state.env, 'PUT', `/v1/cards/${value.id}`, JSON.stringify({ record: value, ...(picture ? { picture: base64(picture) } : {}) }), JSON_TYPE);
const deleteCard = (state, id, body) => call(state.env, 'DELETE', `/v1/cards/${id}`, JSON.stringify(body), JSON_TYPE);
const pull = (state, cards, headers = {}) => call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards }), { ...JSON_TYPE, ...headers });
const index = state => JSON.parse(new TextDecoder().decode(state.env.BUCKET.data.get('cards/index.json'))).cards;
const pictures = state => [...state.env.BUCKET.data.keys()].filter(key => key.endsWith('.jpg'));
async function friend() {
  const device = await keys();
  return { device, id: await cardId(device.raw) };
}
async function publish(state, fields = {}) {
  const text = await code('RYB', state.owner, { device: state.device, ...fields });
  assert.equal((await call(state.env, 'POST', '/v1/badges', text)).status, 200);
  return text;
}
function interleaveIndex(bucket, action) {
  const put = bucket.put.bind(bucket);
  let once = false;
  bucket.put = async (key, value, options) => {
    if (key === 'cards/index.json' && options?.onlyIf && !once) { once = true; await action(); }
    return put(key, value, options);
  };
}
async function withClock(start, body) {
  const realNow = Date.now, clock = { at: start };
  Date.now = () => clock.at;
  try { await body(clock); } finally { Date.now = realNow; }
}

test('a device record without any badge is accepted, served and its picture with it', async () => {
  const state = await setup(), image = jpeg(30, 40), sha = await digest(image);
  const value = await deviceRecord(state.device, { picture: sha });
  assert.equal((await putCard(state, value, image)).status, 200);
  assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${state.id}`)).json(), value);
  assert.deepEqual(new Uint8Array(await (await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).arrayBuffer()), image);
  assert.deepEqual(index(state), { [state.id]: value.updated });
  const newer = await deviceRecord(state.device, { updated: value.updated + 1 });
  assert.equal((await putCard(state, newer)).status, 200);
  assert.deepEqual(index(state), { [state.id]: newer.updated });
  assert.deepEqual(pictures(state), []);
  const replay = await putCard(state, value, image);
  assert.equal(replay.status, 400);
  assert.deepEqual(await replay.json(), { error: 'Card record is not newer' });
});

test('forged or mismatched device keys and ids are refused', async () => {
  const state = await setup(), attacker = await friend();
  const refusal = async (value, path = value.id) => {
    const response = await call(state.env, 'PUT', `/v1/cards/${path}`, JSON.stringify({ record: value }), JSON_TYPE);
    return [response.status, (await response.json()).error];
  };
  // the attacker signs the victim's id with their own key named, which doesn't derive that id
  const claimed = await record(attacker.device, { id: state.id, body: base64(cardBody(state.id)), device: base64(attacker.device.raw) });
  assert.deepEqual(await refusal(claimed), [400, 'Invalid card signature']);
  assert.deepEqual(await refusal({ ...claimed, device: base64(state.device.raw) }), [400, 'Invalid card signature']);
  assert.deepEqual(await refusal(await deviceRecord(attacker.device), state.id), [400, 'Invalid card id or timestamp']);
  const genuine = await deviceRecord(state.device);
  assert.deepEqual(await refusal({ ...genuine, updated: genuine.updated + 1 }), [400, 'Invalid card signature']);
  assert.deepEqual(await refusal({ ...genuine, device: base64(state.device.raw.slice(0, 31)) }), [400, 'Invalid card device']);
  assert.deepEqual(await refusal(await record(state.device)), [400, 'No active badge verifies this record']);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}`)).status, 404);
  assert.equal(state.env.BUCKET.data.has('cards/index.json'), false);
});

test('badge records without a device keep working and a revoked badge leaves device records live', async () => {
  const state = await setup(), badge = await publish(state);
  const legacy = await record(state.device);
  assert.equal((await putCard(state, legacy)).status, 200);
  assert.deepEqual(index(state), { [state.id]: legacy.updated });
  const live = await deviceRecord(state.device, { updated: legacy.updated + 1 });
  assert.equal((await putCard(state, live)).status, 200);
  const { id } = await verifyCode('RYB', badge, state.env);
  assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id }))).status, 200);
  assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${state.id}`)).json(), live);
});

test('delete needs the device signature and removes record, picture and index entry', async () => {
  const state = await setup(), stranger = await keys(), image = jpeg(30, 40);
  const value = await deviceRecord(state.device, { picture: await digest(image) });
  assert.equal((await putCard(state, value, image)).status, 200);
  const refused = async body => {
    const response = await deleteCard(state, state.id, body);
    return [response.status, (await response.json()).error];
  };
  assert.deepEqual(await refused(await deleteRequest(stranger, state.id)), [400, 'Invalid delete signature']);
  assert.deepEqual(await refused(await deleteRequest(state.device, state.id, now() - 601)), [400, 'Invalid card id or timestamp']);
  const signed = await deleteRequest(state.device, state.id);
  assert.deepEqual(await refused({ ...signed, at: signed.at + 1 }), [400, 'Invalid delete signature']);
  assert.deepEqual(await refused({ ...signed, extra: 1 }), [400, 'Invalid card delete']);
  assert.equal((await call(state.env, 'DELETE', `/v1/cards/${state.id}`, JSON.stringify(signed), { 'Content-Type': 'application/json', 'Content-Length': '1025' })).status, 413);
  assert.equal(pictures(state).length, 1);
  const deleted = await deleteCard(state, state.id, signed);
  assert.equal(deleted.status, 200);
  assert.deepEqual(await deleted.json(), { id: state.id });
  assert.equal(state.env.BUCKET.data.has(`cards/${state.id}.json`), false);
  assert.deepEqual(pictures(state), []);
  assert.deepEqual(index(state), {});
  assert.equal((await deleteCard(state, state.id, signed)).status, 404);
  // a copy fetched before the delete stays refused, the owner's next record comes back
  const replay = await putCard(state, value, image);
  assert.deepEqual([replay.status, (await replay.json()).error], [400, 'Card record is not newer']);
  assert.equal((await putCard(state, await deviceRecord(state.device, { updated: value.updated + 1 }))).status, 200);
  assert.deepEqual(Object.keys(index(state)), [state.id]);
});

test('an active badge device may delete its record that carries no device key', async () => {
  const state = await setup(); await publish(state);
  assert.equal((await putCard(state, await record(state.device))).status, 200);
  assert.equal((await deleteCard(state, state.id, await deleteRequest(state.device, state.id))).status, 200);
  assert.deepEqual(index(state), {});
});

test('batch returns newer records, gone ids, requested badges and team designs, never cached', async () => {
  const state = await setup(), other = await friend(), absent = await friend();
  const badge = await publish(state);
  const sha = 'ab'.repeat(32);
  await state.env.BUCKET.put(`packs/${sha}.zip`, new Uint8Array([80, 75]));
  const approval = await code('RYD', state.owner, { sha });
  assert.equal((await call(state.env, 'POST', '/v1/designs', approval)).status, 200);
  const otherBadge = await code('RYB', state.owner, { device: other.device, login: 'friend', role: 1 });
  assert.equal((await call(state.env, 'POST', '/v1/badges', otherBadge)).status, 200);
  const mine = await deviceRecord(state.device), theirs = await deviceRecord(other.device);
  for (const value of [mine, theirs]) assert.equal((await putCard(state, value)).status, 200);
  const original = globalThis.caches, cached = [];
  globalThis.caches = { default: { async match(request) { cached.push(request.url); }, async put(request) { cached.push(request.url); } } };
  try {
    const response = await pull(state, { [state.id]: 0, [other.id]: theirs.updated, [absent.id]: 7 });
    assert.equal(response.status, 200);
    assert.equal(response.headers.get('Cache-Control'), 'no-store');
    const body = await response.json();
    assert.ok(Math.abs(body.now - now()) <= 2);
    assert.equal(body.next, body.now + 900);
    assert.deepEqual(body.records, [mine]);
    assert.deepEqual(body.gone, [absent.id]);
    assert.deepEqual(body.badges.sort(), [badge, otherBadge].sort());
    assert.deepEqual(body.designs, [approval]);
    const theirsOnly = await (await pull(state, { [other.id]: theirs.updated - 1 })).json();
    assert.deepEqual([theirsOnly.records, theirsOnly.gone, theirsOnly.badges, theirsOnly.designs], [[theirs], [], [otherBadge], []]);
    assert.deepEqual(cached, []);
  } finally { if (original === undefined) delete globalThis.caches; else globalThis.caches = original; }
  state.env.PULL_SECONDS = '120';
  const custom = await (await pull(state, { [absent.id]: 0 })).json();
  assert.deepEqual([custom.next - custom.now, custom.records, custom.gone, custom.badges, custom.designs], [120, [], [absent.id], [], []]);
});

test('batch sees a write from the same isolate at once and records stored before the index existed', async () => {
  const written = await setup(), other = await friend();
  await publish(written);
  const legacy = await record(written.device);
  assert.equal((await putCard(written, legacy)).status, 200);
  // same storage as a gate that never kept an index, seen from a fresh isolate
  const state = { ...written, env: { ...written.env, BUCKET: new Bucket() } };
  for (const [key, value] of written.env.BUCKET.data) {
    if (key === 'cards/index.json') continue;
    state.env.BUCKET.data.set(key, value);
    state.env.BUCKET.metadata.set(key, written.env.BUCKET.metadata.get(key));
  }
  const before = await (await pull(state, { [state.id]: 0, [other.id]: 0 })).json();
  assert.deepEqual([before.records, before.gone], [[legacy], [other.id]]);
  assert.equal(state.env.BUCKET.data.has('cards/index.json'), false);
  const theirs = await deviceRecord(other.device);
  assert.equal((await putCard(state, theirs)).status, 200);
  const body = await (await pull(state, { [state.id]: 0, [other.id]: 0 })).json();
  assert.deepEqual([body.records, body.gone], [[legacy, theirs], []]);
  assert.deepEqual(index(state), { [state.id]: legacy.updated, [other.id]: theirs.updated });
});

test('batch validates its body and counts against the read and pull limiters with a next time', async () => {
  const state = await setup(), seen = [];
  const limiter = (name, limit) => {
    let count = 0;
    return { async limit({ key }) { seen.push([name, key]); return { success: ++count <= limit }; } };
  };
  const many = Object.fromEntries(Array.from({ length: 201 }, (_, i) => [i.toString(16).padStart(16, '0'), 0]));
  for (const cards of [{}, many, { [state.id]: -1 }, { [state.id]: 1.5 }, { ABCDEF0123456789: 0 }]) assert.equal((await pull(state, cards)).status, 400);
  assert.equal((await call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards: { [state.id]: 0 }, extra: 1 }), JSON_TYPE)).status, 400);
  assert.equal((await call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards: { [state.id]: 0 } }))).status, 415);
  assert.equal((await call(state.env, 'POST', '/v1/cards/batch', '{}', { ...JSON_TYPE, 'Content-Length': String(16 * 1024 + 1) })).status, 413);
  assert.equal((await pull(state, Object.fromEntries(Object.entries(many).slice(0, 200)))).status, 200);
  state.env.READ_LIMITER = limiter('read', 100);
  state.env.WRITE_LIMITER = limiter('write', 0);
  state.env.PULL_LIMITER = limiter('pull', 10);
  const headers = { 'CF-Connecting-IP': '192.0.2.9' };
  for (let i = 0; i < 10; i++) assert.equal((await pull(state, { [state.id]: 0 }, headers)).status, 200);
  const blocked = await pull(state, { [state.id]: 0 }, headers);
  assert.equal(blocked.status, 429);
  assert.equal(blocked.headers.get('Retry-After'), '60');
  const body = await blocked.json();
  assert.equal(body.error, 'Rate limit exceeded');
  assert.ok(Math.abs(body.next - (now() + 60)) <= 2);
  assert.ok(seen.every(([name, key]) => name !== 'write' && key === '192.0.2.9'));
  assert.equal(seen.filter(([name]) => name === 'read').length, 11);
});

test('CARD_LIMIT refuses a new id but keeps accepting updates and frees room on delete', async () => {
  const state = await setup(), other = await friend();
  state.env.CARD_LIMIT = '1';
  const first = await deviceRecord(state.device);
  assert.equal((await putCard(state, first)).status, 200);
  const full = await putCard(state, await deviceRecord(other.device));
  assert.equal(full.status, 507);
  assert.deepEqual(await full.json(), { error: 'Card service is full' });
  assert.equal(state.env.BUCKET.data.has(`cards/${other.id}.json`), false);
  assert.equal((await putCard(state, await deviceRecord(state.device, { updated: first.updated + 1 }))).status, 200);
  assert.equal((await deleteCard(state, state.id, await deleteRequest(state.device, state.id))).status, 200);
  assert.equal((await putCard(state, await deviceRecord(other.device))).status, 200);
  assert.deepEqual(Object.keys(index(state)), [other.id]);
});

test('scheduled cleanup removes only records idle for more than 90 days, with their pictures', async () => {
  const state = await setup(), other = await friend(), start = Date.now();
  await withClock(start, async clock => {
    const image = jpeg(30, 40), oldSha = await digest(image);
    assert.equal((await putCard(state, await deviceRecord(state.device, { picture: oldSha }), image)).status, 200);
    clock.at += 2 * DAY * 1000;
    const recent = jpeg(40, 30);
    const kept = await deviceRecord(other.device, { picture: await digest(recent) });
    assert.equal((await putCard(state, kept, recent)).status, 200);
    await worker.scheduled({ scheduledTime: start + 89 * DAY * 1000 }, state.env);
    assert.equal(Object.keys(index(state)).length, 2);
    clock.at = start + 91 * DAY * 1000;
    await worker.scheduled({ scheduledTime: clock.at }, state.env);
    assert.deepEqual(index(state), { [other.id]: kept.updated });
    assert.equal(state.env.BUCKET.data.has(`cards/${state.id}.json`), false);
    assert.deepEqual(pictures(state).map(key => key.split('/').at(-1)), [`${kept.picture}.jpg`]);
    assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${other.id}`)).json(), kept);
    const body = await (await pull(state, { [state.id]: 0, [other.id]: 0 })).json();
    assert.deepEqual([body.records, body.gone], [[kept], [state.id]]);
  });
});

test('index stays consistent when card writes and deletes race its conditional update', async () => {
  const state = await setup(), other = await friend();
  const mine = await deviceRecord(state.device), theirs = await deviceRecord(other.device);
  interleaveIndex(state.env.BUCKET, async () => assert.equal((await putCard(state, theirs)).status, 200));
  assert.equal((await putCard(state, mine)).status, 200);
  assert.deepEqual(index(state), { [state.id]: mine.updated, [other.id]: theirs.updated });
  // the old record uploaded while its delete is being indexed must not come back
  interleaveIndex(state.env.BUCKET, async () => assert.equal((await putCard(state, mine)).status, 200));
  assert.equal((await deleteCard(state, state.id, await deleteRequest(state.device, state.id))).status, 200);
  assert.deepEqual(index(state), { [other.id]: theirs.updated });
  assert.equal(state.env.BUCKET.data.has(`cards/${state.id}.json`), false);
  const newer = await deviceRecord(other.device, { updated: theirs.updated + 1 });
  interleaveIndex(state.env.BUCKET, async () => assert.equal((await deleteCard(state, other.id, await deleteRequest(other.device, other.id))).status, 200));
  assert.equal((await putCard(state, newer)).status, 200);
  assert.deepEqual(index(state), {});
  assert.equal(state.env.BUCKET.data.has(`cards/${other.id}.json`), false);
});

test('batch serves stored device records without checking a single signature', async () => {
  const state = await setup(), other = await friend();
  const mine = await deviceRecord(state.device), theirs = await deviceRecord(other.device);
  for (const value of [mine, theirs]) assert.equal((await putCard(state, value)).status, 200);
  const subtle = globalThis.crypto.subtle, verify = subtle.verify;
  let calls = 0;
  subtle.verify = function (...args) { calls++; return verify.apply(this, args); };
  try {
    const body = await (await pull(state, { [state.id]: 0, [other.id]: 0 })).json();
    assert.deepEqual(body.records, [mine, theirs]);
    assert.equal(calls, 0);
  } finally { delete subtle.verify; }
  assert.equal(subtle.verify, verify);
  // a stored record naming a key that doesn't derive its id is still left out
  const stored = JSON.parse(new TextDecoder().decode(state.env.BUCKET.data.get(`cards/${other.id}.json`)));
  await state.env.BUCKET.put(`cards/${other.id}.json`, JSON.stringify({ ...stored, device: mine.device }));
  assert.deepEqual((await (await pull(state, { [other.id]: 0 })).json()).records, []);
});

test('a badge-only record leaves the batch once its badge is revoked', async () => {
  const state = await setup(), badge = await publish(state);
  const legacy = await record(state.device);
  assert.equal((await putCard(state, legacy)).status, 200);
  assert.deepEqual((await (await pull(state, { [state.id]: 0 })).json()).records, [legacy]);
  const { id } = await verifyCode('RYB', badge, state.env);
  assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id }))).status, 200);
  const body = await (await pull(state, { [state.id]: 0 })).json();
  assert.deepEqual([body.records, body.gone, body.badges], [[], [], []]);
});

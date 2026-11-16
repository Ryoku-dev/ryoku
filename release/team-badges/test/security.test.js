import test from 'node:test';
import assert from 'node:assert/strict';
import { base64, digest, verifyCode } from '../src/codec.js';
import { call, cardBody, code, jpeg, keys, now, record, setup } from './helpers.js';

async function publish(state, fields = {}) {
  const text = await code('RYB', state.owner, { device: state.device, ...fields });
  const response = await call(state.env, 'POST', '/v1/badges', text);
  assert.equal(response.status, 200);
  return { text, ...await response.json() };
}
function putRecord(state, value, picture) {
  return call(state.env, 'PUT', `/v1/cards/${state.id}`, JSON.stringify({ record: value, ...(picture ? { picture: base64(picture) } : {}) }), { 'Content-Type': 'application/json' });
}
function interleave(bucket, key, action) {
  const put = bucket.put.bind(bucket);
  let once = false;
  bucket.put = async (path, value, options) => {
    if (path === key && options?.onlyIf && !once) {
      once = true;
      await action(options.onlyIf);
    }
    return put(path, value, options);
  };
}

test('a badge for another key cannot claim an existing card id, even with a signed matching body', async () => {
  const state = await setup(), attacker = await keys();
  await publish(state, { device: attacker, role: 1 });
  const forged = await record(attacker, { id: state.id, body: base64(cardBody(state.id)) });
  assert.equal((await putRecord(state, forged)).status, 400);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}`)).status, 404);
  await publish(state);
  assert.equal((await putRecord(state, forged)).status, 400);
  assert.equal((await putRecord(state, await record(state.device))).status, 200);
});

test('record create-only and etag races cannot let an older upload land last', async () => {
  for (const seeded of [false, true]) {
    const state = await setup(); await publish(state);
    const time = now();
    if (seeded) assert.equal((await putRecord(state, await record(state.device, { updated: time - 1 }))).status, 200);
    const older = await record(state.device, { updated: time }), newer = await record(state.device, { updated: time + 1 });
    interleave(state.env.BUCKET, `cards/${state.id}.json`, async condition => {
      assert.ok(seeded ? condition.etagMatches : condition.etagDoesNotMatch === '*');
      assert.equal((await putRecord(state, newer)).status, 200);
    });
    assert.equal((await putRecord(state, older)).status, 400);
    assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${state.id}`)).json(), newer);
  }
});

test('a newer record retries a lost race and preserves its picture while deleting predecessors', async () => {
  const state = await setup(); await publish(state);
  const time = now(), image = jpeg(30, 40), sha = await digest(image);
  const older = await record(state.device, { updated: time, picture: sha });
  const newer = await record(state.device, { updated: time + 1, picture: sha });
  interleave(state.env.BUCKET, `cards/${state.id}.json`, async () => {
    assert.equal((await putRecord(state, older, image)).status, 200);
  });
  assert.equal((await putRecord(state, newer, image)).status, 200);
  const objects = [...state.env.BUCKET.data.keys()].filter(key => key.endsWith('.jpg'));
  assert.equal(objects.length, 1);
  assert.deepEqual(new Uint8Array(await (await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).arrayBuffer()), image);
  assert.equal((await putRecord(state, await record(state.device, { updated: time + 2, picture: sha }))).status, 200);
  assert.equal([...state.env.BUCKET.data.keys()].filter(key => key.endsWith('.jpg')).length, 1);
});

test('losing same-hash picture uploads clean only their own candidates', async () => {
  const state = await setup(); await publish(state);
  const image = jpeg(30, 40), sha = await digest(image), time = now();
  const older = await record(state.device, { updated: time, picture: sha });
  const newer = await record(state.device, { updated: time + 1, picture: sha });
  interleave(state.env.BUCKET, `cards/${state.id}.json`, async () => {
    assert.equal((await putRecord(state, newer, image)).status, 200);
  });
  assert.equal((await putRecord(state, older, image)).status, 400);
  assert.equal([...state.env.BUCKET.data.keys()].filter(key => key.endsWith('.jpg')).length, 1);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).status, 200);
});

test('design conditional writes compare again after create and update races', async () => {
  for (const seeded of [false, true]) {
    const state = await setup(); await publish(state);
    const sha = 'ab'.repeat(32);
    await state.env.BUCKET.put(`pending/${sha}.zip`, new Uint8Array([80, 75]));
    const approve = async approvedAt => code('RYD', state.owner, { sha, approvedAt });
    if (seeded) assert.equal((await call(state.env, 'POST', '/v1/designs', await approve(100))).status, 200);
    const older = await approve(101), newer = await approve(102);
    interleave(state.env.BUCKET, 'designs/neur0map.txt', async condition => {
      assert.ok(seeded ? condition.etagMatches : condition.etagDoesNotMatch === '*');
      assert.equal((await call(state.env, 'POST', '/v1/designs', newer)).status, 200);
    });
    assert.equal((await call(state.env, 'POST', '/v1/designs', older)).status, 409);
    assert.deepEqual((await (await call(state.env, 'GET', '/v1/list')).json()).designs, [newer]);
    const before = state.env.BUCKET.serial;
    assert.equal((await call(state.env, 'POST', '/v1/designs', newer)).status, 200);
    assert.equal(state.env.BUCKET.serial, before, 'exact replay must not write');
  }
});

test('newer approvals retry older concurrent writes and equal-time alternatives conflict', async () => {
  const state = await setup(); await publish(state);
  const sha = 'ab'.repeat(32);
  await state.env.BUCKET.put(`packs/${sha}.zip`, new Uint8Array([80, 75]));
  const older = await code('RYD', state.owner, { sha, approvedAt: 100 });
  const newer = await code('RYD', state.owner, { sha, approvedAt: 101 });
  interleave(state.env.BUCKET, 'designs/neur0map.txt', async () => {
    assert.equal((await call(state.env, 'POST', '/v1/designs', older)).status, 200);
  });
  assert.equal((await call(state.env, 'POST', '/v1/designs', newer)).status, 200);
  const alternative = await code('RYD', state.mika, { issuer: 2, sha, approvedAt: 101 });
  assert.equal((await call(state.env, 'POST', '/v1/designs', alternative)).status, 409);
});

test('conditional write retries are bounded and abandoned pictures are removed', async () => {
  const state = await setup(); await publish(state);
  const put = state.env.BUCKET.put.bind(state.env.BUCKET);
  let attempts = 0;
  state.env.BUCKET.put = async (key, value, options) => {
    if (options?.onlyIf) { attempts++; return null; }
    return put(key, value, options);
  };
  const image = jpeg(), sha = await digest(image);
  assert.equal((await putRecord(state, await record(state.device, { picture: sha }), image)).status, 409);
  assert.equal(attempts, 8);
  assert.equal([...state.env.BUCKET.data.keys()].filter(key => key.endsWith('.jpg')).length, 0);
  await state.env.BUCKET.put(`packs/${sha}.zip`, new Uint8Array([80, 75]));
  attempts = 0;
  assert.equal((await call(state.env, 'POST', '/v1/designs', await code('RYD', state.owner, { sha }))).status, 409);
  assert.equal(attempts, 8);
});

test('eight active badges per case-insensitive login is a race-safe limit; revocation frees a slot', async () => {
  const state = await setup(), badges = [];
  for (let day = 1; day <= 7; day++) badges.push(await publish(state, { day }));
  const candidates = await Promise.all([8, 9, 10].map(day => code('RYB', state.owner, { device: state.device, day, login: 'Neur0map' })));
  const responses = await Promise.all(candidates.map(text => call(state.env, 'POST', '/v1/badges', text)));
  assert.deepEqual(responses.map(response => response.status).sort(), [200, 409, 409]);
  assert.equal((await call(state.env, 'POST', '/v1/badges', badges[0].text)).status, 200);
  const list = await (await call(state.env, 'GET', '/v1/list')).json();
  assert.equal(list.badges.length, 8);
  const revocation = await code('RYX', state.owner, { id: badges[0].id });
  assert.equal((await call(state.env, 'POST', '/v1/revoke', revocation)).status, 200);
  const replacement = await code('RYB', state.owner, { device: state.device, day: 11 });
  assert.equal((await call(state.env, 'POST', '/v1/badges', replacement)).status, 200);
  const refreshed = await (await call(state.env, 'GET', '/v1/list')).json();
  const revoked = new Set(await Promise.all(refreshed.revoked.map(async text => (await verifyCode('RYX', text, state.env)).id)));
  const active = await Promise.all(refreshed.badges.map(text => verifyCode('RYB', text, state.env)));
  assert.equal(active.filter(badge => !revoked.has(badge.id)).length, 8);
});

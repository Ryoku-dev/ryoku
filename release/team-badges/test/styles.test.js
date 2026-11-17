import test from 'node:test';
import assert from 'node:assert/strict';
import worker from '../src/index.js';
import { base64, cardId, cardPayload, concat, digest, verifyCode } from '../src/codec.js';
import { call, code, deleteRequest, deviceRecord, jpeg, keys, record, setup, sign, styledRecord } from './helpers.js';

const JSON_TYPE = { 'Content-Type': 'application/json' };
const STYLE_REFUSED = 'Custom card styles need an active team badge on this machine';
const putRaw = (state, id, text) => call(state.env, 'PUT', `/v1/cards/${id}`, text, JSON_TYPE);
const putCard = (state, value, picture) => putRaw(state, value.id, JSON.stringify({ record: value, ...(picture ? { picture: base64(picture) } : {}) }));
const pull = async (state, cards) => (await call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards }), JSON_TYPE)).json();
const get = state => call(state.env, 'GET', `/v1/cards/${state.id}`);
const stored = (state, id = state.id) => state.env.BUCKET.data.has(`cards/${id}.json`);
const index = state => JSON.parse(new TextDecoder().decode(state.env.BUCKET.data.get('cards/index.json'))).cards;
const pictures = state => [...state.env.BUCKET.data.keys()].filter(key => key.endsWith('.jpg'));
async function outcome(response) { return [response.status, (await response.json()).error]; }
async function publish(state, device = state.device, fields = {}) {
  const text = await code('RYB', state.owner, { device, ...fields });
  assert.equal((await call(state.env, 'POST', '/v1/badges', text)).status, 200);
  return (await verifyCode('RYB', text, state.env)).id;
}
async function revoke(state, badgeId) {
  assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id: badgeId }))).status, 200);
}
async function withClock(body) {
  const realNow = Date.now, clock = { at: realNow() };
  Date.now = () => clock.at;
  try { await body(clock); } finally { Date.now = realNow; }
}

test('a machine without an active badge cannot store a styled card, with or without its own key', async () => {
  const state = await setup();
  for (const value of [await styledRecord(state.device), await styledRecord(state.device, { device: base64(state.device.raw) })]) {
    assert.deepEqual(await outcome(await putCard(state, value)), [403, STYLE_REFUSED]);
  }
  assert.equal(stored(state), false);
  assert.equal(state.env.BUCKET.data.has('cards/index.json'), false);
  // a style smuggled onto a classic record is refused, the classic record itself still goes through
  const classic = await deviceRecord(state.device);
  assert.deepEqual(await outcome(await putCard(state, { ...classic, style: 'stand' })), [400, 'Invalid record fields']);
  assert.equal((await putCard(state, classic)).status, 200);
  assert.deepEqual(await (await get(state)).json(), classic);
  // a revoked badge counts as none
  const badgeId = await publish(state);
  await revoke(state, badgeId);
  assert.deepEqual(await outcome(await putCard(state, await styledRecord(state.device, { updated: classic.updated + 1 }))), [403, STYLE_REFUSED]);
  assert.deepEqual(await (await get(state)).json(), classic);
});

test('an active badge holder stores a styled card that GET, batch and its picture serve back', async () => {
  const state = await setup(), image = jpeg(30, 40), sha = await digest(image);
  await publish(state);
  const styled = await styledRecord(state.device, { picture: sha });
  assert.equal((await putCard(state, styled, image)).status, 200);
  assert.deepEqual(await (await get(state)).json(), styled);
  assert.deepEqual((await pull(state, { [state.id]: 0 })).records, [styled]);
  assert.deepEqual(new Uint8Array(await (await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).arrayBuffer()), image);
  // naming the badge's own device key is fine, and going back to classic is too
  const named = await styledRecord(state.device, { updated: styled.updated + 1, device: base64(state.device.raw) });
  assert.equal((await putCard(state, named)).status, 200);
  assert.deepEqual((await pull(state, { [state.id]: 0 })).records, [named]);
  const classic = await deviceRecord(state.device, { updated: styled.updated + 2 });
  assert.equal((await putCard(state, classic)).status, 200);
  assert.deepEqual(await (await get(state)).json(), classic);
  const again = await styledRecord(state.device, { updated: styled.updated + 3 });
  assert.equal((await putCard(state, again)).status, 200);
  assert.equal((await call(state.env, 'DELETE', `/v1/cards/${state.id}`, JSON.stringify(await deleteRequest(state.device, state.id)), JSON_TYPE)).status, 200);
  assert.equal(stored(state), false);
});

test('a styled card signed by any key but the badge device is refused', async () => {
  const state = await setup(), attacker = await keys();
  await publish(state);
  // the attacker holds an active badge of their own, which still says nothing about this id
  await publish(state, attacker, { login: 'mallory' });
  const genuine = await styledRecord(state.device);
  const forged = await styledRecord(attacker, { id: state.id, body: genuine.body, updated: genuine.updated });
  assert.deepEqual(await outcome(await putCard(state, forged)), [403, 'Invalid card signature']);
  assert.deepEqual(await outcome(await putCard(state, { ...forged, device: base64(attacker.raw) })), [403, 'Invalid card signature']);
  assert.deepEqual(await outcome(await putCard(state, { ...forged, device: base64(state.device.raw) })), [403, 'Invalid card signature']);
  // the classic signature of the same card does not carry over to a styled one
  const classic = await record(state.device, { updated: genuine.updated });
  assert.deepEqual(await outcome(await putCard(state, { ...classic, v: 3, style: 'stand' })), [403, 'Invalid card signature']);
  // and a styled signature does not verify as classic
  const { style, ...stripped } = genuine;
  assert.deepEqual(await outcome(await putCard(state, { ...stripped, v: 2 })), [400, 'No active badge verifies this record']);
  assert.deepEqual(await outcome(await putCard(state, stripped)), [400, 'Invalid record fields']);
  assert.equal(stored(state), false);
  assert.equal(style, 'stand');
});

test('the style is covered by the signature, so a tampered style is refused', async () => {
  const state = await setup();
  await publish(state);
  const genuine = await styledRecord(state.device);
  // sign the same card over a different style string, then present it as stand
  const { style, ...classic } = genuine, other = new TextEncoder().encode('gold');
  const payload = concat(cardPayload({ ...classic, v: 2 }), new Uint8Array([0, other.length]), other);
  const tampered = { ...genuine, sig: base64(await sign(state.device, 'ryoku-card-v3', payload)) };
  assert.deepEqual(await outcome(await putCard(state, tampered)), [403, 'Invalid card signature']);
  // the length prefix is part of what is signed: dropping it does not verify
  const unprefixed = concat(cardPayload({ ...classic, v: 2 }), new TextEncoder().encode(style));
  assert.deepEqual(await outcome(await putCard(state, { ...genuine, sig: base64(await sign(state.device, 'ryoku-card-v3', unprefixed)) })), [403, 'Invalid card signature']);
  // a v3-domain signature over the card bytes alone leaves the style unsigned, so it does not verify
  assert.deepEqual(await outcome(await putCard(state, { ...genuine, sig: base64(await sign(state.device, 'ryoku-card-v3', cardPayload({ ...classic, v: 2 }))) })), [403, 'Invalid card signature']);
  // a v2-domain signature over the v3 bytes does not verify either
  assert.deepEqual(await outcome(await putCard(state, { ...genuine, sig: base64(await sign(state.device, 'ryoku-card-v2', cardPayload(genuine))) })), [403, 'Invalid card signature']);
  assert.equal(stored(state), false);
  assert.equal((await putCard(state, genuine)).status, 200);
});

test('only allowlisted styles pass, even for a badge holder, and no field rides along', async () => {
  const state = await setup();
  await publish(state);
  const base = await styledRecord(state.device);
  const styles = ['classic', 'Stand', 'STAND', 'stand ', ' stand', 'stand\u0000', 'st\u200band', '\uff53tand', 'ѕtand', 'stand\n', 's'.repeat(33), 'x'.repeat(100000), '', 1, null, true, ['stand'], { stand: 1 }];
  for (const style of styles) {
    assert.deepEqual(await outcome(await putCard(state, { ...base, style })), [400, 'Unknown card style'], JSON.stringify(style).slice(0, 40));
  }
  // the codec refuses to even build a signed payload for one
  assert.throws(() => cardPayload({ ...base, style: 'gold' }), /Unknown card style/);
  for (const extra of [{ styles: 'stand' }, { look: 'stand' }, { design: 'x' }, { picture2: '' }]) {
    assert.deepEqual(await outcome(await putCard(state, { ...base, ...extra })), [400, 'Invalid record fields']);
  }
  for (const v of [1, 4, '3', 3.5]) assert.deepEqual(await outcome(await putCard(state, { ...base, v })), [400, 'Invalid record fields']);
  // JSON keys that JavaScript treats specially are still just unknown keys
  const text = JSON.stringify({ record: base }).replace('"v":3', '"v":3,"__proto__":{"style":"stand"}');
  assert.deepEqual(await outcome(await putRaw(state, state.id, text)), [400, 'Invalid record fields']);
  const wrapped = JSON.stringify({ record: base, style: 'stand' });
  assert.deepEqual(await outcome(await putRaw(state, state.id, wrapped)), [400, 'Invalid card upload']);
  assert.equal(stored(state), false);
  assert.equal((await putCard(state, base)).status, 200);
});

test('revoking the badge takes its styled card down at once, picture included', async () => {
  const state = await setup(), image = jpeg(30, 40), sha = await digest(image);
  const badgeId = await publish(state);
  const styled = await styledRecord(state.device, { picture: sha });
  assert.equal((await putCard(state, styled, image)).status, 200);
  await revoke(state, badgeId);
  assert.equal((await get(state)).status, 404);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).status, 404);
  assert.equal(stored(state), false);
  assert.deepEqual(pictures(state), []);
  assert.deepEqual(index(state), {});
  const body = await pull(state, { [state.id]: 0 });
  assert.deepEqual([body.records, body.gone, body.badges], [[], [state.id], []]);
  // the machine falls back to a classic card, which needs no badge
  assert.equal((await putCard(state, await deviceRecord(state.device, { updated: styled.updated + 1 }))).status, 200);
});

test('a revocation that skipped the route withholds the styled card at once and the cleanup deletes it', async () => {
  const state = await setup(), friend = await keys(), keeper = await keys();
  const friendId = await cardId(friend.raw), keeperId = await cardId(keeper.raw);
  const badgeId = await publish(state), friendBadge = await publish(state, friend, { login: 'friend' });
  // the style's owner on a second machine, matched without case
  await publish(state, keeper, { login: 'NEUR0MAP' });
  // the styled card names its own key too, which must not keep it in a batch once the badge is gone
  const styled = await styledRecord(state.device, { device: base64(state.device.raw) }), friendClassic = await deviceRecord(friend), kept = await styledRecord(keeper);
  for (const value of [styled, friendClassic, kept]) assert.equal((await putCard(state, value)).status, 200);
  await withClock(async clock => {
    // revocations land in R2 directly, as an operator restoring a backup would
    for (const id of [badgeId, friendBadge]) await state.env.BUCKET.put(`revoked/${id}.txt`, await code('RYX', state.owner, { id }));
    clock.at += 31_000;
    assert.equal((await get(state)).status, 404);
    const body = await pull(state, { [state.id]: 0, [friendId]: 0, [keeperId]: 0 });
    assert.deepEqual(body.records, [friendClassic, kept]);
    assert.equal(stored(state), true);
    await worker.scheduled({ scheduledTime: clock.at }, state.env);
  });
  assert.equal(stored(state), false);
  // a classic card under a revoked badge and a styled card under an active badge both stay
  assert.deepEqual(index(state), { [friendId]: friendClassic.updated, [keeperId]: kept.updated });
  assert.deepEqual((await pull(state, { [state.id]: 0 })).gone, [state.id]);
});

test('classic records behave as before for everyone', async () => {
  const state = await setup(), stranger = await keys();
  const plain = await deviceRecord(stranger);
  assert.equal((await putCard(state, plain)).status, 200);
  assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${plain.id}`)).json(), plain);
  assert.deepEqual(await outcome(await putCard(state, await record(state.device))), [400, 'No active badge verifies this record']);
  await publish(state);
  const legacy = await record(state.device);
  assert.equal((await putCard(state, legacy)).status, 200);
  assert.deepEqual((await pull(state, { [state.id]: 0, [plain.id]: 0 })).records, [legacy, plain]);
  await worker.scheduled({ scheduledTime: Date.now() }, state.env);
  assert.deepEqual(index(state), { [state.id]: legacy.updated, [plain.id]: plain.updated });
});

import test from 'node:test';
import assert from 'node:assert/strict';
import worker from '../src/index.js';
import { CARD_STYLE_OWNERS, base64, styleOwnedBy, verifyCode } from '../src/codec.js';
import { call, code, keys, setup, styledRecord, wordedRecord } from './helpers.js';

const JSON_TYPE = { 'Content-Type': 'application/json' };
const NOT_YOURS = 'This card style belongs to another badge holder';
const putCard = (state, value) => call(state.env, 'PUT', `/v1/cards/${value.id}`, JSON.stringify({ record: value }), JSON_TYPE);
const pull = async (state, cards) => (await call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards }), JSON_TYPE)).json();
const get = state => call(state.env, 'GET', `/v1/cards/${state.id}`);
const stored = state => state.env.BUCKET.data.has(`cards/${state.id}.json`);
async function outcome(response) { return [response.status, (await response.json()).error]; }
async function publish(state, login, device = state.device) {
  const text = await code('RYB', state.owner, { device, login });
  assert.equal((await call(state.env, 'POST', '/v1/badges', text)).status, 200);
  return (await verifyCode('RYB', text, state.env)).id;
}

test('a style belongs to its logins, matched without case', () => {
  assert.deepEqual(CARD_STYLE_OWNERS, { eidolon: ['aethctl'], stand: ['neur0map'] });
  for (const login of ['aethctl', 'AETHCTL', 'Aethctl']) assert.equal(styleOwnedBy('eidolon', login), true, login);
  for (const login of ['neur0map', 'friend', '']) assert.equal(styleOwnedBy('eidolon', login), false, login);
  for (const login of ['neur0map', 'Neur0map', 'NEUR0MAP']) assert.equal(styleOwnedBy('stand', login), true, login);
  for (const [style, login] of [['stand', 'friend'], ['stand', 'neur0map2'], ['stand', ''], ['stand', undefined], ['gold', 'neur0map'], ['__proto__', 'neur0map'], ['toString', 'neur0map']]) {
    assert.equal(styleOwnedBy(style, login), false, `${style} ${login}`);
  }
});

test('EIDOLON belongs only to aethctl and cannot be submitted by a different badge', async () => {
  const owner = await setup();
  await publish(owner, 'AETHCTL');
  const card = await styledRecord(owner.device, { style: 'eidolon' });
  assert.equal((await putCard(owner, card)).status, 200);
  assert.deepEqual(await (await get(owner)).json(), card);

  const other = await setup();
  await publish(other, 'neur0map');
  const forged = await styledRecord(other.device, { style: 'eidolon' });
  assert.deepEqual(await outcome(await putCard(other, forged)), [403, NOT_YOURS]);
  assert.equal(stored(other), false);
});

test('the style\'s owner stores it, another active holder may not, but their words pass', async () => {
  const owner = await setup();
  await publish(owner, 'Neur0map');
  const styled = await styledRecord(owner.device), worded = await wordedRecord(owner.device, { updated: styled.updated + 1 });
  assert.equal((await putCard(owner, styled)).status, 200);
  assert.equal((await putCard(owner, worded)).status, 200);
  assert.deepEqual(await (await get(owner)).json(), worded);

  const holder = await setup();
  await publish(holder, 'friend');
  for (const value of [await styledRecord(holder.device), await wordedRecord(holder.device), await wordedRecord(holder.device, { device: base64(holder.device.raw), tagline: undefined })]) {
    assert.deepEqual(await outcome(await putCard(holder, value)), [403, NOT_YOURS], `v${value.v}`);
  }
  assert.equal(stored(holder), false);
  const words = JSON.parse(JSON.stringify(await wordedRecord(holder.device, { style: undefined })));
  assert.equal((await putCard(holder, words)).status, 200);
  assert.deepEqual(await (await get(holder)).json(), words);
  assert.deepEqual((await pull(holder, { [holder.id]: 0 })).records, [words]);
});

test('a style its badges no longer own is withheld from GET and batch, then deleted', async () => {
  const state = await setup();
  // one machine, two badges: the owner's, and another login's that stays active
  const ownerBadge = await publish(state, 'neur0map');
  await publish(state, 'friend');
  const styled = await styledRecord(state.device);
  assert.equal((await putCard(state, styled)).status, 200);
  assert.deepEqual((await pull(state, { [state.id]: 0 })).records, [styled]);
  // the owner's badge is revoked straight in R2: serving notices once the inventory turns
  await state.env.BUCKET.put(`revoked/${ownerBadge}.txt`, await code('RYX', state.owner, { id: ownerBadge }));
  const realNow = Date.now;
  Date.now = () => realNow() + 31_000;
  try {
    assert.equal((await get(state)).status, 404);
    const body = await pull(state, { [state.id]: 0 });
    assert.deepEqual([body.records, body.gone], [[], []]);
    assert.equal(stored(state), true);
    await worker.scheduled({ scheduledTime: Date.now() }, state.env);
  } finally { Date.now = realNow; }
  assert.equal(stored(state), false);
  assert.deepEqual((await pull(state, { [state.id]: 0 })).gone, [state.id]);
  // the remaining badge may still carry its words, just not the style
  assert.deepEqual(await outcome(await putCard(state, await styledRecord(state.device, { updated: styled.updated + 1 }))), [403, NOT_YOURS]);
  assert.equal((await putCard(state, await wordedRecord(state.device, { style: undefined, updated: styled.updated + 2 }))).status, 200);
});

test('revoking the owner\'s badge through the route deletes the style at once, a words-only card stays', async () => {
  const state = await setup(), friend = await keys();
  const ownerBadge = await publish(state, 'neur0map');
  await publish(state, 'friend');
  assert.equal((await putCard(state, await styledRecord(state.device))).status, 200);
  await publish(state, 'friend', friend);
  const words = JSON.parse(JSON.stringify(await wordedRecord(friend, { style: undefined })));
  assert.equal((await putCard(state, words)).status, 200);
  assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id: ownerBadge }))).status, 200);
  assert.equal(stored(state), false);
  assert.deepEqual(await (await call(state.env, 'GET', `/v1/cards/${words.id}`)).json(), words);
  await worker.scheduled({ scheduledTime: Date.now() }, state.env);
  assert.equal(state.env.BUCKET.data.has(`cards/${words.id}.json`), true);
});

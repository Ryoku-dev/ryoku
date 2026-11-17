import test from 'node:test';
import assert from 'node:assert/strict';
import worker from '../src/index.js';
import { base64, cardId, cardPayload, concat, digest, verifyCode } from '../src/codec.js';
import { call, code, deviceRecord, jpeg, keys, record, setup, sign, styledRecord, wordedRecord } from './helpers.js';

const JSON_TYPE = { 'Content-Type': 'application/json' };
const WORDS_REFUSED = 'Card words need an active team badge on this machine';
const putCard = (state, value, picture) => call(state.env, 'PUT', `/v1/cards/${value.id}`, JSON.stringify({ record: value, ...(picture ? { picture: base64(picture) } : {}) }), JSON_TYPE);
const pull = async (state, cards) => (await call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards }), JSON_TYPE)).json();
const get = state => call(state.env, 'GET', `/v1/cards/${state.id}`);
const stored = state => state.env.BUCKET.data.has(`cards/${state.id}.json`);
const index = state => JSON.parse(new TextDecoder().decode(state.env.BUCKET.data.get('cards/index.json'))).cards;
const encode = text => new TextEncoder().encode(text);
const prefixed = text => concat(new Uint8Array([encode(text).length >> 8, encode(text).length & 255]), encode(text));
async function outcome(response) { return [response.status, (await response.json()).error]; }
async function publish(state, device = state.device, fields = {}) {
  const text = await code('RYB', state.owner, { device, ...fields });
  assert.equal((await call(state.env, 'POST', '/v1/badges', text)).status, 200);
  return (await verifyCode('RYB', text, state.env)).id;
}
async function revoke(state, badgeId) {
  assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id: badgeId }))).status, 200);
}

test('a machine without an active badge cannot store its own words, with or without a style or its key', async () => {
  const state = await setup();
  const attempts = [
    await wordedRecord(state.device),
    await wordedRecord(state.device, { device: base64(state.device.raw) }),
    await record(state.device, { v: 4, tagline: 'my own motto' }),
    await record(state.device, { v: 4, standName: 'The World', device: base64(state.device.raw) }),
  ];
  for (const value of attempts) assert.deepEqual(await outcome(await putCard(state, value)), [403, WORDS_REFUSED]);
  assert.equal(stored(state), false);
  // words smuggled onto a classic or a styled record are refused before any badge is asked about
  const classic = await deviceRecord(state.device);
  for (const extra of [{ tagline: 'x' }, { standName: 'x' }]) {
    assert.deepEqual(await outcome(await putCard(state, { ...classic, ...extra })), [400, 'Invalid record fields']);
    assert.deepEqual(await outcome(await putCard(state, { ...(await styledRecord(state.device)), ...extra })), [400, 'Invalid record fields']);
  }
  // a revoked badge counts as none
  const badgeId = await publish(state);
  await revoke(state, badgeId);
  assert.deepEqual(await outcome(await putCard(state, await wordedRecord(state.device))), [403, WORDS_REFUSED]);
  assert.equal(stored(state), false);
});

test('an active badge holder stores words that GET, batch and the picture serve back', async () => {
  const state = await setup(), image = jpeg(30, 40), sha = await digest(image);
  await publish(state);
  const worded = await wordedRecord(state.device, { picture: sha, device: base64(state.device.raw) });
  assert.equal((await putCard(state, worded, image)).status, 200);
  assert.deepEqual(await (await get(state)).json(), worded);
  assert.deepEqual((await pull(state, { [state.id]: 0 })).records, [worded]);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).status, 200);
  // words on a classic card, a motto alone and a Stand's name alone all pass
  let updated = worded.updated;
  for (const fields of [{ style: undefined }, { style: undefined, standName: undefined }, { tagline: undefined }]) {
    const value = await wordedRecord(state.device, { updated: ++updated, ...fields });
    assert.equal((await putCard(state, value)).status, 200, JSON.stringify(fields));
    assert.deepEqual(await (await get(state)).json(), JSON.parse(JSON.stringify(value)));
  }
  // up to the caps in characters, not bytes
  const longest = await wordedRecord(state.device, { updated: ++updated, tagline: '力'.repeat(128), standName: 'ザ'.repeat(64) });
  assert.equal((await putCard(state, longest)).status, 200);
});

test('the words are signed, length-prefixed and under their own domain', async () => {
  const state = await setup(), attacker = await keys();
  await publish(state);
  const genuine = await wordedRecord(state.device);
  for (const tampered of [
    { ...genuine, tagline: 'Muda muda muda.' },
    { ...genuine, standName: 'The World' },
    // moving bytes from one field into the next keeps the concatenation but not the prefixes
    { ...genuine, tagline: 'Yare yare daze.Star', standName: ' Platinum'.trim() },
    { ...genuine, tagline: genuine.standName, standName: genuine.tagline.slice(0, 64) },
  ]) assert.deepEqual(await outcome(await putCard(state, tampered)), [403, 'Invalid card signature']);
  const { tagline, standName, style, ...card } = genuine;
  const v2 = cardPayload({ ...card, v: 2 });
  for (const [domain, payload] of [
    ['ryoku-card-v4', concat(v2, encode(style), encode(tagline), encode(standName))],
    ['ryoku-card-v4', concat(v2, prefixed(style), prefixed(standName), prefixed(tagline))],
    ['ryoku-card-v4', concat(v2, prefixed(style))],
    ['ryoku-card-v3', cardPayload(genuine)],
    ['ryoku-card-v2', cardPayload(genuine)],
  ]) {
    const forged = { ...genuine, sig: base64(await sign(state.device, domain, payload)) };
    assert.deepEqual(await outcome(await putCard(state, forged)), [403, 'Invalid card signature'], domain);
  }
  // a v3 signature does not carry over to the same card with words added
  const styled = await styledRecord(state.device, { updated: genuine.updated });
  assert.deepEqual(await outcome(await putCard(state, { ...styled, v: 4, tagline })), [403, 'Invalid card signature']);
  // another active badge's device signs nothing for this id
  await publish(state, attacker, { login: 'mallory' });
  const forged = await wordedRecord(attacker, { id: state.id, body: genuine.body, updated: genuine.updated });
  assert.deepEqual(await outcome(await putCard(state, forged)), [403, 'Invalid card signature']);
  assert.equal(stored(state), false);
  assert.equal((await putCard(state, genuine)).status, 200);
});

test('oversized, hidden, markup, bidi and unnormalised words are refused, even for a holder', async () => {
  const state = await setup();
  await publish(state);
  const base = await wordedRecord(state.device);
  const bad = [
    '', ' ', ' leading', 'trailing ', 'trailing\u3000', 'a'.repeat(129), '力'.repeat(129),
    'a\u202eb', 'a\u202ab', 'a\u2066b', 'a\u2069b', 'a\u200eb', 'a\u061cb',
    'a\u200bb', 'a\u200cb', 'a\u200db', 'a\u2060b', 'a\ufeffb', 'a\u00adb', 'a\u180eb',
    'a\u0000b', 'a\u0007b', 'a\nb', 'a\u0085b', 'a\u2028b', 'a\u2029b',
    '<b>bold</b>', 'a & b', 'a > b', '\ufffd', 'a\u3164b', 'a\u115fb', 'a\uffa0b',
    'a\ue000b', 'a\ufdd0b', 'a\uffffb', 'a\ud800b', 'a\udc00',
    'Cafe\u0301', 'e\u0301', '\u212b',
    1, null, true, ['x'], { x: 1 },
  ];
  for (const words of bad) {
    assert.deepEqual(await outcome(await putCard(state, { ...base, tagline: words })), [400, 'Invalid card words'], JSON.stringify(words).slice(0, 40));
    if (typeof words !== 'string' || [...words].length <= 64) {
      assert.deepEqual(await outcome(await putCard(state, { ...base, standName: words })), [400, 'Invalid card words'], JSON.stringify(words).slice(0, 40));
    }
  }
  assert.deepEqual(await outcome(await putCard(state, { ...base, standName: 'a'.repeat(65) })), [400, 'Invalid card words']);
  // a v4 record needs words, and carries no other field
  const { tagline, standName, ...plain } = base;
  assert.deepEqual(await outcome(await putCard(state, plain)), [400, 'Invalid record fields']);
  for (const extra of [{ motto: 'x' }, { stand: 'x' }, { words: 'x' }]) assert.deepEqual(await outcome(await putCard(state, { ...base, ...extra })), [400, 'Invalid record fields']);
  assert.deepEqual(await outcome(await putCard(state, { ...base, style: 'gold' })), [400, 'Unknown card style']);
  assert.deepEqual(await outcome(await putCard(state, { ...base, style: '' })), [400, 'Unknown card style']);
  assert.equal(stored(state), false);
  // composed text, emoji and scripts with marks of their own pass
  for (const [i, words] of ['Café', 'ゴゴゴ', 'नमस्ते', 'Ça va 😀', '力と美のために'].entries()) {
    const value = await wordedRecord(state.device, { updated: base.updated + i, tagline: words });
    assert.equal((await putCard(state, value)).status, 200, words);
  }
});

test('revoking the badge withholds the words at once and deletes them, the cron catches the rest', async () => {
  const state = await setup(), image = jpeg(30, 40), sha = await digest(image), keeper = await keys();
  const badgeId = await publish(state), keeperBadge = await publish(state, keeper, { login: 'keeper' });
  const worded = await wordedRecord(state.device, { picture: sha });
  assert.equal((await putCard(state, worded, image)).status, 200);
  await revoke(state, badgeId);
  assert.equal((await get(state)).status, 404);
  assert.equal((await call(state.env, 'GET', `/v1/cards/${state.id}/${sha}.jpg`)).status, 404);
  assert.equal(stored(state), false);
  assert.deepEqual((await pull(state, { [state.id]: 0 })).gone, [state.id]);
  // a revocation that reached R2 without the route: withheld once the inventory turns, then swept
  // a holder who owns no style sends words alone
  const kept = await wordedRecord(keeper, { style: undefined });
  assert.equal((await putCard(state, kept)).status, 200);
  await state.env.BUCKET.put(`revoked/${keeperBadge}.txt`, await code('RYX', state.owner, { id: keeperBadge }));
  const realNow = Date.now;
  Date.now = () => realNow() + 31_000;
  try {
    assert.equal((await call(state.env, 'GET', `/v1/cards/${kept.id}`)).status, 404);
    assert.deepEqual((await pull(state, { [kept.id]: 0 })).records, []);
    await worker.scheduled({ scheduledTime: Date.now() }, state.env);
  } finally { Date.now = realNow; }
  assert.equal(state.env.BUCKET.data.has(`cards/${kept.id}.json`), false);
  assert.deepEqual(index(state), {});
});

test('v2 and v3 records stay valid beside words', async () => {
  const state = await setup(), stranger = await keys();
  // anyone's classic card
  const plain = await deviceRecord(stranger);
  assert.equal((await putCard(state, plain)).status, 200);
  // a holder's existing styled card, as the gaming PC published it, then words, then back again
  await publish(state);
  const styled = await styledRecord(state.device, { device: base64(state.device.raw) });
  assert.equal((await putCard(state, styled)).status, 200);
  assert.deepEqual((await pull(state, { [state.id]: 0, [plain.id]: 0 })).records, [styled, plain]);
  const worded = await wordedRecord(state.device, { updated: styled.updated + 1 });
  assert.equal((await putCard(state, worded)).status, 200);
  const again = await styledRecord(state.device, { updated: styled.updated + 2 });
  assert.equal((await putCard(state, again)).status, 200);
  assert.deepEqual(await (await get(state)).json(), again);
  assert.equal(await cardId(stranger.raw), plain.id);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { base64, concat, unbase64, unhex, verifyCode } from '../src/codec.js';
import { call, code, holderProof, keys, now, setup, sign } from './helpers.js';

const JSON_TYPE = { 'Content-Type': 'application/json' };
const DAY = 86400;
const send = (state, cards, holder, ip = '192.0.2.1') => call(state.env, 'POST', '/v1/cards/batch', JSON.stringify({ cards, ...(holder !== undefined ? { holder } : {}) }), { ...JSON_TYPE, 'CF-Connecting-IP': ip });
const counter = (state, badgeId) => JSON.parse(new TextDecoder().decode(state.env.BUCKET.data.get(`holders/${badgeId}.json`)));
async function holder(state, device = state.device, login = 'neur0map') {
  const badge = await code('RYB', state.owner, { device, login });
  assert.equal((await call(state.env, 'POST', '/v1/badges', badge)).status, 200);
  return { badge, badgeId: (await verifyCode('RYB', badge, state.env)).id, device };
}
// per-address limiters that count, refusing everything while shut
function limiters(state) {
  const seen = { read: [], pull: [] }, open = { read: false, pull: false };
  for (const name of ['read', 'pull']) {
    state.env[`${name.toUpperCase()}_LIMITER`] = { async limit({ key }) { seen[name].push(key); return { success: open[name] }; } };
  }
  return { seen, open };
}
async function withClock(start, body) {
  const realNow = Date.now, clock = { at: start * 1000 };
  Date.now = () => clock.at;
  try { await body(clock); } finally { Date.now = realNow; }
}

test('a badge holder gets ten early refreshes a UTC day, then the normal schedule', async () => {
  const state = await setup(), me = await holder(state), cards = { [state.id]: 0 }, gate = limiters(state);
  const start = now();
  for (let i = 0; i < 10; i++) {
    const response = await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, start + i));
    assert.equal(response.status, 200, `early refresh ${i + 1}`);
    const body = await response.json();
    assert.equal(body.next, body.now);
    assert.equal(body.holderLeft, 9 - i);
  }
  // the per-address limits were never even asked
  assert.deepEqual(gate.seen, { read: [], pull: [] });
  assert.deepEqual(counter(state, me.badgeId), { day: Math.floor(now() / DAY), used: 10, at: start + 9 });
  // the eleventh is an ordinary check: it meets the per-address limits and gets the usual next
  const eleventh = await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, start + 10));
  assert.equal(eleventh.status, 429);
  assert.equal((await eleventh.json()).holderLeft, 0);
  gate.open.read = gate.open.pull = true;
  const ordinary = await (await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, start + 11))).json();
  assert.equal(ordinary.next, ordinary.now + 900);
  assert.equal(ordinary.holderLeft, 0);
  // the eleventh met the read limit (shut), the twelfth the read and the pull limits (open)
  assert.deepEqual([gate.seen.read.length, gate.seen.pull.length], [2, 1]);
  assert.equal(counter(state, me.badgeId).used, 10);
  // a batch without a proof never hears about early refreshes
  assert.equal((await (await send(state, cards)).json()).holderLeft, undefined);
});

test('early refreshes are counted per badge, not per address', async () => {
  const state = await setup(), me = await holder(state), friend = await holder(state, await keys(), 'friend');
  const cards = { [state.id]: 0 }, gate = limiters(state), start = now();
  // one badge from many addresses shares its ten
  for (let i = 0; i < 10; i++) {
    const response = await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, start + i), `198.51.100.${i}`);
    assert.equal(response.status, 200);
  }
  assert.equal((await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, start + 10), '203.0.113.99')).status, 429);
  // another badge behind the very same address still has all of its own
  for (let i = 0; i < 10; i++) {
    const body = await (await send(state, cards, await holderProof(friend.device, friend.badge, friend.badgeId, cards, start + i), '198.51.100.0')).json();
    assert.equal(body.holderLeft, 9 - i);
  }
  assert.deepEqual([counter(state, me.badgeId).used, counter(state, friend.badgeId).used], [10, 10]);
  assert.deepEqual([gate.seen.read, gate.seen.pull], [['203.0.113.99'], []]);
});

test('parallel early refreshes never spend more than ten', async () => {
  const state = await setup(), me = await holder(state), cards = { [state.id]: 0 }, start = now();
  limiters(state);
  const proofs = await Promise.all(Array.from({ length: 25 }, (_, i) => holderProof(me.device, me.badge, me.badgeId, cards, start + i)));
  const replies = await Promise.all(proofs.map(proof => send(state, cards, proof)));
  const granted = (await Promise.all(replies.map(async reply => reply.status === 200 && (await reply.json()).next !== undefined))).filter(Boolean).length;
  assert.ok(granted <= 10, `${granted} early refreshes granted`);
  assert.equal(counter(state, me.badgeId).used, granted);
  // whatever lost its race in parallel can still be spent afterwards, never past ten
  let more = 0;
  for (let i = 0; i < 15; i++) {
    if ((await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, start + 100 + i))).status === 200) more++;
  }
  assert.equal(granted + more, 10);
  assert.equal(counter(state, me.badgeId).used, 10);
});

test('the ten come back at the gate\'s UTC midnight, whatever the binder\'s clock says', async () => {
  const state = await setup(), me = await holder(state), cards = { [state.id]: 0 };
  limiters(state);
  const midnight = (Math.floor(now() / DAY) + 1) * DAY;
  await withClock(midnight - 300, async clock => {
    for (let i = 0; i < 10; i++) assert.equal((await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, midnight - 300 + i))).status, 200);
    // a proof stamped tomorrow by a clock running ahead spends nothing today
    assert.equal((await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, midnight + 200))).status, 429);
    clock.at = (midnight + 5) * 1000;
    const body = await (await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, midnight + 201))).json();
    assert.equal(body.next, body.now);
    assert.equal(body.holderLeft, 9);
    assert.deepEqual(counter(state, me.badgeId), { day: midnight / DAY, used: 1, at: midnight + 201 });
  });
});

test('a holder proof cannot be replayed, borrowed, outlived, forged or carried to another batch', async () => {
  const state = await setup(), me = await holder(state), stranger = await keys(), cards = { [state.id]: 0 };
  const gate = limiters(state);
  gate.open.read = gate.open.pull = true;
  const at = now(), proof = await holderProof(me.device, me.badge, me.badgeId, cards, at);
  const first = await (await send(state, cards, proof)).json();
  assert.deepEqual([first.next === first.now, first.holderLeft], [true, 9]);
  // the same proof again, from anywhere, is an ordinary check and spends nothing
  for (const ip of ['192.0.2.1', '203.0.113.5']) {
    const replay = await (await send(state, cards, proof, ip)).json();
    assert.deepEqual([replay.next, replay.holderLeft], [replay.now + 900, 9]);
  }
  // as is an older proof, or one signed by anything but the badge's own device, or one past its ten
  // minutes, or one signed for a different batch, or the old ryoku-pull-v1 form
  const flipped = unbase64(proof.sig); flipped[0] ^= 1;
  const v1 = { badge: me.badge, at: at + 5, sig: base64(await sign(me.device, 'ryoku-pull-v1', concat(unhex(me.badgeId), (() => { const b = new Uint8Array(8); new DataView(b.buffer).setBigUint64(0, BigInt(at + 5)); return b; })()))) };
  const broken = [
    await holderProof(me.device, me.badge, me.badgeId, cards, at - 1),
    await holderProof(stranger, me.badge, me.badgeId, cards, at + 1),
    await holderProof(me.device, me.badge, me.badgeId, cards, at - 601),
    await holderProof(me.device, me.badge, me.badgeId, cards, at + 601),
    await holderProof(me.device, me.badge, me.badgeId, { [state.id]: 1 }, at + 2),
    await holderProof(me.device, me.badge, me.badgeId, { [state.id]: 0, ['0'.repeat(16)]: 0 }, at + 3),
    { ...proof, at: at + 4 },
    { ...proof, sig: base64(flipped) },
    { ...proof, extra: 1 },
    v1,
    'junk',
  ];
  for (const forged of broken) {
    const response = await send(state, cards, forged);
    assert.equal(response.status, 200);
    const body = await response.json();
    assert.equal(body.next, body.now + 900, JSON.stringify(forged).slice(0, 60));
  }
  assert.equal(counter(state, me.badgeId).used, 1);
  assert.equal(gate.seen.pull.length, 2 + broken.length);
  // a revoked badge's proofs are ordinary checks too
  assert.equal((await call(state.env, 'POST', '/v1/revoke', await code('RYX', state.owner, { id: me.badgeId }))).status, 200);
  const revoked = await (await send(state, cards, await holderProof(me.device, me.badge, me.badgeId, cards, at + 10))).json();
  assert.deepEqual([revoked.next, revoked.holderLeft], [revoked.now + 900, undefined]);
  assert.equal(counter(state, me.badgeId).used, 1);
});

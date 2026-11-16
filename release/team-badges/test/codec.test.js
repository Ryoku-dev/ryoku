import test from 'node:test';
import assert from 'node:assert/strict';
import { base64, cardId, cardPayload, crc16, decodeCode, digest, encodeCode, jpegDimensions, unbase64, verifyCard, verifyCode, verifyUpload } from '../src/codec.js';
import { cardBody, code, jpeg, keys, now, record, requestCode, setup, uploadHeader } from './helpers.js';

test('Crockford aliases, whitespace, request CRC and canonical padding', async () => {
  const { device } = await setup();
  const text = requestCode(device);
  const parsed = await verifyCode('RYQ', text.toLowerCase().replace(/0/g, 'o').replace(/1/g, 'l').replace(/-/g, ' \n-'));
  assert.equal(parsed.cardId, await cardId(device.raw));
  assert.deepEqual(parsed.device, device.raw);
  assert.equal(crc16(new TextEncoder().encode('123456789')), 0x29b1);
  const bytes = decodeCode('RYQ', text); bytes[2] ^= 1;
  await assert.rejects(verifyCode('RYQ', encodeCode('RYQ', bytes)));
  for (const bad of [text + '!', 'RYQ-U', 'RYQ-0', 'x'.repeat(601), text + '0', text.slice(0, -1) + 'Z']) await assert.rejects(verifyCode('RYQ', bad));
  const legacy = decodeCode('RYQ', text); legacy[0] = 1;
  const legacyCRC = crc16(legacy.subarray(0, 33));
  legacy[33] = legacyCRC >> 8; legacy[34] = legacyCRC & 255;
  await assert.rejects(verifyCode('RYQ', encodeCode('RYQ', legacy)), /Unsupported code version/);
});

test('every signed code verifies, rejects forgery, tampering, malformed fields and wrong domain', async () => {
  const { owner, mika, device, env } = await setup(), attacker = await keys();
  const badge = await code('RYB', owner, { device });
  const id = (await verifyCode('RYB', badge, env)).id;
  for (const kind of ['RYB', 'RYX', 'RYD']) {
    const fields = { device, id, sha: 'ab'.repeat(32) };
    const text = await code(kind, owner, fields);
    assert.equal((await verifyCode(kind, text, env)).kind, kind);
    await assert.rejects(verifyCode(kind, await code(kind, attacker, fields), env));
    const bytes = decodeCode(kind, text); bytes[3] ^= 1;
    await assert.rejects(verifyCode(kind, encodeCode(kind, bytes), env));
    bytes[0] = 1;
    await assert.rejects(verifyCode(kind, encodeCode(kind, bytes), env), /Unsupported code version/);
    await assert.rejects(verifyCode(kind, encodeCode(kind, decodeCode(kind, text).slice(0, -1)), env));
  }
  assert.equal((await verifyCode('RYB', await code('RYB', mika, { device, issuer: 2, role: 2 }), env)).role, 2);
  await assert.rejects(verifyCode('RYB', await code('RYB', mika, { device, issuer: 2, role: 4 }), env), /Issuer cannot grant role/);
  await assert.rejects(verifyCode('RYB', await code('RYB', owner, { device, issuer: 3 }), env));
  await assert.rejects(verifyCode('RYB', await code('RYB', owner, { device, login: '-bad' }), env));
  await assert.rejects(verifyCode('RYD', await code('RYD', owner, { sha: 'ab'.repeat(32), login: 'bad_' }), env));
  const bytes = decodeCode('RYB', badge);
  assert.equal(id, (await digest(bytes)).slice(0, 16));
  await assert.rejects(verifyCode('RYD', encodeCode('RYD', bytes), env));
});

test('card records bind timestamp, body, key-derived id and picture to the device', async () => {
  const { device } = await setup(), other = await keys();
  const valid = await record(device);
  assert.equal(await verifyCard(valid, device.raw), true);
  assert.equal(await verifyCard(valid, other.raw), false);
  for (const changed of [{ updated: valid.updated + 1 }, { picture: 'ab'.repeat(32) }]) assert.equal(await verifyCard({ ...valid, ...changed }, device.raw), false);
  for (const changed of [{ v: 1 }, { id: 'abcdef0000000000' }, { updated: -1 }, { updated: 1.5 }, { body: '' }, { extra: 1 }, { sig: 'not base64' }]) await assert.rejects(verifyCard({ ...valid, ...changed }, device.raw));
  // 0x31: a version 3 body with the built-in graphics bit; 0x42: the spare bit in version 4; 0x50: version 5
  for (const [position, value] of [[0, 0x20], [0, 0x3c], [0, 0x31], [0, 0x42], [0, 0x50], [5, 0xc0], [5, 0x18], [14, 255]]) {
    const bytes = cardBody(valid.id); bytes[position] = value;
    assert.throws(() => cardPayload({ ...valid, body: base64(bytes) }));
  }
  for (const first of [0x40, 0x41, 0x45]) {
    const bytes = cardBody(valid.id); bytes[0] = first;
    cardPayload({ ...valid, body: base64(bytes) });
  }
  const invalidUtf8 = new Uint8Array([...cardBody(valid.id).slice(0, 14), 1, 255, 0, 0, 0, 0]);
  assert.throws(() => cardPayload({ ...valid, body: base64(invalidUtf8) }));
  for (const [index, cap] of [14, 20, 7, 10, 8].entries()) {
    const strings = ['', '', '', '', '']; strings[index] = 'a'.repeat(cap);
    cardPayload({ ...valid, body: base64(cardBody(valid.id, strings)) });
    strings[index] += 'a';
    assert.throws(() => cardPayload({ ...valid, body: base64(cardBody(valid.id, strings)) }));
  }
  const full = cardBody(valid.id, [14, 20, 7, 10, 8].map(cap => 'a'.repeat(cap)));
  assert.equal(full.length, 78);
  cardPayload({ ...valid, body: base64(full) });
  assert.throws(() => cardPayload({ ...valid, body: base64(new Uint8Array([...full, 0])) }));
  assert.throws(() => unbase64('AB=='), /Noncanonical/);
});

test('upload authorization binds path, method, digest and a 300-second window', async () => {
  const { owner, env } = await setup(), bytes = new Uint8Array([80, 75, 3, 4]);
  const sha = await digest(bytes), path = `/v1/pending/${sha}`, seconds = now();
  for (const delta of [-300, 0, 300]) {
    const header = await uploadHeader(owner, 1, path, bytes, seconds + delta);
    await verifyUpload(header, 'PUT', path, sha, env, seconds);
  }
  const header = await uploadHeader(owner, 1, path, bytes, seconds);
  for (const [method, changedPath, changedSha, time] of [['POST', path, sha, seconds], ['PUT', path + 'x', sha, seconds], ['PUT', path, '00'.repeat(32), seconds], ['PUT', path, sha, seconds + 301]]) await assert.rejects(verifyUpload(header, method, changedPath, changedSha, env, time));
  await assert.rejects(verifyUpload(undefined, 'PUT', path, sha, env, seconds));
  await assert.rejects(verifyUpload(await uploadHeader(await keys(), 1, path, bytes), 'PUT', path, sha, env, seconds));
});

test('JPEG marker parsing rejects oversized, missing and truncated frames', () => {
  assert.deepEqual(jpegDimensions(jpeg()), { width: 1120, height: 780 });
  for (const bytes of [jpeg(1121, 780), jpeg(1120, 781), jpeg(0, 10), jpeg().slice(0, 12), new Uint8Array([255, 216, 255, 218, 0, 2]), new Uint8Array([0, 0, 0, 0])]) assert.throws(() => jpegDimensions(bytes));
});

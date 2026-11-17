import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { unbase64, verifyCard, verifyCode, verifyDelete, verifyDeviceCard, verifyHolder, verifyUpload } from '../src/codec.js';
import './helpers.js';

const input = JSON.parse(await readFile(new URL('./vectors.json', import.meta.url), 'utf8'));
const vectors = Array.isArray(input) ? input : input.vectors;
assert.ok(Array.isArray(vectors) && vectors.length > 0, 'vectors.json must contain a nonempty array');

test('cross-language Go vectors', async t => {
  for (const [index, vector] of vectors.entries()) {
    await t.test(`${index}: ${vector.name || vector.kind}`, async () => {
      assert.equal(typeof vector.valid, 'boolean', 'Vector must explicitly state valid');
      const env = {
        ...(vector.OWNER_KEY || vector.ownerKey ? { OWNER_KEY: vector.OWNER_KEY || vector.ownerKey } : {}),
        ...(vector.MIKA_KEY || vector.mikaKey ? { MIKA_KEY: vector.MIKA_KEY || vector.mikaKey } : {}),
      };
      const kind = { request: 'RYQ', badge: 'RYB', revoke: 'RYX', design: 'RYD' }[vector.kind] || vector.kind;
      assert.ok(['RYQ', 'RYB', 'RYX', 'RYD', 'card', 'deviceCard', 'cardDelete', 'pullHolder', 'upload'].includes(kind), 'Unknown vector kind');
      let result, valid = false;
      try {
        if (kind === 'card') valid = await verifyCard(vector.record, unbase64(vector.deviceKey));
        else if (kind === 'deviceCard') valid = await verifyDeviceCard(vector.record);
        else if (kind === 'cardDelete') valid = await verifyDelete({ id: vector.id, at: vector.at, sig: vector.sig }, [unbase64(vector.deviceKey)], vector.now);
        else if (kind === 'pullHolder') { await verifyHolder(vector.holder, vector.cards, env, vector.now); valid = true; }
        else if (kind === 'upload') { await verifyUpload(vector.header, vector.method, vector.path, vector.sha, env, vector.now); valid = true; }
        else { result = await verifyCode(kind, vector.text, env); valid = true; }
      } catch { valid = false; }
      assert.equal(valid, vector.valid);
      if (valid && result) for (const field of ['id', 'cardId', 'role', 'login', 'issuer', 'sha', 'day', 'approvedAt']) if (vector[field] !== undefined) assert.equal(result[field], vector[field], `Mismatched ${field}`);
    });
  }
});

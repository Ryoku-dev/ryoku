import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import './helpers.js';
import { canonical, openEnvelope } from '../src/inspection-protocol.js';
import { digest, message, unbase64 } from '../src/codec.js';

const fixture = JSON.parse(await readFile(new URL('./inspection-vectors.json', import.meta.url), 'utf8'));
for (const vector of fixture.vectors) {
  test(`stable signed canonical vector: ${vector.domain}`, async () => {
    const bytes = new TextEncoder().encode(canonical(vector.payload));
    assert.equal(Buffer.from(bytes).toString('hex'), vector.canonicalHex);
    assert.equal(await digest(bytes), vector.payloadSha);
    assert.equal(Buffer.from(message(vector.domain, bytes)).toString('hex'), vector.messageHex);
    assert.deepEqual((await openEnvelope(vector.envelope, vector.domain, vector.publicKey)).payload, vector.payload);
    assert.deepEqual(unbase64(vector.envelope.payload), bytes);
  });
}

test('inspector supervisor shared submission vector is byte-identical in the gate', async () => {
  const vector = JSON.parse(await readFile(new URL('../inspector/protocol-vectors.json', import.meta.url), 'utf8'));
  const envelope = { payload: vector.payload, sig: vector.sig };
  const opened = await openEnvelope(envelope, vector.domain, vector.publicKey);
  assert.equal(canonical(opened.payload), vector.canonicalUtf8);
  assert.equal(opened.sha, vector.payloadSha);
});

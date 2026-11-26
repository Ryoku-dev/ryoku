import { InputError, cardId, cardPayload, concat, unhex, verifySignature } from './codec.js';
import { decode64, fail, fields, hex } from './inspection-protocol.js';

export const PUBLICATION_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const encoder = new TextEncoder();
function prefixed(value) {
  const bytes = encoder.encode(value);
  return concat(new Uint8Array([bytes.length >> 8, bytes.length & 255]), bytes);
}
// The unchanged binary base card is followed by every v5 authority reference.
// Neither the JSON wrapper nor an uploader-supplied key is publication authority.
export function animatedPayload(record) {
  fields(record, ['v', 'id', 'updated', 'body', 'picture', 'sig', 'device', 'publication', 'canonicalSha', 'approvalSha', 'certificateSha']);
  if (record.v !== 5 || !PUBLICATION_ID.test(record.publication) || ![record.picture, record.canonicalSha, record.approvalSha, record.certificateSha].every(hex)) fail('Invalid animated card metadata');
  decode64(record.device, 32);
  const { id, updated, body, picture, sig } = record;
  const base = cardPayload({ v: 2, id, updated, body, picture, sig });
  return concat(base, prefixed(record.device), prefixed(record.publication), unhex(record.canonicalSha), unhex(record.approvalSha), unhex(record.certificateSha));
}
export async function verifyAnimatedRecord(record) {
  const payload = animatedPayload(record), device = decode64(record.device, 32);
  return await cardId(device) === record.id && verifySignature(device, decode64(record.sig, 64), 'ryoku-card-v5', payload);
}
// All checks execute inside the publication authority's serialized operation, not
// from a cached public approval document. The service capability never reaches clients.
export async function authorizeAnimatedRecord(env, record, request = null) {
  if (!env.INSPECTION_QUEUE || typeof env.CARD_GATE_TOKEN !== 'string' || env.CARD_GATE_TOKEN.length < 32) fail('Animated authority unavailable', 503);
  const token = request ? /^Bearer ([^\s]+)$/.exec(request.headers.get('Authorization') || '')?.[1] : undefined;
  if (request && !token) fail('Account authentication required', 401);
  const body = JSON.stringify({ record, ...(request ? { token } : {}) });
  const response = await env.INSPECTION_QUEUE.get(env.INSPECTION_QUEUE.idFromName('private-cards-v1')).fetch(new Request('https://team.ryoku.dev/internal/inspection/records/authorize', { method: 'POST', headers: { Authorization: `Bearer ${env.CARD_GATE_TOKEN}`, 'Content-Type': 'application/json', 'Content-Length': String(encoder.encode(body).length) }, body }));
  if (!response.ok) throw new InputError('Animated publication is not current', response.status);
}
// v5's PNG is the publication's immutable poster. Do not copy it into legacy JPEG
// storage or make a public pack alias that can outlive publication authority.
export async function storeAnimatedRecord(env, record) {
  const key = `cards/${record.id}.json`;
  for (let attempt = 0; attempt < 8; attempt++) {
    const stored = await env.BUCKET.get(key), old = stored ? JSON.parse(await stored.text()) : null;
    if (old && (!Number.isSafeInteger(old.updated) || record.updated <= old.updated)) fail('Card record is not newer');
    if (!await env.BUCKET.put(key, JSON.stringify(record), { onlyIf: stored ? { etagMatches: stored.etag } : { etagDoesNotMatch: '*' }, httpMetadata: { contentType: 'application/json' } })) continue;
    if (stored?.customMetadata?.pictureKey) await env.BUCKET.delete(stored.customMetadata.pictureKey);
    return;
  }
  fail('Card record changed concurrently', 409);
}

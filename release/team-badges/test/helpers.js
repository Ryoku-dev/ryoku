import { webcrypto } from 'node:crypto';
import { base64, cardId, concat, crc16, digest, encodeCode, message, unhex } from '../src/codec.js';
if (!globalThis.crypto) globalThis.crypto = webcrypto;
export const now = () => Math.floor(Date.now() / 1000);
export async function keys() {
  const pair = await crypto.subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify']);
  return { ...pair, raw: new Uint8Array(await crypto.subtle.exportKey('raw', pair.publicKey)) };
}
export async function setup() {
  const owner = await keys(), mika = await keys(), device = await keys();
  return { owner, mika, device, id: await cardId(device.raw), env: { OWNER_KEY: base64(owner.raw), MIKA_KEY: base64(mika.raw), BUCKET: new Bucket() } };
}
export async function sign(key, domain, payload) { return new Uint8Array(await crypto.subtle.sign('Ed25519', key.privateKey, message(domain, payload))); }
export function requestCode(device) {
  const payload = concat(new Uint8Array([2]), device.raw), crc = crc16(payload);
  return encodeCode('RYQ', concat(payload, new Uint8Array([crc >> 8, crc & 255])));
}
export async function code(kind, signer, fields = {}) {
  const issuer = fields.issuer ?? 1, day = fields.day ?? 319;
  const login = new TextEncoder().encode(fields.login ?? 'neur0map');
  // a program makes the badge v3
  const program = fields.program === undefined ? null : new TextEncoder().encode(fields.program);
  let payload;
  if (kind === 'RYB') payload = concat(new Uint8Array([program ? 3 : 2, issuer, fields.role ?? 4]), fields.device.raw, new Uint8Array([day >> 8, day & 255, login.length]), login, ...(program ? [new Uint8Array([program.length]), program] : []));
  if (kind === 'RYX') payload = concat(new Uint8Array([2, issuer]), unhex(fields.id), new Uint8Array([day >> 8, day & 255]));
  if (kind === 'RYD') {
    const approvedAt = new Uint8Array(4);
    new DataView(approvedAt.buffer).setUint32(0, fields.approvedAt ?? 1794697200);
    payload = concat(new Uint8Array([2, issuer]), unhex(fields.sha), approvedAt, new Uint8Array([login.length]), login);
  }
  const domain = fields.domain ?? { RYB: program ? 'ryoku-badge-v3' : 'ryoku-badge-v2', RYX: 'ryoku-revoke-v2', RYD: 'ryoku-design-v2' }[kind];
  return encodeCode(kind, concat(payload, await sign(signer, domain, payload)));
}
export function cardBody(id, strings = ['', '', '', '', '']) {
  return concat(new Uint8Array([0x30, 0, 0, 0, 0, 0]), unhex(id), ...strings.map(text => {
    const bytes = new TextEncoder().encode(text);
    return concat(new Uint8Array([bytes.length]), bytes);
  }));
}
export async function record(device, fields = {}) {
  const { cardDomain, cardPayload } = await import('../src/codec.js');
  const id = await cardId(device.raw);
  const record = { v: 2, id, updated: now(), body: base64(cardBody(id)), picture: '', sig: '', ...fields };
  record.sig = base64(await sign(device, cardDomain(record), cardPayload(record)));
  return record;
}
// a v3 record carrying a built-in style, signed over the style too
export async function styledRecord(device, fields = {}) {
  return record(device, { v: 3, style: 'stand', ...fields });
}
// a v4 record carrying its owner's words, and the Stand style unless fields say otherwise
export async function wordedRecord(device, fields = {}) {
  return record(device, { v: 4, style: 'stand', tagline: 'Yare yare daze.', standName: 'Star Platinum', ...fields });
}
export async function deviceRecord(device, fields = {}) {
  return record(device, { device: base64(device.raw), ...fields });
}
function u64(value) {
  const bytes = new Uint8Array(8);
  new DataView(bytes.buffer).setBigUint64(0, BigInt(value));
  return bytes;
}
export async function deleteRequest(device, id, at = now()) {
  return { id, at, sig: base64(await sign(device, 'ryoku-card-delete-v1', concat(unhex(id), u64(at)))) };
}
// a holder proof for exactly this batch of cards
export async function holderProof(device, badge, badgeId, cards, at = now()) {
  const { batchBytes } = await import('../src/codec.js');
  return { badge, at, sig: base64(await sign(device, 'ryoku-pull-v2', concat(unhex(badgeId), u64(at), unhex(await digest(batchBytes(cards)))))) };
}
export async function uploadHeader(signer, issuer, path, bytes, seconds = now(), method = 'PUT') {
  const payload = new TextEncoder().encode(`${method}\n${path}\n${seconds}\n${await digest(bytes)}`);
  return `${issuer}.${seconds}.${base64(await sign(signer, 'ryoku-upload-v1', payload))}`;
}
export function jpeg(width = 1120, height = 780) {
  return new Uint8Array([255, 216, 255, 224, 0, 2, 255, 192, 0, 11, 8, height >> 8, height & 255, width >> 8, width & 255, 1, 1, 0x11, 0, 255, 217]);
}
export class Bucket {
  data = new Map();
  metadata = new Map();
  serial = 0;
  async put(key, value, options = {}) {
    const bytes = typeof value === 'string' ? new TextEncoder().encode(value) : new Uint8Array(await new Response(value).arrayBuffer());
    const current = this.metadata.get(key);
    if (options.onlyIf?.etagMatches !== undefined && current?.etag !== options.onlyIf.etagMatches) return null;
    if (options.onlyIf?.etagDoesNotMatch === '*' && current) return null;
    const metadata = { etag: String(++this.serial), customMetadata: options.customMetadata ?? {} };
    this.data.set(key, bytes);
    this.metadata.set(key, metadata);
    return { key, ...metadata };
  }
  async get(key) {
    const value = this.data.get(key), metadata = this.metadata.get(key);
    return value ? { ...metadata, body: new Response(value).body, text: async () => new TextDecoder().decode(value) } : null;
  }
  async head(key) { return this.data.has(key) ? { key, ...this.metadata.get(key) } : null; }
  async delete(keys) { for (const key of [keys].flat()) { this.data.delete(key); this.metadata.delete(key); } }
  async list({ prefix, cursor }) {
    const matches = [...this.data.keys()].filter(key => key.startsWith(prefix)).sort();
    const start = Number(cursor || 0), end = start + 2;
    return { objects: matches.slice(start, end).map(key => ({ key })), truncated: end < matches.length, cursor: end < matches.length ? String(end) : undefined };
  }
}
export async function call(env, method, path, body, headers = {}) {
  const { default: worker } = await import('../src/index.js');
  const bytes = body === undefined ? undefined : typeof body === 'string' ? new TextEncoder().encode(body) : body;
  return worker.fetch(new Request(`https://team.ryoku.dev${path}`, { method, ...(bytes ? { body: bytes } : {}), headers: { ...(bytes ? { 'Content-Type': 'text/plain', 'Content-Length': String(bytes.length) } : {}), ...headers } }), env);
}

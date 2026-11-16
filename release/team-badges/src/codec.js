const encoder = new TextEncoder();
const alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
const loginPattern = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$/;
export const CARD_ID = /^[0-9a-f]{16}$/;
export const SHA = /^[0-9a-f]{64}$/;
export const DEFAULT_KEYS = {
  OWNER_KEY: 'tlIg8prWfO0dMfd9IO3qxklONGHbSq7w3bUiu6QWuro=',
  MIKA_KEY: 'TtnSdWqRm4UPQ4JXa5IG8yBKp/xbxTudgO/C/AeBnUg=',
};
const domains = { RYB: 'ryoku-badge-v2', RYX: 'ryoku-revoke-v2', RYD: 'ryoku-design-v2' };

export class InputError extends Error {
  constructor(message, status = 400) { super(message); this.status = status; }
}
function requireThat(condition, message) {
  if (!condition) throw new InputError(message);
}
export function concat(...parts) {
  const bytes = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let offset = 0;
  for (const part of parts) { bytes.set(part, offset); offset += part.length; }
  return bytes;
}
export function message(domain, payload) { return concat(encoder.encode(domain), new Uint8Array(1), payload); }
export function hex(bytes) { return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join(''); }
export function unhex(text) { return Uint8Array.from(text.match(/../g) || [], b => parseInt(b, 16)); }
export async function digest(bytes) { return hex(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))); }
export async function cardId(device) { return (await digest(message('ryoku-card-id-v1', device))).slice(0, 16); }
export function base64(bytes) { return btoa(Array.from(bytes, b => String.fromCharCode(b)).join('')); }
export function unbase64(text) {
  requireThat(typeof text === 'string' && /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(text), 'Invalid base64');
  const bytes = Uint8Array.from(atob(text), c => c.charCodeAt(0));
  requireThat(base64(bytes) === text, 'Noncanonical base64');
  return bytes;
}
export function encodeCode(prefix, bytes) {
  let bits = 0, value = 0, encoded = '';
  for (const byte of bytes) {
    value = (value << 8) | byte; bits += 8;
    while (bits >= 5) { bits -= 5; encoded += alphabet[(value >>> bits) & 31]; }
    value &= (1 << bits) - 1;
  }
  if (bits) encoded += alphabet[(value << (5 - bits)) & 31];
  return prefix + '-' + encoded.match(/.{1,5}/g).join('-');
}
export function decodeCode(prefix, text) {
  requireThat(typeof text === 'string' && text.length <= 600, 'Invalid code length');
  const compact = text.toUpperCase().replace(/[-\s]/gu, '').replace(/[IL]/g, '1').replace(/O/g, '0');
  requireThat(compact.startsWith(prefix), 'Invalid code prefix');
  const body = compact.slice(prefix.length);
  requireThat(body.length > 0, 'Empty code');
  let bits = 0, value = 0;
  const bytes = [];
  for (const char of body) {
    const digit = alphabet.indexOf(char);
    requireThat(digit >= 0, 'Invalid code character');
    value = (value << 5) | digit; bits += 5;
    if (bits >= 8) { bits -= 8; bytes.push((value >>> bits) & 255); }
    value &= (1 << bits) - 1;
  }
  requireThat(bits < 5 && value === 0, 'Invalid code padding');
  return new Uint8Array(bytes);
}
export function crc16(bytes) {
  let crc = 0xffff;
  for (const byte of bytes) {
    crc ^= byte << 8;
    for (let i = 0; i < 8; i++) crc = ((crc << 1) ^ ((crc & 0x8000) ? 0x1021 : 0)) & 0xffff;
  }
  return crc;
}
export async function verifySignature(publicKey, signature, domain, payload) {
  try {
    const raw = typeof publicKey === 'string' ? unbase64(publicKey) : publicKey;
    if (raw.length !== 32 || signature.length !== 64) return false;
    const key = await crypto.subtle.importKey('raw', raw, { name: 'Ed25519' }, false, ['verify']);
    return await crypto.subtle.verify({ name: 'Ed25519' }, key, signature, message(domain, payload));
  } catch { return false; }
}
function issuerKey(issuer, env) {
  requireThat(issuer === 1 || issuer === 2, 'Unknown issuer');
  return env[issuer === 1 ? 'OWNER_KEY' : 'MIKA_KEY'] ?? DEFAULT_KEYS[issuer === 1 ? 'OWNER_KEY' : 'MIKA_KEY'];
}
function readLogin(payload, offset) {
  const length = payload[offset];
  requireThat(length >= 1 && length <= 39 && payload.length === offset + 1 + length, 'Invalid login length');
  const login = new TextDecoder('utf-8', { fatal: true }).decode(payload.subarray(offset + 1));
  requireThat(loginPattern.test(login), 'Invalid login');
  return login;
}
export async function verifyCode(kind, text, env = {}) {
  const bytes = decodeCode(kind, text);
  requireThat(bytes[0] === 2, 'Unsupported code version');
  if (kind === 'RYQ') {
    requireThat(bytes.length === 35 && crc16(bytes.subarray(0, 33)) === new DataView(bytes.buffer).getUint16(33), 'Invalid request CRC or length');
    return { kind, code: encodeCode(kind, bytes), cardId: await cardId(bytes.subarray(1, 33)), device: bytes.slice(1, 33) };
  }
  requireThat(domains[kind] && bytes.length >= 76, 'Invalid code kind or length');
  const payload = bytes.subarray(0, -64), signature = bytes.subarray(-64), issuer = payload[1];
  const key = issuerKey(issuer, env);
  let fields;
  if (kind === 'RYB') {
    requireThat(payload.length >= 39, 'Invalid badge length');
    const role = payload[2], device = payload.slice(3, 35);
    requireThat(role >= 1 && role <= 5 && (issuer === 1 || role <= 2), 'Issuer cannot grant role');
    fields = { role, cardId: await cardId(device), device, day: new DataView(payload.buffer, payload.byteOffset).getUint16(35), login: readLogin(payload, 37), id: (await digest(bytes)).slice(0, 16) };
  } else if (kind === 'RYX') {
    requireThat(payload.length === 12, 'Invalid revocation length');
    fields = { id: hex(payload.subarray(2, 10)), day: new DataView(payload.buffer, payload.byteOffset).getUint16(10) };
  } else {
    requireThat(payload.length >= 40, 'Invalid approval length');
    fields = { sha: hex(payload.subarray(2, 34)), approvedAt: new DataView(payload.buffer, payload.byteOffset).getUint32(34), login: readLogin(payload, 38) };
  }
  requireThat(await verifySignature(key, signature, domains[kind], payload), 'Invalid signature');
  return { kind, code: encodeCode(kind, bytes), issuer, ...fields };
}
export function cardPayload(record) {
  requireThat(record && typeof record === 'object' && !Array.isArray(record), 'Invalid card record');
  requireThat(Object.keys(record).sort().join(',') === 'body,id,picture,sig,updated,v', 'Invalid record fields');
  requireThat(record.v === 2 && typeof record.id === 'string' && CARD_ID.test(record.id) && Number.isSafeInteger(record.updated) && record.updated >= 0, 'Invalid card metadata');
  requireThat(record.picture === '' || (typeof record.picture === 'string' && SHA.test(record.picture)), 'Invalid picture hash');
  const body = unbase64(record.body);
  requireThat(body.length >= 19 && body.length <= 78 && body[0] >> 4 === 3 && (body[0] & 3) === 0 && ((body[0] >> 2) & 3) <= 2 && body[5] >> 5 <= 5 && ((body[5] >> 2) & 7) <= 5, 'Invalid card body');
  requireThat(hex(body.subarray(6, 14)) === record.id, 'Card id mismatch');
  let offset = 14;
  for (const cap of [14, 20, 7, 10, 8]) {
    requireThat(offset < body.length, 'Missing card string');
    const length = body[offset++];
    requireThat(length <= cap && offset + length <= body.length, 'Invalid card string length');
    try { new TextDecoder('utf-8', { fatal: true }).decode(body.subarray(offset, offset + length)); } catch { throw new InputError('Invalid card UTF-8'); }
    offset += length;
  }
  requireThat(offset === body.length, 'Trailing card bytes');
  const numbers = new Uint8Array(10), view = new DataView(numbers.buffer);
  view.setBigUint64(0, BigInt(record.updated)); view.setUint16(8, body.length);
  return concat(unhex(record.id), numbers, body, record.picture ? unhex(record.picture) : new Uint8Array(32));
}
export async function verifyCard(record, device) {
  return verifySignature(device, unbase64(record.sig), 'ryoku-card-v2', cardPayload(record));
}
export async function verifyUpload(header, method, path, sha, env, now) {
  requireThat(typeof header === 'string', 'Missing upload authorization');
  const match = /^([12])\.(0|[1-9][0-9]*)\.([A-Za-z0-9+/=]+)$/.exec(header);
  requireThat(match && Number.isSafeInteger(Number(match[2])) && Math.abs(now - Number(match[2])) <= 300, 'Invalid upload authorization window');
  const payload = encoder.encode(`${method}\n${path}\n${match[2]}\n${sha}`);
  requireThat(await verifySignature(issuerKey(Number(match[1]), env), unbase64(match[3]), 'ryoku-upload-v1', payload), 'Invalid upload signature');
}
export function jpegDimensions(bytes) {
  requireThat(bytes.length >= 4 && bytes[0] === 255 && bytes[1] === 216, 'Invalid JPEG header');
  let offset = 2;
  while (offset < bytes.length) {
    requireThat(bytes[offset++] === 255, 'Invalid JPEG marker');
    while (bytes[offset] === 255) offset++;
    const marker = bytes[offset++];
    requireThat(marker !== undefined && marker !== 0 && marker !== 216 && marker !== 217 && marker !== 218, 'Missing JPEG frame');
    if (marker === 1 || (marker >= 208 && marker <= 215)) continue;
    requireThat(offset + 2 <= bytes.length, 'Truncated JPEG segment');
    const length = (bytes[offset] << 8) | bytes[offset + 1];
    requireThat(length >= 2 && offset + length <= bytes.length, 'Truncated JPEG segment');
    if (marker >= 192 && marker <= 207 && ![196, 200, 204].includes(marker)) {
      requireThat(length >= 8, 'Invalid JPEG frame');
      const height = (bytes[offset + 3] << 8) | bytes[offset + 4], width = (bytes[offset + 5] << 8) | bytes[offset + 6];
      requireThat(width > 0 && height > 0 && width <= 1120 && height <= 780, 'JPEG dimensions exceed 1120x780');
      return { width, height };
    }
    offset += length;
  }
  throw new InputError('Missing JPEG frame');
}

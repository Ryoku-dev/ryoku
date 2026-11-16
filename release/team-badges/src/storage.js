import { InputError, verifyCode } from './codec.js';

const ATTEMPTS = 8;
const condition = object => object ? { etagMatches: object.etag } : { etagDoesNotMatch: '*' };
const conflict = message => { throw new InputError(message, 409); };

export async function reserveBadge(env, badge, initialActive) {
  const key = `logins/${badge.login.toLowerCase()}.json`;
  for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
    const stored = await env.BUCKET.get(key);
    const ids = stored ? JSON.parse(await stored.text()) : initialActive.filter(b => b.login.toLowerCase() === badge.login.toLowerCase()).map(b => b.id);
    if (ids.includes(badge.id)) return;
    const active = [];
    for (const id of ids) {
      const revoked = await env.BUCKET.get(`revoked/${id}.txt`);
      let valid = false;
      if (revoked) {
        try {
          const revocation = await verifyCode('RYX', await revoked.text(), env);
          const original = await env.BUCKET.get(`badges/${id}.txt`);
          const issuer = original ? (await verifyCode('RYB', await original.text(), env)).issuer : null;
          valid = revocation.id === id && (revocation.issuer === 1 || issuer === 2);
        } catch { /* Corrupt revocations must not release a badge slot. */ }
      }
      if (!valid) active.push(id);
    }
    if (active.length >= 8) conflict('Login already has eight active badges');
    if (await env.BUCKET.put(key, JSON.stringify([...active, badge.id]), { onlyIf: condition(stored) })) return;
  }
  conflict('Badge registration changed concurrently');
}

export async function storeDesign(env, design) {
  const key = `designs/${design.login.toLowerCase()}.txt`;
  for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
    const stored = await env.BUCKET.get(key);
    if (stored) {
      const existing = await verifyCode('RYD', await stored.text(), env);
      if (existing.code === design.code) return false;
      if (design.approvedAt <= existing.approvedAt) conflict('Design approval is not newer');
    }
    if (!await env.BUCKET.head(`packs/${design.sha}.zip`)) {
      const pack = await env.BUCKET.get(`pending/${design.sha}.zip`);
      if (!pack) throw new InputError('Design pack not found', 404);
      await env.BUCKET.put(`packs/${design.sha}.zip`, pack.body, { httpMetadata: { contentType: 'application/zip' } });
    }
    if (await env.BUCKET.put(key, design.code, { onlyIf: condition(stored) })) return true;
  }
  conflict('Design approval changed concurrently');
}

export async function storeRecord(env, record, picture) {
  const key = `cards/${record.id}.json`;
  // Each candidate owns its picture object. Losing writes can delete theirs without
  // deleting a concurrent winner's identical hash or a later reuse of an old image.
  const pictureKey = record.picture ? `cards/${record.id}/${crypto.randomUUID()}/${record.picture}.jpg` : '';
  let published = false;
  try {
    for (let attempt = 0; attempt < ATTEMPTS; attempt++) {
      const stored = await env.BUCKET.get(key);
      const old = stored ? JSON.parse(await stored.text()) : null;
      if (old && (!Number.isSafeInteger(old.updated) || record.updated <= old.updated)) throw new InputError('Card record is not newer');
      if (record.picture && !await env.BUCKET.head(pictureKey)) {
        let bytes = picture;
        if (!bytes && old?.picture === record.picture && stored.customMetadata?.pictureKey) bytes = (await env.BUCKET.get(stored.customMetadata.pictureKey))?.body;
        if (!bytes) throw new InputError('Missing picture');
        await env.BUCKET.put(pictureKey, bytes, { httpMetadata: { contentType: 'image/jpeg' } });
      }
      if (!await env.BUCKET.put(key, JSON.stringify(record), {
        onlyIf: condition(stored), httpMetadata: { contentType: 'application/json' }, customMetadata: { pictureKey },
      })) continue;
      published = true;
      if (stored?.customMetadata?.pictureKey) await env.BUCKET.delete(stored.customMetadata.pictureKey);
      return old;
    }
    conflict('Card record changed concurrently');
  } finally {
    if (!published && pictureKey) await env.BUCKET.delete(pictureKey);
  }
}

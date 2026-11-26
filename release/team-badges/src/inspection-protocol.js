import { InputError, base64, unbase64, digest, cardId, message } from './codec.js';

export const LIMITS = Object.freeze({ canonical: 8 * 1024 * 1024, poster: 8 * 1024 * 1024, preview: 24 * 1024 * 1024, report: 8 * 1024 * 1024 });
export const fail = (text, status = 400) => { throw new InputError(text, status); };
export const hex = value => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
export function fields(value, required, optional = []) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || required.some(key => !Object.hasOwn(value, key)) || Object.keys(value).some(key => !required.includes(key) && !optional.includes(key))) fail('Invalid protocol fields');
}
export function text(value, max = 256) {
  if (typeof value !== 'string' || !value.length || value.length > max || /[\u0000-\u001f\u007f\ud800-\udfff]/u.test(value)) fail('Invalid protocol string');
  return value;
}
export function canonical(value, depth = 0) {
  if (depth > 12) fail('Protocol nesting limit');
  if (value === null || typeof value === 'boolean') return JSON.stringify(value);
  if (typeof value === 'number') { if (!Number.isSafeInteger(value) || Object.is(value, -0)) fail('Invalid protocol integer'); return String(value); }
  if (typeof value === 'string') { if (value.length > 4096 || /[\ud800-\udfff]/u.test(value)) fail('Invalid protocol Unicode'); return JSON.stringify(value); }
  if (Array.isArray(value)) { if (value.length > 4096) fail('Protocol array limit'); return `[${value.map(item => canonical(item, depth + 1)).join(',')}]`; }
  if (value && Object.getPrototypeOf(value) === Object.prototype) {
    const keys = Object.keys(value);
    if (keys.length > 64 || keys.some(key => !/^[A-Za-z][A-Za-z0-9]{0,63}$/.test(key))) fail('Invalid canonical object keys');
    return `{${keys.sort().map(key => `${canonical(key, depth + 1)}:${canonical(value[key], depth + 1)}`).join(',')}}`;
  }
  fail('Invalid canonical value');
}
export const encoded = value => new TextEncoder().encode(canonical(value));
export function decode64(value, size) {
  if (typeof value !== 'string' || value.length > 65536) fail('Invalid base64');
  let bytes; try { bytes = unbase64(value); } catch { fail('Invalid base64'); }
  if (base64(bytes) !== value || (size !== undefined && bytes.length !== size)) fail('Invalid base64 length');
  return bytes;
}
export async function openEnvelope(envelope, domain, publicKey) {
  fields(envelope, ['payload', 'sig']);
  const raw = decode64(envelope.payload), signature = decode64(envelope.sig, 64);
  let payload; try { payload = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(raw)); } catch { fail('Invalid signed JSON'); }
  if (new TextDecoder().decode(raw) !== canonical(payload)) fail('Noncanonical signed JSON');
  const key = await crypto.subtle.importKey('raw', decode64(publicKey, 32), 'Ed25519', false, ['verify']);
  if (!await crypto.subtle.verify('Ed25519', key, signature, message(domain, raw))) fail('Invalid envelope signature', 403);
  return { payload, sha: await digest(raw) };
}
export function inventory(files, names = null) {
  if (!Array.isArray(files) || !files.length || files.length > 24) fail('Invalid file inventory');
  let previous = '';
  for (const file of files) {
    fields(file, ['name', 'sha', 'bytes']);
    if (typeof file.name !== 'string' || !/^[a-z0-9][a-z0-9_.-]{0,63}$/.test(file.name) || file.name <= previous || !hex(file.sha) || !Number.isSafeInteger(file.bytes) || file.bytes <= 0 || file.bytes > 24 * 1024 * 1024) fail('Invalid file inventory');
    previous = file.name;
  }
  if (names && files.map(file => file.name).join(',') !== names.join(',')) fail('Incomplete output inventory');
}
export async function submission(envelope, audience, now, fresh = true) {
  // the candidate key selects verification, it does not grant admission
  let candidate; try { candidate = JSON.parse(new TextDecoder().decode(decode64(envelope?.payload))); } catch { fail('Invalid manifest'); }
  const opened = await openEnvelope(envelope, 'ryoku-submit-v1', candidate?.device), p = opened.payload;
  fields(p, ['v', 'cardId', 'device', 'originalSha', 'files', 'termsVersion', 'attestationVersion', 'ownOrLicensed', 'policyAccepted', 'consentToReview', 'nonce', 'submittedAt', 'audience', 'kind']);
  if (p.v !== 1 || !/^[0-9a-f]{16}$/.test(p.cardId) || p.cardId !== await cardId(decode64(p.device, 32)) || !hex(p.originalSha) || p.audience !== audience || !['design', 'hero'].includes(p.kind) || p.ownOrLicensed !== true || p.policyAccepted !== true || p.consentToReview !== true || !Number.isSafeInteger(p.submittedAt) || (fresh && Math.abs(now - p.submittedAt) > 600) || !/^[A-Za-z0-9_-]{22,86}$/.test(p.nonce)) fail('Invalid submission manifest');
  text(p.termsVersion, 80); text(p.attestationVersion, 80); inventory(p.files);
  if (p.kind === 'design' && (!p.files.some(f => f.name === 'design.json') || p.files.some(f => f.name !== 'design.json' && !/^[a-z0-9][a-z0-9_-]{0,31}\.(png|webp|jpg|jpeg|gif)$/.test(f.name)))) fail('Invalid design inventory');
  if (p.kind === 'hero' && (p.files.length !== 1 || !/^[a-z0-9][a-z0-9_-]{0,31}\.(png|jpg|jpeg|gif|webp)$/.test(p.files[0].name))) fail('Invalid hero inventory');
  if (p.files.reduce((sum, f) => sum + f.bytes, 0) > 24 * 1024 * 1024) fail('Expanded archive too large', 413);
  return opened;
}
export function registry(config, id) {
  let keys; try { keys = JSON.parse(config); } catch { fail('Signing authority unavailable', 503); }
  const key = keys && Object.hasOwn(keys, id) ? keys[id] : null;
  if (!key || key.revoked !== false || !key.publicKey) fail('Signing authority unavailable or revoked', 403);
  return key;
}
export async function certificate(envelope, job, env, now) {
  let candidate; try { candidate = JSON.parse(new TextDecoder().decode(decode64(envelope?.payload))); } catch { fail('Invalid certificate'); }
  text(candidate?.keyId, 80);
  const authority = registry(env.INSPECTOR_KEYS, candidate.keyId);
  const opened = await openEnvelope(envelope, 'ryoku-inspect-v1', authority.publicKey), p = opened.payload;
  fields(p, ['v', 'keyId', 'jobId', 'leaseSha', 'manifestSha', 'originalSha', 'canonicalSha', 'files', 'posterSha', 'previewSha', 'reportSha', 'pipelineVersion', 'policyVersion', 'inspectedAt', 'technical', 'moderation']);
  fields(p.technical, ['archive', 'decode', 'canonical', 'render']);
  if (p.v !== 1 || p.jobId !== job.id || p.leaseSha !== job.leaseSha || p.manifestSha !== job.manifestSha || p.originalSha !== job.manifest.originalSha || !Number.isSafeInteger(p.inspectedAt) || p.inspectedAt > now + 60 || p.inspectedAt < job.claimedAt || p.pipelineVersion !== env.INSPECTOR_PIPELINE_VERSION || p.policyVersion !== env.INSPECTOR_POLICY_VERSION || !p.pipelineVersion || !p.policyVersion || Object.values(p.technical).some(v => v !== 'pass') || !['unreviewed', 'clear', 'flagged', 'unavailable'].includes(p.moderation)) fail('Certificate does not authorize current job', 403);
  inventory(p.files, Object.keys(LIMITS).sort());
  for (const file of p.files) {
    const object = job.outputs[file.name];
    if (!object || object.sha !== file.sha || object.bytes !== file.bytes || file.bytes > LIMITS[file.name] || p[`${file.name}Sha`] !== file.sha) fail('Certificate output mismatch', 403);
  }
  return opened;
}
export async function approval(envelope, expected, env) {
  let candidate; try { candidate = JSON.parse(new TextDecoder().decode(decode64(envelope?.payload))); } catch { fail('Invalid design authority'); }
  text(candidate?.keyId, 80);
  const authority = registry(env.DESIGN_APPROVAL_KEYS, candidate.keyId);
  const opened = await openEnvelope(envelope, 'ryoku-design-approval-v3', authority.publicKey);
  fields(opened.payload, [...Object.keys(expected), 'keyId']);
  if (authority.role !== expected.reviewerRole || canonical(opened.payload) !== canonical({ ...expected, keyId: candidate.keyId })) fail('Design authority scope mismatch', 403);
  return opened;
}

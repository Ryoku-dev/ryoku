// Prints the roles the web library resolves for one palette object, as JSON.
// Usage: node roles.mjs <palette.json> <follow|dark|light> [standard|high] [off|low|medium]
import { readFileSync } from 'node:fs';
import { rolesFromPalette } from '../web/palette.js';

const [file, mode, contrast = 'standard', tint = 'off'] = process.argv.slice(2);
const roles = rolesFromPalette(JSON.parse(readFileSync(file, 'utf8')), { mode, contrast, tint });

function hex(value) {
  const text = String(value).trim();
  let match = text.match(/^#([0-9a-f]{6})([0-9a-f]{2})?$/i);
  if (match) return `#${match[1]}${match[2] ?? 'ff'}`.toLowerCase();
  match = text.match(/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:\s*[,/]\s*([\d.]+%?))?\s*\)$/i);
  if (!match) throw new Error(`unreadable colour ${text}`);
  const byte = (n) => Math.round(Number(n)).toString(16).padStart(2, '0');
  const alpha = match[4] === undefined ? 1 : match[4].endsWith('%') ? Number(match[4].slice(0, -1)) / 100 : Number(match[4]);
  return `#${byte(match[1])}${byte(match[2])}${byte(match[3])}${byte(alpha * 255)}`;
}

const out = { source: roles.source };
for (const [role, value] of Object.entries(roles)) {
  if (role !== 'source' && typeof value === 'string') out[role] = hex(value);
}
console.log(JSON.stringify(out));

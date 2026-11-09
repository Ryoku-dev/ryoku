// Compares role sets printed by the stacks against the first one. Exits 1 on any difference.
// Usage: node compare.mjs <case> go=<file> qml=<file> [web=<file>]
import { readFileSync } from 'node:fs';

const [name, ...pairs] = process.argv.slice(2);
const sets = pairs.map((pair) => {
  const [stack, file] = pair.split('=');
  return [stack, JSON.parse(readFileSync(file, 'utf8') || '{}')];
});
const [reference, expected] = sets[0];

// Qt keeps colours as floats, so one step per channel is rounding, not drift.
function near(a, b) {
  if (typeof a !== 'string' || typeof b !== 'string' || a.length !== 9 || b.length !== 9) return a === b;
  for (let i = 1; i < 9; i += 2) {
    if (Math.abs(parseInt(a.slice(i, i + 2), 16) - parseInt(b.slice(i, i + 2), 16)) > 1) return false;
  }
  return true;
}

// Every role in the reference must match in every stack. The web resolver also
// exports conveniences CSS cannot compute (jasperInk), which are not compared.
const problems = Object.keys(expected).length === 0 ? [`${reference} printed nothing`] : [];
for (const [role, value] of Object.entries(expected)) {
  for (const [stack, roles] of sets.slice(1)) {
    if (role === 'source' && roles.source === undefined) continue;
    if (!near(value, roles[role])) problems.push(`${role}: ${reference} ${value}, ${stack} ${roles[role]}`);
  }
}

if (problems.length === 0) {
  console.log(`ok   ${name}`);
} else {
  console.log(`FAIL ${name}\n     ${problems.join('\n     ')}`);
  process.exit(1);
}

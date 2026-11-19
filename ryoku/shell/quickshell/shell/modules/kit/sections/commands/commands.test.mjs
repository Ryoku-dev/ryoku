import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { execFileSync } from 'node:child_process';
const logic = vm.createContext({});
vm.runInContext(fs.readFileSync(new URL('./commands.js', import.meta.url), 'utf8'), logic);
const plain = value => JSON.parse(JSON.stringify(value));
const tr = text => ({ 'Updates': 'Actualizaciones', 'Read version': 'Leer versión' }[text] || text);
const entries = [
    { id: 'a', library: 'ryoku', group: 'updates', title: 'Read version', description: 'Print version', command: 'ryoku version', source: 'catalogue', tags: ['release'] },
    { id: 'b', library: 'linux', group: 'files', title: 'List', description: 'Files', command: 'ls', source: 'catalogue' },
    { id: 'c', library: 'saved', group: 'my-commands', title: 'Mine', description: '', command: 'printf ok', source: 'user' }
];
const libraries = [{ groups: [{ id: 'updates', label: 'Updates' }, { id: 'files', label: 'Files' }] }];
assert.equal(logic.pageId('group:linux:files'), 'linux');
assert.equal(logic.pageId('library:saved'), 'saved');
assert.equal(logic.pageId('bad'), 'ryoku');
assert.equal(logic.list(entries, libraries, [], 'ryoku', 'leer actualizaciones', tr).length, 1);
assert.equal(logic.list(entries, libraries, [], 'ryoku', 'RELEASE', tr).length, 1);
assert.equal(logic.list(entries, libraries, [], 'linux', '', tr).length, 1);
assert.deepEqual(plain(logic.list(entries, libraries, ['a'], 'saved', '', tr).map(row => row.entry.id)), ['a', 'c']);
assert.equal(logic.list(entries, libraries, [], 'saved', 'missing', tr).length, 0);
assert.deepEqual(plain(logic.placeholders('cp <source> <destination>; echo <source>')), ['source', 'destination']);
assert.equal(logic.fill('printf %s <name>', { name: "a'b $(echo unsafe)" }), "printf %s 'a'\\''b $(echo unsafe)'");
for (const command of ['cmd [option]', 'cmd a|b', 'cmd ...', 'cmd …', 'cmd\nother']) assert.equal(logic.runnable({ command }), false);
assert.equal(logic.runnable({ command: 'echo ok', danger: true }), false);
assert.equal(logic.runnable({ command: 'echo ok', onlyOn: 'niri' }), false);
assert.equal(logic.runnable({ command: 'printf %s <name>' }), true);
assert.equal(execFileSync('bash', ['-c', logic.terminalLine('printf marker; exit 3', 'Press Enter to close this window.')], { input: '\n', encoding: 'utf8' }), 'marker\nPress Enter to close this window.\n');
console.log('commands: list, translated search, saved pages, placeholders, run guards and keep-open terminal pass');
for (const template of ["printf %s <value>", 'printf %s \"<value>\"', "printf %s '<value>'"]) {
    const value = "a'b $(printf unsafe) `printf bad` \\\" \\\\ end";
    assert.equal(execFileSync('bash', ['-c', logic.fill(template, { value })], { encoding: 'utf8' }), value);
}

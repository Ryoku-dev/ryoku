import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const path = fileURLToPath(new URL('./KeepsakeSequence.js', import.meta.url));
const state = vm.runInNewContext(fs.readFileSync(path, 'utf8') + '\nstep;', {});

function sequence(chars, gap = 100, modified = false, repeated = false) {
    let index = 0, at = 0, complete = false, clock = 10000;
    for (const char of chars) {
        const out = state(index, at, char, clock, modified, repeated);
        index = out.index; at = out.at; complete = out.complete;
        clock += gap;
    }
    return { index, complete };
}

test('only the exact eight letters open the keepsake', () => {
    assert.equal(sequence('WWSSADAD').complete, true);
    assert.equal(sequence('WWSSADA').complete, false);
    assert.equal(sequence('WWSAADAD').complete, false);
});

test('the sequence expires when typing stops', () => {
    assert.equal(sequence('WWSSADAD', 3000).complete, false);
});

test('shortcuts and key repeats cannot complete the sequence', () => {
    assert.equal(sequence('WWSSADAD', 100, true).complete, false);
    assert.equal(sequence('WWSSADAD', 100, false, true).complete, false);
});

test('a mismatch starts over at a possible new W', () => {
    assert.equal(sequence('XWWSSADAD').complete, true);
    assert.equal(sequence('WQWWSSADAD').complete, true);
});

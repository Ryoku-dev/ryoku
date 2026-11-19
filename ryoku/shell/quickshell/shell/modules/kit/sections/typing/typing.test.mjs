import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const qml = readFileSync(new URL('./TypingTestEngine.qml', import.meta.url), 'utf8');
function method(name) {
    const start = qml.indexOf(`function ${name}(`);
    assert.ok(start >= 0, name);
    const body = qml.indexOf('{', start);
    let depth = 1, end = body + 1;
    while (depth && end < qml.length) {
        if (qml[end] === '{') depth++;
        if (qml[end] === '}') depth--;
        end++;
    }
    return qml.slice(start, end).replace(/\)\s*:\s*var\s*\{/, ') {');
}
function engine(options = {}) {
    let clock = 10000;
    const root = {
        state: 'ready', mode: 'words', zenGuided: false, timeLimitSeconds: 30,
        wordLimit: 2, punctuation: false, numbers: false, finishOnLastWord: true,
        languagePack: { id: 'test', words: ['cat', 'dog', 'sun', 'moon'] },
        seed: 1, _randomState: 1, targetWords: ['cat', 'dog'], targetText: 'cat dog',
        targetChars: Array.from('cat dog'), inputText: '', previousInputText: '',
        correctInputEvents: 0, incorrectInputEvents: 0, currentWordIndex: 0,
        startedAt: 0, finishedAt: 0, elapsedSeconds: 0, samples: [],
        _lastSampleSecond: 0, _lastSampleChars: 0,
        charTyped() {}, charDeleted() {}, resetRequested() {}, started() {}, finished() {},
        ...options
    };
    const context = vm.createContext({ root, Date, Math, Array, Number, String });
    for (const name of qml.matchAll(/function (\w+)\(/g))
        root[name[1]] = vm.runInContext(`(${method(name[1])})`, context);
    root.now = () => clock;
    root.advance = ms => { clock += ms; root.tick(); };
    const getters = {
        isReady: () => root.state === 'ready', isRunning: () => root.state === 'running',
        isFinished: () => root.state === 'finished', hasTarget: () => root.targetRequired(),
        endlessTarget: () => root.targetIsEndless(), rawCharacterCount: () => root.codePointCount(root.inputText),
        correctCharacterCount: () => root.hasTarget ? root.countCharacterBreakdown().correct : root.rawCharacterCount,
        wpm: () => root.calculateWpm(root.correctCharacterCount, root.elapsedSeconds),
        rawWpm: () => root.calculateWpm(root.rawCharacterCount, root.elapsedSeconds),
        accuracy: () => root.correctInputEvents + root.incorrectInputEvents ? 100 * root.correctInputEvents / (root.correctInputEvents + root.incorrectInputEvents) : 100,
        characterBreakdown: () => root.countCharacterBreakdown()
    };
    for (const [name, get] of Object.entries(getters)) Object.defineProperty(root, name, { get });
    return root;
}
const e = engine();
assert.equal(e.calculateWpm(250, 60), 50);
assert.equal(e.calculateWpm(12, 0), 0);
assert.equal(e.codePointCount('a😀é'), 3);
e.updateInput('cat '); e.advance(6000); e.updateInput('cat dog');
assert.equal(e.state, 'finished');
assert.ok(Math.abs(e.wpm - 14) < 1e-9);
assert.equal(e.accuracy, 100);
assert.deepEqual({ ...e.characterBreakdown }, { correct: 7, incorrect: 0, extra: 0, missed: 0 });
const payload = e.resultPayload();
assert.equal(payload.modeValue, 2); assert.equal(payload.wpm, 14); assert.equal(payload.duration, 6);
e.updateInput('cat dog LEAK'); assert.equal(e.inputText, 'cat dog');
const correction = engine();
correction.updateInput('x'); correction.updateInput(''); correction.updateInput('c');
assert.equal(correction.correctInputEvents, 1); assert.equal(correction.incorrectInputEvents, 1);
assert.equal(correction.accuracy, 50);
const wrong = engine({ inputText: 'caat do', previousInputText: 'caat do' });
assert.deepEqual({ ...wrong.characterBreakdown }, { correct: 4, incorrect: 2, extra: 1, missed: 1 });
const unicode = engine({ targetWords: ['café', '😀'], targetChars: Array.from('café 😀'), inputText: 'café 😀' });
assert.equal(unicode.characterBreakdown.correct, 6);
const timed = engine({ mode: 'time', timeLimitSeconds: 2 });
timed.updateInput('cat'); timed.advance(2500);
assert.equal(timed.state, 'finished'); assert.equal(timed.elapsedSeconds, 2); assert.equal(timed.wpm, 18);
const zen = engine({ mode: 'zen' }); zen.updateInput('write freely'); zen.advance(6000); zen.finish();
assert.equal(zen.characterBreakdown.correct, 12); assert.ok(Math.abs(zen.wpm - 24) < 1e-9);
const samples = engine({ samples: [{ burst: 60 }, { burst: 60 }] });
assert.equal(samples.consistency(), 100); samples.samples = []; assert.equal(samples.consistency(), 0);
const seed = engine(); seed.seed = 83; seed.reset(true); const target = seed.targetText; seed.reset(true); assert.equal(seed.targetText, target);
console.log('typing engine: scoring, Unicode, corrections, clock, completion, late input, zen, consistency and seeds pass');

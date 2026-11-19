import assert from 'node:assert/strict';
import { copyFileSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import vm from 'node:vm';
import { test } from 'node:test';
const source = readFileSync(new URL('./devtools.js', import.meta.url), 'utf8');
const engine = vm.runInNewContext(source + '\n;({ runTool, workLimits, regexRequest });', { TextEncoder, TextDecoder });
const run = engine.runTool;

const random = () => ({ bytes: Array.from({ length: 32768 }, (_, i) => i % 251), index: 0 });

test('JSON formats, recursively sorts arrays, minifies and reports invalid input', () => {
    assert.equal(run('json_formatter', '{"z":[{"b":1,"a":2}],"a":true}', { sortKeys: true, indent: 'minified' }).output, '{"a":true,"z":[{"a":2,"b":1}]}');
    assert.equal(run('json_formatter', '{"x":1}', { indent: 'tab' }).output, '{\n\t"x": 1\n}');
    assert.match(run('json_formatter', '{"x":}').error, /JSON syntax error/);
    assert.equal(run('json_formatter', ' ').output, '');
});
test('Base64 handles UTF-8, URL-safe data and invalid characters', () => {
    const encoded = run('base64', 'café 日本 🌍').output;
    assert.equal(encoded, 'Y2Fmw6kg5pel5pysIPCfjI0=');
    assert.equal(run('base64', encoded, { mode: 'decode' }).output, 'café 日本 🌍');
    assert.equal(run('base64', '🚀', { urlSafe: true }).output, '8J-agA');
    assert.equal(run('base64', '8J-agA', { mode: 'decode', urlSafe: true }).output, '🚀');
    assert.ok(run('base64', '!!!!', { mode: 'decode' }).error);
});
test('URL transforms distinguish whole URLs and components', () => {
    assert.equal(run('url_encode', 'a b&c').output, 'a%20b%26c');
    assert.equal(run('url_encode', 'https://example.com/a b', { component: false }).output, 'https://example.com/a%20b');
    assert.equal(run('url_encode', 'a%20b', { mode: 'decode' }).output, 'a b');
    assert.ok(run('url_encode', '%ZZ', { mode: 'decode' }).error);
});
test('JWT reads header and claims without implying signature verification', () => {
    const output = run('jwt_decoder', 'eyJhbGciOiJub25lIn0.eyJzdWIiOiJleGFtcGxlIn0.').output;
    assert.match(output, /Signature is not verified/);
    assert.match(output, /Subject \(sub\): example/);
    assert.match(output, /"alg": "none"/);
    assert.ok(run('jwt_decoder', 'not-a-token').error);
});
test('timestamp accepts epoch seconds, milliseconds, ISO dates and rejects invalid dates', () => {
    assert.match(run('unix_timestamp', '0').output, /1970-01-01T00:00:00.000Z/);
    assert.match(run('unix_timestamp', '1700000000000').output, /Unix \(seconds\):  1700000000/);
    assert.match(run('unix_timestamp', '2024-01-01T00:00:00Z').output, /1704067200/);
    assert.ok(run('unix_timestamp', 'not a date').error);
});
test('colour converts RGB, HSL, shorthand and alpha', () => {
    assert.match(run('color_converter', 'rgb(255, 0, 0)').output, /hsl\(0, 100%, 50%\)/);
    assert.match(run('color_converter', 'hsl(120, 100%, 50%)').output, /#00FF00/);
    assert.match(run('color_converter', '#abcd').output, /#AABBCCDD/);
    assert.ok(run('color_converter', 'no colour').error);
});
test('regex reports groups and positions, handles zero-length matches and invalid patterns', () => {
    const output = run('regex_tester', 'cat 12 dog 34', { pattern: '(\\d+)', flags: 'g' }).output;
    assert.match(output, /Found 2 matches/);
    assert.match(output, /at index 4 \(length 2\): "12"/);
    assert.match(output, /Group 1: "34"/);
    assert.match(run('regex_tester', 'ab', { pattern: '(?:)', flags: 'g' }).output, /Found 3 matches/);
    assert.ok(run('regex_tester', '', { pattern: '[' }).error);
});
test('diff preserves shared lines and reports additions and removals', () => {
    assert.equal(run('text_diff', 'alpha\nbeta\n===DIFF_SPLIT===\nalpha\ngamma').output, '--- Original (2 lines)\n+++ Modified (2 lines)\n@@ -1 +1 @@\n  alpha\n- beta\n+ gamma');
    assert.match(run('text_diff', 'same\n===DIFF_SPLIT===\nsame').output, /@@ -0 \+0 @@/);
});
test('chmod round trips special bits and rejects invalid permissions', () => {
    assert.match(run('chmod_calculator', '4755').output, /rwsr-xr-x/);
    assert.match(run('chmod_calculator', '-rw-r--r--').output, /Octal     644/);
    assert.match(run('chmod_calculator', 'rwxrwxrwt').output, /Octal     1777/);
    assert.match(run('chmod_calculator', 'rwSr-xr-x').output, /Octal     4655/);
    assert.ok(run('chmod_calculator', '999').error);
});
test('password uses supplied secure bytes and all selected character classes', () => {
    const state = random();
    const output = run('password', '', { length: 32, quantity: 3, _randomState: state }).output;
    assert.equal(output.split('\n').length, 3);
    for (const value of output.split('\n')) {
        assert.equal(value.length, 32);
        assert.match(value, /[A-Z]/);
        assert.match(value, /[a-z]/);
        assert.match(value, /[2-9]/);
        assert.match(value, /[^a-zA-Z0-9]/);
        assert.doesNotMatch(value, /[IOlo01]/);
    }
    assert.ok(state.index > 0);
    assert.throws(() => run('password'), /Secure random source unavailable/);
    assert.throws(() => run('password', '', { _randomState: { bytes: [], index: 0 } }), /depleted/);
});
test('UUID respects version, variant, quantity, uppercase and hyphen options', () => {
    assert.match(run('uuid', '', { _randomState: random() }).output, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
    const v7 = run('uuid', '', { version: '7', quantity: 2, uppercase: true, hyphens: false, _randomState: random() }).output.split('\n');
    assert.equal(v7.length, 2);
    for (const value of v7) assert.match(value, /^[0-9A-F]{12}7[0-9A-F]{3}[89AB][0-9A-F]{15}$/);
    assert.ok(run('missing').error);
});

test('input limits reject oversized work for every tool before execution', () => {
    for (const id of ['json_formatter', 'base64', 'url_encode', 'jwt_decoder', 'unix_timestamp', 'color_converter', 'text_diff', 'chmod_calculator', 'password', 'uuid']) {
        const result = run(id, 'x'.repeat(engine.workLimits().input + 1));
        assert.equal(result.output, '');
        assert.match(result.error, /Input exceeds 32768 characters/);
    }
    assert.equal(run('base64', 'x'.repeat(engine.workLimits().input)).error, undefined);
    assert.match(run('regex_tester', 'x'.repeat(engine.workLimits().regexInput + 1), { pattern: '.' }).error, /8192/);
    assert.match(run('regex_tester', '', { pattern: 'x'.repeat(1025) }).error, /pattern exceeds/);
    assert.match(run('regex_tester', '', { flags: 'g'.repeat(17) }).error, /flags exceed/);
});

test('JSON nesting is bounded before parsing and strings do not count as structure', () => {
    const depth = engine.workLimits().jsonDepth;
    assert.equal(run('json_formatter', '['.repeat(depth) + '0' + ']'.repeat(depth), { sortKeys: true, indent: 'minified' }).error, undefined);
    const nested = '['.repeat(depth + 1) + '0' + ']'.repeat(depth + 1);
    assert.match(run('json_formatter', nested, { sortKeys: true }).error, /nesting exceeds 64/);
    const quoted = JSON.stringify({ text: '[{'.repeat(100) + '\\"\\\\' });
    assert.equal(run('json_formatter', quoted, { indent: 'minified' }).output, quoted);
    const header = Buffer.from('{}').toString('base64url');
    const payload = Buffer.from(nested).toString('base64url');
    assert.match(run('jwt_decoder', `${header}.${payload}.`).error, /nesting exceeds 64/);
});

test('diff guards line count and matrix area while permitting the exact cell budget', () => {
    const text = Array.from({ length: 256 }, (_, index) => String(index)).join('\n');
    assert.match(run('text_diff', text + '\n===DIFF_SPLIT===\n' + text).output, /@@ -0 \+0 @@/);
    assert.match(run('text_diff', text + '\nx\n===DIFF_SPLIT===\n' + text).error, /65536 line comparisons/);
    assert.match(run('text_diff', '\n'.repeat(512) + '\n===DIFF_SPLIT===\nx').error, /512 lines/);
});

test('regex limits report errors rather than returning silently truncated matches', () => {
    assert.match(run('regex_tester', 'x'.repeat(1000), { pattern: 'x' }).output, /Found 1000 matches/);
    const result = run('regex_tester', 'x'.repeat(1001), { pattern: 'x' });
    assert.equal(result.output, '');
    assert.match(result.error, /exceeds 1000 matches/);
    const captures = run('regex_tester', 'x'.repeat(8192), { pattern: '(?=' + '(x*)'.repeat(33) + ')' });
    assert.equal(captures.output, '');
    assert.match(captures.error, /Output exceeds/);
    assert.match(run('regex_tester', '🌍', { pattern: '(?:)', flags: 'gu' }).output, /Found 2 matches/);
});

test('oversized output is rejected without a partial successful result', () => {
    const result = run('url_encode', '日'.repeat(engine.workLimits().input));
    assert.equal(result.output, '');
    assert.match(result.error, /Output exceeds 262144/);
});

test('regex requests project options and bound UTF-8 including escaped surrogates', () => {
    for (const input of ['日'.repeat(8192), '\\'.repeat(8192), '\ud800'.repeat(8192), '🌍'.repeat(4096)]) {
        assert.equal(input.length, engine.workLimits().regexInput);
        const payload = engine.regexRequest(input, { pattern: 'x'.repeat(1024), flags: 'g', _randomState: { secret: true }, ignored: 'x'.repeat(100000) });
        assert.equal(payload.error, undefined);
        assert.ok(Buffer.byteLength(payload.request) <= engine.workLimits().regexRequestBytes);
        assert.deepEqual(Object.keys(JSON.parse(payload.request).options), ['pattern', 'flags']);
    }
});

test('secure random exhaustion bounds rejection sampling and maximum generator work', () => {
    assert.throws(() => run('password', '', { _randomState: { bytes: Array(32768).fill(255), index: 0 } }), /depleted/);
    const values = run('password', '', { length: Infinity, quantity: Infinity, _randomState: random() }).output.split('\n');
    assert.equal(values.length, 50);
    assert.ok(values.every(value => value.length === 128));
    assert.equal(run('uuid', '', { quantity: Infinity, _randomState: random() }).output.split('\n').length, 50);
});

const qs = (process.env.PATH || '').split(delimiter).map(directory => join(directory, 'qs')).find(path => existsSync(path));
const qtTest = { skip: qs ? false : 'qs is required for the actual offscreen regex subprocess integration' };

function regexHarness(input, options, action = 'run', unresponsive = false) {
    const directory = mkdtempSync(join(tmpdir(), 'kit-regex-'));
    try {
        const runtime = join(directory, 'runtime');
        mkdirSync(runtime, { mode: 0o700 });
        for (const file of ['RegexJob.qml', 'RegexRunner.qml', 'devtools.js'])
            copyFileSync(new URL('./' + file, import.meta.url), join(directory, file));
        const moduleUrl = pathToFileURL(directory + '/').href;
        const runnerPath = join(directory, 'RegexRunner.qml');
        const unresponsivePath = join(directory, 'UnresponsiveRunner.qml');
        if (unresponsive)
            writeFileSync(unresponsivePath, `import QtQuick
import Quickshell
ShellRoot {
    Timer {
        interval: 0
        running: true
        onTriggered: { while (true) {} }
    }
}`);
        const harness = join(directory, 'shell.qml');
        writeFileSync(harness, `import QtQuick
import Quickshell
import ${JSON.stringify(moduleUrl)} as Tools
Tools.RegexJob {
    id: job
    runnerPath: ${JSON.stringify(unresponsive ? unresponsivePath : runnerPath)}
    property int completed: 0
    property var childPid: null
    property real startedAt: Date.now()
    property Timer pidTimer: Timer {
        interval: 10
        running: true
        repeat: true
        onTriggered: {
            if (job.process && job.process.processId) {
                job.childPid = job.process.processId;
                stop();
            }
        }
    }
    property Timer actionTimer: Timer {
        interval: 100
        running: ${action !== 'run'}
        onTriggered: {
            job.childPid = job.process ? job.process.processId : null;
            if (${JSON.stringify(action)} === "cancel") {
                job.cancel();
                settle.start();
            } else {
                job.runnerPath = ${JSON.stringify(runnerPath)};
                job.run(12, ${JSON.stringify(engine.regexRequest('cat 12', { pattern: '(\\d+)' }).request)});
            }
        }
    }
    property Timer settle: Timer {
        interval: 100
        onTriggered: {
            console.log("KIT_TEST_RESULT:" + JSON.stringify({ cancelled: !job.running, completed: job.completed, pid: job.childPid }));
            Qt.quit();
        }
    }
    Component.onCompleted: run(11, Quickshell.env("KIT_TEST_REQUEST"))
    onFinished: (ticket, result) => {
        completed++;
        console.log("KIT_TEST_RESULT:" + JSON.stringify({ ticket: ticket, result: result, completed: completed, pid: childPid, running: running, elapsed: Date.now() - startedAt }));
        Qt.quit();
    }
}`);
        const environment = {
            ...process.env,
            QT_QPA_PLATFORM: 'offscreen',
            QML_DISABLE_DISK_CACHE: '1',
            QS_NO_RELOAD_POPUP: '1',
            XDG_RUNTIME_DIR: runtime,
            XDG_CACHE_HOME: join(directory, 'cache'),
            XDG_STATE_HOME: join(directory, 'state'),
            XDG_CONFIG_HOME: join(directory, 'config'),
            XDG_DATA_HOME: join(directory, 'data'),
            KIT_TEST_REQUEST: engine.regexRequest(input, options).request
        };
        for (const key of Object.keys(environment))
            if (key.startsWith('__QUICKSHELL_CRASH_')) delete environment[key];
        const result = spawnSync(qs, ['--no-color', '-p', harness], { env: environment, encoding: 'utf8', timeout: 10000, killSignal: 'SIGKILL', maxBuffer: 2 * 1024 * 1024 });
        assert.equal(result.error, undefined, result.error?.message);
        assert.equal(result.status, 0, result.stdout + result.stderr);
        const line = (result.stdout + '\n' + result.stderr).split('\n').find(value => value.includes('KIT_TEST_RESULT:'));
        assert.ok(line, result.stdout + result.stderr);
        return JSON.parse(line.slice(line.indexOf('KIT_TEST_RESULT:') + 'KIT_TEST_RESULT:'.length));
    } finally {
        rmSync(directory, { recursive: true, force: true });
    }
}

test('actual offscreen subprocess preserves groups, backreferences and Unicode indices', qtTest, () => {
    const input = '🌍 cat cat 12';
    const options = { pattern: '(?<word>cat) \\k<word>|(\\d+)', flags: 'gu' };
    const result = regexHarness(input, options);
    assert.equal(result.ticket, 11);
    assert.equal(result.result.output, run('regex_tester', input, options).output);
    assert.equal(result.result.error, undefined);
});

test('actual adversarial regex completes with a valid result or the execution deadline', qtTest, () => {
    const result = regexHarness('a'.repeat(128) + '!', { pattern: '(a+)+$', flags: 'g' });
    assert.equal(result.running, false);
    assert.ok(result.elapsed < 5000);
    if (result.result.error) {
        assert.equal(result.result.output, '');
        assert.match(result.result.error, /1500 ms execution limit/);
    } else {
        assert.equal(result.result.output, 'No matches found.');
    }
});

test('actual production deadline kills an unresponsive runner without a partial result', qtTest, () => {
    const result = regexHarness('', {}, 'run', true);
    assert.equal(result.result.output, '');
    assert.match(result.result.error, /1500 ms execution limit/);
    assert.equal(result.running, false);
    assert.ok(result.elapsed < 5000);
    assert.ok(result.pid);
    assert.throws(() => process.kill(result.pid, 0), { code: 'ESRCH' });
});

test('actual cancellation kills the child without delivering a stale result', qtTest, () => {
    const result = regexHarness('', {}, 'cancel', true);
    assert.equal(result.cancelled, true);
    assert.equal(result.completed, 0);
    assert.ok(result.pid);
    assert.throws(() => process.kill(result.pid, 0), { code: 'ESRCH' });
});

test('actual replacement ignores a cancelled request and delivers only its successor', qtTest, () => {
    const result = regexHarness('', {}, 'replace', true);
    assert.equal(result.ticket, 12);
    assert.equal(result.completed, 1);
    assert.match(result.result.output, /Found 1 match/);
    assert.ok(result.pid);
    assert.throws(() => process.kill(result.pid, 0), { code: 'ESRCH' });
});

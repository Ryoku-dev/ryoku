import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const E = require("./jobevents.js");

const run = lines => lines.reduce((s, l) => E.reduce(s, E.parse(l)), E.initial());

const twoSteps = [
    '{"event":"plan","action":"remove","steps":[{"manager":"pacman","name":"a","version":"1"},{"manager":"pacman","name":"b","version":"2","required_by":["c"]}]}',
    '{"event":"step","index":0,"phase":"start"}',
    'not an event',
    '{"event":"log","index":0,"line":"( 1/4) removing a"}',
    '{"event":"progress","index":0,"done":1,"total":4}'
];

test("events count the work across the whole plan as it goes", () => {
    assert.equal(E.fraction(E.initial()), -1, "nothing counted yet: the blade sweeps");
    let s = run(twoSteps);
    assert.equal(s.plan.length, 2);
    assert.deepEqual(s.plan[1].requiredBy, ["c"]);
    assert.equal(E.stepName(s), "a");
    assert.equal(s.done, 1);
    assert.equal(s.total, 4);
    assert.equal(E.fraction(s), 0.125);
    s = E.reduce(s, E.parse('{"event":"step","index":0,"phase":"done","exit":0}'));
    assert.equal(E.fraction(s), 0.5);
    s = E.reduce(s, E.parse('{"event":"step","index":1,"phase":"start"}'));
    assert.equal(E.fraction(s), 0.5, "a step with no counts keeps what is done");
    assert.equal(s.total, 0);
    assert.equal(E.stepName(s), "b");
    s = E.reduce(s, E.parse('{"event":"done","ok":true,"exit":0,"error":""}'));
    assert.equal(s.finished, true);
    assert.equal(s.ok, true);
    assert.equal(E.fraction(s), 1);
});

test("counts never run past their total and a bad event changes nothing", () => {
    let s = run(['{"event":"progress","done":9,"total":3}']);
    assert.equal(s.done, 3);
    s = E.reduce(s, E.parse('{"event":"progress","done":1,"total":0}'));
    assert.equal(s.total, 3);
    assert.equal(E.parse(""), null);
    assert.equal(E.parse('{"no":"event"}'), null);
    assert.equal(E.parse("{broken"), null);
    const before = E.initial();
    E.reduce(before, E.parse('{"event":"log","line":"x"}'));
    assert.equal(before.lastLog, "", "the old state is never changed");
});

test("a failed job says what the tool said, pkexec's own codes only when the helper never ended", () => {
    const s = run(twoSteps);
    const pk = ["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove", "--manager", "pacman", "a"];
    assert.deepEqual(E.failure(pk, 126, E.initial(), "", false), { kind: "dismissed", text: "" });
    assert.deepEqual(E.failure(pk, 127, E.initial(), "", false), { kind: "denied", text: "" });
    assert.deepEqual(E.failure(pk, 1, s, "", true), { kind: "cancelled", text: "" });
    const ended = E.reduce(s, E.parse('{"event":"done","ok":false,"exit":126,"error":"target not found: a"}'));
    assert.deepEqual(E.failure(pk, 126, ended, "", false), { kind: "message", text: "target not found: a" });
    assert.deepEqual(E.failure(["gpk"], 1, E.initial(), "one\nlast words\n\n", false), { kind: "message", text: "last words" });
    assert.deepEqual(E.failure(["gpk"], 1, s, "", false), { kind: "message", text: "( 1/4) removing a" });
    assert.deepEqual(E.failure(["gpk"], 1, E.initial(), "", false), { kind: "fallback", text: "" });
});

test("only a job that can hear it is offered Cancel", () => {
    assert.equal(E.cancellable(["gpk", "remove", "-m", "flatpak", "--yes", "--events", "x"]), true);
    assert.equal(E.cancellable(["pkexec", "ryoku-host", "apps", "--stdin-cancel", "remove"]), true);
    assert.equal(E.cancellable(["pkexec", "ryoku-host", "apps", "remove"]), false);
    assert.equal(E.cancellable([]), false);
    assert.equal(E.cancelsByStdin(["pkexec", "ryoku-host", "apps", "--stdin-cancel"]), true);
    assert.equal(E.cancelsByStdin(["gpk", "remove"]), false);
});

test("stderr keeps only its tail", () => {
    let t = "";
    for (let i = 0; i < 2000; i++)
        t = E.tail(t, "line " + i + "\n", 4096);
    assert.equal(t.length, 4096);
    assert.ok(t.endsWith("line 1999\n"));
    assert.equal(E.tail("ab", "cd", 3), "bcd");
});

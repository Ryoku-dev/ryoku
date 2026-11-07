import assert from "node:assert/strict";
import test from "node:test";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const bus = require("./busclient.js");
const { apply } = require("./buspatch.js");

// A client with a fake socket: every write is split into wire lines.
function harness(name = "shell") {
    const lines = [];
    const states = [];
    const logs = [];
    const c = bus.create({
        name,
        pid: 42,
        write: (text) => {
            for (const l of text.split("\n"))
                if (l.length)
                    lines.push(l);
        },
        applyPatch: apply,
        onConnected: (v) => states.push(v),
        log: (t) => logs.push(t),
    });
    const sent = () => lines.splice(0).map((l) => (l.startsWith("bus ") ? { bus: JSON.parse(l.slice(4)) } : JSON.parse(l)));
    const recv = (msg) => c.receive(JSON.stringify(msg));
    return { c, sent, recv, states, logs };
}

function opened(h) {
    h.c.socketOpened();
    h.recv({ op: "hello", v: 1, topics: [], methods: [] });
    return h.sent();
}

test("the handshake names the client and pid, then connected flips on hello", () => {
    const h = harness("hub");
    h.c.socketOpened();
    assert.deepEqual(h.sent(), [{ bus: { v: 1, client: "hub", pid: 42 } }]);
    assert.deepEqual(h.states, []);
    h.recv({ op: "hello", v: 1 });
    assert.deepEqual(h.states, [true]);
});

test("frames are encoded one per line and garbage lines are ignored", () => {
    assert.equal(bus.encode({ a: "x\ny" }), "{\"a\":\"x\\ny\"}\n");
    assert.equal(bus.decode("not json"), null);
    assert.equal(bus.decode("[1]"), null);
    assert.deepEqual(bus.decode("{\"op\":\"hello\"}"), { op: "hello" });
    const h = harness();
    opened(h);
    h.c.receive("}{");
    h.c.receive("");
});

test("client name: env wins, else the shell dir, quickshell stands for its parent", () => {
    assert.equal(bus.clientName("keys", "/x/hub"), "keys");
    assert.equal(bus.clientName("", "/home/u/.config/quickshell/hub"), "hub");
    assert.equal(bus.clientName("", "/home/u/.config/quickshell/hub/"), "hub");
    assert.equal(bus.clientName("", "/repo/ryoku/hub/quickshell"), "hub");
    assert.equal(bus.clientName("", "/repo/ryoku/shell/quickshell/shell"), "shell");
    assert.equal(bus.clientName(undefined, ""), "shell");
});

test("backoff doubles from 250 ms and stops at 5 s", () => {
    assert.deepEqual([0, 1, 2, 3, 4, 5, 9].map(bus.backoff), [250, 500, 1000, 2000, 4000, 5000, 5000]);
});

test("socketClosed returns growing delays and hello resets them", () => {
    const h = harness();
    assert.equal(h.c.socketClosed(), 250);
    assert.equal(h.c.socketClosed(), 500);
    assert.equal(h.c.socketClosed(), 1000);
    opened(h);
    assert.equal(h.c.socketClosed(), 250);
    assert.deepEqual(h.states, [true, false]);
});

test("calls made while down queue in order and flush behind the bus line", () => {
    const h = harness();
    const got = [];
    h.c.call("wm.act", { action: "a" }, (ok, r) => got.push(["a", ok, r]));
    h.c.run("reload", (ok, r) => got.push(["b", ok, r]));
    h.c.ipc("hub", "section", ["display", 3, true], (ok, r) => got.push(["c", ok, r]), "hub");
    h.c.ipcGet("shell", "barStyle", (ok, r) => got.push(["d", ok, r]));
    assert.equal(h.c.queued(), 4);
    h.c.subscribe("wm", () => {});
    h.c.serve("shell", {});
    h.c.socketOpened();
    const out = h.sent();
    assert.deepEqual(out.map((m) => m.bus ? "bus" : m.op), ["bus", "sub", "serve", "call", "run", "ipc", "ipc"]);
    assert.deepEqual(out[3], { op: "call", method: "wm.act", args: { action: "a" }, id: 1 });
    assert.deepEqual(out[4], { op: "run", line: "reload", id: 2 });
    assert.deepEqual(out[5], { op: "ipc", target: "hub", fn: "section", args: ["display", "3", "true"], client: "hub", id: 3 });
    assert.deepEqual(out[6], { op: "ipc", target: "shell", prop: "barStyle", id: 4 });
    assert.equal(h.c.queued(), 0);
    h.recv({ op: "reply", id: 3, ok: true, result: "" });
    h.recv({ op: "reply", id: 1, ok: true, result: { applied: true } });
    h.recv({ op: "reply", id: 2, ok: false, error: "reload busy" });
    h.recv({ op: "reply", id: 4, ok: true, result: "orpheus" });
    h.recv({ op: "reply", id: 4, ok: true, result: "twice" });
    assert.deepEqual(got, [["c", true, ""], ["a", true, { applied: true }], ["b", false, ""], ["d", true, "orpheus"]]);
});

test("run passes stdin and an error text reaches the callback", () => {
    const h = harness();
    opened(h);
    let seen = null;
    h.c.run("keyring-respond 7", (ok, out, err) => { seen = [ok, out, err]; }, "s3cret");
    assert.deepEqual(h.sent(), [{ op: "run", line: "keyring-respond 7", stdin: "s3cret", id: 1 }]);
    h.recv({ op: "reply", id: 1, ok: false, error: "no prompt" });
    assert.deepEqual(seen, [false, "", "no prompt"]);
});

test("the queue is bounded: the oldest request fails when it overflows", () => {
    const h = harness();
    const failed = [];
    for (let i = 0; i < bus.QUEUE_MAX + 2; i++)
        h.c.call("m", { i }, (ok, r, err) => failed.push([i, ok, err]));
    assert.equal(h.c.queued(), bus.QUEUE_MAX);
    assert.deepEqual(failed, [[0, false, "queue full"], [1, false, "queue full"]]);
    h.c.socketOpened();
    const calls = h.sent().filter((m) => m.op === "call");
    assert.equal(calls.length, bus.QUEUE_MAX);
    assert.equal(calls[0].args.i, 2);
    assert.equal(calls[calls.length - 1].args.i, bus.QUEUE_MAX + 1);
});

test("cancel drops a queued request without calling back, not a sent one", () => {
    const h = harness();
    let called = 0;
    const id = h.c.run("clip-copy x", () => called++);
    const keep = h.c.run("status", () => called++);
    assert.equal(h.c.cancel(id), true);
    assert.equal(h.c.cancel(id), false);
    h.c.socketOpened();
    const runs = h.sent().filter((m) => m.op === "run");
    assert.deepEqual(runs.map((m) => m.id), [keep]);
    assert.equal(h.c.cancel(keep), false);
    assert.equal(called, 0);
});

test("requests on the wire fail with disconnected when the socket drops", () => {
    const h = harness();
    opened(h);
    const got = [];
    h.c.call("m", {}, (ok, r, err) => got.push([ok, r, err]));
    h.c.run("x", (ok, r, err) => got.push([ok, r, err]));
    assert.equal(h.sent().length, 2);
    h.c.socketClosed();
    assert.deepEqual(got, [[false, "", "disconnected"], [false, "", "disconnected"]]);
    // nothing is resent on reconnect
    h.c.socketOpened();
    assert.deepEqual(h.sent().filter((m) => m.op === "call" || m.op === "run"), []);
});

test("subscribe sends sub once per topic and gets the cached frame at once", () => {
    const h = harness();
    opened(h);
    const a = [];
    const b = [];
    h.c.subscribe("network", (d, info) => a.push([d, info]));
    assert.deepEqual(h.sent(), [{ op: "sub", topics: ["network"] }]);
    h.recv({ op: "state", topic: "network", rev: 3, data: { up: true } });
    const t = h.c.subscribe("network", (d, info) => b.push([d, info]));
    assert.deepEqual(h.sent(), []);
    assert.deepEqual(a, [[{ up: true }, { rev: 3, paths: null }]]);
    assert.deepEqual(b, [[{ up: true }, { rev: 3, paths: null }]]);
    assert.equal(a[0][0], b[0][0], "one parsed frame is shared");
    h.c.unsubscribe(t);
    assert.deepEqual(h.sent(), []);
});

test("the last unsubscribe sends unsub and drops the cache", () => {
    const h = harness();
    opened(h);
    const t = h.c.subscribe("weather", () => {});
    h.recv({ op: "state", topic: "weather", rev: 1, data: { t: 20 } });
    h.sent();
    h.c.unsubscribe(t);
    h.c.unsubscribe(t);
    assert.deepEqual(h.sent(), [{ op: "unsub", topics: ["weather"] }]);
    // a late frame for a dropped topic goes nowhere; a new subscriber waits for a fresh one
    h.recv({ op: "state", topic: "weather", rev: 2, data: { t: 21 } });
    const seen = [];
    h.c.subscribe("weather", (d) => seen.push(d));
    assert.deepEqual(seen, []);
    assert.deepEqual(h.sent(), [{ op: "sub", topics: ["weather"] }]);
});

test("a delta at the client's rev patches the shared copy and names the paths", () => {
    const h = harness();
    opened(h);
    const seen = [];
    h.c.subscribe("settings", (d, info) => seen.push([d, info]));
    const base = { bar: { style: "orpheus" }, theme: { motion: { scale: 1 } } };
    h.recv({ op: "state", topic: "settings", rev: 5, data: base });
    h.recv({ op: "state", topic: "settings", rev: 6, base: 5, patch: [{ p: ["bar", "style"], v: "kairos" }, { p: ["language"], v: "fr" }] });
    assert.equal(seen.length, 2);
    const [next, info] = seen[1];
    assert.deepEqual(info, { rev: 6, paths: [["bar", "style"], ["language"]] });
    assert.equal(next.bar.style, "kairos");
    assert.equal(next.language, "fr");
    assert.equal(next.theme, seen[0][0].theme, "an untouched subtree keeps its identity");
    assert.equal(seen[0][0].bar.style, "orpheus", "the previous frame is not mutated");
});

test("a delta off base re-subs once and waits for the full frame", () => {
    const h = harness();
    opened(h);
    const seen = [];
    h.c.subscribe("settings", (d, info) => seen.push(info));
    h.sent();
    h.recv({ op: "state", topic: "settings", rev: 5, data: { a: 1 } });
    h.recv({ op: "state", topic: "settings", rev: 8, base: 7, patch: [{ p: ["a"], v: 2 }] });
    h.recv({ op: "state", topic: "settings", rev: 9, base: 8, patch: [{ p: ["a"], v: 3 }] });
    assert.deepEqual(h.sent(), [{ op: "sub", topics: ["settings"] }]);
    assert.equal(seen.length, 1);
    h.recv({ op: "state", topic: "settings", rev: 9, data: { a: 3 } });
    assert.deepEqual(seen[1], { rev: 9, paths: null });
    // in sync again: the next delta applies
    h.recv({ op: "state", topic: "settings", rev: 10, base: 9, patch: [{ p: ["a"], v: 4 }] });
    assert.deepEqual(seen[2], { rev: 10, paths: [["a"]] });
});

test("a delta before any full frame also resyncs", () => {
    const h = harness();
    opened(h);
    h.c.subscribe("settings", () => {});
    h.sent();
    h.recv({ op: "state", topic: "settings", rev: 2, base: 1, patch: [{ p: ["a"], v: 2 }] });
    assert.deepEqual(h.sent(), [{ op: "sub", topics: ["settings"] }]);
});

test("events reach every subscriber in order with seq and dropped, uncached", () => {
    const h = harness();
    opened(h);
    const seen = [];
    h.c.subscribe("keypress", (d, info) => seen.push([d.key, info]));
    h.recv({ op: "event", topic: "keypress", seq: 1, data: { key: "a" } });
    h.recv({ op: "event", topic: "keypress", seq: 5, dropped: 3, data: { key: "b" } });
    assert.deepEqual(seen, [
        ["a", { seq: 1, dropped: 0, event: true }],
        ["b", { seq: 5, dropped: 3, event: true }],
    ]);
    const late = [];
    h.c.subscribe("keypress", (d) => late.push(d));
    assert.deepEqual(late, []);
});

test("reconnect re-subs every live topic and re-serves every live target", () => {
    const h = harness();
    opened(h);
    h.c.subscribe("wm", () => {});
    h.c.subscribe("wm.focus", () => {});
    const handler = {};
    h.c.serve("shell", handler);
    h.c.serve("osd", {});
    h.c.unserve("osd", {});
    h.sent();
    h.c.socketClosed();
    h.c.socketOpened();
    assert.deepEqual(h.sent(), [
        { bus: { v: 1, client: "shell", pid: 42 } },
        { op: "sub", topics: ["wm", "wm.focus"] },
        { op: "serve", targets: ["shell", "osd"] },
    ]);
});

test("serve and unserve go out once per target, not per handler", () => {
    const h = harness();
    opened(h);
    const a = {};
    const b = {};
    h.c.serve("shell", a);
    h.c.serve("shell", b);
    h.c.serve("shell", b);
    assert.deepEqual(h.sent(), [{ op: "serve", targets: ["shell"] }]);
    h.c.unserve("shell", a);
    assert.deepEqual(h.sent(), []);
    h.c.unserve("shell", b);
    assert.deepEqual(h.sent(), [{ op: "unserve", targets: ["shell"] }]);
});

test("cmd runs the newest handler's function with converted args", () => {
    const h = harness();
    opened(h);
    const calls = [];
    const old = { openSurface: () => "old" };
    const handler = {
        openSurface(mon, id) { calls.push([mon, id]); return 7; },
        toggle(on) { calls.push([on]); },
        flag(v) { return v; },
        barStyle: "orpheus",
        _secret() { return "x"; },
        targetChanged() {},
    };
    h.c.serve("shell", old);
    h.c.serve("shell", handler);
    h.sent();
    h.recv({ op: "cmd", id: 1, target: "shell", fn: "openSurface", args: ["DP-1", "launcher"] });
    h.recv({ op: "cmd", id: 2, target: "shell", fn: "toggle", args: ["true"] });
    h.recv({ op: "cmd", id: 3, target: "shell", fn: "flag", args: ["false"] });
    h.recv({ op: "cmd", id: 4, target: "shell", fn: "flag", args: ["True"] });
    h.recv({ op: "cmd", id: 5, target: "shell", prop: "barStyle" });
    h.recv({ op: "cmd", id: 6, target: "nope", fn: "x", args: [] });
    h.recv({ op: "cmd", id: 7, target: "shell", fn: "missing", args: [] });
    h.recv({ op: "cmd", id: 8, target: "shell", fn: "_secret", args: [] });
    h.recv({ op: "cmd", id: 9, target: "shell", fn: "targetChanged", args: [] });
    h.recv({ op: "cmd", id: 10, target: "shell", fn: "toString", args: [] });
    h.recv({ op: "cmd", id: 11, target: "shell", prop: "nope" });
    h.recv({ op: "cmd", id: 12, target: "shell", prop: "flag" });
    assert.deepEqual(calls, [["DP-1", "launcher"], [true]]);
    assert.deepEqual(h.sent(), [
        { op: "done", id: 1, ok: true, result: "7" },
        { op: "done", id: 2, ok: true, result: "" },
        { op: "done", id: 3, ok: true, result: "false" },
        { op: "done", id: 4, ok: true, result: "True" },
        { op: "done", id: 5, ok: true, result: "orpheus" },
        { op: "done", id: 6, ok: false, error: "target not found" },
        { op: "done", id: 7, ok: false, error: "function not found" },
        { op: "done", id: 8, ok: false, error: "function not found" },
        { op: "done", id: 9, ok: false, error: "function not found" },
        { op: "done", id: 10, ok: false, error: "function not found" },
        { op: "done", id: 11, ok: false, error: "property not found" },
        { op: "done", id: 12, ok: false, error: "property not found" },
    ]);
});

test("a throwing handler answers with its error instead of hanging the caller", () => {
    const h = harness();
    opened(h);
    h.c.serve("shell", { boom() { throw new Error("bad monitor"); } });
    h.sent();
    h.recv({ op: "cmd", id: "x1", target: "shell", fn: "boom", args: [] });
    assert.deepEqual(h.sent(), [{ op: "done", id: "x1", ok: false, error: "Error: bad monitor" }]);
});

test("arg conversion touches only the exact boolean spellings", () => {
    assert.deepEqual(bus.cmdArgs(["true", "false", "TRUE", "1", "", "3.5"]), [true, false, "TRUE", "1", "", "3.5"]);
    assert.deepEqual(bus.cmdArgs(undefined), []);
    assert.equal(bus.resultText(undefined), "");
    assert.equal(bus.resultText(false), "false");
    assert.equal(bus.resultText(0), "0");
});

test("a subscriber that throws does not starve the others", () => {
    const h = harness();
    opened(h);
    const seen = [];
    h.c.subscribe("t", () => { throw new Error("x"); });
    h.c.subscribe("t", (d) => seen.push(d));
    h.recv({ op: "state", topic: "t", rev: 1, data: 1 });
    assert.deepEqual(seen, [1]);
    assert.equal(h.logs.length, 1);
});

test("a subscriber may unsubscribe another mid-delivery", () => {
    const h = harness();
    opened(h);
    const seen = [];
    let second = 0;
    h.c.subscribe("t", () => { seen.push("a"); h.c.unsubscribe(second); });
    second = h.c.subscribe("t", () => seen.push("b"));
    h.recv({ op: "state", topic: "t", rev: 1, data: 1 });
    assert.deepEqual(seen, ["a"]);
});

test("a cmd with the wrong argument count is refused the way qs ipc refuses it", () => {
    const h = harness();
    opened(h);
    const calls = [];
    h.c.serve("shell", { open(mon, id) { calls.push([mon, id]); } });
    h.sent();
    h.recv({ op: "cmd", id: 1, target: "shell", fn: "open", args: ["DP-1"] });
    h.recv({ op: "cmd", id: 2, target: "shell", fn: "open", args: ["DP-1", "a", "b"] });
    h.recv({ op: "cmd", id: 3, target: "shell", fn: "open" });
    assert.deepEqual(calls, []);
    assert.deepEqual(h.sent(), [
        { op: "done", id: 1, ok: false, error: "Too few arguments provided (2 required but 1 were provided.)" },
        { op: "done", id: 2, ok: false, error: "Too many arguments provided (2 required but 3 were provided.)" },
        { op: "done", id: 3, ok: false, error: "Too few arguments provided (2 required but 0 were provided.)" },
    ]);
});

test("serveAs moves a handler between targets and off the bus", () => {
    const h = harness();
    opened(h);
    const handler = {};
    h.c.serveAs(handler, "shell");
    h.c.serveAs(handler, "shell");
    assert.deepEqual(h.sent(), [{ op: "serve", targets: ["shell"] }]);
    h.c.serveAs(handler, "osd");
    assert.deepEqual(h.sent(), [{ op: "unserve", targets: ["shell"] }, { op: "serve", targets: ["osd"] }]);
    h.c.serveAs(handler, "");
    assert.deepEqual(h.sent(), [{ op: "unserve", targets: ["osd"] }]);
});

// The process side of the ryoku-shell daemon bus: one connection carrying every
// topic subscription, call, verb, cross-process IpcHandler call and served
// target. The daemon half and the wire protocol live in ryoku/shell/ipc/bus.go.
// This file is the state machine; Bus.qml owns the socket and the reconnect
// timer and feeds it. Plain module (no `.pragma library`) so node can test it.

var VERSION = 1;
var QUEUE_MAX = 256;
var BACKOFF_MIN = 250;
var BACKOFF_MAX = 5000;

function backoff(attempt) {
    return Math.min(BACKOFF_MAX, BACKOFF_MIN * Math.pow(2, Math.max(0, attempt)));
}

// The qs config this process is: RYOKU_BUS_CLIENT, else the shell directory's
// name. A dev checkout runs `qs -p ryoku/hub/quickshell`, so a directory
// called quickshell stands for its parent.
function clientName(env, shellDir) {
    if (env)
        return String(env);
    var parts = String(shellDir || "").split("/").filter(function (p) { return p.length > 0; });
    var base = parts.length ? parts[parts.length - 1] : "";
    if (base === "quickshell" && parts.length > 1)
        base = parts[parts.length - 2];
    return base || "shell";
}

// One wire line. JSON.stringify escapes newlines, so a value never splits it.
function encode(msg) {
    return JSON.stringify(msg) + "\n";
}

function decode(line) {
    try {
        var msg = JSON.parse(line);
        return msg !== null && typeof msg === "object" && !Array.isArray(msg) ? msg : null;
    } catch (e) {
        return null;
    }
}

// cmd arguments arrive as strings, as qs ipc passes them. Only the two boolean
// spellings are converted; a typed QML parameter coerces the rest.
function cmdArgs(args) {
    var out = [];
    for (var i = 0; args && i < args.length; i++) {
        var a = args[i];
        out.push(a === "true" ? true : a === "false" ? false : a);
    }
    return out;
}

// A served function's return value as qs ipc prints it.
function resultText(ret) {
    return ret === undefined ? "" : String(ret);
}

// What a cmd may reach on a handler: the functions its file declares, not the
// object machinery every QML object carries.
var DENIED = { destroy: true, toString: true, deleteLater: true, valueOf: true, constructor: true };

function callable(handler, name) {
    var n = String(name || "");
    return n.length > 0 && n[0] !== "_" && !DENIED[n] && !/Changed$/.test(n)
        && typeof handler[n] === "function";
}

function readable(handler, name) {
    var n = String(name || "");
    return n.length > 0 && n[0] !== "_" && n in handler && typeof handler[n] !== "function";
}

// qs ipc refuses a call whose argument count differs from the declaration;
// a typed QML function would otherwise see "undefined" for a missing string.
function arityError(want, got) {
    if (typeof want !== "number" || want === got)
        return "";
    return (got < want ? "Too few" : "Too many") + " arguments provided (" + want
        + " required but " + got + " were provided.)";
}

// opts: { name, pid, write(text), applyPatch(root, ops), onConnected(bool), log(text) }
function create(opts) {
    var write = opts.write;
    var log = opts.log || function () {};
    var applyPatch = opts.applyPatch;

    var open = false;        // socket up, bus line sent
    var helloed = false;     // daemon answered hello
    var attempt = 0;
    var nextId = 0;
    var nextToken = 0;

    // topic -> { subs: {token: fn}, count, has, data, rev, resync }
    var topics = {};
    var tokenTopic = {};
    // target -> [handler, ...], newest last
    var targets = {};
    // queued requests while the socket is down, oldest first
    var queue = [];
    // id -> { kind, cb } for requests on the wire
    var inflight = {};

    function setConnected(v) {
        if (helloed === v)
            return;
        helloed = v;
        if (opts.onConnected)
            opts.onConnected(v);
    }

    function notify(t, data, info) {
        var subs = t.subs;
        for (var k in subs) {
            // the fn may unsubscribe itself or others mid-loop
            if (!(k in subs))
                continue;
            try {
                subs[k](data, info);
            } catch (e) {
                log("bus: topic subscriber failed: " + e);
            }
        }
    }

    function finish(kind, cb, ok, result, error) {
        if (!cb)
            return;
        var r = result;
        if (kind !== "call" && (r === undefined || r === null))
            r = "";
        try {
            cb(ok, r, error || "");
        } catch (e) {
            log("bus: reply callback failed: " + e);
        }
    }

    function send(req) {
        inflight[req.msg.id] = { kind: req.kind, cb: req.cb };
        write(encode(req.msg));
    }

    function request(kind, msg, cb) {
        var id = ++nextId;
        msg.id = id;
        var req = { kind: kind, msg: msg, cb: cb };
        if (open) {
            send(req);
            return id;
        }
        if (queue.length >= QUEUE_MAX) {
            var dropped = queue.shift();
            finish(dropped.kind, dropped.cb, false, "", "queue full");
        }
        queue.push(req);
        return id;
    }

    function cancel(id) {
        for (var i = 0; i < queue.length; i++) {
            if (queue[i].msg.id === id) {
                queue.splice(i, 1);
                return true;
            }
        }
        return false;
    }

    function subscribe(topic, fn) {
        var name = String(topic);
        var t = topics[name];
        if (!t) {
            t = topics[name] = { subs: {}, count: 0, has: false, data: undefined, rev: 0, resync: false };
            if (open)
                write(encode({ op: "sub", topics: [name] }));
        }
        var token = ++nextToken;
        t.subs[token] = fn;
        t.count++;
        tokenTopic[token] = name;
        if (t.has) {
            try {
                fn(t.data, { rev: t.rev, paths: null });
            } catch (e) {
                log("bus: topic subscriber failed: " + e);
            }
        }
        return token;
    }

    function unsubscribe(token) {
        var name = tokenTopic[token];
        if (name === undefined)
            return;
        delete tokenTopic[token];
        var t = topics[name];
        if (!t || !(token in t.subs))
            return;
        delete t.subs[token];
        if (--t.count > 0)
            return;
        delete topics[name];
        if (open)
            write(encode({ op: "unsub", topics: [name] }));
    }

    function serve(target, handler) {
        var name = String(target);
        var list = targets[name];
        if (!list)
            list = targets[name] = [];
        if (list.indexOf(handler) >= 0)
            return;
        list.push(handler);
        if (list.length === 1 && open)
            write(encode({ op: "serve", targets: [name] }));
    }

    function unserve(target, handler) {
        var name = String(target);
        var list = targets[name];
        if (!list)
            return;
        var at = list.indexOf(handler);
        if (at < 0)
            return;
        list.splice(at, 1);
        if (list.length > 0)
            return;
        delete targets[name];
        if (open)
            write(encode({ op: "unserve", targets: [name] }));
    }

    // handler serves exactly target from now on ("" = nothing), so a handler
    // whose target or enabled flag moves needs no memory of its own.
    function serveAs(handler, target) {
        var want = String(target || "");
        var names = Object.keys(targets);
        for (var i = 0; i < names.length; i++)
            if (names[i] !== want && targets[names[i]].indexOf(handler) >= 0)
                unserve(names[i], handler);
        if (want !== "")
            serve(want, handler);
    }

    // The socket connected. Everything rides behind the bus line at once; the
    // daemon reads the connection in order, so nothing waits for hello.
    function socketOpened() {
        open = true;
        var text = "bus " + JSON.stringify({ v: VERSION, client: opts.name, pid: opts.pid }) + "\n";
        var live = Object.keys(topics);
        for (var i = 0; i < live.length; i++)
            topics[live[i]].resync = false;
        if (live.length)
            text += encode({ op: "sub", topics: live });
        var served = Object.keys(targets);
        if (served.length)
            text += encode({ op: "serve", targets: served });
        var pending = queue;
        queue = [];
        for (var j = 0; j < pending.length; j++) {
            inflight[pending[j].msg.id] = { kind: pending[j].kind, cb: pending[j].cb };
            text += encode(pending[j].msg);
        }
        write(text);
    }

    // The socket dropped or a connect attempt failed. Requests on the wire
    // fail (the daemon may or may not have run them, so they are not resent);
    // the return value is the delay before the next attempt.
    function socketClosed() {
        var wasOpen = open;
        open = false;
        setConnected(false);
        if (wasOpen) {
            var lost = inflight;
            inflight = {};
            for (var id in lost)
                finish(lost[id].kind, lost[id].cb, false, "", "disconnected");
        }
        return backoff(attempt++);
    }

    function onState(msg) {
        var t = topics[msg.topic];
        if (!t)
            return;
        if ("data" in msg) {
            t.data = msg.data;
            t.rev = msg.rev;
            t.has = true;
            t.resync = false;
            notify(t, t.data, { rev: t.rev, paths: null });
            return;
        }
        if (!Array.isArray(msg.patch))
            return;
        if (!t.has || t.rev !== msg.base) {
            if (!t.resync && open) {
                t.resync = true;
                write(encode({ op: "sub", topics: [msg.topic] }));
            }
            return;
        }
        t.data = applyPatch(t.data, msg.patch);
        t.rev = msg.rev;
        var paths = [];
        for (var i = 0; i < msg.patch.length; i++)
            paths.push(msg.patch[i].p);
        notify(t, t.data, { rev: t.rev, paths: paths });
    }

    function onCmd(msg) {
        var done = { op: "done", id: msg.id, ok: false };
        var list = targets[msg.target];
        var handler = list && list.length ? list[list.length - 1] : null;
        try {
            if (!handler) {
                done.error = "target not found";
            } else if (msg.prop !== undefined && msg.prop !== null && msg.prop !== "") {
                if (!readable(handler, msg.prop)) {
                    done.error = "property not found";
                } else {
                    done.ok = true;
                    done.result = resultText(handler[msg.prop]);
                }
            } else if (!callable(handler, msg.fn)) {
                done.error = "function not found";
            } else {
                var f = handler[msg.fn];
                var args = cmdArgs(msg.args);
                var bad = arityError(f.length, args.length);
                if (bad) {
                    done.error = bad;
                } else {
                    done.ok = true;
                    done.result = resultText(f.apply(handler, args));
                }
            }
        } catch (e) {
            done.ok = false;
            done.error = String(e);
        }
        if (open)
            write(encode(done));
    }

    function receive(line) {
        var msg = decode(line);
        if (!msg)
            return;
        switch (msg.op) {
        case "hello":
            attempt = 0;
            setConnected(true);
            break;
        case "state":
            onState(msg);
            break;
        case "event": {
            var t = topics[msg.topic];
            if (t)
                notify(t, msg.data, { seq: msg.seq, dropped: msg.dropped || 0, event: true });
            break;
        }
        case "reply": {
            var req = inflight[msg.id];
            if (!req)
                break;
            delete inflight[msg.id];
            finish(req.kind, req.cb, msg.ok === true, msg.result, msg.ok === true ? "" : String(msg.error || "failed"));
            break;
        }
        case "cmd":
            onCmd(msg);
            break;
        case "error":
            log("bus: " + (msg.topic ? msg.topic + ": " : "") + msg.error);
            break;
        }
    }

    return {
        subscribe: subscribe,
        unsubscribe: unsubscribe,
        call: function (method, args, cb) {
            return request("call", { op: "call", method: String(method), args: args === undefined ? {} : args }, cb);
        },
        run: function (line, cb, stdin) {
            var msg = { op: "run", line: String(line) };
            if (stdin !== undefined && stdin !== null)
                msg.stdin = String(stdin);
            return request("run", msg, cb);
        },
        ipc: function (target, fn, args, cb, client) {
            var list = [];
            for (var i = 0; args && i < args.length; i++)
                list.push(String(args[i]));
            var msg = { op: "ipc", target: String(target), fn: String(fn), args: list };
            if (client)
                msg.client = String(client);
            return request("ipc", msg, cb);
        },
        ipcGet: function (target, prop, cb, client) {
            var msg = { op: "ipc", target: String(target), prop: String(prop) };
            if (client)
                msg.client = String(client);
            return request("ipc", msg, cb);
        },
        cancel: cancel,
        serve: serve,
        unserve: unserve,
        serveAs: serveAs,
        socketOpened: socketOpened,
        socketClosed: socketClosed,
        receive: receive,
        isOpen: function () { return open; },
        queued: function () { return queue.length; }
    };
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { create, backoff, clientName, encode, decode, cmdArgs, resultText, callable, readable, QUEUE_MAX };

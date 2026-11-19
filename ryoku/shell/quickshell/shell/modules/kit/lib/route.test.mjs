import { createRequire } from "node:module";
import { spawn, spawnSync } from "node:child_process";
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync, existsSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const R = require("./route.js");

function equal(actual, expected, label) {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) {
        console.error(label, "\nexpected", expected, "\nactual", actual);
        process.exitCode = 1;
    }
}

equal(R.parse("cheatsheet"), { kit: true, toggle: true, section: "", page: "" }, "Super+K is the bare id: a toggle");
equal(R.parse("cheatsheet#phone"), { kit: true, toggle: false, section: "phone", page: "" }, "a section route");
equal(R.parse("cheatsheet#phone/screen"), { kit: true, toggle: false, section: "phone", page: "screen" }, "a section and a page");
equal(R.parse("cheatsheet#commands/group:saved:my-commands").page, "group:saved:my-commands", "a page id keeps its colons");
equal(R.parse("cheatsheet#commands/a/b").page, "a/b", "and anything after the first slash");
equal(R.parse("cheatsheet#downloads").section, "downloads", "Downloads is a section now");
equal(R.parse("cheatsheet#bogus/x"), { kit: true, toggle: false, section: "", page: "" }, "an unknown section opens Kit where it was, never closes it");
equal(R.parse("cheatsheet#"), { kit: true, toggle: false, section: "", page: "" }, "an empty route opens too");
equal(R.parse("ask#downloads"), { kit: true, toggle: false, section: "downloads", page: "queue" }, "the daemon's Downloads route belongs to Kit");
equal(R.parse("cheatsheets").kit, false, "nor a longer name");
equal(R.parse("").kit, false, "nor nothing");

for (const section of R.SECTIONS)
    equal(R.parse("cheatsheet#" + section + "/p:q/r"), { kit: true, toggle: false, section, page: "p:q/r" }, "each section keeps its whole page id: " + section);

const here = dirname(fileURLToPath(import.meta.url));
const script = resolve(here, "../../../../../scripts/ryoku-kit");
const delay = ms => new Promise(done => setTimeout(done, ms));
async function until(check, label) {
    const deadline = Date.now() + 10000;
    while (!check()) {
        if (Date.now() >= deadline)
            throw new Error("Timed out: " + label);
        await delay(20);
    }
}

function launcherFixture(running = false) {
    const box = mkdtempSync(join(tmpdir(), "kit-route-"));
    const log = join(box, "calls");
    const runtime = join(box, "runtime");
    const config = join(box, "config");
    const entry = join(config, "quickshell/shell/kit.qml");
    mkdirSync(runtime);
    mkdirSync(dirname(entry), { recursive: true });
    writeFileSync(entry, "");
    writeFileSync(log, "");
    const stub = join(box, "qs");
    writeFileSync(stub, "#!" + process.execPath + "\n" + String.raw`
const fs = require("node:fs");
const path = require("node:path");
const box = process.env.KIT_TEST_BOX;
const file = name => path.join(box, name);
const log = event => fs.appendFileSync(file("calls"), JSON.stringify(event) + "\n");
const delay = ms => new Promise(done => setTimeout(done, ms));
const [flag, entry, ...command] = process.argv.slice(2);
async function locked(fn) {
    while (true) {
        try { fs.mkdirSync(file("state-lock")); break; }
        catch (error) { if (error.code !== "EEXIST") throw error; await delay(5); }
    }
    try { return fn(); }
    finally { fs.rmdirSync(file("state-lock")); }
}
function apply(state, request) {
    state.open = request.action === "toggle" ? !state.open : true;
    state.request = request;
    fs.writeFileSync(file("state"), JSON.stringify(state));
}
(async () => {
    if (flag !== "-p") process.exit(2);
    if (command.length) {
        log({ type: "attempt", entry, command });
        if (!fs.existsSync(file("ready"))) process.exit(1);
        const accepted = await locked(() => {
            if (!fs.existsSync(file("ready"))) return false;
            const [ipc, call, target, action, ...args] = command;
            if (ipc !== "ipc" || call !== "call" || target !== "kit") return false;
            if (action !== "open" && action !== "toggle") return false;
            const request = action === "toggle"
                ? { action, monitor: args[0] }
                : { action, section: args[0], page: args[1], args: args[2], monitor: args[3] };
            apply(JSON.parse(fs.readFileSync(file("state"), "utf8")), request);
            log({ type: "ipc", entry, request });
            return true;
        });
        process.exit(accepted ? 0 : 1);
    }
    fs.writeFileSync(file("pid"), String(process.pid));
    log({ type: "start", entry });
    fs.writeFileSync(file("started"), "");
    await delay(350);
    const request = {
        action: process.env.RYOKU_KIT_ACTION,
        section: process.env.RYOKU_KIT_SECTION,
        page: process.env.RYOKU_KIT_PAGE,
        args: process.env.RYOKU_KIT_ARGS,
        monitor: process.env.RYOKU_KIT_MONITOR
    };
    await locked(() => {
        apply({ open: false }, request);
        log({ type: "initial", entry, request });
        fs.writeFileSync(file("ready"), "");
    });
    while (!fs.existsSync(file("stop"))) await delay(20);
    fs.unlinkSync(file("ready"));
    if (fs.existsSync(file("quit-delay"))) {
        fs.writeFileSync(file("quitting"), "");
        await delay(500);
    }
    log({ type: "exit" });
    fs.writeFileSync(file("exited"), "");
})().catch(error => { console.error(error); process.exit(1); });
`);
    chmodSync(stub, 0o755);
    if (running) {
        writeFileSync(join(box, "state"), JSON.stringify({ open: false }));
        writeFileSync(join(box, "ready"), "");
    }
    const env = {
        ...process.env,
        PATH: box + ":" + process.env.PATH,
        XDG_CONFIG_HOME: config,
        XDG_RUNTIME_DIR: runtime,
        KIT_TEST_BOX: box
    };
    const events = () => readFileSync(log, "utf8").split("\n").filter(Boolean).map(line => JSON.parse(line));
    return {
        box, entry, events,
        state: () => JSON.parse(readFileSync(join(box, "state"), "utf8")),
        run: args => {
            const out = spawnSync("bash", [script, ...args], { env, encoding: "utf8", timeout: 15000 });
            if (out.error) throw out.error;
            return out.status;
        },
        concurrent: args => new Promise((done, reject) => {
            const child = spawn("bash", [script, ...args], { env, stdio: "ignore", timeout: 15000 });
            child.once("error", reject);
            child.once("close", (code, signal) => signal ? reject(new Error(signal)) : done(code));
        }),
        resetLog: () => writeFileSync(log, ""),
        async quitDuringRequest() {
            writeFileSync(join(box, "quit-delay"), "");
            writeFileSync(join(box, "stop"), "");
            await until(() => existsSync(join(box, "quitting")), "IPC disappears before the lifetime lock");
            rmSync(join(box, "stop"));
            rmSync(join(box, "quit-delay"));
        },
        started: () => existsSync(join(box, "started")),
        async stop() {
            writeFileSync(join(box, "stop"), "");
            if (!existsSync(join(box, "started"))) return;
            await until(() => existsSync(join(box, "exited")), "Kit exits");
            const lock = join(runtime, "ryoku", readdirSync(join(runtime, "ryoku")).find(name => name.endsWith(".lock")));
            await until(() => spawnSync("flock", ["-n", lock, "true"]).status === 0, "no launcher holds the lock after Kit exits");
        },
        async dispose() {
            try { await this.stop(); }
            finally {
                if (existsSync(join(box, "pid")) && !existsSync(join(box, "exited"))) {
                    try { process.kill(Number(readFileSync(join(box, "pid"), "utf8")), "SIGTERM"); }
                    catch (error) { if (error.code !== "ESRCH") throw error; }
                }
                rmSync(box, { recursive: true, force: true });
            }
        }
    };
}

if (!existsSync(script)) {
    console.error("ryoku-kit is missing at", script);
    process.exitCode = 1;
} else {
    const fixtures = [];
    try {
        const warm = launcherFixture(true);
        fixtures.push(warm);
        function checkWarm(args, command, label) {
            warm.resetLog();
            equal(warm.run(args), 0, label + " succeeds");
            equal(warm.events(), [
                { type: "attempt", entry: warm.entry, command },
                { type: "ipc", entry: warm.entry, request: command[3] === "toggle"
                    ? { action: "toggle", monitor: command[4] }
                    : { action: "open", section: command[4], page: command[5], args: command[6], monitor: command[7] } }
            ], label);
        }
        checkWarm([], ["ipc", "call", "kit", "open", "", "", "{}", ""], "bare launcher restores the remembered section on focus");
        for (const section of ["downloads", "keys", "commands", "phone", "devtools", "typing"])
            checkWarm([section], ["ipc", "call", "kit", "open", section, "", "{}", ""], section + " routes directly to Kit");
        const payload = JSON.stringify({ urls: ["https://example.test/a?b=c&d=e"], text: "two words\nnext", intake: true });
        checkWarm(["--monitor", "DP-2", "--args", payload, "downloads", "queue"],
            ["ipc", "call", "kit", "open", "downloads", "queue", payload, "DP-2"], "arguments and monitor preserve their bytes");
        checkWarm(["commands", "group:saved:my-commands"], ["ipc", "call", "kit", "open", "commands", "group:saved:my-commands", "{}", ""], "page ids keep colons");
        checkWarm(["", "screen"], ["ipc", "call", "kit", "open", "", "screen", "{}", ""], "an explicitly empty section preserves remembered selection");
        checkWarm(["--toggle", "--monitor", "HDMI-A-1"], ["ipc", "call", "kit", "toggle", "HDMI-A-1"], "toggle has only its monitor argument");
        warm.resetLog();
        const override = join(warm.box, "entry with spaces.qml");
        equal(warm.run(["--qml", override, "phone", "screen"]), 0, "explicit QML entry succeeds");
        equal(warm.events()[0].entry, override, "explicit QML entry overrides XDG config");
        for (const args of [["bogus"], ["keys", "a", "b"], ["--monitor"], ["--args"], ["--qml"], ["--close"]]) {
            warm.resetLog();
            equal(warm.run(args), 2, "invalid CLI rejected: " + JSON.stringify(args));
            equal(warm.events(), [], "invalid CLI never calls Quickshell");
        }

        const cold = launcherFixture();
        fixtures.push(cold);
        equal(cold.run(["--monitor", "DP-1", "--args", payload, "downloads", "queue"]), 0, "cold launcher returns with detached Kit alive");
        await until(() => cold.events().some(event => event.type === "initial"), "initial environment request");
        equal(cold.events().filter(event => event.type === "start").length, 1, "cold start launches one process");
        equal(cold.events().filter(event => event.type === "initial").map(event => event.request), [{
            action: "open", section: "downloads", page: "queue", args: payload, monitor: "DP-1"
        }], "cold request is delivered by environment with all arguments");
        equal(cold.events().filter(event => event.type === "ipc"), [], "the cold starter never replays its request over IPC");
        await cold.stop();

        const toggles = launcherFixture();
        fixtures.push(toggles);
        equal(toggles.run(["--toggle"]), 0, "first cold toggle starts Kit");
        await until(toggles.started, "cold toggle process starts");
        equal(await toggles.concurrent(["--toggle"]), 0, "racing second toggle waits for IPC");
        equal(toggles.events().filter(event => event.type === "start").length, 1, "double cold toggle starts only one process");
        equal(toggles.events().filter(event => event.type === "initial").length, 1, "first toggle is applied once");
        equal(toggles.events().filter(event => event.type === "ipc").map(event => event.request.action), ["toggle"], "only the second toggle is sent over IPC");
        equal(toggles.state().open, false, "double cold toggle closes Kit");
        await toggles.stop();

        const racing = launcherFixture();
        fixtures.push(racing);
        equal(racing.run(["keys"]), 0, "first racing request starts Kit");
        await until(racing.started, "racing process starts");
        equal(await Promise.all(["phone", "devtools", "typing"].map(section => racing.concurrent([section]))), [0, 0, 0], "all racing startup requests succeed");
        equal(racing.events().filter(event => event.type === "start").length, 1, "racing requests do not start duplicate processes");
        equal(racing.events().filter(event => event.type === "initial").map(event => event.request.section), ["keys"], "the first request remains environment-only");
        equal(racing.events().filter(event => event.type === "ipc").map(event => event.request.section).sort(), ["devtools", "phone", "typing"], "each racing request is delivered exactly once");
        await racing.stop();

        const quitting = launcherFixture();
        fixtures.push(quitting);
        equal(quitting.run(["keys"]), 0, "quitting fixture starts");
        await until(() => quitting.events().some(event => event.type === "initial"), "quitting fixture is ready");
        await quitting.quitDuringRequest();
        equal(await quitting.concurrent(["--toggle"]), 0, "request during shutdown cold-starts after the lock frees");
        await until(() => quitting.events().filter(event => event.type === "initial").length === 2, "replacement handles its initial request");
        equal(quitting.events().filter(event => event.type === "start").length, 2, "one replacement starts");
        equal(quitting.events().filter(event => event.type === "ipc"), [], "replacement never replays the cold request");
        equal(quitting.events().filter(event => event.type === "initial").map(event => event.request.action), ["open", "toggle"], "the quitting request is delivered once");
        equal(quitting.state().open, true, "shutdown toggle opens the replacement");
        await quitting.stop();
    } finally {
        for (const fixture of fixtures)
            await fixture.dispose();
    }
}

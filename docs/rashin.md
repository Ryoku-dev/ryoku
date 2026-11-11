# Ryoku Rashin

Rashin (羅針, "compass needle") is Ryoku's optional AI system. It is one Go
daemon that serves four things:

- **Ask**: questions about this machine, or plain chat, answered by free models
  through Rashin's built-in model router. Ask reads the machine and, unless
  told otherwise, never changes it: it proposes commands for you to run, or
  hands the work to an Agent. A conversation in the app can be told it may
  act, asking first or not.
- **Agent**: an external coding harness you already use (Hermes, Claude Code,
  Codex, OpenCode, Gemini CLI, omp and more) driven over the Agent Client
  Protocol, with Rashin owning the sessions, files, terminals and approvals.
- **Index**: project and system indexing with semantic search (the potion code
  embedding model), also served as the `rashin` command line.
- **Teams**: standing groups of agents that take tasks on their own, on a
  schedule or when asked, each agent in a sandbox, with a credential vault
  whose values they never see.

The machine map every agent reads is the **Ryoku vault**, owned by Ryoku core
and independent of Rashin. `ryoku vault refresh` maintains it even when Rashin
is off or absent: see [vault.md](vault.md).

This page is the contract the implementation follows. The wire reference is
`ryoku/rashin/internal/protocol/README.md`.

## The rules

1. **Opt in, zero at rest.** Rashin draws nothing until you turn it on: no
   process, no socket, no timer, no watcher, no QML object, no probe at login.
   Turning it off returns to exactly that state.
2. **The daemon owns the work.** Agent processes, terminals, files, sessions
   and model calls belong to `rashind`. Clients only render and send input, so
   quitting the app never loses a session.
3. **Idle means gone.** An idle daemon exits; an idle agent process stops.
   Sessions live on disk and resume on the next prompt. Two exceptions are
   asked for by name: a daemon that serves the local API stays up, because
   other programs hold its address, and so does one with a team routine on,
   because routines run from its clock.
4. **Ask reads unless told otherwise.** Ask starts read-only: anything that
   changes the machine is a proposed command the user runs, or a handoff to
   an Agent. A conversation changes the machine only after the user set that
   conversation to, and an ask from the bar or a terminal never does.
5. **One owner per fact.** The daemon is the only writer of Rashin state. The
   vault is Ryoku's. Harnesses keep their own model choices.
6. **Nobody starts as a stranger.** Every request is told about Ryoku, the
   Ryoku vault and Rashin (`internal/brief`): Ask in its prompt, an agent
   ahead of its first prompt.

## Pieces

| Piece | Binary or file | Stack | Job |
|---|---|---|---|
| Daemon | `rashind` | Go, no cgo | Gateway, Ask, Agent adapters, sessions, index, teams |
| Command line | `rashin` | Go, no cgo | Lifecycle, ask from a terminal, index queries, any method |
| App | `rashin-app` | Go + Gio, Lithic | Ask, Agent, Index, Teams, Settings |
| Bar | Alt+Space | Quickshell QML | Quick Ask and Agent turns, Stash tools, Web search |
| Hub page | Ryoku Settings > Rashin | Quickshell QML | On/off switch, status, open the app |
| Usage collectors | `ryoku-ai-usage` and friends | sh | Coding-agent usage for the bar's usage pill |

Source layout, one module (`ryoku/rashin/go.mod`, module `ryoku-rashin`):

```
ryoku/rashin/
  cmd/rashind/          daemon entry
  cmd/rashin/           command line (one file per command group)
  cmd/rashin-app/       Gio app entry
  internal/protocol/    wire types, method and event registry
  internal/paths/       every path Rashin reads or writes
  internal/config/      rashin.json: schema, defaults, v1 migration
  internal/llm/         chat-completion types shared by gateway and Ask
  internal/core/        interfaces between the daemon and its subsystems
  internal/client/      socket client, daemon start, used by CLI and app
  internal/daemon/      server, dispatch, lifecycle, session manager
  internal/session/     event log store
  internal/gateway/     built-in free-model router
  internal/ask/         native Ask runner, tools, command safety
  internal/attachment/  what an attached file is, read for a model
  internal/brief/       what every request is told about Ryoku, the vault and Rashin
  internal/harness/     ACP adapters for external harnesses
  internal/index/       index engine, embedding model, queries
  internal/teams/       Teams: teams, tasks, approvals, routines, budgets, ledger
  internal/vault/       the credential vault: sealed values, mail account, record of use
  internal/sandbox/     the bubblewrap sandbox team agents run in
  internal/app/         Gio client (shell, state, pages, chat widgets, desktop, demo)
  systemd/              rashind.socket, rashind.service, rashind-routines.timer and .service
  usage/                coding-agent usage collectors
```

The bar lives with the shell (`ryoku/shell/quickshell/shell/modules/ask/`), the
Hub page with the Hub (`ryoku/hub/quickshell/pages/RashinPage.qml`).

## Lifecycle

### States

| State | What exists | Alt+Space | Super+Alt+Space |
|---|---|---|---|
| Not installed | nothing | unbound | unbound |
| Off | package files only | bar opens and says Rashin is off | app opens and says Rashin is off |
| On, idle | socket unit (systemd) | bar | app |
| On, working | `rashind` plus any agent process | bar | app |

The Hub switch owns the package: **on** installs `ryoku-rashin` when absent,
then runs `rashin on`; **off** runs `rashin off`, then removes the package. The
installer's Rashin question is opt in (default no); yes installs the package and
turns Rashin on at first login.

### Starting the daemon

- **systemd** (Arch, CachyOS, Fedora): `rashind.socket` listens on
  `%t/rashin/rashin.sock` (`SocketMode=0600`, `DirectoryMode=0700`) and
  activates `rashind.service`. The service has no `[Install]` section.
  `rashind-routines.timer` (`OnStartupSec=1min`, `OnUnitActiveSec=30min`)
  runs `rashin routines wake`, which starts the daemon only when Rashin is
  on and a team routine is on; the daemon enables the timer when the first
  routine turns on and disables it when the last turns off.
- **runit and anything else** (Void): no daemon service; the user service
  `rashind-routines` ships with a `down` file and runs `rashin routines wake`
  every 30 minutes while a routine is on, the daemon enabling and disabling it
  like the timer. A client that finds no socket while Rashin is on starts
  `rashind` itself, detached (`setsid`, its own process group, stdio to the
  daemon log), then waits up to 3 s for the
  socket. `internal/client` implements this once for every Go client; the bar
  calls `rashin ensure` when it opens, and the Hub only reads
  `rashin status --json`, so opening the Hub never starts the daemon.
- The daemon takes `$XDG_RUNTIME_DIR/rashin/rashind.lock` (flock) so two
  starts cannot race. With `LISTEN_FDS` it serves the inherited socket;
  otherwise it binds the path itself (removing a stale socket only while it
  holds the lock).
- A daemon started while Rashin is off exits 0 at once.

### Stopping

- **Idle exit:** the daemon exits after `daemon.idleExitMinutes` (default 5)
  when no client is connected, no turn is running, no agent process is alive,
  no index job is running and no team task is working. The check is event
  driven (re-armed on state changes), never a poll. A daemon that serves the
  local API (see Gateway), or one with a routine on in a team that is not
  paused (see Teams), drops its loaded indexes at that point and stays up.
  With no routine on and no window open it exits as before, so Teams costs
  nothing while unused.
- **Agent idle stop:** an agent process stops after `agent.idleStopMinutes`
  (default 10) without a turn, even while the app shows it. The next prompt
  starts it again and resumes the session (`session/load` when the harness
  supports it).
- **Making way for an update** (`internal/daemon/replaced.go`): an update
  installs a new `rashind` while the old one runs on for as long as an app is
  connected, and that app would then talk to code older than itself. The
  daemon watches the file it was started from (inotify on its folder, no
  poll). Once another program has stood at that path for two seconds and
  nothing is running (no turn, no agent process, no index job), it exits; the
  next connection starts the new one, and clients reconnect by themselves. A
  path that is gone or holds no program is an uninstall and changes nothing.
- **Shutdown** (`shutdown` method, `rashin off`, SIGTERM): cancel running turns
  (each ends with `turn_end` stop `cancelled`), stop agent processes, flush
  logs, close stores, remove the socket when the daemon bound it, exit 0.

### Commands

| Command | Does |
|---|---|
| `rashin on` | writes `enabled: true`; systemd: enables and starts `rashind.socket` and `ryoku-ai-usage.timer` through `ryoku-host svc --user enable --now`; runit: enables the `ryoku-ai-usage` service |
| `rashin off` | writes `enabled: false`; asks a running daemon to `shutdown`; disables and stops the socket, service, usage timer and routines wake timer (runit: stops the usage and routines wake services and kills a daemon left running through the lock holder's pid) |
| `rashin routines wake` | exit 0 at once when Rashin is off or no routine is on in an unpaused team (read from the teams' `state.json`, no daemon); else starts the daemon through its socket |
| `rashin open --route <route>` | opens a Teams mode place (see Teams, In the app) in a running app through `ui.open` and raises it, or starts the app there |
| `rashin status [--json]` | enabled, running, version, counts; never starts the daemon |
| `rashin ensure` | exit 0 once the daemon answers `hello`; exit 3 when off; exit 1 on failure |
| `rashin migrate` | idempotent upgrade from Rashin 1 and Prowl (below) |

`ryoku doctor` converges the units to `enabled`: on means the socket and timer
are enabled, off means both disabled, the routines wake timer disabled and no
daemon running. The routines wake timer follows the daemon while Rashin is
on. With Rashin not installed it stops a daemon left running, stops and
disables the socket, service, usage timer and routines wake timer (runit: the
usage and routines wake services) and removes their leftover unit links. It
also runs `rashin migrate` and removes the Rashin 1 and Prowl units.

### Zero at rest, checked

With Rashin off or absent, all of these hold, and the live proof checks each:

- `systemctl --user is-enabled rashind.socket ryoku-ai-usage.timer rashind-routines.timer` reports
  disabled or not found; no `rashind`, `rashin-app` or agent process exists;
  `$XDG_RUNTIME_DIR/rashin/rashin.sock` does not exist.
- The shell starts no Rashin process at login: no singleton probes Rashin; the
  bar's QML is created only when Alt+Space opens it, and the Off state needs no
  process to render.
- No file watcher, timer or socket belongs to Rashin.

## Paths

Every path comes from `internal/paths`; nothing else spells one.

| Path | Holds |
|---|---|
| `$XDG_CONFIG_HOME/ryoku/rashin.json` | settings (below) |
| `$XDG_STATE_HOME/ryoku/rashin/sessions/<id>/` | `meta.json`, `events.jsonl` |
| `$XDG_STATE_HOME/ryoku/rashin/gateway.db` | provider keys, the local API key, usage, cooldowns (bbolt) |
| `$XDG_STATE_HOME/ryoku/rashin/master.key` | 32 random bytes, mode 0600 |
| `$XDG_STATE_HOME/ryoku/rashin/indexes.json` | index registry |
| `$XDG_STATE_HOME/ryoku/rashin/harness-capabilities.json` | what each harness binary answered at initialize, keyed by path and mtime |
| `$XDG_STATE_HOME/ryoku/rashin/rashind.log` | warnings and errors, rotated at 1 MiB |
| `$XDG_STATE_HOME/ryoku/rashin/projects.json` | projects, their settings, Rashin-made worktrees, tasks, review comments |
| `$XDG_STATE_HOME/ryoku/rashin/activity/<project>.jsonl` | a project's action ledger |
| `$XDG_STATE_HOME/ryoku/rashin/teams/` | Teams: `state.json` (teams, routines, approvals), `tasks/<id>.json`, `ledger.jsonl` (Activity), `spend.json` |
| `$XDG_STATE_HOME/ryoku/rashin/vault.db` | the credential vault: sealed values, the mail account, its record of use (bbolt, mode 0600) |
| `~/Teams/<slug>` | a team's folder, where its agents work (default) |
| `$XDG_DATA_HOME/ryoku/worktrees/<project>/<name>` | worktrees Rashin creates (default) |
| `$XDG_CACHE_HOME/ryoku/rashin/index/<id>/index.db` | one index (bbolt) |
| `$XDG_RUNTIME_DIR/rashin/rashin.sock` | the socket |
| `$XDG_RUNTIME_DIR/rashin/rashind.lock` | daemon lock |
| `$XDG_RUNTIME_DIR/rashin/config.lock` | serialises writes of `rashin.json` |
| `$XDG_RUNTIME_DIR/rashin/app.lock` | app single instance |
| `$XDG_RUNTIME_DIR/rashin/agents/<session>/rashin.sock` | a team task's own socket, the only way its sandboxed agent reaches the daemon |

The directories are created 0700. Tests and sandboxes move all of them with the
XDG variables; `RASHIN_SOCKET` overrides the socket for a client.

## Settings

`rashin.json` (version 2). Unknown keys are kept on write.

```json
{
  "version": 2,
  "enabled": true,
  "ask":    {"model": "auto", "habits": true, "approvals": "read-only"},
  "agent":  {"harness": "", "approvals": "read-only", "idleStopMinutes": 10},
  "daemon": {"idleExitMinutes": 5},
  "index":  {"system": true, "semantic": true},
  "api":    {"enabled": false, "port": 3600},
  "teams":  {
    "maxRunning": 3, "perTeam": 2, "routinesWake": true,
    "defaults": {
      "harness": "", "model": "", "approvals": "ask", "notify": "all",
      "budget": {"monthTokens": 2000000, "warnPercent": 80, "taskTurns": 20, "taskMinutes": 60}
    },
    "approvalHours": 24, "hidden": null, "keepDays": 0
  }
}
```

- `ask.model`: `auto`, `auto:fast`, `auto:smart`, or `<provider>/<model>`.
- `ask.habits`: include a short summary of the user's shell habits (tools,
  frequent commands, secrets filtered) in Ask prompts.
- `ask.approvals`: what a new ask in the app starts with: `read-only`
  (proposes, changes nothing), `ask` (may change the machine, each change
  waits for the user) or `auto` (changes it without asking; a destructive
  command still waits). The daemon does not apply it by itself: the app
  passes it when it creates a session, so an ask from the bar or a terminal
  stays read-only.
- `agent.harness`: default harness id for new Agent sessions; empty picks the
  first installed one in the table order below.
- `agent.approvals`: `read-only` (reads run, writes ask), `ask` (everything
  asks), `auto` (nothing asks).
- `agent.instances`: custom harnesses, each an id, a name, an ACP command and
  extra environment variables (a second account, another config folder).
- `index.system`: maintain the system index; `index.semantic`: compute vectors.
- `api.enabled`: serve the local API (see Gateway); `api.port`: where, 1024 to
  65535.
- `teams.maxRunning`: how many team tasks run at once across all teams, 1 to
  6. `teams.perTeam`: how many of one team's, 1 to 4 and never above
  `maxRunning`, unless the team sets its own.
- `teams.paused`: holds every team: nothing starts, no routine fires and
  working tasks stop. Each team's own pause stays underneath.
- `teams.routinesWake`: lets the routines wake unit start Rashin while a
  routine is on. Off, routines run only while Rashin is up.
- `teams.defaults`: what a new team starts with: permissions, budget
  (`taskMinutes` 0 is no time limit) and notify level. `harness`, `model` and
  `options` are the engine of every member left on the teams default, in every
  team. Changing a default later changes no existing team otherwise.
- `teams.approvalHours`: how long a question waits before it counts as no, 1
  to 168. `teams.quietHours` (`{"from": "22:00", "to": "07:30"}`, local time,
  may cross midnight): no desktop notification inside the window.
- `teams.hidden`: paths every team agent sees as empty. `null` is the built-in
  list (`~/.ssh`, `~/.gnupg`, `~/.password-store`, cloud and cluster
  credentials, browser profiles, `~/.local/share/keyrings`); `[]` hides
  nothing beyond what Rashin always hides. At most 64.
- `teams.keepDays`: 0 keeps finished tasks forever; 30, 90 or 365 deletes older
  ones with their conversations.

Clients change settings only through `settings.set`; the daemon writes the file
atomically and broadcasts `settings.changed`. A patch that touches `teams` or
`agent.instances` comes only from the Rashin app.

## Protocol

Unix stream socket, newline-delimited compact JSON, one object per line, lines
up to 16 MiB.

```
request   {"id": 7, "method": "session.prompt", "params": {...}}
response  {"id": 7, "result": {...}}  or  {"id": 7, "error": {"code": "...", "message": "..."}}
event     {"event": "session.event", "data": {...}}
```

The first request on every connection is `hello` (`client`: `app`, `bar`,
`cli`, `hub`). Error codes: `bad_request`, `not_found`, `unavailable`,
`conflict`, `disabled`, `unsupported`, `forbidden`, `internal`.
`internal/protocol` holds
every method with its params and result types and every event; `rashin call
<method> [json]` and `rashin events` reach any of them.

Method groups: `hello status subscribe shutdown`, `session.*` (list, new,
attach, detach, prompt, cancel, permission, set_model, set_mode, set_option,
start, rename, delete, stop, handoff, queue, unqueue, steer, fork, read,
revert_turn), `ask`
(new session, attach, prompt in one call), `providers.*`, `models.list`,
`gateway.usage|api|api_key`, `harnesses.*`, `index.*`, `settings.get|set`,
`ui.open`,
Work: `projects.*`, `worktrees.*`, `git.*`, `review.*`, `tasks.*`,
`activity.list`, and Teams: `teams.*` and `vault.*` (see Teams; most of them
answer only the Rashin app).

`subscribe` returns a snapshot (status, recent sessions, settings, providers,
harnesses, indexes, projects, tasks, teams with their open and recent tasks,
approvals, routines and the vault without any value) and from then on the
connection receives the global events: `sessions.changed`, `session.info`,
`settings.changed`, `providers.changed`, `api.changed`, `harnesses.changed`,
`index.progress`, `index.changed`,
`projects.changed`, `worktrees.changed`, `tasks.changed`, `review.changed`,
`team`, `team.removed`, `team.task`, `team.task.removed`, `team.approval`,
`routine`, `routine.removed`, `vault`,
`ui.focus` (app clients only), `notice`. `session.attach` adds `session.event`
for that session; `projects.watch` adds `git.changed` for a project.

## Sessions

A session is either `ask` or `agent`. Its log is `events.jsonl`: one
`SessionEvent{seq, ts, kind, data}` per line, `seq` strictly increasing from 1.
`meta.json` holds `SessionInfo` plus private runner state (the harness's own
session id for resume). Both are written by the daemon only.

### Event kinds

| Kind | Data |
|---|---|
| `user` | text, attached files (`images`: paths, of any kind), terminal context, handoff; or `run`, the command the user ran in place of a question |
| `text` | message id, delta: visible answer text |
| `thought` | message id, delta: reasoning text |
| `tool` | call id, title, kind, status, input, output, diff, terminal, locations; later events for the same call id overwrite the fields they carry |
| `permission` | request id, call id, title, kind, options |
| `permission_resolved` | request id, chosen option or `cancelled` |
| `plan` | entries with content, status, priority |
| `terminal` | terminal id, command, output delta, exit code, done |
| `turn_start` | turn number |
| `turn_end` | turn number, stop reason, token usage |
| `error` | message, code |
| `model` | current model, options |
| `mode` | current mode, options |
| `options` | every other choice the harness offers: id, name, category, current value, choices |
| `commands` | slash commands the harness offers |
| `answer` | Ask only, before `turn_end`: actions, command plan, `needsAgent` and reason, `via` (provider/model) |
| `title` | new title |

Rules:

- `text`, `thought` and `terminal` deltas are coalesced in the daemon to at most
  one event per 50 ms per message or terminal, and each emitted event is the
  persisted one, so live and replayed streams are identical.
- `session.attach {id, since}` replays every event with `seq > since`, then
  streams. A reconnecting client passes the last seq it applied.
- Every open permission is answered exactly once: by the user, by policy, or
  with `cancelled` when the turn is cancelled or the agent stops.
- Ask sessions keep the newest 200; Agent sessions stay until deleted.
- A session nobody named takes its first prompt as its title, on one line of
  at most 60 characters, or the name of its first attachment when it has no
  words. A title the harness sends later replaces it.
- A prompt carries the files attached to it in `images`, whatever they are.
  The field keeps its name from when it only held pictures. Each runner hands
  them to its model the way that model can take them (see Ask and Agent), and
  a prompt that waits in the queue keeps its own.
- `SessionInfo.state`: `idle`, `working`, `waiting` (open permission),
  `stopped` (agent process not running), `failed`.
- `SessionInfo.mode` is the harness's mode for an Agent session. For an Ask
  session it is that conversation's approvals value, set at `session.new`
  (`mode`) and changed with `session.set_mode`; empty reads as `read-only`.
- `session.start` brings an Agent session's process up with no prompt, so its
  `model`, `mode` and `options` events exist before the first one. The idle
  stop takes it down again if nothing follows. `ui.open` with only a session
  is completed by the daemon with the tab that session lives in.

Ids are `<kind>-<yyyymmdd>-<hhmmss>-<4 hex>`, which sort by creation.

## Gateway

The built-in router replaces Prowl. It runs inside the daemon and serves Ask.
Other programs reach it only through the local API below, which is off until
asked for; without it the router opens no port.

- **Catalogue** (`internal/gateway/catalog.json`, embedded): curated
  OpenAI-compatible providers with free or renewable tiers. Each entry has an
  id, name, base URL, auth (`bearer`, `none`, `cloudflare` account plus token),
  `keyless`, signup URL, extra headers, and its free chat models (id, name,
  context, tools, reasoning, vision, quality rank, speed rank). Derived from
  Prowl's MIT catalogue (attributed in `ryoku/rashin/NOTICE`). Non-chat models
  (speech, transcription, guard, embedding, image) are never listed.
- **Keys**: one record per key in `gateway.db`, the secret sealed with
  AES-256-GCM under `master.key`. Health is honest: `healthy` only after a real
  completion through that key, `error` only after the provider rejects the key
  (401, or 403 with an invalid-key message), `unknown` otherwise. A model list
  that answers proves nothing.
- **Availability**: a model is available when its provider is enabled, has a
  key that is not `error` (or is keyless), and neither the model, key nor
  provider is cooling down.
- **Routes**: `auto` orders available models by quality rank then recent
  success; `auto:fast` by measured time to first token, then speed rank;
  `auto:smart` by quality rank only; `<provider>/<model>` pins one model and
  fails over to `auto` only when the user's request did not pin it. A request
  with tools only considers models with tool support, and a request with a
  picture only models with `vision`: one without it answers about a picture
  it never saw, or refuses the whole request. When that leaves nothing, the
  error names the providers that would do (`llm.NoRoute`, which callers show
  as it is, since it already says what to do).
- **Failover**: at most 6 attempts and 45 s before the first streamed byte,
  then the error is final. Classification: invalid key → mark the key `error`,
  try the next key or provider; 429 → cool the model and key down for
  Retry-After or 60 s, or until UTC midnight when the message names a daily
  quota; 402 or exhausted credit → provider cooldown 24 h; 404 or 410 model
  missing → model cooldown 24 h; 413 or context too long → next model with a
  larger context; 5xx, timeout or transport error → provider cooldown 2 min.
  Once a byte is streamed there is no failover: the stream ends with an error.
- **Usage**: per day, per provider and model: requests, successes, failures,
  tokens (exact when the provider reports them, otherwise estimated and marked).
- **Zero setup**: keyless providers work with no key, so Ask answers on a fresh
  machine when one of them is up. When nothing is available Ask says so and
  names the next step (add a free key in Rashin > Settings > Providers).

### Local API

With `api.enabled` the daemon listens on `127.0.0.1` and `[::1]` at `api.port`
(3600) and serves the router in the shape of OpenAI's API, so any program that
takes a base URL and a key uses the same providers, keys, failover and usage
counts as Ask. `internal/gateway/api.go` is the surface, `internal/daemon/api.go`
the listener.

- `GET /v1/models`: the three routes, then every model that can answer now.
- `POST /v1/chat/completions`, streamed as server-sent events or whole.
  Messages with the roles `system`, `developer`, `user`, `assistant` and
  `tool`; text parts and inline base64 image parts (which send the request
  to a model with vision, or get 503 when none is in use); function tools and
  tool calls; `temperature` and `max_tokens`. `top_p`, `stop`, `seed`, the two
  penalties, `response_format`, `tool_choice`, `parallel_tool_calls` and
  `reasoning_effort` are passed on as sent, and every other field is dropped,
  because free providers reject fields they do not know. Reasoning comes back
  as `reasoning_content`.
- **Model**: a route or `<provider>/<model>`. A named model is that model or
  an error, never another in its place. No model, or an `auto:` name the
  router does not have (Prowl had more), is `auto`.
- **Key**: `rashin-` and 48 hex digits, made on first use and sealed in
  `gateway.db` like a provider key. Every request carries it as
  `Authorization: Bearer` or `X-Api-Key`; it is compared in constant time.
  `gateway.api_key` with `rotate` replaces it. Loopback is not a boundary:
  every account and every web page on the machine can reach the port. So no
  route answers without the key, the `Host` header has to name localhost or a
  loopback address (a page that points its own name at 127.0.0.1 still sends
  that name), and no CORS header is ever sent.
- **Refused**: an image named by a web address, so a caller cannot make the
  daemon fetch for it; more than one choice (`n`); a body over 32 MiB.
- **Errors** have OpenAI's shape. Before the first byte they are ordinary HTTP
  errors (400 for the request, 401 for the key, 502 when the provider fails,
  503 when nothing can answer); after it the stream ends with an error frame.
- **Lifetime**: the address answers while the daemon runs, and a daemon that
  serves it does not exit at idle. Nothing starts the daemon at login, so after
  a login the address answers from the first use of Rashin. When the port is
  taken the API stays down, `gateway.api` says why, and the daemon exits at
  idle as usual.

`rashin api` prints the state and the address; `rashin api on [--port n]` and
`rashin api off` switch it; `rashin api key [--rotate]` prints the key. The app
has the same under Settings > Local API.

## Ask

The Ask runner (`internal/ask`) is Rashin's own agent loop on the gateway.

- **Prompt**: Ryoku identity, what this conversation may do, the vault
  contract and `desktop.md` map from the Ryoku vault (trimmed to budget), what
  Rashin itself is and how it is reached (`internal/brief`), the user's habits
  when enabled, the terminal context when the ask came from a terminal, and
  the rules: answer GUI first (name the Hub page, shell picker or bar style
  setting before any command), cite vault wiki pages by name, plain
  sentence-case prose, no em dashes.
- **Tools** (read-only, at most 8 rounds per turn):
  `system_query` (a shell command that the command classifier rates
  read-only, given two minutes; in a read-only conversation anything else is
  refused with the reason and "propose it instead"),
  `read_file` (bounded, refuses secrets: keys, tokens, credential stores,
  `~/.ssh`, browser profiles), `list_dir`, `search_system` (the system index),
  `search_code` (a project index), `read_wiki` (a vault wiki page),
  `fetch_url` (GET over http(s), text only, 200 KiB, public addresses only),
  `doctor` (`ryoku doctor --json`, cached 2 min), and `propose` (a command plan
  for the user: commands with a why and a tier).
- **Acting** (`internal/ask/act.go`): a conversation whose mode is `ask` or
  `auto` is also handed `run_command` (one shell command, 2 min, 24 KiB of
  output) and `write_file` (one whole text file up to 256 KiB, written beside
  the old one and renamed over it, through a link without replacing the link,
  never a secret path), gets 14 rounds, and is told so in its prompt: make the
  smallest change, go through Ryoku's own commands and `ryoku owner <path>`
  before a config file, check that the change took. A command that only reads
  runs at once in every mode. In `ask` each change is a `permission` event
  (allow once, allow in this conversation, refuse) and the turn waits; a
  refusal is told to the model as final. In `auto` changes run unasked, except
  a command of the `danger` tier, which always waits and is never offered
  "allow in this conversation". A file's change is shown as a diff before the
  answer is given, and what the file held before is kept in the session's
  `before/` folder. `sudo`, `doas` and `su` are refused: they ask for a
  password on a terminal Ask does not have, so they stay proposals. A
  read-only conversation is not handed either tool, and a model that calls
  one anyway is refused.
  Once a conversation may act, the mode decides, not the tool the model
  happened to pick. A `system_query` that is more than a read is cleared and
  run like `run_command`, where it used to be turned down toward `propose`. A
  `propose` whose commands all run as the user is handed back once with "run
  them yourself", so the answer carries no plan; a plan that needs root, or a
  second proposal, stands. A command that names a credential file or sweeps a
  tree that holds them (`command.ReadsSecret`) never runs in any mode, since
  what it prints is sent to the model.
- **Running where it can be watched** (`internal/ask/run.go`): a command
  that says nothing until it ends reads as a hang, so its output is a
  `terminal` of the session, named after the tool call that owns it and sent
  on as it comes (whole characters only, up to 256 KiB). A command Ask runs
  itself becomes one after 700 ms, so a quick look stays a plain result; one
  the user started is one from the start.
- **A run by the user**: `session.prompt` with `run` in place of text is a
  turn whose question is a command. It works in every mode, since the user
  started it: the daemon refuses it before the turn exists when the session
  is not an Ask or the command needs a terminal (`command.NeedsTerminal`: it
  asks for a password, or names a credential), so the client can still send
  it to one. The command runs as the user from the home folder with ten
  minutes and no keyboard, `session.cancel` kills its process group, and
  when it ends Ask is handed the command, its exit status and what it printed
  and answers from that. A later turn's history holds "I ran this command
  myself" and the answer. A proposed plan marks each command that needs a
  terminal (`terminal: true`).
- **Command classifier**: tiers `read`, `write`, `system` (root or package
  manager), `danger` (destructive or irreversible). Wrappers, pipes, segments,
  redirects, substitutions and `find -exec` are classified by their worst part;
  `sudo`, `doas`, `pkexec`, `su`, `run0` are never read-only. Throwing a
  stream away (`2>/dev/null`, `&>/dev/null`) or copying one (`2>&1`) is not a
  write, `head -20` and `tail +3` count as the reads they are, and `~` alone
  is the user's own home. `find -exec` and `xargs` are reads when what they
  run only says something about each name (`du`, `ls`, `stat`, `file`, `wc`,
  a checksum) and never prints the file, so a walk over a whole tree cannot
  spill a secret; `{}` alone is their placeholder, any other brace a group.
- **Answer**: the streamed text, then one `answer` event with actions extracted
  from the final text (paths to files and folders that exist, URLs, colours,
  commands) and the plan when `propose` ran. `/` and the directories right
  under it, and devices such as `/dev/null`, are named in passing and offered
  as nothing; a path is a command only when it is a program. When the work
  needs changes the model ends with `NEEDS_AGENT: <reason>`; the runner strips
  it and sets `needsAgent`.
- **Attachments** (`internal/ask/attach.go`, `internal/attachment`): what a
  file is comes from its first bytes, never its name. A picture (PNG, JPEG,
  GIF, WebP, up to 8 MB) rides on the message, which routes the turn to a
  model with vision. A text file is written into the message, up to 24 KB,
  with a line that says how much of it that is: attaching a file is the reader
  showing it to the model, so it does not wait for `read_file`, which may
  refuse the path. Anything else is named. An attachment that cannot be read
  stops the turn with the reason.
- **Follow-ups**: an Ask session keeps its conversation; earlier turns are
  replayed into the prompt within a token budget. Their attachments go again
  newest first, within 16 MB of pictures and 48 KB of text; one that is gone,
  or that there is no room for, is said in its place.
- **Handoff**: `session.handoff` opens an Agent session (default harness unless
  named) whose first user event carries the question, what was attached to
  it, the answer and the reason; with `run` it prompts at once.

### In a terminal

`rashin <question>` (or `rashin ask <question>`) asks with origin `terminal`
and the terminal context (cwd, last command and its exit status from the shell
hooks' `RASHIN_LAST_CMD` and `RASHIN_LAST_STATUS`). Prose streams to stderr;
the plan follows. `--buffer` prints only the plan's commands on stdout (one per
line) for the shell hooks to place on the prompt; nothing runs until the user
presses Enter. `-c` continues the last terminal session, `--resume` lists recent
asks, `--last` prints the last answer again. Fish, Bash and Zsh hooks
(`ryoku/apps/*/rashin.*`) wrap the command and bind Alt+R to turn the current
command line into a proposed command; they do nothing while `rashin` is absent.

## Agent

The harness runner (`internal/harness`) speaks ACP version 1 over the agent's
stdio. The daemon spawns the agent in the session's working directory (chosen
when the session is created, default the home directory).

| Id | Harness | Command |
|---|---|---|
| `claude` | Claude Code | `claude-code-acp`, else `claude-agent-acp` |
| `codex` | Codex | `codex-acp` |
| `opencode` | OpenCode | `opencode acp` |
| `gemini` | Gemini CLI | `gemini --experimental-acp` |
| `qwen` | Qwen Code | `qwen --experimental-acp` |
| `hermes` | Hermes | `hermes acp` |
| `omp` | Oh My Pi | `omp acp` |
| `goose` | Goose | `goose acp` |

A harness is installed when its command resolves on `PATH` (Hermes also from
`~/.hermes/bin` and `~/.local/bin`). Its version comes from `--version` with a
2 s timeout, cached per binary mtime.

Client capabilities Rashin offers: `fs.readTextFile`, `fs.writeTextFile`,
`terminal`. Rashin reads and writes files for the agent (recording each write as
a `tool` event diff), and runs its terminals (pipes, output capped at 1 MiB per
terminal, killed on release or session stop). A team agent, which runs in a
sandbox (see Teams), is offered neither: Rashin refuses its `fs/` and
`terminal/` requests, so nothing it asks for runs outside the sandbox.

- **Attachments**: a picture goes as an ACP `image` block to an agent that
  advertised `promptCapabilities.image`. Every other file, and a picture for
  an agent that takes none or one over 8 MB, goes as a `resource_link` to the
  file, which every agent takes and opens with its own tools.
- **Approvals** follow `agent.approvals`, read at each request, so a change
  made in a composer or in Settings holds for every session from then on.
  Under `read-only`, tool calls of kind `read`, `search`, `fetch` and `think`
  are allowed at once; the rest ask.
- **Permissions** arrive as `permission` events; a client answers with
  `session.permission`. A request that arrives before its tool call carries the
  tool call and is shown at once.
- **Models, modes and options**: the agent's `configOptions` are read whole,
  flat or grouped. The one of category `model` is the session's model (the
  legacy `models` list is read too), one of category `mode` is its mode unless
  the agent also listed `modes` the older way, and every other select (how
  hard to think, and whatever a harness adds) is an `options` event. Each is
  set with `session/set_config_option` where the agent offers the option, else
  `session/set_model` and `session/set_mode`, and the agent's answer is the
  new state. An agent repeats its whole list on every change, so an event is
  written only when what it says changed: a model list runs to hundreds.
- **Before the first prompt**: `session.start` runs `initialize` and
  `session/new` and stops there. The app calls it when a session that never
  ran is opened, so the composer shows the model, mode and options the
  harness will really use, and stops the agent again if the session is left
  with nothing asked.
- **The brief** (`internal/brief`, `internal/daemon/brief.go`): the first
  prompt of an agent session carries one text block ahead of the user's own:
  that it runs inside Rashin on Ryoku, the session's folder, project and
  branch, where the Ryoku vault is and how to use it, what Rashin is, and the
  code search commands over its folder with the languages the index holds, so
  an agent whose own index or search skill skips a language knows to come
  here. A session the agent loads back is not briefed again; one that had to
  start over is. The folder's repository is indexed when the session's agent
  is created (see Index), so the search is there when the agent looks.
- **Rashin's tools** (`internal/mcp`, `cmd/rashin/mcp*.go`,
  `internal/daemon/tools.go`): every agent session is handed one stdio MCP
  server, `rashin mcp`, in the `mcpServers` of `session/new` and
  `session/load`, since every ACP agent takes stdio MCP. It serves the code
  search (`search`, `find`, `def`, `outline`, `references`, `impact`,
  `peek`), the session's `goal`, and, where the desktop can host one, the
  screen tools. `RASHIN_SESSION` and `RASHIN_CWD` tell it its session.
- **Subagents** (`internal/harness/subagents.go`): omp's task tool reports its
  subagents in the call's `rawOutput.details` (progress while they run,
  results after), and Claude Code reports an Agent or Task call and then each
  call its subagent makes, marked with the parent in
  `_meta.claudeCode.parentToolUseId`. Both become `agents` on the parent's
  `tool` event: each subagent's name, task, status, what it is doing, its last
  steps, step count, tokens, model, time and answer. A subagent's own calls
  are its steps, never transcript rows of their own. An omp subagent with no
  description is named by the ID the model gave it, since omp's task text
  opens with its template.
- **Goals** (`internal/daemon/goals.go`, `protocol/goal.go`): `/goal <what>`
  in an agent session sets a goal that Rashin follows itself, since a
  harness's own goal loop does not run over ACP. After each turn under an
  active goal the daemon prompts the next one ("Continuing the goal, turn N of
  M") until the agent calls the `goal` tool to mark it done or blocked, the
  turn budget (30 by default, `/goal turns <n>`) runs out, a turn fails or is
  stopped, or two turns in a row do nothing. `/goal pause`, `resume`, `stop`
  and `clear` count only as the whole message, so a goal that starts with one
  of those words is still a goal. A blocked goal needs the user.
- **The agent's screen** (`internal/screen`, `internal/daemon/screens.go`): an
  agent that needs a graphical app gets a hidden desktop of its own, a nested
  session of the user's compositor started through `ryoku wm nest` where the
  provider claims `nestedSession` (see [compositors.md](compositors.md)). Its
  apps run with their own home folder and a private session bus; the screen
  takes a virtual pointer (pure Go) and `wtype`, and is read with `grim`, all
  of which ryoku-desktop brings. The first screen tool asks the user with a
  request card Rashin draws itself. The user can watch the screen (frames once
  a second, only while a client watches and only when they changed), take it
  over and hand it back, and the agent can ask them for something only they
  should do, such as a sign-in (`screen_ask_user`). An agent's action waits
  while the user has the screen. Harnesses give up on a tool call after half
  a minute or so, so a call that waits on the user returns after 20 s saying
  nothing was done yet, and the agent calls again; the request, the hand-over
  and the ask stay up meanwhile. A screen ends with its session, after ten
  idle minutes, or on Close the screen.
- **Its own setup**: `harnesses.list` carries what a harness will run with.
  `setup` is read from the harness's own files where Rashin knows them (omp:
  the model behind each role and the thinking level, from
  `~/.omp/agent/config.yml`), and `capabilities.started` is the model and
  option values it came up with the last time it ran. Both are shown, never
  written.
- **Resume**: the harness's session id is kept in `meta.json`. A stopped
  session restarts its agent on the next prompt with `session/load` when the
  agent advertised `loadSession`, otherwise with a new session and a `notice`.
- The harness keeps its own model and provider configuration; Rashin never
  edits a harness's files.

## Index

The index engine (`internal/index`) is pure Go.

- **Profiles**: `system` (id `system`) and projects (id from the root path).
  The system profile covers the Ryoku vault, `$XDG_CONFIG_HOME` (minus caches,
  backups, browser and chat-app profiles, keyrings, credential files,
  databases and files over 512 KiB), `/usr/share/ryoku` and `/etc/ryoku`; a
  config file identical to its shipped copy is indexed once. A project is any
  directory added with `rashin index add` or registered as a Work project.
  The repository an agent session works in is registered and refreshed when
  the session's agent is created, and the index commands of the command line
  register the git root of the current directory on first use, so the search
  answers wherever an agent or a person asks. `/`, the home directory and its
  ancestors are never an index root, and roots are absolute.
- **Walk**: `.gitignore` at every level, extra ignores, symlinks that escape
  the root are skipped, binary files are skipped. Projects also skip vendored
  and generated trees (`vendor/`, `node_modules/`, build outputs, virtual
  environments), minified bundles, source maps, lock files, structured data
  over 256 KiB and any file over 1 MiB.
- **Extraction**: Go through `go/parser`; Python, JavaScript, TypeScript, Rust,
  C and C++, Lua, Bash and Fish, QML, KDL, Markdown, JSON, YAML, TOML, INI and
  CSS through line-oriented extractors (definitions, their spans, signatures,
  doc comments, imports). Chunks are 40-line windows of at most 8 KiB that keep
  a symbol of up to 80 lines whole.
- **C and C++** (`extract/cfamily.go`): comments, strings and preprocessor
  lines are blanked first, then classes, structs, unions and enums (a typedef'd
  anonymous one by its name), functions, `Class::method` definitions with
  their class as parent, member declarations, `using` and typedef types,
  constants and operators are read, through constructor initialiser lists,
  templates and attributes. A `.h` with C++ in it is read as C++. `find`
  prefers a definition to its declaration, and a qualified name finds its
  bare symbol. Each index records the extractor version it was built with;
  when an extractor improves, the next refresh rereads only that extractor's
  languages.
- **Redaction**: secrets (key patterns, private key blocks, `password`/`token`
  style assignments in every common syntax, credentials in URLs) are replaced
  before anything is stored, and again on the source text `peek`, `def` and
  `references` return.
- **Storage**: one bbolt file per index: files (path, hash, size, mtime,
  language), symbols, chunks, an inverted index for BM25, int8 vectors and the
  counts `list` reports. Refresh is incremental by content hash and runs before
  a query when files changed; vectors of unchanged chunks are kept. Each index
  refreshes on its own: listing, status, removal and queries on other indexes
  never wait for it, and removing an index cancels its refresh.
- **Embeddings**: `minishlab/potion-code-16M` (MIT), int8-quantized (256
  dimensions, 61,826 tokens), embedded in `rashind` with its pure-Go WordPiece
  tokenizer and loaded on first semantic use. Vectors are computed in the
  background after the lexical refresh, in bounded batches.
- **Search**: BM25 over chunks and cosine over vectors, fused with reciprocal
  rank (k 60); lexical only when vectors are off or not yet computed. Ask
  searches with `existing` set: only registered indexes, never waiting for a
  refresh (one starts in the background), so a question never builds an index.
- **Queries**: `search`, `find` (exact, prefix, camel/snake parts, fuzzy),
  `def` (body up to 200 lines), `outline`, `references` (whole-word call sites),
  `peek` (up to 200 lines), `overview` (counts, languages, entry points,
  largest files and most imported files), `impact` (files that import a path,
  transitively, from Go, Python, JavaScript/TypeScript, C/C++, QML, Lua and
  shell `source` edges), `wip` and `changed` (git state against the index).

Command line: `rashin index add|remove|list|refresh|status`, `rashin search`,
`find`, `def`, `outline`, `references`, `peek`, `overview`, `impact`, `wip`,
`changed`, each with `--index <id>` and `--format human|json|toon` (human on a
terminal, TOON when piped). The daemon runs the engine; the command line is a
client, so it says Rashin is off when it is.

## Work

Everyday Rashin is Ask. Work is what Rashin becomes when it is pointed at code:
projects, Git worktrees, a commit graph, review, checkpoints and tasks, owned by
the daemon so several agents run in parallel without touching each other's
files. The ideas come from Orca (repository, worktree and agent model, agent
board, decision gates, setup hooks, diff comments sent to the agent, cleanup
with reasons), T3 Code (lazy sessions, per-turn checkpoints, queue and steer,
harness instances, capability-driven UI), omp (agent hub, plan mode, fork from
here), Paperclip (tasks separate from sessions, board, dependencies, ledger,
work products) and GitKraken, GitLens and lazygit (lane graph, a WIP row per
worktree, worktree badges, details panel). Nothing in Work runs until a project
is opened, and nothing runs at rest.

### Model

```
project -> worktree -> session -> turn (with a checkpoint)
project -> task -> worktree + sessions
```

- A **project** is a registered folder, usually a Git repository; its id is the
  index id of its root, and registering it registers that index. Projects and
  their settings live in `$XDG_STATE_HOME/ryoku/rashin/projects.json`.
- A **worktree** is a real `git worktree`; its absolute path is its identity.
  Git stays the source of truth for what exists. Rashin records the worktrees it
  creates (owned), with base ref and base commit, parent worktree (sidebar
  nesting only), task, setup state and last activity. Rashin-made worktrees
  live under `$XDG_DATA_HOME/ryoku/worktrees/<project>/<name>` (a project
  setting) on branch `<prefix><name>` (prefix `rashin/`).
- A **session** may bind to a project and a worktree (Agent sessions only).
- A **task** is a durable work item separate from the sessions that work on it:
  title, goal, board column, dependencies, decision gates, its worktree and
  sessions, and its products (branch, commits, files and lines changed).

### Worktrees

- Create from the project base ref (`origin/HEAD` when it resolves, else the
  main worktree's branch), a branch, a commit or a remote branch: new branch,
  existing branch or detached, optionally locked with a reason (atomic
  `--lock`). Never forced: a branch checked out elsewhere names that worktree
  and offers to open it.
- After creation the project's listed gitignored files (`copyIgnored`, such as
  `.env`) are copied from the main worktree, then setup runs when asked: the
  project's setup command (typed by the user, so trusted) or a repository script
  `.rashin/setup` only after the user trusted that exact content (its sha256 is
  kept in the project settings). Output is kept; states none, running, done,
  failed, untrusted.
- Status: porcelain v2 (branch, upstream, ahead and behind, staged, modified,
  untracked, conflicted, operation in progress) plus ahead, behind, added and
  removed lines against the worktree's base.
- Removal is two steps. The preview lists dirty and untracked files, unpushed
  commits, unmerged state, lock and running sessions; removal needs force
  exactly when the preview says so; deleting the branch is a separate choice
  (`-d`, `-D` only when asked). The main worktree is never removed. Lock,
  unlock, move, repair, prune (dry run first).
- Cleanup policy per project, off by default: remove Rashin-made worktrees whose
  branch merged into the base some days ago, or that sat inactive for some
  days, protecting uncommitted changes (or any local file), never one with a
  running session; every kept or removed worktree comes with its reason. It runs
  on demand, or when the daemon is awake for that project anyway.
- Ports: listening sockets of processes whose working directory is inside the
  worktree, read from /proc when asked, with a localhost URL.
- The app opens the user's terminal (`ryoku-app terminal`) or file manager in a
  worktree; no terminal emulator lives in the app.

### Graph

- `git.log` pages the whole repository newest first in topological order with
  full history. The first page carries the refs (branches, remotes, tags, every
  worktree's HEAD) and one WIP row per worktree with uncommitted changes or an
  agent session. Scopes: every ref, one branch, one worktree; search by message,
  author or hash.
- The app lays lanes out incrementally (`internal/gitgraph`): a lane waits for a
  parent, the leftmost incoming lane continues, first parents run straight,
  extra parents (octopus merges too) branch out, colours are deterministic, and
  rows already drawn never change when more history loads. More than 24 lanes
  collapse behind a marker that expands.
- Decorations: branch, remote and tag labels, HEAD, a worktree badge on every
  checked-out branch (Open worktree replaces checkout there), and a WIP row
  above each worktree's HEAD with its counts and the agent's state.
- Interaction: selecting a row fills the details panel (message, author,
  parents, refs, files, diff); up, down and page keys, `/` to search, a ref
  finder, the scope filter. Row actions: create a worktree here, create a branch
  here, check out in a worktree, merge into a worktree, cherry-pick, revert,
  rebase a worktree onto it, copy the hash, compare with a worktree. Every
  mutation shows what it affects first. A conflict leaves the worktree in the
  operation with Abort, Continue and Resolve with agent.

### Review

- A worktree's changes: staged, unstaged and untracked files, and the branch
  against its base, with hunks and line numbers. Stage, unstage, discard (with a
  confirmation), commit with a message, push (force with lease only when asked),
  fetch, pull (fast-forward only).
- Comments pin to a diff line (file, line, side); resolve or delete them. Send
  review to agent batches every unresolved comment into one prompt for the
  worktree's session.
- Every action that changes Git or the worktrees is written to the project's
  ledger (`activity.list`): time, actor (you, Rashin, or a session), action,
  target and result.

### Checkpoints

Before every Agent turn in a Git worktree the daemon snapshots the worktree's
files (tracked and untracked, ignored excluded) into the hidden ref
`refs/rashin/checkpoints/<session>/<turn>`, without touching the index, HEAD or
the branch. The app shows each turn's own changes (its checkpoint against the
next one, or against the current files) and offers Revert files to before this
turn, refused while another session runs in the same worktree. The conversation
stays: a harness cannot rewind its own context. Deleting a session deletes its
checkpoint refs.

### Sessions in Work

- A new Agent session picks a project, then the project folder, an existing
  worktree or a new one (name, base, branch), then a harness (built in or a
  custom instance from Settings). Move to Agent from Ask asks the same.
- Queue and steer: while a turn runs, Send queues the message, with what is
  attached to it, for when the turn ends (queued messages sit above the
  composer and can be removed); Steer (Ctrl+Enter) cancels the turn and sends
  at once.
- Fork from here copies a session up to an event into a new session; an Agent
  fork starts its harness fresh with the copied transcript as context.
- A session is unread when a turn ended and nobody looked; its activity line
  names the tool it is running.

### Tasks

- States: backlog, ready (dependencies done), working, needs you (an open
  permission or gate), review (a turn ended with changes), done, blocked. The
  app lists tasks grouped by state, what needs you first. The daemon moves a
  task between working, needs you and review as its session changes; the user
  moves it anywhere else and marks it done.
- Starting a task creates a worktree from the base, named from the title, and an
  Agent session in it prompted with the goal; tasks run in parallel, each in its
  own worktree. A task with unfinished dependencies is blocked.
- A decision gate is a question with options on a task. While it is open the
  task needs you; resolving it records the answer and sends it to the task's
  session.
- Plan asks the free-model router to split a goal into drafts with
  dependencies; nothing is created until the user picks drafts.
- Products: branch, commits ahead of the base, files and lines changed,
  refreshed after each turn.

### Agents

The Agents view is the hub across projects: Needs you (open permissions and
gates, failed turns, conflicts), Working (with the activity line), Done
(unread) and Idle. Each card names the harness, model, project, worktree and
branch and opens the session. Stop all ends every running turn.

## Teams

Teams is the second mode of the app: agents that work on their own, on a
schedule or when given a task, and hand back a report. A team is a standing
group of agents with one purpose on this computer. Its agents run in a
sandbox, reach the outside world with a credential only through Rashin, and
wait for the user before anything that leaves the machine in their name
(`internal/teams`, `internal/vault`, `internal/sandbox`, `protocol/teams.go`,
`protocol/vault.go`). Nothing in Teams runs while no task is working and no
routine is on.

- A **team** has a name, a purpose, a folder (default `~/Teams/<slug>`), a
  lead and up to five more members (each a name, a title, an engine and
  instructions), its tools (a screen of its own, mail), how far its agents go
  before asking (read only, ask before changes, act without asking: an
  approvals value), a budget (tokens a month, turns and minutes per task),
  how many of its tasks run at once (`running`, 0 follows `teams.perTeam`),
  what its sandbox adds (`access`: folders it may change besides its own,
  paths hidden from it) and a notify level (`all`, `needs`, `none`). A paused
  team starts no task and fires no routine, and neither does any team while
  `teams.paused` is on.
- **Templates**: Research desk, Mail desk, Repo watch, System care and New
  team. Repo watch brings the routine Morning review (`0 9 * * 1-5`) and
  System care one at `0 18 * * *`, both off.
- `teams.export` writes a team's definition into its folder, without history
  or credential values; `teams.import` makes a paused team from one, its
  routines off.

### Engines

A member's engine is its harness, its model and the harness's own options,
such as effort.

- A member with a harness of its own runs on it with its own model and
  options. A member left on the teams default runs `teams.defaults.harness`,
  else the Agents harness (`agent.harness`), else the first installed one,
  with the defaults' model and options where it names none
  (`protocol.ResolveEngine`, the one chain the daemon and the app share). A
  default always names what it resolves to: "Teams default (Claude Code)",
  "Same as Agents (Codex)", "Opus 5.5 (harness default)".
- A harness's models, its default model and its options are learned each
  time it starts and kept per harness id with the binary's path and
  modification time, so two instances of one binary keep their own lists;
  `harnesses.changed` tells the app. Until a harness has started, a model is
  typed by its id.
- Check (`harnesses.probe`, the app's alone) starts the harness once in a
  team sandbox with the chosen model and options, without a prompt and within
  30 seconds, and says whether it took them: "Claude Code started in the
  sandbox and took Haiku 4.5." or the harness's own error. It records what it
  learned.
- A member may name a harness that is not installed any more; it reads "not
  installed" wherever the engine shows. An id this computer never knew is
  refused.
- A started task keeps the harness, model and options it ran on
  (`TeamTask.harness`, `model`, `options`), so a finished task still says it
  after its session is gone. Give a task can pick another model and options
  for that task alone.
- The engine shows on the crew chart, the team's tile and header, the member
  rows of its settings, a task's header and details, the Work board and the
  Spend view's By engine list. Activity says "Researcher started work on it
  with Claude Code, haiku."

### Tasks

- A task is an ordinary Agent session in the team's folder, driven by the
  goal machinery (see Agent, Goals): the goal is the task's brief and the turn
  budget is the team's turns per task. It ends when the agent
  calls `report`; the report lands in the Inbox unread. A task that cannot
  finish ends blocked and says why.
- States use the Work board's words: ready (waiting for room), working,
  needs you, done, blocked, stopped. A task came from you, a routine or the
  lead. Stop, retry with a note and delete are the user's.
- The lead takes the team's tasks and hands pieces to members with
  `delegate`: one level deep, at most four open at a time. It is held until
  their reports are in and then continued with them, and `team_status` shows
  it what it handed out. Stopping a task stops what it delegated.
- At most `teams.perTeam` tasks of one team run at once (default 2, or the
  team's own `running`), and `teams.maxRunning` (default 3) across all teams;
  the rest wait their turn. A task still working past its minutes per task
  stops blocked and says so.
- While a member works, its session carries when the turn started and how
  many tool calls it made (`SessionInfo.turnStarted`, `steps`), so the crew
  chart, the Inbox, the Work board and the task's page show how long it has
  worked and how many steps it took.
- The tools come from `rashin mcp`, which the daemon starts with the team's
  environment (`RASHIN_TEAM`, `RASHIN_TEAM_TOKEN`, `RASHIN_TEAM_LEAD`):
  `report`, `ask_user`, `credentials`, `http_request`, `send_mail`,
  `request_credential`, and for the lead `delegate` and `team_status`, beside
  the usual code search, goal and screen tools. Each is a `teams.agent_*`
  call carrying the run's token, which the daemon checks against that
  session's live run.
- Only the daemon starts a task's session (`session.new` naming a team is
  refused), and only the Rashin app answers for it, prompts, steers or ends
  it.

### Approvals

- An approval holds the exact action it shows: a mail's whole text, or a
  request's method, address, headers and body. Approved, that action runs
  once, as shown. `send_mail` and `http_request` ask by themselves, so agents
  are told not to ask first with `ask_user`.
- Kinds: `mail`, `credential` (a credential's first use on a host),
  `request` (an agent asks for one its team lacks), `action` (`ask_user`) and
  `budget`.
- Answers are once, always or deny, with an optional note the agent reads;
  a `request` is answered with the value itself.
  Always is offered for one credential on one host for one team, or one mail
  recipient for one team.
- **Choice questions**: `ask_user` with `options` (2 to 6) asks the user to
  pick one, by click or by its letter, or none of them. The agent hears "The
  user chose: <option>" and any note.
- An approval nobody answers expires as denied after `teams.approvalHours`
  (default 24). A budget card does not expire: the team stays paused until it
  is answered.
- They are answered in the Inbox or on the task's page, and every decision
  goes into Activity.

### Routines

- A routine is a brief a team takes on by itself on a cron schedule: the
  presets every hour, day, weekday, week or month, or a custom five-field
  expression, shown as a sentence with its next five runs
  (`teams.routine_preview`). Times are local.
- At most one run every 15 minutes, and at most 20 routines on. A run missed
  while the computer was off or asleep is skipped by default (`missed`:
  `skip` or `once`); a run due while the last still works is skipped by
  default (`overlap`: `skip` or `queue`). Five failures in a row turn a
  routine off.
- Each run is a task whose origin is the routine. `teams.routine_run` starts
  one at once.
- **Quiet runs**: a run with nothing new calls `report` with `quiet`. The task
  ends done and read, nothing is notified, Activity records "Nothing new", and
  the routine counts quiet runs in a row ("Quiet for 3 runs").
- While a routine is on in a team that is not paused, the daemon stays up,
  and with `teams.routinesWake` on the routines wake unit is enabled, so
  routines pick up again after a login, a reboot or a daemon exit (see
  Lifecycle). The daemon enables the unit when the first routine turns on and
  disables it when the last turns off. The Teams board says what keeps
  Rashin up and when the next run is.

### Budgets

- Tokens are counted per turn from the usage the harness reports and kept
  per day, member, routine and engine (`teams.spend`, the team's Spend view).
  A new team that names no budget takes the one in Settings > Teams, New
  teams start with (2,000,000 tokens a month, a warning at 80%, 20 turns and
  60 minutes per task); a limit of 0 is no limit.
- The warning comes once, at the warn mark. At 100% the team pauses, its open
  tasks stop, and the Inbox asks whether to raise the limit by half or keep
  the team paused.

### Activity and notices

- **Activity** is an append-only ledger (`teams/ledger.jsonl`) of who did
  what: you, a routine, an agent or Rashin. Its filters are All, Nobody
  watching, Approvals, Credentials and Failures, plus a team. Nobody watching
  marks what happened while no Rashin window was open.
- With no window open, the daemon sends a desktop notification for what needs
  the user and for reports, by the team's notify level; pressing it opens the
  place it names. Inside `teams.quietHours` nothing is notified; what needs
  the user still waits in the Inbox.
- Finished tasks older than `teams.keepDays` are deleted with their
  conversations when the daemon starts and once a day (`teams.prune`); a
  team's Clear finished tasks deletes all of its finished tasks at once. What
  they made stays in the team's folder, and the ledger records how many went.
  The ledger keeps the newest 5,000 entries.

### Vault

Rashin's credential vault (`internal/vault`, `vault.*`) holds what teams need
to sign in somewhere. It is not the Ryoku vault, and agents never see a
value.

- Values are sealed with AES-256-GCM (the credential's id and version as
  additional data) in `vault.db` (bbolt, 0600, in a 0700 directory). The key
  lives only in the login keyring (Secret Service, attributes
  `application=rashin`, `purpose=vault-key`), made with the first value; there
  is no key file to fall back to, so a locked keyring closes the vault and a
  missing one makes it unavailable. Replacing a value keeps one previous
  version. A fingerprint (the first eight hex digits of an HMAC of the value,
  keyed from the vault key) tells two values apart without showing either.
- A credential has a name in upper case (`GITHUB_TOKEN`), a description, how
  it is sent (a bearer token, a named header with an optional prefix, or the
  mail account's password), the hosts it may go to (`*.example.org` covers
  subdomains; https only, plain http only to localhost) and the teams that
  may use it.
- `http_request` sends one HTTPS request with the credential added by the
  daemon, only to an allowed host, with no redirects, no proxy and TLS 1.2 or
  newer. An agent may not set `Authorization`, a method override, `Range`,
  `If-Range`, `Accept-Encoding` or `TE`, and a body over 8 KiB is refused, so
  the card can show the whole request. The response is scrubbed of the value
  in raw, base64, URL and JSON-escaped forms, and one with a
  `Content-Encoding` nobody asked for is withheld. The agent is shown the
  status, the headers it may keep and the first 16 KiB. With `save_as`, a
  path inside the team's folder (no absolute path, no `..`, no link out of
  the folder), the scrubbed answer, up to 8 MiB, is written there (0600) and
  the agent is told how many bytes went where; the card says where it saves.
  The first use on a host asks; Always trusts that credential on that host
  for that team.
- **Mail**: one mail account (address, name, server, port, STARTTLS, TLS, or
  none for localhost only, user name, and a credential for the password).
  `send_mail` asks with the exact mail and the daemon sends it once approved.
  Test the login checks the account without sending (`vault.mail_test`).
- `request_credential` asks the user for a credential the team lacks, for
  exact hosts only. The user types the value into the Inbox or the Vault
  card. Providing it trusts nothing: the first use still asks.
- Values are scrubbed from the ledger, the vault's record of use, approval
  titles and details, reports and failure text.

### Sandbox

Every team agent runs inside bubblewrap (`bwrap`, `internal/sandbox`,
`internal/daemon/sandbox.go`):

- The whole filesystem is read-only. The team's folder, the folders the team
  may change (`access.writable`), the harness's own state (such as `~/.omp`
  or `~/.claude`) and `~/.cache` are writable. Rashin's state and cache, the
  login keyrings, `~/Teams` (the other teams' folders) and the Ryoku vault are
  always hidden, and so are `teams.hidden` (by default `~/.ssh`, `~/.gnupg`,
  cloud and cluster credentials and browser profiles) and the team's own
  `access.hidden`; an agent sees them empty. `/tmp`, `/dev/shm` and
  `XDG_RUNTIME_DIR` are private, and PID and IPC namespaces are its own. There
  is no session bus, no display, no SSH agent. The network is shared:
  harnesses reach their model online, so an agent reaches what you can, and
  Settings > Teams says so. The daemon's own programs stay visible, read-only,
  wherever they run from.
- The agent reaches Rashin only through its task's own socket,
  `$XDG_RUNTIME_DIR/rashin/agents/<session>/rashin.sock`, which the sandbox
  sees at the main socket's path. It serves that session's team tools, goal
  and screen and the index methods that read; everything else is refused
  ("An agent can use only its own tools here.").
- Programs an agent opens on its screen run in the same sandbox, with the
  screen's display and private bus. A harness Check runs in a sandbox like a
  team agent's, in a folder of its own that goes when it ends.
- On the main socket, only the Rashin app may call the user's methods: every
  `teams.*` and `vault.*` method except the ones that only read and the
  agents' `teams.agent_*`, and every control of a team session. The app is
  known by the peer's executable: `rashin-app` beside the daemon or
  `/usr/bin/rashin-app`. Anything else is told "Only the Rashin app can do
  this." `rashin-app --script` runs only with `--demo`.
- The daemon is not dumpable: no core dumps, and no reading its memory
  through `/proc` as the same user.
- Without bubblewrap, team tasks do not start. They end blocked with "Teams
  run their agents in a sandbox, which needs bubblewrap. Install the
  bubblewrap package." Every package lane depends on it.

### In the app

- The Teams button right of the search field, or Ctrl T, switches the window
  to Teams mode and back. Ctrl 1 to 6 pick its destinations: Inbox, Teams,
  Work, Routines, Vault and Activity. Settings opened from Teams mode opens on
  Settings > Teams, in five groups: Running (pause every team, tasks at once,
  tasks at once in one team, wake Rashin for routines), New teams start with
  (engine, permissions, budget, notifications), Asking and telling (how long a
  question waits, quiet hours), Sandbox (the hidden list and the network) and
  History (how long finished tasks stay). Toggles and selects save as they
  change; numbers and text save with their group's Save.
- A team's Settings tab holds everything that team decides, its budget and
  limits included. Its Overview adds Next steps (a member's harness not
  installed, a credential it asked for, a routine that is off, a team that
  never ran, each with one action and each hideable) and Files, the newest
  files in its folder with the report each came from.
- Routes: `inbox`, `teams`, `teams/new[/<template>]`, `teams/<team>[/<view>]`
  (views `overview`, `work`, `routines`, `spend`, `settings`),
  `work[/<task>]`, `routines`, `routines/new[/<team>]`,
  `routines/<routine>`, `vault[/<credential>]` and `activity`
  (`shell.TeamsRoute`). `rashin open --route <route>` opens `inbox`,
  `activity`, `teams[/<team>[/<view>]]`, `work[/<task>]`,
  `routines[/<routine>]` or `vault[/<credential>]` in a running window, or
  starts the app there; a notification opens its place the same way.
- A task's page shows its transcript, what it waits on, its report and the
  engine it ran on, and answers its approvals inline.

### Limits

The sandbox and the vault keep a team's agents from your secrets and from
acting unasked, not everything else. A program you run yourself, outside a
team, still runs as you: it can reach your login keyring and start the app.
The network is shared, so abstract Unix sockets and the system bus stay
reachable from a sandbox, and `/run` outside the runtime directory is visible
read-only. A team agent reads the rest of your files, read-only, like any
program you run; only the hidden paths above are kept from it. The app check
is a gate for agents, not a boundary against someone already running code as
you who can start a `rashin-app` and inject into it.

## The app

`rashin-app` is a Gio client drawn with the Lithic library
(`docs/lithic/library/go`), built the way Ryotunes 2 is: a connection and
reducer store over the protocol, a shell that owns the window and overlays,
pages, and a demo daemon for headless renders. Its window is Lithic's
conversation window (`docs/lithic/layout.md`); that page owns every number.

- **Window**: title `Rashin`, app id `dev.ryoku.rashin`, 1280x820 by default,
  size, sidebar fold and zoom persisted; compositor rules float it. One
  instance (`app.lock`): a second launch sends `ui.open` to the daemon and
  exits; the running app switches to what it names.
- **Frame**: a top bar on the ground (mark, sidebar toggle, back and forward,
  the one search field, window controls), a sidebar that folds to a rail
  (the New plaque, the destinations Ask, Agents, Projects and Index, the
  current destination's list, Settings at the foot), the content pane with a
  header and a body, and a side panel beside a conversation that changes
  files. Expanded from 1100 dp, medium from 760 (rail, the sidebar as a
  sheet), compact below (bottom bar for the destinations, the list as a
  sheet).
- **Ask** is home and opens on a new conversation: one sentence, the composer,
  things to try and one line that says which providers can answer now; no
  session exists until the first message is sent, and sending it moves the
  composer to the foot of the column. Its list groups asks by day. An open ask
  shows the transcript in the reading column with the composer at its foot,
  the route choice (Auto, Fast, Smart or one free model), what the
  conversation may do (Read only, Ask first or YOLO, see Ask), answer actions
  and Move to Agent. With no conversation open that choice is what the next
  one starts with and is `ask.approvals`; the line over the composer says it
  in words, and YOLO is drawn in the attention tone wherever it shows. A
  change Ask asks for is a request card like an agent's. An answer that was
  stopped or cut off says so under it.
- **Agents** opens on its Now board: what needs you in one sentence, a
  composer that starts a session in a project and worktree with a harness, then
  the sessions that need you, are working or finished recently. A board with no
  session lists the projects a session can work in and the way to add one;
  with no harness installed it lists the harnesses Rashin knows, each with its
  install page, in place of the composer. The harness picked for a session
  becomes `agent.harness`, so the board, the dialogs and Settings always agree,
  and so does the asking choice beside it (`agent.approvals`: ask before
  changes, ask every time, YOLO). Under the board's composer stands what the
  picked harness will run with (its own setup, see Agent). A session's
  composer carries the model, the mode and every option its harness offers,
  each as a select, and the asking choice; opening a session that never ran
  brings its harness up to learn them, with "Starting" in their place
  meanwhile (`pages/agent/controls.go`).
  Its list groups sessions by Needs you, Working, Done and Idle. A session
  shows its project, worktree, branch and harness in the header, the transcript
  (work as single lines that unfold, permission requests, plans, diffs), queue
  and steer, each turn's changes with Revert files, fork from here, and the
  Changes side panel.
- **The composer** is the same on every page (`internal/app/chat`): attach
  opens the system file chooser for files of any kind and falls back to a
  path field, Ctrl V also attaches an image the clipboard holds, Up and Down
  in an empty field walk the prompts sent in that conversation, `/` lists the
  commands the harness offers, and what is typed and not sent stays with its
  conversation. Asking again sends the question with what was attached to it.
  It has the keyboard when its page opens and gets it back when a menu in its
  row, the search field or a dialog closes. The selects in its row stand at
  their own widths and, when the row is too narrow for that, the widest give
  way first, each cutting its own label (`shareRow`).
- **A select's list** (`internal/app/look/picker.go`): a short list of plain
  names is a menu. One whose options carry a second line, or that has nine or
  more, opens as a list instead: the second line wraps and is never cut, a
  field at the top narrows the list as you type (every word has to match the
  name or the second line), the arrow keys and Enter pick, and it opens on
  the current choice. That is what makes a harness's hundreds of models
  usable.
- **A waiting request** is answered with its plaques, or from the composer:
  with nothing typed, Enter allows once and Escape refuses, and the composer's
  hint says so. Words in the field are a message, so Enter sends them and
  Escape does nothing. A key never answers a request that is not on screen or
  was drawn less than half a second ago (`chat.View.Asking` and `Answer`).
- **Answer text** (`internal/app/chat/inline*.go`): inline Markdown is parsed
  to spans (strong, emphasis, code, links, a web address written out) and set
  on lines by the app, one piece per unbreakable stretch, with the shaper used
  only to measure and draw. `*` marks emphasis wherever it touches a word and
  `_` only at a word's edge, so `XDG_RUNTIME_DIR` and `snake_case` keep their
  underscores. Lines break where Unicode allows and after any blank; a word
  longer than a line breaks inside itself and code that fits on a line stays
  whole. A table gives narrow columns their width and wraps the wide ones.
  Nothing an answer says is cut to an ellipsis. A paragraph that reads right
  to left is set as plain text, since spans set one at a time would come out
  in the wrong order. Only what is on screen keeps its set lines.
  An answer is set for reading (`chat/reading.go`): proof type on a 24 line,
  list items 8 apart. Code in a sentence is told apart by its face alone, at
  13 so it stands as tall as the words around it, and takes no room of its
  own: a plate with room either side pushed brackets and commas off the names
  they belong to, and on a palette whose stones stand close the plate did not
  show, only the gaps.
- **A command the answer offers to run** stands once (`chat/answer.go`): one
  slab with the command, why, how far it reaches and Run. When the text shows
  the same command in a block of code, the slab takes that block's place;
  otherwise it stands under the answer. **Run** runs it in the conversation,
  in the background (`chat.ActRunHere`, `session.prompt` with `run`): the
  command stands where a question would, its output shows under a Running
  line as it comes, the composer's Stop ends it, and Ask answers from what it
  printed. Asking that turn again runs the command again, and Run waits while
  a turn is running. A small terminal icon beside it opens the command in the
  user's terminal instead, and a command that needs a terminal (a password, a
  credential) has only that, as Run in terminal: it is shown in the terminal
  as if typed, runs from the home folder, says its exit status when it
  failed, and leaves the user's shell (`env.RunInTerminal`).
- **A running command** (`chat/work.go`): its line carries the Working flakes
  and, from three seconds on, how long it has run, so a silent minute does
  not read as a hang. While it runs the unfolded output shows its last twelve
  lines, and its first twelve once it has finished.
- **Subagents** (`chat/agents.go`): a call that runs subagents stands as its
  own box, never folded into a group of steps: "Running 3 subagents", how
  many are working, done or failed, and how long. One subagent shows its name
  and task in the head. Unfolded, each subagent is a row with its mark, name,
  task and steps, and unfolds to what it did, step by step, and its answer.
- **A finished turn** (`chat/fold.go`) folds its work behind one line, "Worked
  for 2 min 14 s" with its steps and thoughts, or "You stopped it after ..."
  when it was stopped, when that work shows more than one line; the answer
  stays open under it. While a turn runs, the mark at its foot says how long
  it has worked from three seconds on ("Working for 12 s"), so a quiet stretch
  between steps does not read as a hang.
- **A goal** (`pages/agent/goal.go`, `chat/goal.go`) stands in a strip over
  the composer with its turn count and Pause or Resume and Stop goal or Clear;
  the sidebar row says "Goal, turn N of M", and `/goal` is in the command
  list. Each turn the goal starts is drawn quietly, without the prompt Rashin
  wrote.
- **The agent's screen** (`pages/agent/screen.go`): the header's Screen action
  opens it in the side panel, and it opens there by itself when the agent asks
  for help. Its enlarge action shows it in place of the conversation, as large
  as the pane allows, until Back to the conversation. It shows the latest frame, what the agent did last, and what
  it asks; Take control turns the frame's outline to the attention tone and
  sends clicks, drags, the wheel, typing and chords to it in order, and Hand
  back returns it. A session whose agent waits on the user there is in Needs
  you.
- **A redraw is never lost** (`shell/redraw.go`): the toolkit ignores a
  redraw asked for while a frame is being drawn, so the last event of an
  answer, or a daemon coming back, could stay off screen until the pointer
  moved. A change that lands mid-frame is asked for again, every 20 ms and at
  most five times, until a frame has started after it; a hidden window that
  draws nothing is then left alone.
- **Marked text**: every paragraph, question and block of code can be marked
  with the pointer and copied with Ctrl C or from the message menu, which
  also quotes it into the reply (`chat.ActQuote`). A second
  press takes a word, read the way a terminal reads one so a path comes whole,
  or all of a piece of code. One stretch is marked at a time and only while
  the transcript holds the keyboard; Escape, or a press that marks nothing,
  gives the keyboard back to the composer (`chat.ActRelease`).
- **Projects** lists projects with their worktrees in the sidebar and opens
  on the project last shown. A project has the tabs Overview (worktrees with
  their status, setup, ports, sessions and actions), Graph (the commit graph
  with the details of what is selected beside it), Changes, Tasks (grouped by
  state, what needs you first) and Activity (the ledger in plain words).
- **Index** searches one index at a time, the one selected in the sidebar: a
  field that travels from the centre of the pane to its top with the first
  search, a choice between words, meaning or both, and results that unfold to
  their lines.
- **Opening a session**: the replay that opens a transcript and the events
  the session streams afterwards reach the store on different goroutines, so
  the store holds a session's live events from `Open` until its replay has
  been applied (`state.Store.Open`, `ApplyAttach`, `Opened`). Without that a
  replay from zero that landed late wiped the question just asked.
- **Settings** is a full page with a section index: Rashin (on and off,
  version, the log), Providers (each unfolds to its keys and says what it
  answered this week), Local API (the switch, the address, the key, the port),
  Ask (the model route, what a new ask may do, shell habits), Agents (the
  default harness, approvals, notices, the idle stop, the harnesses found with
  what each will run with, how to install the rest, your own harnesses), Teams
  (Tasks at once), Index, Appearance (zoom, reduce motion) and Keyboard (every
  shortcut). Its list runs to the pane's edge, so the wheel moves it from
  beside the column too.
  Project settings live with the project.
- **The desktop** is reached through `internal/app/desktop` only: the file
  chooser (XDG portal `FileChooser`), the clipboard's image (`wl-paste`),
  notices (`org.freedesktop.Notifications`) and starting other programs: the
  user's terminal and file manager through `ryoku-app terminal|files`, what
  opens a file or an address through `xdg-open`, and `rashin on|off`. The
  frame hands these to pages as `env.Desktop`; a render without a window has
  none, so it opens nothing.
- **In front** (`internal/app/desktop/front.go`): a desktop that floats this
  window opens whatever the app starts in the layer underneath, where it takes
  the keyboard and cannot be seen. So when this window floats, the window that
  appears after a terminal, a folder, a file or the file chooser was asked for
  is placed in front of it, centred on it and kept on its screen. The
  compositor is asked through `ryoku wm state` and `ryoku wm act
  window.place`, so the app names none, and without that door nothing is
  placed. A web address is left to the browser's own window; a floating
  Rashin says "Opened in your browser".
- **Notices**: while the window is not in front, a desktop notification says
  when an agent starts needing you, fails or finishes; pressing it opens the
  session. It follows the store and not the frames, and what already stood so
  when the app started is not announced. The switch is in Settings > Agents.
- **Reopening**: the window comes back to the agent session or project view
  it had open when it closed. A question is not kept; Ask opens on a new one.
- **Search**: the top bar field (Ctrl+K) finds sessions, projects, worktrees
  and commands; its results open under it at its width.
- **Keys**: Ctrl+N new, Ctrl+B the sidebar, Ctrl+\ the side panel, Ctrl+1 to 4
  the destinations (1 to 6 in Teams mode), Ctrl+T Teams mode and back, Ctrl+,
  Settings, Ctrl +, - and 0 zoom, Alt+Left and Right
  history. Escape closes the topmost thing the frame has open and is otherwise
  left to the page: it denies an open permission request. While a dialog is
  open the frame's shortcuts stand down.
- **Motion** follows `docs/lithic/motion.md`; an idle window draws no frames.
- **Off or unreachable**: one pane saying so with the next step; no retry
  loop faster than the client's 2 s reconnect.
- `rashin-app --demo --png out.png [--size WxH --mode dark|light --route ...
  --zoom N]` renders against an in-process demo daemon; with `--script` and
  `--shots` it plays a motion story and returns its frames side by side. The
  stories live in `internal/app/shell/testdata/motion`.
- **Driven like a person** (`internal/app/shell/stage_test.go`): the tests run
  the whole app against the demo daemon with a real input router and no
  window, find controls by the name a screen reader would say, and press and
  type. One of them walks every page in each window class and asks, for each
  control, what a press on it would be delivered to; a control that is drawn
  under something that keeps its presses fails it. A new control gets a name
  (`look.Named` where its label is drawn elsewhere) and a test that presses it.

Widgets the Lithic library does not have live in the app, built from Lithic
roles, tokens and kit parts: the frame's own parts in `internal/app/look`
(the folding sidebar's rows, the pane header, motion helpers) and the
conversation in `internal/app/chat` (composer, Markdown, work lines, the
transcript).

## The bar

Alt+Space opens the bar (`ryoku/shell/quickshell/shell/modules/ask/`). Its QML
is created when it opens and released 15 s after it closes. It talks to the
daemon over the socket (Quickshell `Socket`, newline-delimited JSON), connecting
when it opens and closing when it is released; when the socket is missing it
runs `rashin ensure` once and retries.

- **Ask**: one-shot asks with streamed text, answer actions, recent asks,
  cancel, Move to Agent.
- **Agent**: the most recent Agent session (or a new one), prompts,
  permissions, model choice, Open in Rashin.
- **Tools** (Stash) and **Web** are shell features and work whether Rashin is
  on or not.
- **Open in Rashin** runs `rashin open --session <id>`, which asks a running app
  to switch (`ui.open`) and raises it, or launches it. `rashin open --project
  <id or path> --view graph|changes|tasks|activity|overview [--worktree
  <path>]` opens a project the same way, and `rashin open --route <route>` a
  Teams mode place (see Teams, In the app).

## Packaging and delivery

`ryoku-rashin` ships `rashind`, `rashin`, `rashin-app`, the units, the usage
collectors and their timer, the desktop entry and icon, and the notices. It
depends on `ryoku-desktop` and `bubblewrap`, which team agents run in; no
harness is a dependency. The Ryoku vault, its wiki and the `ryoku` skill ship
with Ryoku core.

Every edition builds it from the committed `vendor/` tree (`-mod=vendor`):
PKGBUILD and `payload.sh` (Arch, CachyOS, Void), the RPM spec (Fedora), the
template (Void), the Nix package. The app needs Wayland, xkbcommon, EGL and
Vulkan headers at build time (`release/repo/build-toolchain.packages`).
Every lane carries bubblewrap: a dependency of the PKGBUILD, the RPM spec and
the Void template, and on the Nix package's `PATH`. The routines wake units
ship in every lane as well: `payload.sh` installs `rashind-routines.timer`
and `.service` for systemd (Arch, CachyOS, Fedora's staged package), the Nix
package installs them with its own paths, and Void has the runit user
service `void/init/user/rashind-routines` with a `down` file.

Service manifest rows: `rashind` (`rashind.socket`, user, on-demand), the
usage timer, and the routines wake (`rashind-routines`: the timer, or the
runit service, user, on-demand) with its worker row for
`rashind-routines.service`; `rashin on|off` owns the first two, the daemon
and `rashin off` the routines wake.

### From Rashin 1 and Prowl

`ryoku vault migrate` separately moves the Rashin 1 vault into Ryoku core. It
merges user and agent notes without overwriting conflicts and is run by the
doctor and update refresh whether Rashin is installed or not.

`rashin migrate` runs from `ryoku doctor` (once per installed package version)
and from `deploy.sh`, and does each step once:

1. Settings: `enabled` stays true only when it was true and not `optedOut`;
   `chatAgent` becomes `agent.harness`; `approvals` becomes `agent.approvals`;
   the rest is dropped.
2. Provider keys: when Prowl's state exists
   (`$XDG_DATA_HOME/prowl-agent/gateway/gateway.db` and `master.key`) and
   `sqlite3` is installed, keys for providers in Rashin's catalogue are decrypted
   and stored in Rashin's key store. Subscription logins are not imported.
3. Ask history: `$XDG_STATE_HOME/ryoku/rashin-asks.jsonl` becomes Ask sessions
   with origin `import`.
4. Old state: `rashin-chat/`, `rashin-ask/`, the old lock and pid files and the
   Rashin 1 and Prowl user units are removed. Vault migration remains the Ryoku
   CLI's job (see [vault.md](vault.md)).

Ryoku no longer ships Prowl or a gateway unit. `ryoku-rashin` stops depending on
the `prowl` package but never replaces or removes it: a box that uses Prowl as
its own tool keeps it (an orphaned dependency the user may remove). A harness
that was routed through Prowl keeps its own configuration; `rashin migrate`
reports any harness config that still points at the old gateway address so the
user can repoint it.

## Testing

- Unit tests per package, isolated by the XDG variables; no test touches the
  user's real socket, config or keys.
- `internal/teams` tests cover approvals, budgets, credentials, egress, mail,
  routines and tasks with a fake clock and the vault's key held in memory in
  place of the login keyring. `internal/sandbox` tests run `bwrap` for real
  and skip, saying why, where bubblewrap cannot run;
  `internal/daemon/agentsocket_test.go` checks what an agent's socket serves
  and what the main socket leaves to the app.
- Fakes: an OpenAI-compatible SSE server for the gateway, a scripted ACP agent
  for the harness runner, a small repository fixture for the index, the demo
  daemon for the app, and a private session bus that knows no service files
  for the app's file chooser and notices, so a test can never open the real
  chooser on the desktop it runs on.
- Sandbox: run `rashind` with all XDG variables under `/dev/shm/<run>/` and
  drive it with `rashin call` and `rashin events`.
- Live proof: Ask with a free provider, an Agent turn with an installed harness,
  the bar, the app, the index commands, a team task through a real harness
  with an approval answered in the app, and the zero-at-rest checks above in
  both the off and the absent states.

# Ryoku Rashin

Rashin (羅針, "compass needle") is Ryoku's optional AI system. It is one Go
daemon with three clients:

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
   Sessions live on disk and resume on the next prompt. One exception is asked
   for by name: a daemon that serves the local API stays up, because other
   programs hold its address.
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
| Daemon | `rashind` | Go, no cgo | Gateway, Ask, Agent adapters, sessions, index |
| Command line | `rashin` | Go, no cgo | Lifecycle, ask from a terminal, index queries, any method |
| App | `rashin-app` | Go + Gio, Lithic | Ask, Agent, Index, Settings |
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
  internal/app/         Gio client (shell, state, pages, chat widgets, desktop, demo)
  systemd/              rashind.socket, rashind.service
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
- **runit and anything else** (Void): no service. A client that finds no
  socket while Rashin is on starts `rashind` itself, detached (`setsid`, its
  own process group, stdio to the daemon log), then waits up to 3 s for the
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
  when no client is connected, no turn is running, no agent process is alive and
  no index job is running. The check is event driven (re-armed on state
  changes), never a poll. A daemon that serves the local API (see Gateway)
  drops its loaded indexes at that point and stays up.
- **Agent idle stop:** an agent process stops after `agent.idleStopMinutes`
  (default 10) without a turn, even while the app shows it. The next prompt
  starts it again and resumes the session (`session/load` when the harness
  supports it).
- **Shutdown** (`shutdown` method, `rashin off`, SIGTERM): cancel running turns
  (each ends with `turn_end` stop `cancelled`), stop agent processes, flush
  logs, close stores, remove the socket when the daemon bound it, exit 0.

### Commands

| Command | Does |
|---|---|
| `rashin on` | writes `enabled: true`; systemd: enables and starts `rashind.socket` and `ryoku-ai-usage.timer` through `ryoku-host svc --user enable --now`; runit: enables the `ryoku-ai-usage` service |
| `rashin off` | writes `enabled: false`; asks a running daemon to `shutdown`; disables and stops the socket, service and usage timer (runit: stops the usage service and kills a daemon left running through the lock holder's pid) |
| `rashin status [--json]` | enabled, running, version, counts; never starts the daemon |
| `rashin ensure` | exit 0 once the daemon answers `hello`; exit 3 when off; exit 1 on failure |
| `rashin migrate` | idempotent upgrade from Rashin 1 and Prowl (below) |

`ryoku doctor` converges the units to `enabled`: on means the socket and timer
are enabled, off means both disabled and no daemon running. It also runs
`rashin migrate` and removes the Rashin 1 and Prowl units.

### Zero at rest, checked

With Rashin off or absent, all of these hold, and the live proof checks each:

- `systemctl --user is-enabled rashind.socket ryoku-ai-usage.timer` reports
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
| `$XDG_DATA_HOME/ryoku/worktrees/<project>/<name>` | worktrees Rashin creates (default) |
| `$XDG_CACHE_HOME/ryoku/rashin/index/<id>/index.db` | one index (bbolt) |
| `$XDG_RUNTIME_DIR/rashin/rashin.sock` | the socket |
| `$XDG_RUNTIME_DIR/rashin/rashind.lock` | daemon lock |
| `$XDG_RUNTIME_DIR/rashin/config.lock` | serialises writes of `rashin.json` |
| `$XDG_RUNTIME_DIR/rashin/app.lock` | app single instance |

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
  "api":    {"enabled": false, "port": 3600}
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

Clients change settings only through `settings.set`; the daemon writes the file
atomically and broadcasts `settings.changed`.

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
`conflict`, `disabled`, `unsupported`, `internal`. `internal/protocol` holds
every method with its params and result types and every event; `rashin call
<method> [json]` and `rashin events` reach any of them.

Method groups: `hello status subscribe shutdown`, `session.*` (list, new,
attach, detach, prompt, cancel, permission, set_model, set_mode, set_option,
start, rename, delete, stop, handoff, queue, unqueue, steer, fork, read,
revert_turn), `ask`
(new session, attach, prompt in one call), `providers.*`, `models.list`,
`gateway.usage|api|api_key`, `harnesses.*`, `index.*`, `settings.get|set`,
`ui.open`, and
Work: `projects.*`, `worktrees.*`, `git.*`, `review.*`, `tasks.*`,
`activity.list`.

`subscribe` returns a snapshot (status, recent sessions, settings, providers,
harnesses, indexes, projects, tasks) and from then on the connection receives
the global events: `sessions.changed`, `session.info`, `settings.changed`,
`providers.changed`, `api.changed`, `harnesses.changed`, `index.progress`,
`index.changed`,
`projects.changed`, `worktrees.changed`, `tasks.changed`, `review.changed`,
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
| `user` | text, attached files (`images`: paths, of any kind), terminal context, handoff |
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
terminal, killed on release or session stop).

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
  code search commands over its folder. A session the agent loads back is not
  briefed again; one that had to start over is. The folder's repository is
  indexed when the session's agent is created (see Index), so the search is
  there when the agent looks.
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
  slab with the command, why, how far it reaches and Run in terminal. When the
  text shows the same command in a block of code, the slab takes that block's
  place; otherwise it stands under the answer. Run in terminal shows the
  command in the terminal as if typed, runs it from the home folder, says the
  exit status when it failed, and leaves the user's shell
  (`env.RunInTerminal`).
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
  what each will run with, how to install the rest, your own harnesses), Index,
  Appearance (zoom, reduce motion) and Keyboard (every shortcut). Its list
  runs to the pane's edge, so the wheel moves it from beside the column too.
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
  the destinations, Ctrl+, Settings, Ctrl +, - and 0 zoom, Alt+Left and Right
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
  <path>]` opens a project the same way.

## Packaging and delivery

`ryoku-rashin` ships `rashind`, `rashin`, `rashin-app`, the units, the usage
collectors and their timer, the desktop entry and icon, and the notices. It
depends on `ryoku-desktop`; no harness is a dependency. The Ryoku vault, its
wiki and the `ryoku` skill ship with Ryoku core.

Every edition builds it from the committed `vendor/` tree (`-mod=vendor`):
PKGBUILD and `payload.sh` (Arch, CachyOS, Void), the RPM spec (Fedora), the
template (Void), the Nix package. The app needs Wayland, xkbcommon, EGL and
Vulkan headers at build time (`release/repo/build-toolchain.packages`).

Service manifest rows: `rashind` (`rashind.socket`, user, on-demand) and the
usage timer (user, on-demand); `rashin on|off` owns them.

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
- Fakes: an OpenAI-compatible SSE server for the gateway, a scripted ACP agent
  for the harness runner, a small repository fixture for the index, the demo
  daemon for the app, and a private session bus that knows no service files
  for the app's file chooser and notices, so a test can never open the real
  chooser on the desktop it runs on.
- Sandbox: run `rashind` with all XDG variables under `/dev/shm/<run>/` and
  drive it with `rashin call` and `rashin events`.
- Live proof: Ask with a free provider, an Agent turn with an installed harness,
  the bar, the app, the index commands, and the zero-at-rest checks above in
  both the off and the absent states.

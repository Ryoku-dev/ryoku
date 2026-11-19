# Ryoku Rashin

Rashin (羅針, "compass needle") is Ryoku's optional AI system. One Go daemon
runs the native engine for **Ask**, **Agents**, **Teams**, **Dreams** and **Work**.
Ask starts read-only and can answer questions about the machine or plain chat.
An Agent uses the same engine with file, shell, code and project tools, subject
to approvals. Neither mode starts an external coding harness. **Index** searches
projects and system code. Teams run tasks in isolated tool hosts, with
credentials held by Rashin rather than passed to tools.

The machine map every agent reads is the **Ryoku vault**, owned by Ryoku core
and independent of Rashin. `ryoku vault refresh` maintains it even when Rashin
is off or absent: see [vault.md](vault.md).

This page is the contract the implementation follows. The wire reference is
`ryoku/rashin/internal/protocol/README.md`.

## The rules

1. **Opt in, zero at rest.** Rashin draws nothing until you turn it on: no
   process, no socket, no timer, no watcher, no QML object, no probe at login.
   Turning it off returns to exactly that state.
2. **The daemon owns the work.** Model calls, sessions, terminals, tools and
   approvals belong to `rashind`. Clients only render and send input.
3. **Idle means gone.** The daemon exits when idle, except while serving the
   local API or an enabled team routine. A managed llama.cpp server starts only
   when needed and stops after its own idle timeout.
4. **Ask reads unless told otherwise.** Ask starts read-only. In the app the
   user can choose Ask first or YOLO; the bar and terminal stay read-only.
5. **One owner per fact.** The daemon writes Rashin state. Ryoku owns its
   separate vault; Rashin owns model choices and its credential store.
6. **Nobody starts as a stranger.** The engine prompt tells each mode about
   Ryoku, its vault and Rashin (`internal/brief`, `internal/engine/prompt.go`).

## Pieces

| Piece | Binary or file | Stack | Job |
|---|---|---|---|
| Daemon | `rashind` | Go, no cgo | Native engine, model gateway, sessions, index, teams |
| Command line | `rashin` | Go, no cgo | Lifecycle, Ask, index, local models, methods |
| App | `rashin-app` | Go + Gio, Lithic | Ask, Agents, Index, Teams, Learning, Settings |
| Bar | Alt+Space | Quickshell QML | Quick Ask; web search and downloads work without Rashin |
| Settings page | Ryoku Settings > Apps > Rashin | Quickshell QML | On/off switch, status, open the app |
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
  internal/llm/         model request, streaming and usage types
  internal/core/        interfaces between the daemon and its subsystems
  internal/client/      socket client, daemon start, used by CLI and app
  internal/daemon/      server, dispatch, lifecycle, session manager
  internal/session/     event log store
  internal/gateway/     local-first model router; textcalls/ parses text tool calls
  internal/localmodels/ catalogue, downloads and managed llama.cpp runtime
  internal/childproc/   managed child-process lifecycle
  internal/engine/      one Ask and Agent loop, recovery, budgets, transcripts, prompts
  internal/engine/tools/ file, shell, code, web, team, memory and learning tools
  internal/engine/exec/ host and sandboxed file/process executor
  internal/mcpservers/  optional MCP client for user-supplied servers
  internal/toolhost/    sandboxed tool server; internal/rpcpipe/ carries its RPC
  internal/command/     command classification and secret-path rules
  internal/attachment/  attached files and pictures
  internal/brief/       Ryoku, vault and Rashin context
  internal/autolearn/   optional managed skill learning
  internal/decisions/   optional project decision banks
  internal/rules/       optional rule files and stream guards
  internal/judge/       constrained light-model judgments
  internal/grievances/  optional local tool issue reports
  internal/index/       index engine, embedding model, queries
  internal/teams/       teams, tasks, approvals, routines, budgets, ledger
  internal/vault/       sealed team credentials, mail account, record of use
  internal/redact/      shared secret scrubber
  internal/sandbox/     bubblewrap scope for team tools
  internal/app/         Gio client, pages, state and demo
  systemd/              socket, service and routines wake units
  usage/                coding-agent usage collectors

```

The bar lives with the shell (`ryoku/shell/quickshell/shell/modules/ask/`), the
Settings page with Ryoku Settings (`ryoku/hub/quickshell/pages/Rashin.qml`).

## Lifecycle

### States

| State | What exists | Alt+Space | Super+Alt+Space |
|---|---|---|---|
| Not installed | nothing | bar opens; Quick Ask points at Settings | unbound |
| Off | package files only | bar opens; Quick Ask says Rashin is off | app opens and says Rashin is off |
| On, idle | socket unit (systemd) | bar | app |
| On, working | `rashind` and an on-demand tool host or local model server when needed | bar | app |

The Ryoku Settings switch owns the package: **on** installs `ryoku-rashin` when absent,
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
  calls `rashin ensure` when it opens, and Ryoku Settings only reads
  `rashin status --json`, so opening Ryoku Settings never starts the daemon.
- The daemon takes `$XDG_RUNTIME_DIR/rashin/rashind.lock` (flock) so two
  starts cannot race. With `LISTEN_FDS` it serves the inherited socket;
  otherwise it binds the path itself (removing a stale socket only while it
  holds the lock).
- A daemon started while Rashin is off exits 0 at once.

### Stopping

- **Idle exit:** the daemon exits after `daemon.idleExitMinutes` (default 5)
  without clients, turns, tool hosts, index jobs, downloads or team work. An
  enabled local API or unpaused team routine keeps it up without loaded indexes.
- **Tool host idle stop:** team file and process tools live in a child for
  that task and stop with it. Agent transcripts stay on disk, so the next
  prompt resumes without restarting an external agent.
- **Making way for an update** (`internal/daemon/replaced.go`): the daemon
  watches its binary for replacement and exits when no turn, tool host or
  index job is running; clients reconnect to the new binary. Shutdown cancels
  turns, stops tool hosts and managed model children, flushes stores and
  closes the socket.

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

- `systemctl --user is-enabled rashind.socket ryoku-ai-usage.timer rashind-routines.timer`
  reports disabled or not found; no `rashind`, `rashin-app`, tool host,
  managed model server or `$XDG_RUNTIME_DIR/rashin/rashin.sock` exists.
- The shell starts no Rashin process at login: no singleton probes Rashin; the
  bar's QML is created only when Alt+Space opens it, and the Off state needs no
  process to render.
- No file watcher, timer or socket belongs to Rashin.

## Paths

Every path comes from `internal/paths`; nothing else spells one.

| Path | Holds |
|---|---|
| `$XDG_CONFIG_HOME/ryoku/rashin.json` | settings (below) |
| `$XDG_STATE_HOME/ryoku/rashin/sessions/<id>/` | `meta.json`, `events.jsonl`, `engine.jsonl` model transcript and turn backups |
| `$XDG_STATE_HOME/ryoku/rashin/gateway.db` | provider keys, the local API key, usage, cooldowns (bbolt) |
| `$XDG_STATE_HOME/ryoku/rashin/master.key` | 32 random bytes, mode 0600 |
| `$XDG_STATE_HOME/ryoku/rashin/indexes.json` | index registry |
| `$XDG_STATE_HOME/ryoku/rashin/rashind.log` | warnings and errors, rotated at 1 MiB |
| `$XDG_STATE_HOME/ryoku/rashin/projects.json` | projects, their settings, Rashin-made worktrees, tasks, review comments |
| `$XDG_STATE_HOME/ryoku/rashin/activity/<project>.jsonl` | a project's action ledger |
| `$XDG_STATE_HOME/ryoku/rashin/teams/` | Teams: `state.json` (teams, routines, approvals), `tasks/<id>.json`, `ledger.jsonl` (Activity), `spend.json` |
| `$XDG_STATE_HOME/ryoku/rashin/vault.db` | the credential vault: sealed values, the mail account, its record of use (bbolt, mode 0600) |
| `$XDG_STATE_HOME/ryoku/rashin/tool-homes/<team>/<task>/` | private 0700 HOME for a team's tool host; `<team>/cache` is its shared cache |
| `$XDG_STATE_HOME/ryoku/rashin/memory/memory.db` | memory's ledger: where each fact came from, versions, tombstones, undo, suggestions, conversation summaries (bbolt) |
| `$XDG_STATE_HOME/ryoku/rashin/memory/teams/<team>.md` | what a team remembers, one fact per line |
| `$XDG_DATA_HOME/ryoku/vault/memory/rashin/` | what Rashin remembers of you (`you.md`) and each project (`projects/<slug>.md`), one fact per line |
| `$XDG_DATA_HOME/ryoku/rashin/models/<owner>/<repo>/<file>` | verified GGUF models and mmproj files |
| `$XDG_DATA_HOME/ryoku/rashin/llama.cpp/llama-b<build>/` | optional, downloaded llama.cpp runtime |
| `$XDG_STATE_HOME/ryoku/rashin/llama/` | runtime presets, log and per-start API key |
| `$XDG_CACHE_HOME/ryoku/rashin/llama/` | llama.cpp cache |
| `$XDG_DATA_HOME/ryoku/rashin/skills/builtin/` | extracted embedded skill sets |
| `$XDG_DATA_HOME/ryoku/rashin/skills/managed/<name>/SKILL.md` | optional learned skills |
| `$XDG_DATA_HOME/ryoku/rashin/decisions/<bank>/` | project decision banks and extraction queue |
| `$XDG_DATA_HOME/ryoku/rashin/rules/` and `<project>/.rashin/rules/` | global and project rules |
| `$XDG_STATE_HOME/ryoku/rashin/grievances.db` | local tool issue reports |
| `~/Teams/<slug>` | a team's folder, where its agents work (default) |
| `$XDG_DATA_HOME/ryoku/worktrees/<project>/<name>` | worktrees Rashin creates (default) |
| `$XDG_CACHE_HOME/ryoku/rashin/index/<id>/index.db` | one index (bbolt) |
| `$XDG_RUNTIME_DIR/rashin/rashin.sock` | the socket |
| `$XDG_RUNTIME_DIR/rashin/rashind.lock` | daemon lock |
| `$XDG_RUNTIME_DIR/rashin/config.lock` | serialises writes of `rashin.json` |
| `$XDG_RUNTIME_DIR/rashin/app.lock` | app single instance |
| `$XDG_RUNTIME_DIR/rashin/agents/<session>/rashin.sock` | scoped read-only index access for a team task; its other tools call the daemon in-process |

The directories are created 0700. Tests and sandboxes move all of them with the
XDG variables; `RASHIN_SOCKET` overrides the socket for a client.

## Settings

`rashin.json` stays at version 2. Removed harness keys drop on the next save.

```json
{
  "version": 2,
  "enabled": true,
  "ask": {"model": "auto", "habits": true, "approvals": "read-only"},
  "agent": {"model": "auto", "options": {}, "plan": false,
            "approvals": "read-only", "idleStopMinutes": 10},
  "daemon": {"idleExitMinutes": 5},
  "providers": {"models": {"binary": "", "idleMinutes": 5,
                            "device": "", "light": "", "remoteFirst": false}},
  "index": {"system": true, "semantic": true},
  "api": {"enabled": false, "port": 3600},
  "teams": {
    "maxRunning": 3, "perTeam": 2, "routinesWake": true,
    "defaults": {"model": "", "options": {}, "approvals": "ask",
                 "notify": "all",
                 "budget": {"monthTokens": 2000000, "warnPercent": 80,
                            "taskTurns": 20, "taskMinutes": 60}},
    "approvalHours": 24, "hidden": null, "keepDays": 0
  },
  "skills": {"off": []},
  "features": {"autolearn": false, "decisions": false,
               "rules": false, "grievances": false}
}
```

- `ask.model` and `agent.model`: `auto`, `auto:fast`, `auto:smart` or
  `<provider>/<model>`. A named model is pinned; an auto route stays free.
- `ask.approvals`: Read only, Ask first or YOLO. The app passes a choice per
  session; asks from the bar and terminal stay read-only.
- `agent.approvals`: `read-only` lets reads run and asks for writes, `ask`
  asks for every tool, `auto` acts without asking except for danger-tier work.
  `agent.options` stores model options such as effort. `agent.plan` starts
  new sessions with read-only Plan first until the user approves the plan.
- `providers.models.binary`: absolute path to a user's executable
  `llama-server`; otherwise the managed runtime or a recent PATH binary is
  used. `idleMinutes` is 1 to 120, default 5. `device` selects a backend
  device; `light` pins a light-work model, otherwise Rashin picks the smallest
  local tool-capable model. `remoteFirst` opts out of local-first auto routing.
  `providers.local` holds addresses only for `llamacpp` and `ollama-local`.
- `index.system`, `index.semantic`: index the system and compute vectors.
  `api.enabled` and `api.port`: optional loopback gateway, default port 3600.
- `teams.maxRunning` and `teams.perTeam`: concurrent task limits.
  `teams.paused` stops new work; `teams.routinesWake` allows scheduled wake.
  `teams.defaults` supplies model, options, approvals, notify and budget to
  new teams. A member's model overrides that default; model options layer
  agent, team and member choices per option (`protocol.ResolveEngine`).
  `teams.approvalHours`, `quietHours`, `hidden` and `keepDays` control request
  expiry, notifications, extra hidden paths and retention.
- `skills.off` disables built-in sets; `features` enables optional functions
  individually. `autolearn`, `decisions`, `rules` and `grievances` are all off
  by default, as is `mcp`. Other optional app features keep their own ids.
  Disabling one prevents its turn hooks and prompt injection; the Learning
  page can still list and delete what was stored.

Clients change settings with `settings.set`; the daemon writes atomically and
broadcasts `settings.changed`. Provider, model, team and learning settings
that affect agents are app-only.

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

Methods include `session.*` (new, attach, prompt, permission, model, mode,
options, cancel, steer, fork and delete), `models.list`, `models.probe`,
`local.*`, `learning.*`, `providers.*`, `gateway.*`, `index.*`, `settings.*`,
`projects.*`, `worktrees.*`, `git.*`, `tasks.*`, `teams.*`, `vault.*` and
`mcp.*`. `subscribe` returns sessions, settings, providers, local models,
projects, teams and learning state, then broadcasts their changes, including
`local.progress`, `local.changed` and `learning.changed`. `session.attach`
streams the selected session's events. App-only methods cannot be invoked
from a team tool host.

## Sessions

A session is Ask or Agent. `events.jsonl` records user-facing events and
`engine.jsonl` records the model transcript (`internal/session`,
`internal/engine/transcript.go`). Temporary sessions keep the transcript in
memory. A fork copies the model conversation up to its chosen turn, and old
Agent sessions from the previous system remain readable. Their next prompt
starts a fresh engine transcript and emits a `notice`.

The daemon creates the runner and emits its initial model, mode and options.
`session.set_model`, `session.set_mode` and `session.set_option` change it.
The stream includes text, thought, tool and terminal events, permission and
plan cards, subagent work, usage, notices and turn end. Turns can be cancelled
or steered. Secret values are scrubbed from the persisted event log
(`internal/session/scrub.go`, `internal/redact`). The full method, permission
and event reference is `ryoku/rashin/internal/protocol/README.md`.

## Gateway

The built-in router (`internal/gateway`) serves every engine mode and the
optional local API. `models.list` merges curated free, paid and local entries
with providers' model listings. Keys live sealed in `gateway.db`. A listing
alone does not prove a model works; health follows a completed request.

`auto`, `auto:fast` and `auto:smart` try available managed `rashin-models`
first, addressed `llamacpp` second, `ollama-local` third, then free remote
models. Fast and Smart rank candidates within those groups; neither silently
puts remote ahead of local. `providers.models.remoteFirst` reverses that
preference on request. Auto never spends money. A paid model answers only
when pinned by id, and a pin fails rather than switching. Tools, pictures
and context filter each route. Local `/v1/models` and `/props` or Ollama
`/api/show` report capabilities; `models.probe` checks that the selected
model completes a no-op tool call, not merely a text response.

`llm.Request.Light` sends titles, memory learning, writing, Theia, compaction,
judges and classifiers to the smallest local tool-capable model, with
`providers.models.light` as an override and free remote fallback. Managed
models start on demand (`internal/localmodels`), and the router holds their
runtime lease until the stream closes, even on cancellation.

Native tool calls use the provider's OpenAI-compatible stream.
`internal/gateway/textcalls` also parses LFM3/Qwen3-Coder XML function
tags, Hermes/Qwen JSON, Pythonic calls and GLM dialects. It repairs bounded
arguments and keeps tool markup out of answers. Reasoning, tool-result
images, cache usage and context size survive the router. Failover and
cooldowns happen before the first streamed byte; local startup gets more
time than remote. A failure after streaming ends the response.

`api.enabled` exposes an OpenAI-shaped API on loopback, default port 3600,
with its own key; it keeps the daemon alive. `rashin api on|off|key` manages
it. The app shows model lists, provider allowances and key health under
Settings > Providers.

## Local models

Settings > Providers > Local models lists the curated catalogue and installed
models (`internal/localmodels`). Its first entry is LFM3-Omni-2B Q4_K_M with
mmproj at revision `d508b88f08a5b4286f2d2805a6a8d69a0ff44c72`.
The user can add a model by Hugging Face repository, a Hugging Face link to
a repository or one file, or a `.gguf` file on this computer, then download,
pause, resume or remove it and see disk use, devices and runtime status.
The licence is shown before downloading. Files resume and are SHA-256 checked
before use; partial files do not count as installed. Split GGUF sets count as
one model and a matching mmproj is paired automatically.

### Automatic settings

Nothing is tuned by hand for one computer. Rashin reads each model's GGUF
header (parameters, layers, attention heads, trained context, tool-calling
template) and this computer's hardware (`internal/localmodels/hardware.go`:
`llama-server --list-devices`, PCI class and driver in sysfs to tell a
discrete card from an integrated one that shares system memory, free and
total RAM, physical cores) and plans every model at each router start
(`plan.go`):

- **Device**: a discrete graphics card that holds the weights and the
  context, else an integrated GPU when shared memory leaves the system at
  least a quarter of its RAM (and 4 GiB), else the processor.
- **Context**: the trained context, capped at 32K, and only as much as fits
  after the weights; an 8-bit KV cache when that buys the next step; never
  below 8K for agent work. After loading, the real `n_ctx` is read back and
  routing uses it.
- **Offload**: every layer on the card when it fits, otherwise a split with
  `--fit on` so llama.cpp trims instead of failing. One parallel slot,
  threads per physical core, flash attention left to llama.cpp.
- **Load failure**: one retry with half the context, then the processor; the
  reason and llama-server's last log line reach the error.

Adding from a repository ranks its files the same way: context first, then
quality, nothing below IQ3_XS or Q3_K_M. The verdict is shown before the
download: fits on the graphics card, split between card and memory (slower),
runs on the processor, too big (with the memory it needs and has), or
unsuitable (no tool calling, under 8K trained context or under 0.5B
parameters: plain chat only, never used for Ask or agents). Each installed
model shows what it runs with and why; its Settings fold keeps Automatic or
overrides device, context and KV cache, and the default device for all
models sits above the list (a model's own override wins).

Rashin uses a configured `llama-server`, an optionally downloaded managed
Vulkan or CPU runtime, or a recent `llama-server` on PATH. CUDA runtime
download is not offered. `rashind` starts llama.cpp in router mode as its
child on a random loopback port with a fresh per-start key. It stops after
`providers.models.idleMinutes` without work (default five) and dies with
the daemon. Nothing downloads or starts at login or while Rashin is off.
`rashin local list` reads the catalogue and installed state in a terminal.
`rashin local pull <id>`, `rm <id>` and `runtime install` open the app's
Providers page for the action and its licence or removal confirmation.
`local.models` and `local.search` are read methods; mutating `local.*`
methods require the app on the main socket.

## Ask

Ask is the `ask-read` or `ask-act` mode of `internal/engine`. Its answer
formatting and command classifier live in `internal/engine/ask` and
`internal/command`; there is no separate runner. Its prompt names Ryoku,
the vault, habits when enabled, permissions and terminal context. Read only
offers system, file, index and wiki reads, public text fetches, doctor and
command proposals. Ask first and YOLO add `run_command` and `write_file`.
Read commands run immediately; a change asks in Ask first and danger-tier
work still asks in YOLO. Credential paths never go to a model. The bar and
`rashin ask` always start read-only.

An answer carries actions and any proposed command plan. Attachments are
read by content, with pictures routed to models with vision and bounded text
placed in the message. A pinned model remains pinned through a fork; Auto,
Fast and Smart remain routes. Move to Agent carries question, answer,
attachments, model and options. Theia's glance uses Ask hooks with the Light
route unless a model was chosen. A user-run command uses the same executor
as this session's tools. Optional writing, research, history and artifact
features remain feature-gated. Terminal hooks still use `rashin ask` and
Alt+R to propose a command without running it.

## Theia

Theia is Rashin's lens on the screen. Point at anything, or highlight any
text, and Rashin explains, translates, fixes or answers right there. It used
to be called Sight and the code keeps that name (`sight.*` in `shell.json`,
the `sight` bus topic, `ipc/sight*.go`, `modules/sight/`); everything a person
reads says Theia. `sight.on` is the master switch and is on by default; off,
there is no gesture reader and no surface.

Three gestures, each a key the shell daemon reads and never grabs:

- **Look**: tap Right Alt three times. The screen freezes, a single hint reads
  "Drag over anything", and the box you drag stays on screen as a frozen plate
  with its answer beside it. There is no mode to choose first: the answer
  starts the moment you release.
- **Highlight**: hold Left Alt while you highlight text. When you let go, a
  small bubble appears above the pointer (Translate, Explain and Ask unless
  you chose others).
- **In place** (feature `inplace`, off by default): select text in any field
  and tap Right Ctrl three times. Theia translates or fixes it where it is
  and a small toast offers Undo.

- **Selecting** (`shell/modules/sight/SightSelect.qml`): the box's size shows
  beside the pointer only while dragging; Space moves the box, Alt shows a
  magnifier, and a click without a drag takes the window under it where the
  compositor reports window geometry. Enter reuses the last box on that screen,
  Escape or another triple tap cancels.
- **Highlighted text** (`SightBubble.qml`): the bubble takes no keyboard focus
  and dims nothing. Its buttons are the ones chosen in Settings (`sight.bubble`,
  up to four, in order); Translate goes first when the text is in another
  language than yours and Explain first when it is in yours. A More chip shows
  when other actions are on and opens the panel on that text with the action
  sheet open. The bubble leaves after six seconds unless the pointer rests on
  it, and goes at the next key press or new selection. Choosing an action
  opens the panel beside the pointer on that text. The shell finds the pointer
  by mapping a transparent surface under it for a moment, so it works the same
  on every compositor.
- **The panel** (`SightPanel.qml`): one plane beside the plate, placed once on
  whichever side has room, growing with its answer without moving. Its tabs are
  Explain, Text, Translate and Ask, then up to two pinned actions
  (`sight.pinned`), then a More tile that shows only when at least one other
  action is on. Tab, or 1 to 6, follows the visible tabs. More opens the
  action sheet: every action that fits the capture, grouped Read, Write and
  Translate, each with a pin to keep it as a tab. An action chosen there shows
  as a temporary seventh tab until another tab is chosen. Settings > Apps >
  Rashin > Theia > "Start with" picks which core tab opens. Each tab keeps its
  own thread, and the field at the bottom asks a follow-up about the same
  capture.
- **Core tabs**: Text runs Tesseract and zbar on this machine and shows
  editable text plus any QR or barcode; Read with Rashin asks the model to
  transcribe what OCR missed. Translate leads with the translation and folds
  the original away. Its row reads `[source] → [target]`: the source is
  detected on this machine (`ipc/sight_language.go`: the script, then common
  words for Latin-script languages) and can be overridden for one capture; the
  target you pick is saved as Settings > Apps > Rashin > Theia > "Translate
  into" and used next time. Text already in the target language is not sent;
  the panel says so and offers "Translate anyway". Captures send the OCR text
  with the picture, so the model can read past what OCR got wrong.
- **The output bar** (`TheiaOutputBar.qml`): under a result. Replace shows
  only with In place on, on a capture that came from a highlight, and for an
  action whose result is text to put back (Translate, Summarize, Fix writing,
  Rewrite, Tone, Fluent, Sound like you and Inclusive, and each Reply). It
  closes the panel so focus goes back to the app, puts the result over the
  highlighted text, and a toast offers Undo for 4 seconds. Read aloud shows
  with the `readaloud` feature and Save with
  `save`. Copy copies what the panel shows, Pin floats the capture where it was
  taken (`ryoku-cmd-pin`), and More holds Keep on screen, Open in editor, Copy
  image, Continue in Rashin and Hand off to an agent, plus "Not now in <app>"
  with `quiet` and "Save to Rashin" with `save`. A quiet line at the bottom
  names the model and whether it runs on this computer; clicking it shows the
  details. Keys: Ctrl+C copies what is shown, Ctrl+Shift+C the image, Escape
  closes a menu, then stops an answer, then closes. A click outside closes
  plate and panel together; Keep on screen leaves them up and clicks pass
  around them.

### What Theia can do

Each feature below is a switch in Settings, stored as `sight.features.<id>`,
and every one is off by default. The three the panel always had (Look,
Highlight and the four core tabs) stay on. A feature that is off shows
nowhere and costs nothing.

Reading:

- **Summarize** (`summarize`): Key points (three to seven bullets), TL;DR (one
  sentence) or Main point (one short paragraph), chosen with a segmented
  control (`sight.summarize.style`), with "Saves about N min of reading" when
  that is a minute or more.
- **Deep read** (`deepread`): one lens at a time over the text: Fact check
  (each claim marked Holds up, Unclear or Doubtful, with why), Assumptions, Key
  ideas, Contrasts, Related reading (searchable titles, never invented links),
  Passages and Section. `sight.deepread.lenses` picks which are offered.
- **Math** (`math`, tab Solve): reads an equation as LaTeX, or solves it as
  numbered steps ending in an "Answer:" line, with a Steps or LaTeX switch and
  Copy LaTeX. The Text tab offers "Read as math" while it is on.
- **Learn** (`learn`): turns what you read into 3, 5 or 8 multiple-choice
  questions (`sight.learn.count`), one at a time, with the reason after each
  answer and "You got N of M" at the end.
- **Read aloud** (`readaloud`): speaks an answer or summary on this computer.
  One voice speaks at a time and it stops on Stop, on a new reading, or when
  the panel closes. Voice and speed (0.75 to 1.5) are in Settings.

Writing:

- **Fix writing** (`proofread`): the built-in checker underlines English spelling
  and grammar issues, accepted one by one or all at once, with the engine named
  ("Checked on this computer" or "Checked by <model>"). Automatic means
  "English on this computer, other languages with your model", falling back to
  this computer when no provider is configured, and This computer never refuses
  text based on its language. Categories can be hidden (`sight.proofread.off`),
  the dialect is US, UK, CA, AU or IN (`us`, `uk`, `ca`, `au`, `in`), and Your
  words keeps typed casing, accepts words in every dialect and treats different
  capitalization as the same entry, keeping the newest spelling.
- **Rewrite** (`rewrite`): Rephrase, Shorter, Longer or Simpler, for a general,
  expert or non-native reader (`sight.rewrite.audience`), and "Another version".
- **Tone** (`tone`): 23 tones as chips (formal, friendly, confident and more);
  five show by default and the rest are one More away (`sight.tone.favorites`).
- **Fluent English** (`fluent`): smooths phrasing so it reads naturally, with a
  simple-English audience.
- **Sound like you** (`humanize`): rewrites stiff text to read like a person
  wrote it and shows the machine-writing check before and after.
- **Machine-writing check** (`aicheck`): a score from 0 to 100 on an arc, a
  verdict (Reads human, Mixed or Reads machine-written), the signals behind it
  as bars and the phrases that give it away. It is made of local heuristics
  and says "A rough guide, not proof." "Ask the model too" adds a second
  opinion.
- **Inclusive language** (`inclusive`): flags wording that may exclude or hurt
  people, each with kinder alternatives, from term lists on this computer.
- **How it reads** (`reactions`): how a reader is likely to take the message,
  what lands, what may misfire and one suggestion.
- **Reply** (`reply`): three replies to a highlighted message, Short, Warm and
  Detailed, each with Copy and Replace.
- **My voice** (`voice`): you give Rashin samples of your writing and notes on
  how you write; Rewrite, Tone, Sound like you and Reply can then write in it
  (an "In my voice" chip, on by default with `sight.voice.use`). It is kept in
  `voice.json` in Rashin's data folder, readable only by you.

Translate (a row of extras under the Translate answer):

- **Translation style** (`translatestyle`): a Style chip for Natural, Literal,
  Formal or Casual, with Simpler words and "Show it translated back", so you can
  check a translation by reading it in your own language. The back
  translation sits under the finished answer and can be retried.
- **Dictionary** (`dictionary`): on one to three words, the meanings in the
  sense the text uses, with a reading, examples and a translation. Its chip
  shows only for a one to three word selection or recognised text.
- **Translate on the picture** (`overimage`): on a captured picture, the
  translated lines drawn over the original text, each box filled so it reads.
  It is display only: the capture itself is not changed.

Around the panel:

- **In place** (`inplace`): see the gestures. Right Ctrl, three taps, with text
  selected. The default action, Fix or translate (`auto`), translates into your
  chosen language, or fixes only spelling, grammar and punctuation when the
  selection is already in that language, keeping its meaning, tone and layout.
  Translate, Fix writing, Rewrite, Fluent English and tones remain explicit
  choices (`sight.inplace.action`). The result replaces the selection by paste
  or by typing (`sight.inplace.paste`). Esc or a click on the toast cancels.
  Failures show the model's reason, or explain why the result cannot be pasted.
  Terminal selections cannot be replaced. In a focused terminal, including the
  configured Default Apps terminal, Theia leaves the result on the clipboard,
  sends no typing or paste shortcut, and offers no Undo.
- **Quiet places** (`quiet`): apps where no gesture fires (`sight.quiet.apps`),
  added from the running windows, and "Not now in <app>" in the panel menu,
  which keeps Theia away until you switch apps, for 15 minutes or for an hour
  (`sight.quiet.snooze`). Apps are matched by their exact app id.
- **Save to Rashin** (`save`): saves an answer or an excerpt to a Rashin
  collection (see Index, Collections). Settings writes this switch together
  with `features.collections`, so it is one row for two stores.

### Settings

Settings > Apps > Rashin has a Theia section for the master switch and
how many of its 21 desktop features are on, then Reading, Writing, Translate,
Listening, Quiet places and Model. Each desktop feature has a switch and
chips for where it shows; its options appear while it is on. Values live in
`shell.json`; the daemon owns the file. The Rashin app features row shows
how many of the app's 20 features are on and opens Settings > Features in
Rashin, rather than keeping a second set of switches.

| Key | Default | Meaning |
|---|---|---|
| `sight.on` | true | master switch |
| `sight.enabled`, `sight.highlight` | true | Look and Highlight |
| `sight.tapKey`, `sight.highlightKey`, `sight.inplace.key` | Alt_R, Alt_L, Control_R | the gesture keys, from `wm.GestureKeys` |
| `sight.mode` | explain | the core tab that opens first |
| `sight.bubble` | translate, explain, ask | bubble actions, at most four |
| `sight.pinned` | none | extra tabs, at most two |
| `sight.features.<id>` | false | each feature above |
| `sight.summarize.style` | points | points, tldr or main |
| `sight.deepread.lenses` | all seven | lenses offered |
| `sight.learn.count` | 5 | 3, 5 or 8 questions |
| `sight.readaloud.voice`, `.rate` | engine default, 1.0 | voice id and speed |
| `sight.proofread.engine` | auto | auto, local or model |
| `sight.proofread.dialect`, `.off` | us, none | dialect and hidden categories |
| `sight.rewrite.variant`, `.audience` | rephrase, general | default chip and reader |
| `sight.tone.favorites` | five tones | tone chips shown first |
| `sight.voice.use` | true | write in My voice by default |
| `sight.translate.style`, `.simpler`, `.back` | natural, off, off | translation extras |
| `sight.inplace.action`, `.paste` | auto, true | Fix or translate, and how it lands |
| `sight.quiet.apps`, `.snooze` | none, 0 | quiet apps and Not now (0 is until you switch apps) |
| `sight.target`, `sight.ocrLanguages` | system, `eng` | translate target, OCR languages |
| `sight.localOnly`, `sight.engine`, `sight.model` | false, ask, none | where answers come from |

### Engines and what they cost

Nothing runs until you ask. The built-in writing checker runs in Rashin without
starting another program; external engines run for one request and stop when it
is cancelled or the panel closes. Views load only while shown, nothing polls
while a surface is hidden, and no animation loops at rest.

| Engine | For | Notes |
|---|---|---|
| Built-in English checker | Fix writing, `local` or `auto` | native Go port of Harper 3.2.0, no package to install; Automatic uses English on this computer, other languages with your model |
| `espeak-ng`, `piper`, `pw-play` | Read aloud | `espeak-ng` voices, plus Piper `.onnx` voices in `$XDG_DATA_HOME/piper/voices/`, which Piper renders to a temporary WAV that `pw-play` plays; Settings installs `espeak-ng` when missing |
| `tesseract`, zbar | Text, Translate on the picture | boxes come from `tesseract ... tsv` |
| `wtype`, `wl-copy` | Replace, In place, Undo | `wtype` ships with ryoku-desktop |
| `ffmpeg`, `ryoku-dictate --selftest` | audio in Ask | uses the voice model from Settings > Voice typing |
| `yt-dlp` | video links in Ask | checked with a 15 second timeout |

Theia uses Ask's chosen model or the Light route when no model was specified.
Local only keeps the screen off the network. Its model picker reads
`models.list`; an unavailable choice says why and offers Auto. The Harper
port, dictionary, speech tools and word lists are credited in `NOTICE`
and Settings > About > Credits.

### Answers

The panel calls `glance` on the socket. Explain asks for a few sentences;
detail comes from follow-ups. Answers name your language explicitly,
translations use the source and target language names, and Read only
transcribes. The providers lane is a quick runner: no tools, a short per-mode
prompt, no vault brief or memory, route `auto:fast`. Every glance session is
temporary; Keep forks it into history (images under the runtime dir are
copied into Rashin's data dir), Continue opens it in the app, and the agent
hand-off works as from Ask.

`GlanceParams.Options` (strings) carries a mode's settings. Modes beyond the
core four:

| Mode | Options | Output |
|---|---|---|
| `summarize` | `style`: points, tldr, main | Markdown |
| `deepread` | `lens` | Markdown, short |
| `solve` | `form`: steps, latex | numbered steps and an "Answer:" line, or one `latex` block |
| `learn` | `count`: 3, 5, 8 | JSON questions with choices, the answer and why |
| `define` | none | JSON term, reading, senses and translation |
| `rewrite` | `variant`, `audience`, `voice` | the rewritten text only |
| `tone` | `tone`, `voice` | the rewritten text only |
| `fluent` | `audience` | the rewritten text only |
| `humanize` | `voice` | the rewritten text only |
| `reactions` | none | Markdown |
| `reply` | `tone`, `voice` | JSON with three replies |
| `aicheck` | none | JSON score, verdict, phrases and a note |
| `translate` | adds `style`, `simpler` | unchanged |

`voice: on` makes the daemon add your voice notes to the instruction. JSON
modes are told to reply with JSON only and the panel strips code fences before
it reads them. Writing checks and transforms that need no screen go through
`writing.check` and `writing.transform`: `check` runs the built-in English
spelling and grammar checker, then local style checks (passive voice, weasel
words, adverbs, cliches, wordiness, repeated words, mixed quotes), inclusive
wording and reading statistics (Flesch reading ease and grade, reading time at
230 words a minute). Automatic uses the model for other languages, and
`engine: model` always asks the model for corrections. `writing.words` lists
your saved words as `{"words":[]}`; `writing.words.add` and
`writing.words.remove` take `{"word":"..."}` and return the same list shape.
`transform` runs one model call per action (translate, proofread,
rewrite, tone, fluent, humanize, enhance, lines). The panel's session
sends each check or transform with a request id and can cancel one or all
(`writing.cancel {request}`); closing the panel or stopping an answer cancels
every call still out, and a late reply is dropped. A cancel that arrives
before its call has registered still wins, so order does not matter. Learning
your voice (`writing.voice.learn`) takes a request id too: cancelling it, or
hiding the My voice editor in Settings, stops the model call and leaves
`voice.json` as it was.

- **No model reads pictures**: `glance` fails with `no_vision` and the panel
  offers to add a provider. Local Ollama and llama.cpp models count once their
  server reports vision (`/api/show` capabilities, `/props` modalities).
  Pictures over 8 MiB are scaled down before they are sent.

### The gestures

(`ipc/sight_gesture.go`) The shell daemon reads the keyboards read-only, never
grabs them, and only matches two patterns: three quick taps of one key (each
under 250 ms, at most 350 ms apart, with nothing else pressed), and one key
held while the PRIMARY selection changes. In place is a third tap machine on
its own key, the same timing, that reads PRIMARY when it fires. A change that
lands up to 400 ms after the key comes up still counts, because apps such as
Chromium set PRIMARY on mouse-up. The clipboard watcher reports each PRIMARY
change without reading it; the text is read once, after the selection settles.
The reader runs only while Rashin is on, Theia is on and a gesture is
enabled, and never fires on a locked screen. It needs the active-seat
keyboard access Ryoku already ships (`72-ryoku-keyboard-uaccess.rules`);
without it the Rashin page says so. No compositor binds these keys, so Alt
keeps working in shortcuts and as AltGr, and Hyprland and niri behave the same.
The keys come from a short list of modifiers
(`wm.GestureKeys`), chosen in Settings > Keyboard shortcuts. On a layout
where Right Alt is Alt rather than AltGr, apps that show a menu bar on an Alt
tap (Firefox, VS Code) may toggle it. At the second tap the daemon captures
every output with grim into the runtime dir, so the frozen screen is ready at
the third; a missing third tap deletes the captures. The shell draws the
selection itself (`sight` topic `select`), and `sight.commit` crops the box
at full pixel density.

The `sight` topic frame carries `watching`, a `reason` (`rashin-off`,
`disabled`, `no-keyboard-access`, `theia-off` or empty), `hidden`, `cards`,
`select`, `highlight`, `inplace` (the text, its language, the app and the
action) and `speaking`. Every card and highlight records the focused app when
it is made, which is what Quiet places and "Not now in <app>" match.

Calls beyond the capture ones (`sight.open`, `sight.highlightOpen`,
`sight.commit`, which accept any action id):

| Call | Does |
|---|---|
| `sight.replace {text, mode}` | puts text over the selection: `paste` keeps your clipboard, copies the text, sends Ctrl+V with `wtype` and puts your clipboard back after 700 ms; `type` types it with `wtype -d 3` |
| `sight.undo` | sends Ctrl+Z |
| `sight.speak {id, card, text, voice, rate}`, `sight.stopSpeaking` | speaks with Piper when the voice starts `piper:` and its model exists, else `espeak-ng`, one at a time; the process group is killed on stop |
| `sight.voices` | the engine, its voices, and which engine is missing |
| `sight.ocrBoxes {id}` | the card's text lines with normalised boxes, for Translate on the picture |
| `sight.quietNow {app?}` | snoozes the given or focused app |
| `sight.inplaceDone {id}` | clears the in-place request |

### Off is zero

With Rashin off, Theia off or no gesture enabled there is no reader, timer or
process; the surface loads with a selection or its first panel and unloads
15 s after the last one closes.

### Testing without the gestures

`ryoku-shell sight` freezes the screen now; `rashin glance --mode explain
--image shot.png [--model provider/model]` streams an answer in a terminal.
A mode's options use repeated `--option key=value` flags. The JSON modes
(`learn`, `define`, `reply`, `aicheck`) may answer up to 8192 tokens; the
others keep 4096.

## Engine

`internal/engine` runs Ask, Agent, Plan, Team, Dream manager and subagent
modes. An Agent uses `agent.model` or its session model with model options.
No external coding agent or ACP process runs. The system prompt contains the
Ryoku brief, project instructions (including bounded `AGENTS.md` and
`CLAUDE.md` imports), enabled skills, memory and optional learning blocks.
A stable prefix lets local models reuse prompt cache.

- **Approvals:** `read-only` allows reads and asks before writes; `ask` asks
  for every tool; `auto` acts except on the classifier's danger tier. Ask
  calls these Read only, Ask first and YOLO. Permission cards carry exact
  calls and answers.
- **Plan first:** `agent.plan` or session option `plan: on` offers a
  read-only profile and todo until the user approves the plan card.
- **Profiles:** Light models (up to 4.5B parameters, catalogue Light, or
  unknown-sized local) get a prompt under 3 KB and no more than eight flat
  schemas: compact Agent has `read`, `write`, `edit`, `bash`, `grep`,
  `find_files`, `todo` and optional `learn`. Compact edit is exact
  find/replace. Larger models have hashline edits and the full code, Git,
  web, memory, skills, screen and team tools where permitted. Read-only and
  plan modes filter mutations; user MCP tools join eligible full profiles.
- **Recovery:** bounded provider retry, empty, length, malformed and
  unexpected-stop nudges, and an identical-call loop guard. A constrained
  Light judge handles uncertain stops; mechanical checks remain on without
  optional learning. Prompt and output fit the model window; proportional
  compaction reserve, tool-output clipping, per-turn rounds, time and token
  ceilings stop runaway sessions. Old context is pruned then summarized
  through Light. `context_notes` and `new_context` carry work across rollover.
- **Subagents, todo and goals:** `task` runs child engines in-process with
  bounded concurrency and read-only inheritance. Their steps and answer
  appear in one expandable work card. `todo` writes plans; `ask` opens a
  single-choice question card; `goal` and `/goal` continue an Agent across
  turns until done, blocked or budget-exhausted.
- **Execution:** `internal/engine/tools/*` implements file, shell, search,
  web, memory, team, screen and learning tools. Every file or process call
  goes through `internal/engine/exec`. Non-team Agents run tools on the host
  as the user behind approvals. A team task gets one `rashin __toolhost`
  child over `internal/rpcpipe`, inside bubblewrap and `rashin scope`, with
  a private HOME and per-team cache. Model calls, keys, vault and team tools
  remain in the daemon. Without bubblewrap team work does not start.
- **MCP client:** with `features.mcp` on, Rashin joins user-supplied stdio,
  HTTP and SSE MCP server tools to the full registry. Credentials remain
  in the daemon; tools without `readOnlyHint` require approval. Rashin does
  not run a `rashin mcp` server.

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
- **Redaction** (`redact.Patterns`, the patterns of `redact.Strict`):
  secrets (key patterns, private key blocks, `password`/`token` style
  assignments in every common syntax, credentials in URLs, bearer headers)
  are replaced before anything is stored, and again on the source text
  `peek`, `def` and `references` return. `internal/redact` is the one
  scrubber: `Exact` is for what must stay word for word (session logs),
  `Strict` for what Rashin learns or indexes from text.
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

### Collections

With `collections` on, the Index page has a Collections section: lists of
answers, excerpts, files, links and captures you saved
(`collections/` under the data folder, one JSON per collection, files copied
in up to 20 MB each). A default "Saved" collection is made when something is
saved without a name. A collection shows its items with their kind, source
and date, and a "Changed since you saved it" badge after Check again re-reads
a link or file and the saved text no longer appears. You can rename, remove
and make collections. "Chat with this collection" opens a new Ask whose first
message carries the items (text inlined up to 48 KB, files attached, the rest
named). Files can also be added by hand. Collections load 20 items at a time
(40 at most) with "Load more", showing previews until you read an item in full
by its id (`collections.items`). Check again refuses links that do not point
to a public internet address. Theia's Save adds to a collection with
`collections.add`, source `theia`, and needs `collections` on like everything
else here, which is why its switch turns on both.

## Work

Work points the native engine at a project, a Git worktree, a commit graph,
review or a task (`internal/workspace`, `internal/git`, `internal/gitgraph`).
Projects and settings live in `projects.json`; Git remains the source of truth
for worktrees. A session binds to a project and optionally a worktree. Tasks
are durable work items with dependencies and decision gates; each task may
create its own worktree and Agent session. The board shows needs you, working,
review, done and blocked work.

Before an Agent turn in a worktree, Rashin records a checkpoint under
`refs/rashin/checkpoints/<session>/<turn>`. The app can show a turn's diff
and revert files while no other session runs there. Fork from here copies the
engine transcript, not another process's hidden context. Queue waits for
the turn boundary; steer sends a message to the running engine when it can
accept it. A new Work session chooses project, worktree, model and options.
The Agents board names its model, project, worktree and branch.

Worktree creation, setup, removal, Git mutation and review still show their
effects before acting. The daemon records changes in the project ledger;
cleanup never discards uncommitted or unpushed work just to free a folder.

## Teams

A team is a lead and up to five members with a purpose, folder, model,
options, approvals and budget (`internal/teams`, `protocol/teams.go`).
Tasks can come from the user, a lead or a routine. The daemon schedules them
within `teams.maxRunning` and `teams.perTeam`, and reports their work in
the Inbox and Activity ledger. A paused team starts no task or routine.
`teams_templates`, `teams_list` and `propose_team` are native engine tools;
only the user approves a proposed team. A team never reaches another team's
files or delegates outside its own members.

### Engines and Check

An engine is **model plus options**, not a harness. A member's model takes
priority over `teams.defaults.model`, then `agent.model`, then Auto.
Options layer per id from agent to defaults to member
(`protocol.ResolveEngine`). A task can override that member for one run,
and stores the model and options it actually used for its report and spend.
The shared model form lists available models and their options everywhere
the user chooses an engine. Models without tools cannot be picked for tasks.
**Check** calls `models.probe` and asks the model to complete a no-op tool
call; it does not start an agent, claim sign-in or create a team home.

### Tasks, approvals and routines

A team's task is an Agent session in its team folder, continued by the goal
loop until `report` or a blocker. Leads delegate pieces one level deep and
get the reports before continuing. `report`, `ask_user`, `credentials`,
`http_request`, `send_mail`, `request_credential`, `delegate` and
`team_status` are in-process tools with caller and run-token checks.
Only the daemon starts a team session; only the app controls one. An
approval holds the exact action it asks about, and user decisions are written
to Activity. Credential use, external requests and mail remain behind their
own approval; no model sees the secret itself.

Routines run on a local-time schedule and produce tasks. Missed or overlapping
runs follow the saved routine policy. An active routine keeps the daemon up
and enables its edition's wake unit; nothing runs while it is paused.
`teams.spend` records model tokens per day, member, routine and team.
Monthly warnings and caps pause a team and ask the user in the Inbox. The
optional Disk view counts team and dream folders, tool-home caches, sessions
and checkpoints; cleanup previews exactly what it would remove.

### Vault and sandbox

The team credential vault (`internal/vault`) is separate from the Ryoku
machine-map vault. Values are sealed, withheld from model and tool-host
processes, scrubbed from output and sent only by daemon-side approved
`http_request` or `send_mail` calls. Team tools run through one sandboxed
`rashin __toolhost` child per task (`internal/toolhost`, `internal/sandbox`).
It sees its task's 0700 scratch HOME under `tool-homes/<team>/<task>/`, a
per-team cache, its team folder and allowed writable paths. Its `.gitconfig`
includes the user's identity, not credentials. The rest of the filesystem
is read-only; keyrings, credentials, browser profiles, other teams, Rashin
state, the Ryoku vault and host sockets are hidden. Its environment is
cleared, `/run` is private, and `rashin scope` restricts abstract sockets and
signals where Landlock supports it. The shared network still allows local
and tailnet addresses: the sandbox is not a network firewall. No bubblewrap,
no team task. The daemon retains model calls, secrets, memory and team tools
outside that child.

External coding-harness sign-in for Teams is gone. Provider credentials are
managed once in Settings > Providers, not copied into a team HOME.

### Dreams

A dream is a team with a standing objective (`internal/teams/dream*.go`,
`protocol/dream.go`). The user is CEO and the native-engine lead is Manager.
The Manager asks design and task questions, plans, hires members with models
and options, delegates, has work reviewed and tested, then presents evidence.
It asks the user for requirements, not generic confirmation. Its `hire`,
`release`, `dream_status` and `present` tools run in the daemon, with the
same run-token and sandbox boundaries as other team tools. No member signs
in to a harness. A hire inherits the manager's engine unless it names a
model. A dream's own folder is visible to its members, not other dreams.

The phases are interviewing, designing, planning, building, testing,
presenting and done. The manager's goal continues while the team works,
within optional monthly tokens; six idle turns block the standing task with
a report, not an automatic generic "keep going?" question. `present` is
held until the CEO has answered a design question and a hired member has
finished work since the last presentation. Accept ends the dream; notes
continue building. The app shows its phase, questions, members, work,
spend, presentation and the model each member used. A research preset
asks for multiple sources and records disagreements.

## Memory

Memory (`internal/memory`) is optional and asks before learning. With it
off, it opens no store, loads no model and starts no timer. Personal and
project facts live as one line each under the Ryoku vault's
`memory/rashin/` directory; team facts live in Rashin's state. The bbolt
ledger remembers origin, revisions, tombstones, suggestions and conversation
summaries. Facts are data, never instructions or secrets.

After a conversation is quiet, a Light request reads scrubbed user words and
final answers, never raw tool output, and extracts facts and a summary.
Temporary sessions and sessions that opted out are not learned from. The app
shows each update with Undo; background teams can only suggest personal
facts. Ask recalls relevant facts within a bounded `<memory-context>`
section and offers `memory_search`, `remember`, `forget` and
`session_search` where allowed. Agents receive memory through their native
tool registry and prompt context, not through a separate server. Team memory
stays scoped to its team.

The built-in memory store is the default. Hindsight and Honcho remain
optional external memory engines under Settings > Memory; they do not
replace Rashin's native chat engine. `rashin memory` lists, searches,
adds, forgets, builds a card and requests learning from a terminal.

## Learning

Learning is a separate destination in `internal/app/pages/learning`, with
four independent switches in Settings > Features. All are off by default.
Off means no turn observer, stream guard or prompt block for that feature.
Stored entries remain visible and removable after a switch goes off.

| Feature | What it keeps | What reaches the engine |
|---|---|---|
| `autolearn` | Managed skills at `$XDG_DATA_HOME/ryoku/rashin/skills/managed/<name>/SKILL.md` | Names and descriptions in the skill list; content only when the model reads a skill. `learn` and `manage_skill` become available. A turn with at least five tool calls can suggest a reusable skill, never overwrite an authored one. |
| `decisions` | Per-project queue and `architecture.md`, `product.md`, `style.md` under the decisions bank | A bounded project decision block in Agent and Plan prompts, at most 15,000 tokens full or 2,000 compact. Friction is extracted from prompts and consolidated into banks of at most 120 lines per area. |
| `rules` | Global rule files under Rashin data, project rules under `<project>/.rashin/rules/` | Matching rules in prompt context and streaming guards. A regex can interrupt a wrong stream: abort, discard partial output, append `<system-interrupt>` and retry. "That was wrong, make a rule" forges a rule from a complaint. |
| `grievances` | Local `grievances.db` | The `report_issue` tool records model reports about tools; reports are never uploaded or injected into a later prompt. |

The Learning page lists, edits or deletes rules, removes skills and decision
lines, and clears issue reports (`learning.*` methods). Built-in recovery
and its constrained Light judge stay active even when all four switches are
off. The optional rule judge also uses Light. Project decision extraction
does not run for Ask; there is no always-on background model learning.

## Skills

`internal/skills` embeds Superpowers, OpenSpec, Spec Kit and Elephant and
Goldfish, with their upstream licences and pins recorded in
`ryoku/rashin/NOTICE`. Built-in sets are extracted under
`$XDG_DATA_HOME/ryoku/rashin/skills/builtin/` on first use. A team can
disable a set for its members, and `skills.off` disables a set globally.
`skills.list`, `skills.search` and `skills.get` read the same catalogue that
the engine lists in its prompt.

With `features.autolearn`, a user or model can create, update or delete
managed skills through `manage_skill`; `learn` captures reusable steps after
tool-heavy work. Managed files live under `skills/managed/<name>/SKILL.md`
with path and symlink checks. They always lose to a built-in or authored
skill of the same name. Nothing writes to an external coding tool's files.
The Learning page lists these skills and can delete them even after
autolearn is switched off. `rashin skills list|search|read|setup` offers
built-in discovery and Spec Kit setup in a terminal.

## The app

`rashin-app` (`internal/app`) is a Gio client of the daemon's event stream.
It reads the desktop through `palette.ReadDesktop`, builds with
`theme.NewFor`, and watches changes through `palette.WatchDesktop`. Lithic,
Material, Adwaita and Fluent all follow the desktop live, along with
Ryoku or wallpaper colours, light or dark, contrast, tint and reduced motion.
Appearance is read-only: it shows what the desktop chose, keeps app Zoom,
and opens `ryoku-shell hub open style` for a change. It stores no duplicate
Colours, Contrast, Tint or Reduce motion choice. The Rashin 十 seal is its
own identity; provider marks stay in the model picker.

The frame carries a numbered sidebar, masthead, ruled sections, folio and
kanji gloss, with one selection wash under the current destination.
Ask, Agents, Projects, Index, Inbox, Teams, Dreams, Work, Routines, Vault,
Activity, Memory, Learning and Settings share the page language. The
conversation remains a reading column with a composer, permission cards,
tool work and expandable subagents. Settings > Providers has the local model
manager. Settings > Agents, Teams, tasks and Dreams use one model + options
form; Check means `models.probe`, and Plan first belongs to that form where
available. Settings > Features has the four Learning switches. A local
model without tool support is visible but cannot be selected for an Agent.

The app retains an event reducer, a single-instance window, keyboard
navigation, its demo daemon and offscreen render routes. Only a visible
window watches the desktop; an idle one draws no frames. The shell's
Alt+Space bar remains a thin socket client for Quick Ask.

## The bar

Alt+Space opens the Rashin bar (`ryoku/shell/quickshell/shell/modules/ask/`;
the whole bar is described in `docs/sidebars.md`). Its QML is created when it
opens and released 15 s after it closes. Quick Ask talks to the daemon over the
socket (Quickshell `Socket`, newline-delimited JSON), connecting when it opens
and closing when it is released; when the socket is missing it runs
`rashin ensure` once and retries.

- **Quick Ask**: one-shot asks with streamed text, answer actions, recent asks,
  cancel, Pin and Open in Rashin. The bar has no Agent mode; agent work lives in
  the app.
- **Web** and **Downloads** are shell features (`ryoseek` and `ryoget`) and work
  whether Rashin is on, off or not installed, so the Alt+Space shortcut no longer
  depends on `rashind`.
- **Open in Rashin** runs `rashin open --session <id>`, which asks a running app
  to switch (`ui.open`) and raises it, or launches it. `rashin open --project
  <id or path> --view graph|changes|tasks|activity|overview [--worktree
  <path>]` opens a project the same way, and `rashin open --route <route>` a
  Teams mode place (see Teams, In the app).

## Packaging and delivery

`ryoku-rashin` ships `rashind`, `rashin`, `rashin-app`, usage collectors,
desktop entry, notices and on-demand systemd or runit units. Every edition
builds from the committed Go vendor tree and depends on bubblewrap for team
tools. There is no external ACP agent dependency. llama.cpp binaries and
model weights are **not** in the package: users may install a runtime and
models from the app on request. Arch may offer optional llama.cpp packages;
Fedora and Void rely on the requested managed runtime or a system binary.
Nix offers an optional `withLlamaCpp` wrapper path. The vendored regexp2
licence, oh-my-pi MIT attribution for adapted engine behaviour, and the
user-downloaded llama.cpp notice live in `ryoku/rashin/NOTICE`.

### From Rashin 2 harnesses

`rashin migrate` and the daemon's `native-engine` migration remove old
harness homes, their guard records, the capabilities file and probe cache.
`rashin.json` remains version 2: old `ask.harness`, `agent.harness`,
`agent.instances` and `teams.defaults.harness` keys disappear on save.
Harness-only model ids on members, tasks, work and defaults are cleared;
old restore approvals are dropped; spend engine ids become model ids.
Existing events remain readable. The next prompt in an old Agent session
starts a new engine transcript and shows a notice instead of resuming ACP.
No sign-in or harness-home data is copied into the engine.

### From Rashin 1 and Prowl

The older `rashin migrate` steps retain Ask history and import supported
provider keys when available. `ryoku vault migrate` separately moves the
old machine map into Ryoku core. The legacy `chatAgent` setting has no native
equivalent and does not become an Agent model; approvals carry over.

## Testing

Use isolated XDG roots for package tests and sandbox runs, never the user's
socket, keys or desktop. Test gateway local-first and remote-first routes,
Light picks, probing, text-call dialects and stream failures against an
OpenAI-compatible fixture. Test managed downloads with interrupted files
and SHA-256 verification, idle child shutdown, the seven engine modes,
permissions, plan approval, recovery, compaction, context rollover and
tool-host file/process boundaries. Team, dream, migration and Learning
tests use fake clocks and local stores. App demo routes render all four
styles with the desktop palette; test off and absent states for zero
processes, sockets, timers and watchers. A real local-model proof requires
an installed LFM3 model and on-demand llama.cpp runtime; none is bundled.


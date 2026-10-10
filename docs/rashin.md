# Ryoku Rashin

Rashin (羅針, "compass needle") is Ryoku's optional agent OS: a machine-generated
knowledge vault, a local daemon with a web dashboard, and a one-click Hermes
agent setup. It gives any coding agent (Hermes, Claude Code, codex, opencode,
omp/pi) an exact map of the system instead of burning tokens rediscovering it.
Everything is local, off by default, and enabled from Ryoku Settings under
Advanced.

## What it is, and is not

It is:

- **Optional.** The `ryoku-rashin` binary ships with the desktop but stays inert
  until you enable it. Optional means not running, not absent.
- **Local.** The daemon binds `127.0.0.1` only. WebSocket upgrades verify the
  Origin host is localhost. No auth, because the surface is single-user local.
- **Off by default.** Nothing runs, indexes, or wires until you flip the gate.

It is not:

- **An MCP server.** Markdown files are the interface every agent already
  speaks. MCP is a possible v2.
- **Remote.** No listener leaves the loopback interface.
- **A bundled LLM.** Hermes brings its own provider; you pick one during setup.

## The vault

The vault is the knowledge base every agent reads and writes, at
`~/.local/share/ryoku/rashin/` (respects `XDG_DATA_HOME`).

| Path | What it holds |
|---|---|
| `AGENTS.md` | The entry contract, read natively by codex, opencode, and omp |
| `CLAUDE.md` | A symlink to `AGENTS.md` for Claude Code |
| `system.md` | Generated: hardware, kernel, drivers, displays |
| `desktop.md` | Generated: the Ryoku map (configs, owners, reload commands) |
| `packages.md` | Generated: package sets, versions, update state |
| `ryoku-repo.md` | Generated: the Ryoku source tree map, pre-indexed and shipped |
| `user.md` | Generated: where this user's config diverges from the shipped baseline |
| `habits.md` | Generated: this user's directories, tool stack, and shell rhythms (feeds both ask lanes) |
| `ownership.md` | Generated: who owns each config path and where a change belongs |
| `logs.md` | Generated: where the logs live and the one command that gathers them |
| `memory/` | Agent-writable; Hermes `MEMORY.md` and `USER.md` live here |
| `journal/` | Agent-writable dated notes, one file per day |

**Fence markers.** Every generated file is fenced between
`<!-- rashin:generated:begin -->` and `<!-- rashin:generated:end -->`. A reindex
rewrites only the content inside the fence; anything a user or agent adds outside
it survives. `AGENTS.md` is written from a template only when absent, then owned
by the user and agents.

**Write rules for agents.**

- Generated files (`system.md`, `desktop.md`, `packages.md`, `ryoku-repo.md`,
  `user.md`, `habits.md`, `ownership.md`, `logs.md`) are read-only. Do not edit
  inside the fence; a reindex overwrites it.
- Read `desktop.md` before searching the filesystem or guessing paths. It names
  where every config lives, which binary owns it, and how to reload it.
- Changes listed in `user.md` are the user's own choices; never revert them to
  shipped defaults without being asked.
- Write durable notes to `memory/` and dated notes to `journal/YYYY-MM-DD.md`.
- Before editing any file, run `ryoku owner <path>`; it says who writes the file
  and where the change belongs. `ownership.md` is those rules for the whole box.
  Never edit a path it calls `ryoku`, `generated`, or `store`.
- When something breaks, run `ryoku-rashin logs <app>` first and read `logs.md`;
  gather before you change anything.

Reindex triggers: daemon start, `ryoku-rashin index`, a 6h timer, the
dashboard's reindex button, and `ryoku update` (both channels reindex after
configs land, so agents see the new system immediately). The user layer also
reindexes on its own: the daemon fingerprints the live `~/.config` every two
minutes and rewrites `user.md` when it drifts.

## The pre-indexed source map

`ryoku-repo.md` maps the monorepo that produced the system: layout with file
counts, key entry points, and the docs list. The installed target has no
checkout, so the map ships as a snapshot:

- **Packaged:** the `ryoku-rashin` PKGBUILD runs `ryoku-rashin repo-index` over
  the exact release tree and installs the result to
  `/usr/share/ryoku/rashin/ryoku-repo.md`. A system update replaces the
  snapshot with the new release's, and the post-update reindex folds it in.
- **Dev checkout:** `ryoku/shell/deploy.sh` writes the same snapshot to
  `~/.local/state/ryoku/rashin-repo.md` on every deploy.
- **Live regeneration:** with `RYOKU_RASHIN_REPO` pointing at a checkout,
  reindex regenerates the map from the tree instead of copying a snapshot.

## The user-owned changes layer

`user.md` diffs the shipped base config (`/usr/share/ryoku/config`, the tree
`ryoku materialize` lays down) against the live `~/.config` by content hash,
and lists three classes: dedicated user-override files (`hypr/user.lua`,
`kitty/user.conf`, `fish/user.fish`, `hypr/monitors_user.lua`), shipped files
the user edited in place, and shipped files the user removed. Agents treat
everything listed there as the user's own choices, distinct from Ryoku
defaults. On a dev checkout without the base tree, the layer degrades to a
note saying the diff is unavailable.

## Ownership and logs

Two generated docs and one command make the wired agents Linux- and Ryoku-aware
when they act, not just when they read.

`ownership.md` is the machine's ownership map: the rules in plain words, the
overlay path and its current forks, the user-override files and what each is for,
the generated files, the tool stores and their writers, the seeds, and the
Ryoku-owned trees as a compact table. Its body is `ryoku owner --map`; a single
path is classified by `ryoku owner <path> [--json]` (see `docs/cli.md`), which
names the class, the writer, and the path a change should edit instead. Until the
CLI carries the verb, the doc falls back to the rules in prose.

`logs.md` records where the logs live on this box, discovered at index time: the
Ryoku user units and their journal commands, the running Quickshell instances
and their `qs log` lines, the compositor's journal and runtime log (resolved
through the window-manager seam, never a hardcoded compositor), ryogami's log
files, the doctor report and update log, `/var/log/pacman.log`, and coredumps.

`ryoku-rashin logs <target>` (also `rashin logs <target>`) gathers all of it for
one target into a bounded markdown bundle, each section headed by the exact
command or file it came from. `<target>` is a component alias (`shell`,
`wallpaper`, `hub`, `idle`, `audio`, `portals`, `updates`, `compositor`,
`rashin`) or any program name; for a program it pulls the matching units, the
journal by `_COMM=`, coredumps, OOM kills, the app's own `*.log` files, and its
package's recent pacman transactions. Every probe is read-only, short-timeout,
and never needs root; `--since` (default 6h) and `--lines` (default 60) size the
window.

The `AGENTS.md` contract carries these as two rules every wired agent reads:
before you edit a file, run `ryoku owner <path>` and edit where it points, never
a `ryoku`, `generated`, or `store` path; when something breaks, run
`ryoku-rashin logs <app>` first, read `logs.md`, check the doctor report and
recent package changes, prefer a reversible fix, and record what you did in
`journal/YYYY-MM-DD.md`. The `ryoku` skill's `troubleshoot.md` is the full
playbook.

## The ryoku skill

Rashin ships an agent skill, `ryoku`, so any agent finds the desktop's safety
rules and command catalogue the way it finds a hub- or agent-grown skill, not
only through the vault pointer block. It lives in the repo at
`ryoku/rashin/skills/ryoku/` (`SKILL.md`, `gui.md`, `bar.md`, `plugins.md`,
`feature.md`, `troubleshoot.md`); the package
installs it to `/usr/share/ryoku/skills/ryoku`, and a dev deploy resolves the
checkout copy through the repo pointer.

`SKILL.md` covers when to use it, the vault-first rule, the safety split (lead
with `ryoku owner <path>`, never edit a shipped file; a user override goes to
`~/.config/ryoku/user_edits` or a
command), the command catalogue (`ryoku`, `ryoku-shell`, `ryoku-hub`,
`ryogami`, `ryoku-rashin`), the decision framework, and worked examples.
`bar.md` is the QS Bar and dock guide; `plugins.md` is the plugin contract and
the `ryoku plugin` CLI; `feature.md` is the ladder for a feature the desktop
does not have: `ryostore catalog` first, then the machine's own catalogues,
and only then a new plugin; `troubleshoot.md` is the break/fix playbook (gather
logs with `ryoku-rashin logs`, diagnose, fix through the owning command, verify,
roll back). The vault's `AGENTS.md` and `desktop.md` point at the same ladder,
so an agent that only reads the vault still climbs it.

`ryoku-rashin wire` symlinks the skill dir into every agent's skills directory:
`~/.agents/skills/ryoku`, `~/.claude/skills/ryoku`, `~/.codex/skills/ryoku`,
`~/.omp/agent/skills/ryoku`, `~/.hermes/skills/ryoku`, and each
`~/.hermes/profiles/*/skills/ryoku`. `~/.agents` and `~/.hermes` are created;
the rest are wired only when the agent's home already exists. `unwire` removes
only the symlinks that point at the skill dir, and `status --json` reports a
`skillWired` flag per agent. The skill dir resolves in one order:
`RYOKU_RASHIN_SKILLS`, then `/usr/share/ryoku/skills`, then
`<repo>/ryoku/rashin/skills` via `~/.local/state/ryoku/repo`. The doctor's
rashin reconciler re-runs `wire` whenever a link is missing, so an update keeps
the skill in place.

## The daemon: `ryoku-rashin`

One Go program (module `ryoku-rashin`), stdlib plus one dependency
(`github.com/coder/websocket`) for the chat and vitals sockets. It follows
`ryoku-shell` conventions: atomic writes, `RYOKU_*` env overrides, single
instance via a flock. The gate and port live in `~/.config/ryoku/rashin.json`
(respects `XDG_CONFIG_HOME`):

```json
{ "enabled": false, "port": 3600 }
```

Subcommands:

| Command | Job |
|---|---|
| `serve [--if-enabled]` | HTTP and WebSocket on `127.0.0.1:3600`, embedded dashboard. `--if-enabled` exits 0 immediately when the gate is off (the autostart path) |
| `index` | Regenerate all vault maps: `system.md`, `desktop.md`, `packages.md`, `ryoku-repo.md`, `user.md`, `habits.md`, `ownership.md`, `logs.md` |
| `logs <target> [--since <dur>] [--lines <n>]` | Gather everything relevant to `<target>` (a component alias or a program name) when it broke: units, journal, coredumps, OOM kills, the app's own logs, and its package history, in one bounded bundle. Read-only, no root; always exits 0. Also `rashin logs <target>` |
| `fix doctor [finding]` / `fix tip <id>` / `fix app <name> [what happened]` | Fix with AI from anywhere: opens your agent harness with the problem as its first message, in this terminal when run from one, in a new terminal window otherwise (see "Fix with AI" below). Also `rashin fix doctor`; `rashin fix the wifi` stays a plain ask |
| `repo-index <root> [out]` | Build the Ryoku source map from a checkout; used by the PKGBUILD and `deploy.sh` |
| `ask <question>` | One-shot quick ask, built for the launcher's `\` prefix: POSTs to `/api/ask` and pipes streamed `@working`/`@perm`/`@answer` markers to stdout. `ask --recent` prints the resume history as JSON; `ask --cancel` stops the running turn. See "Quick asks: two lanes" below |
| `setup` | One-click actuator: install Hermes, connect it through Prowl, wire, enable |
| `wire [agent]` | Apply vault pointers to all detected agents, or one named agent |
| `unwire [agent]` | Remove vault pointers, keeping the file |
| `status [--json]` | Report Rashin, Prowl, vault, Hermes, and wiring state |
| `enable [--at-boot]` / `disable` | Start or stop Rashin and its Prowl gateway. With systemd, `enable` starts `ryoku-rashin.service`, which wants the bound Prowl unit; `--at-boot` adds user lingering. Without systemd the daemon manages both detached processes |

Rashin runs as `ryoku-rashin.service`, a systemd user unit installed under
`/usr/lib/systemd/user` by the package and under `~/.config/systemd/user` by
`deploy.sh`. Prowl has a separate `ryoku-prowl.service` with no install target
and listens on port 8788 unless `RYOKU_PROWL_PORT` overrides it. Both ship off.
Starting or enabling Rashin starts Prowl; stopping Rashin stops
Prowl too. The `enabled` gate in `rashin.json` remains the source of truth, and
the detached fallback starts and stops both processes when no systemd user
manager is available.

The dashboard serves on `http://127.0.0.1:3600`. Its localhost API includes
status, vitals, system inventory, theme, wallpaper, vault, memory, skills,
harnesses, doctor, fixes, quick-lane settings, and the two chat WebSockets.
`GET /api/status` reports both Rashin and Prowl. For GET, POST, PUT, PATCH, and
DELETE, `/api/prowl/{rest...}` proxies these authenticated gateway groups:
`keys`, `providers`, `profiles`, `fallback`, `models`, `usage`, `logs`,
`logins`, `signin`, `settings`, `health`, `catalog`, `free-tier`, `cache`,
`update`, `projects`, `setup`, and `code`. It never exposes `/v1` to the
browser. Code requests use Rashin's source mirror by default. The earlier
`/api/prowl`, `/api/prowl/search`, `/api/code/*`, and `/api/providers` routes
are retired. Vitals come from `/proc` and `statfs`, with GPU data from
`nvidia-smi` when present.

## The two lanes

Rashin holds two conversations with the agent, and each has one purpose.

- **The Ryoku lane** is the machine agent. The Needle works on THIS machine:
  it has the vault, the `ryoku` skill (with `wm.md` for the window-manager
  seam and `build.md` for building things the Ryoku way), the wiki, prowl over
  the Ryoku source, and the approval modes Ask / Reads run / All run. The
  Alt+Space bar, the `rashin` terminal command and the companion window's
  first sheet all live here, and so does the quick path below.
- **The Chat lane** is a plain conversation with the harness (Hermes). No
  machine map, no Needle persona, no quick path: the harness answers as
  itself, with its own SOUL and skills. The console's Chat sheet is its only
  surface. Approvals still govern its tool calls; the mode is one daemon-wide
  setting that every surface on both lanes shows and may change.

Each lane is its own hub with its own agent process, spawned when a surface
first joins it (`GET /ws/chat?lane=ryoku|chat`; an absent lane is the Ryoku
lane, so the shell and the terminal built before lanes kept their meaning).
The agent learns what a lane is for through the one system prompt ACP has: the
context file at the session's cwd. The Ryoku lane runs in the vault, whose
`AGENTS.md` the daemon generates (who the Needle is, the one rule, the skills,
the wiki, the code index); the Chat lane runs in a bare directory
(`$XDG_STATE_HOME/ryoku/rashin-chat`), so Hermes, which reads `AGENTS.md`
from the cwd only, finds nothing to load. A lane's history drawer lists the
sessions opened in its own cwd.

### Quick asks: the fast path

A launcher ask does not always need the full agent. `/api/ask` keeps the quick
path inside the Ryoku lane:

1. **Fast lane.** Rashin posts each ask to Prowl's authenticated
   `/v1/chat/completions` gateway. The selected route defaults to `auto`, which
   uses the active routing set. `auto:smart`, `auto:fast`, `auto:cheap`,
   `auto:reliable`, `auto:balanced`, `auto:efficient`, and
   `auto:<lowercase set name>` select a capability or named set without tying
   Rashin to a provider. The bounded loop keeps read-only tools for machine facts,
   files, the Prowl code index, and URL reads. Work that needs broader tools,
   edits, or an interactive browser escalates.
2. **Session lane.** Escalated asks continue in the connected harness with its
   full toolset. Rashin pre-warms the resident session so this path avoids a
   cold start.

Both paths write into the Ryoku transcript, so continuing with the agent keeps
the full exchange. `GET /api/quick` reports the available Prowl routes and
gateway readiness; `POST /api/quick` selects one. The shell Ask bar renders the
same route list. The only stored setting is the route:

```json
{ "quick": { "route": "auto" } }
```

### Action chips

The `@answer` payload carries an `actions` array: entities the daemon found in
the answer text and verified against this machine. The launcher renders each as
a chip that does the obvious thing, so an answer is a launch point, not a
dead end:

| Kind | Detected as | Chip does |
|---|---|---|
| `file` | a path that exists and is a file | opens it in `nvim` (a kitty window) |
| `dir` | a path that exists and is a directory | opens it in the file manager |
| `url` | an `http(s)` link | opens it in the browser |
| `cmd` | a backtick span whose first word is on `PATH` | copies the command |
| `color` | a hex color, shown with a live swatch | copies the hex |

Plus a COPY chip for the whole answer, CONTINUE WITH THE AGENT, and OPEN RASHIN. The
answer text itself is selectable for mouse-copying a fragment. Nonexistent
paths and non-runnable backtick spans are dropped, so a chip never lies.

### Continue while it works, and cancel

While the agent is still working, three options sit under the pulsing strip:
**CONTINUE WITH THE AGENT** opens the bar's Agent mode, where the same turn is
streaming live (the daemon runs each turn on a background context, so it keeps
going even after the launcher closes), **OPEN RASHIN** raises the companion
window (see "The Rashin app"), and **CANCEL** stops it. Escape cancels a
working ask; the daemon interrupts both the fast lane and any session-lane
turn.

### The header drawers

Two icon buttons sit on the Ask bar's header, beside the `RASHIN // ASK` mark.
The **history** drawer lists the recent chats (the shared session's stored
sessions, newest first; picking one switches the chat to it) and the recent
asks (the `\resume` history; picking one recalls its answer without a model
call). The **model** drawer switches the two halves of inference: the fast
lane's provider (`ryoku-rashin backend`, "Follow Hermes" clears the override)
and the agent's chat models (a pick rides the live session and every chat
surface follows). `Ctrl+Shift+H` and `Ctrl+Shift+M` toggle the two drawers
without the mouse; with one open, Up/Down move its selection and Enter picks,
and Escape closes the drawer before it closes the bar.

### `\resume`

Typing `\resume` lists recent quick asks (persisted at
`$XDG_STATE_HOME/ryoku/rashin-asks.jsonl`, newest first). Picking one recalls
its stored answer instantly, chips and all, with no model call. Every completed
ask, from either path, is recorded there. The Ask bar's history drawer and the
Rashin app's Ryoku sheet read the same file through the same CLI.

## The Rashin app

The quick bar is one surface; the companion window is another. `rashin-app`
(`ryoku/apps/rashin-app/`) is a small GTK3 + WebKitGTK window that hosts the
Rashin console the daemon serves: it opens like a normal window
(Super+Alt+Space, the launcher, `rashin-app`, or the Ask bar's OPEN RASHIN
chip), is single-instance (a `GtkApplication` id plus the `ryoku-summon`
flock), shows a paper boot page until `/api/ping` answers, and then loads
`http://127.0.0.1:3600/#/ryoku`. It is a client of the daemon, never a second
brain, and it carries no UI of its own: the console is one Svelte app
(`ryoku/rashin/web/`, see its README) that is also the dashboard a browser
sees on the same port. Links to other origins open in the default browser,
notifications are granted, and the window remembers its size. Its icon is
Rashin's own seal (`ryoku/assets/brand/rashin-mark.svg`, linked as
`ryoku/apps/rashin-app/logo.svg` so the package and the dev deploy install the
same file as `rashin-app` in the hicolor theme); the console's favicon and the
Ask bar's header use the small cut of the same mark, see `docs/ui-ux.md`.

| Sheet | What it holds |
|---|---|
| Ryoku | The machine agent (the Ryoku lane) as a workspace: the sessions pane with the recent asks, the transcript (user plates, the Needle's replies with thinking folds, tool rows with input/output peeks and file diffs, inline approvals), the composer with slash commands, image attachments, a Quick / Agent switch and a beam while the agent works, and the inspector (session, model, usage, tools, commands) |
| Chat | The same workspace on the Chat lane: a plain conversation with the harness, nothing about the machine in front of it |
| Wiki | The shipped guides for someone new (Linux basics, the desktop, Hyprland in Lua, niri in KDL, Quickshell QML, the Go tools, Rashin), rendered at the reading measure with deep links |
| Overview | The wallpaper hero with the desktop clock, the live vitals strip, the code card led by measured token savings with a prowl search, the health band with Fix with AI, and the vault index card |
| System | The machine as a home server: services, timers, cron, containers, sockets, processes, filesystems, the Doctor tab, deterministic tips; copy, never run |
| Vault | The grouped tree, the rendered document at a reading measure, deep links, reindex |
| Memory | The provider tiles, the force graph of the vault's notes, the activity heatmap, the Hermes session history |
| Skills | One tab per harness, grouped and counted |
| Prowl | Overview, providers and keys, Rashin routing sets, activity, indexed projects, connected harnesses, and their toolkit |
| About | What Rashin is, the system manifest with live facts, and the shortcuts |

Everything the console shows is the daemon's answer: each lane's sheet is a
projection of that lane's `/ws/chat` stream and the same reducer the Ask bar uses
(`ryoku/shell/.../lib/chatstate.js`, aliased into the web build), so a turn
started anywhere is live everywhere, and every other sheet rides the HTTP API
above. The console's own state is UI-only (pane collapse, drafts, the last
vault file). It wears the desktop's live palette from `GET /api/theme`,
retinting while open.

## In the terminal

The launcher's `\` ask has a sibling on the command line: the `rashin`
command. `rashin take me to the fastfetch config` answers from the same brain
and drops a ready-to-run command on the fish prompt; `rashin scan Documents
for pngs and move them to Pictures` returns the one-liner (it knows the
directory is `Pictures`, from `habits.md`). It never runs anything itself, the
buffer is the confirmation, and every command carries a danger tier
(read/write/system/danger). It shares the daemon, the vault, and the ask
history with the launcher and the Ask chat, so `\resume`, `rashin --resume`,
and "continue in chat" all see one conversation. Repeated asks become saved
recipes (`rr-<name>` fish abbreviations). Full design and UX in
`docs/rashin-terminal.md`.

## The dashboard

The same console, in a browser: `http://127.0.0.1:3600` serves the Svelte app
built from `ryoku/rashin/web/` (Svelte 5, bits-ui for every headless control,
the Libraries.dev effects for the Needle's face, the thinking orbs and the
composer's beam). The build output is committed under
`ryoku/rashin/backend/web/dist` and `go:embed`-ed, so `go build` alone ships
it and no node runs on an installed box; the fonts ship as subset woff2 inside
the bundle; nothing is fetched from a CDN. The dashboard is a Ryoku surface,
not a product with its own costume: it speaks the Hub's paper and ink
(`docs/ui-ux.md`) and wears the desktop's live palette. `GET /api/theme`
resolves the Material roles the way `Tokens.qml` does (a named scheme, then
the wallpaper, then the signature default), and the page retints within 15
seconds of a wallpaper or scheme change. Emphasis is inversion (a bone plate
for the active sheet, file, or segment), colour is data, and the 力 seal stays
vermillion. Fraunces sets titles, Space Grotesk the language and numerals,
Space Mono the tracked labels and paths, Noto Sans CJK JP the kanji gloss
beside every sheet name.

Navigation is three floating islands, like the default QS Bar: the seal, the
sheets, and the daemon/hermes/prowl lamps with a clock. The Overview opens on
the wallpaper that is on screen right now (`GET /api/wallpaper` serves the file
`~/.local/state/ryoku-wallpaper` names; a live wallpaper plays muted while the
sheet is visible), with the desktop clock's numerals and a sysmon readout over
it. Motion is small and purposeful (a sheet eases in, the numerals rise once,
bars sweep) and yields to the OS reduced-motion setting and to
`theme.motion.reduce` in `shell.json`.

| Panel | Content |
|---|---|
| Overview | The live wallpaper with the clock, host, kernel, uptime, and a CPU/memory/disk/GPU sysmon readout; a health band when Ryoku's health check found something a person must decide, with Fix with AI; then the code intelligence card led by measured token savings from the Prowl index and the system summary card |
| System | The machine as a home server: services (running/stopped/user), timers (firing and dormant), cron/anacron/at, docker containers, listening sockets with reach, top processes, filesystems, the Doctor tab (`ryoku doctor --json`, read-only: every finding that needs attention, advisory notes folded away), and deterministic tips. Commands copy to your clipboard; Fix with AI on a tip or finding hands it to the agent. The sheet itself never runs anything |
| Vault | Grouped tree (maps, memory, journal; the agent-facing source mirror collapsed), rendered markdown, reindex button, generated-file badges |
| Memory | Provider tiles (builtin or external, with Obsidian vault detection), the 2D force graph of the vault's notes and their references with a data-driven legend, a 26-week activity heatmap, and the Hermes session history read from `~/.hermes/state.db` |
| Skills | One tab per installed harness: Hermes skills grouped by category with origin counts (bundled, hub, agent-grown) and the enabled toolbelt grouped into families; every other harness lists the skills it carries, grouped by origin when long |
| Prowl | A left rail for Overview, Providers, Routing, Activity, Projects, Harnesses, and Toolkit. Providers holds keys and sign-ins; Routing holds Prowl sets plus Rashin's Quick and Chat routes; Harnesses connects each detected CLI and shows its ledger |
| About | What Rashin is, the system manifest with live facts, quick start, a command crib (`hermes -h`, `prowl overview`), and the privacy note |

### The Ask chat

The GUI chats are Chat mode of the Alt+Space Ask bar and the Chat page of the
Rashin app, both live views of the shared agent session over `/ws/chat`.
Thinking streams in the open while the agent works and
then folds to a line the reader can reopen; each tool call is one row with a
peek at its output; approvals sit inline on the row that asked, governed by the
read-only auto-approve switch (`approvals` in `rashin.json`, `read-only` by
default or `ask`); and a searchable model picker switches the model live.

### Fix with AI

Wherever Ryoku already knows something is wrong, one button hands it to the
agent: every tip on the System sheet, every finding on the Doctor tab (and "Fix
all" for the lot), the Overview's health band, the Hub's Updates page after an
update whose health check found issues, and `ryoku doctor` itself, which offers
`ryoku-rashin fix doctor` when Rashin is on. From a terminal,
`rashin fix app firefox it crashes when I open a PDF` does the same for any app.

A fix opens in the user's own agent harness, in a real terminal, not in a chat
panel: the harness's own interface already streams its thinking, shows each
command and its output, asks its own approvals, and switches models. The harness
is the chat agent picked in Rashin when it can open a repair (Oh My Pi, Hermes,
or Claude Code), otherwise the first of those installed. It starts in the vault,
so the machine map is its working directory, with the Needle persona as extra
system prompt (Hermes, which takes none, gets it at the top of the message) and
the problem as its first message: a one-line `Fix with AI: <problem>` and a brief
that names the problem, the commands most relevant to it, and how to work:
gather the evidence with `ryoku-rashin logs`, check `ryoku owner` before editing
any file, explain the cause, ask before anything destructive or anything that
needs sudo, verify, say how to undo it, and write the findings to the vault
journal. Hermes cannot start interactively with a first message, so its fix runs
the brief as one query and then resumes that session in the same terminal.

`ryoku-rashin fix` resolves the problem against what the machine reports right
now (the tip by id from the live scan, the findings from the doctor scan). Run
from a terminal, the harness takes over that terminal; spawned from the Hub or
the shell, it opens the terminal chosen in Default Apps (`ryoku-app terminal`).
The dashboard's buttons go through `POST /api/fix`, which opens that terminal
the same way. The window runs as its own transient user unit
(`ryoku-fix-*.service`), so restarting the daemon never closes a repair, and a
harness that exits with an error leaves its message on screen until Enter.

## Prowl ships with Rashin

Rashin is the app people open. Prowl is its model gateway and code-intelligence
engine, shipped as the `prowl` package and `/usr/bin/prowl`. The two stay
separate processes so Prowl can keep its gateway and index engine, but there is
no second setup or management app.

Both processes ship off. Starting or enabling Rashin starts
`ryoku-prowl.service` on port 8788; stopping Rashin stops it. The Prowl unit has
no install target, so it does not come up on its own. `ryoku-rashin ensure`
recovers either process, and the detached fallback owns the same lifecycle on a
machine without a systemd user manager.

The `ryoku-rashin` package depends on `prowl`, so `pacman -Syu` updates them
together. The Prowl binary is stamped as pacman-managed and will not overwrite
itself. A source-installed binary on a development box may still use
`prowl update`.

Prowl keeps its existing state directories at
`$XDG_DATA_HOME/prowl-agent/gateway/` and
`$XDG_STATE_HOME/prowl-agent/registry.json`. The old name remains only there
because Prowl owns those durable paths and their migration; Ryoku never renames
them.

## Prowl integration

The console's **Prowl** section is the full control surface. Its Overview,
Providers, Routing, Activity, Projects, Harnesses, and Toolkit pages manage
provider keys and sign-ins, routing sets, request history, code indexes, harness
connections, and portable skills. Every harness connected through Rashin points
at Prowl's authenticated `/v1` gateway and uses the `auto` route. Changing a
provider or routing set therefore applies to Rashin and every connected harness
in one place.
Detected harnesses keep their own models until you connect them:
`ryoku-rashin wire` and updates re-route only harnesses you connected, and
Disconnect is never undone by a later update.

Rashin's own chat lanes follow the same rule. Connecting the chat agent makes
Prowl's `auto` its default for new chats, and the agent answering chat cannot be
disconnected until another one takes over. The picker lists Prowl once, as
**Active set** (the set active in Prowl's Routing page), followed by every model
the agent offers from its own providers, which answer directly rather than
through Prowl. Claude Code lists its models as they are. Until a provider can
serve `auto`, the agent stays pending, chat keeps answering on its previous
model, and the console says why.

Rashin proxies the gateway's management API under `/api/prowl/`, adding the
machine-local token itself. The browser never receives that token and cannot
proxy `/v1`. Code requests default to Rashin's source mirror, while the Projects
page can register and reindex other trees.

Prowl parses code with tree-sitter and stores its lexical index in SQLite FTS5.
Prose uses Porter stemming. Semantic search uses minishlab's
`potion-code-16M` static embedding model, compiled into the binary with no
runtime download, and stores vectors in sqlite-vec. Reciprocal-rank fusion
combines the FTS and vector rankings. An optional local Ollama model is used
only by `--smart` for query rewriting and reranking, never for embeddings.

## The source mirror

On a packaged box there is no source checkout for prowl to index, so the
prowl MCP server and `search_code` would otherwise answer only on a
maintainer's machine. Every reindex closes that gap: Rashin mirrors the live
config (`~/.config/quickshell`, `~/.config/hypr`, and
`~/.config/ryoku/*.json`) into
`~/.local/share/ryoku/rashin/source/` (with rsync when available, else a Go
copy that skips symlinks and files over 2 MB), writes a short `README.md`
marking it read-only, and runs `prowl init` there under a 120 s budget. The
mirror carries Prowl's index and is read-only; edits there are overwritten and
never reach the desktop.

`prowlRepo()` prefers a dev checkout that carries a `.prowl` index (the
deploy-recorded checkout, honouring `RYOKU_RASHIN_REPO` and
`~/.local/state/ryoku/repo`), and falls back to the mirror when it carries one,
so the code index answers everywhere while a dev checkout still wins on a
maintainer's machine. The whole step is best effort and bounded: a missing
prowl or a copy error degrades it and never fails the reindex.

## One-click setup

The `setup` verb runs in a floating kitty (the Extras pattern), streaming
progress as JSON to `$XDG_RUNTIME_DIR/ryoku-rashin/setup.json`, which the Hub
page watches live. The flow:

1. **Preflight:** check `curl`, a Python toolchain (`uv` or `python3`), network
   reachability, and disk space; detect an existing Hermes. `uv` and `nodejs`
   ship as `ryoku-rashin` dependencies, so a stock box needs no downloaded
   build toolchain.
2. **Install Hermes** via its official installer under `$HOME` when absent.
   Setup never runs with sudo.
3. **Connect:** point Hermes at Prowl's gateway, make `auto` its active model,
   and install the Prowl toolkit. Once `hermes acp --check` passes, setup skips
   Hermes's interactive provider step because providers live in Rashin.
4. **Wire:** ensure the vault, reindex, point Hermes's workspace at the vault so
   `MEMORY.md` and sessions live there, and write the vault `AGENTS.md` pointers.
5. **Global pointers:** append a marker-fenced block to each detected agent's
   global instructions file (see below).
6. **Enable** Rashin and its Prowl gateway, then open the console.

### Two Hermes safety rules

Hermes is the resident agent, and setup treats an existing install as sacred.

1. **Never clobber an existing Hermes.** If Hermes is already installed and
   configured, setup skips install and onboarding entirely and only wires. Your
   provider and model choices are untouched. Wiring uses the supported interface,
   never a raw edit of `~/.hermes/config.yaml`.
2. **Wiring is re-checked on serve start; drift shows in status.** Hermes's own
   onboarding can rewrite its config, so wiring runs after `hermes setup`
   finishes, and `ryoku-rashin serve` re-checks the wiring on start and re-applies
   it if it was lost. `status` reports drift so the Hub and dashboard can offer a
   re-wire action.

## Agent pointers

Wiring appends one marker-fenced block to each detected agent's global
instructions, telling it the vault exists and to read it first. The block is
idempotent (wire replaces an existing block or appends a fresh one) and reversible
(unwire removes the block and leaves the file):

```markdown
<!-- ryoku-rashin:begin -->
## Ryoku Rashin system vault

This machine runs Ryoku (Arch Linux, Hyprland desktop). A maintained map of the
system lives at `~/.local/share/ryoku/rashin/`. Before exploring the machine or
guessing paths, read `AGENTS.md` there: it says where every config lives, which
binary owns it, and how to reload it. Write durable notes to `memory/` and
dated notes to `journal/YYYY-MM-DD.md`.
<!-- ryoku-rashin:end -->
```

Wire targets, one per detected agent:

| Agent | File |
|---|---|
| Claude Code | `~/.claude/CLAUDE.md` |
| Codex CLI | `~/.codex/AGENTS.md` |
| opencode | `~/.config/opencode/AGENTS.md` |
| Oh My Pi | `~/.omp/agent/AGENTS.md` |
| Hermes | `~/.hermes/memories/MEMORY.md` |

Blocks are additive and only touch agents that are already present; Rashin never
creates an agent's own directory (except opencode's `~/.config/opencode`).

## Testing from a terminal

The vault is plain markdown, so any agent that reads `AGENTS.md` sees the same
map. To confirm the wiring end to end:

```sh
cd ~/.local/share/ryoku/rashin
hermes
```

Ask it something about the machine ("what GPU is in here and how do I switch
graphics modes?"). A wired Hermes reads `AGENTS.md`, follows it to `desktop.md`,
and answers from the vault instead of probing. Then write a note:

```sh
echo '- tried the vault, it works' >> journal/$(date +%F).md
```

Reopen the dashboard's Vault panel and the new journal entry is there, because the
terminal and the Ask chat share one workspace.

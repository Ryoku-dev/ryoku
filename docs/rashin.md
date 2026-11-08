# Ryoku Rashin

Rashin (羅針, "compass needle") is Ryoku's optional agent OS: a machine-generated
knowledge vault, a local daemon, and frontends for installed agent harnesses.
The Agent can run Hermes, Codex through `codex-acp`, Claude Code through
`claude-code-acp` or `claude-agent-acp`, omp, opencode, or Gemini CLI. Prowl is
an external model gateway, code index, and skill source that plugs into those
harnesses without owning their model choices.
Everything is local, off by default, and enabled from Ryoku Settings under
Advanced.

## What it is, and is not

It is:

- **Optional.** On every edition `ryoku-rashin` is installed by default but is
  not a desktop dependency. The switch on Ryoku Hub's Rashin page owns it: off
  stops the agent and removes the package, on installs it again and starts it
  with the desktop. Every AI piece goes with the package (the agent, the Rashin
  window, the bar's AI usage collectors) and so do its shortcuts: the Ask bar
  (Alt+Space) and the Rashin window (Super+Alt+Space) are catalogue binds that
  need `ryoku-rashin`, so the compositor registers no chord for them while it
  is absent and Kit's Keys tab lists them as not installed. Kit opens with
  Super+K and puts Keys, Commands, Phone, Dev tools, and Typing test behind one
  search field. Answering No to the installer's Rashin question or removing the
  package with the system's package manager reach the same state; the doctor
  delivers it once and never puts it back. Answering Yes leaves a first-login
  note that opens the Hermes setup once the machine is online.
- **Local.** The daemon binds `127.0.0.1` only. WebSocket upgrades verify the
  Origin host is localhost. Mutating routes reject cross-origin browser calls
  and simple requests that could skip a CORS preflight. No auth is needed
  because the surface is single-user and local.
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
`bar.md` is the guide to subshells, bar styles and the dock; `plugins.md` is the plugin contract and
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
| `ask <question>` | One-shot read-only ask, built for the launcher's `\` prefix: POSTs to `/api/ask` and pipes streamed `@working`/`@answer` markers to stdout. `ask --recent` prints the separate Ask history as JSON; `ask --cancel` stops the running ask without touching the Agent. See "Ask, Agent, and Chat" below |
| `terminal [--lane ryoku\|chat]` | Resume the current Agent or Chat session in the harness's own terminal UI when that harness supports it |
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

## Ask, Agent, and Chat

Rashin keeps quick questions, machine work, and plain harness chat separate:

| Mode | Surfaces | State and purpose |
|---|---|---|
| **Ask** | Alt+Space Ask, the launcher's `\` prefix, the console's Ask sheet, and the `rashin` terminal command | Fast, read-only answers with their own history in `$XDG_STATE_HOME/ryoku/rashin-asks.jsonl` |
| **Agent** | Alt+Space Agent, the pinned bubble, and the console's Agent sheet | The persistent machine-aware session, with the vault, `ryoku` skill, wiki, code index, tools, and approvals |
| **Chat** | The console's Chat sheet | A plain conversation with the selected harness, without the machine map or Needle identity |

The Agent and Chat each have their own hub, harness process, transcript, and
session history. Agent runs in the vault. Chat runs in
`$XDG_STATE_HOME/ryoku/rashin-chat`. An ask never enters either transcript.
Nothing it reads or answers appears in the Agent conversation unless the user
hands it off.

### How Ask answers

The Ask route picker is separate from the Agent model picker. Its first choice
is **Agent model**, which uses the selected harness and model. Prowl routes
follow when Prowl is connected and routable:

1. A Prowl route starts with the authenticated `/v1/chat/completions` fast lane.
   Its bounded loop has read-only tools for machine facts, files, the Prowl code
   index, and URL reads.
2. The Agent model route skips that fast lane. A Prowl answer that needs broader
   inspection, or a route that is unavailable, also takes this path.
3. Rashin opens a fresh private harness session for that one ask. The session is
   strictly read-only and never presents an approval. A denied mutation or a
   request that needs work returns `needsAgent` with a short explanation of what
   the Agent would do.
4. The answer is appended to `rashin-asks.jsonl`, with the route or harness and
   model that answered it. It is not broadcast to the Agent or Chat.

`GET /api/quick` reports **Agent model** first, then the available Prowl routes.
`POST /api/quick` changes only the Ask route. Ask works without Prowl. Cancel
stops only the in-flight ask, including its private read-only session.

### Hand an ask to the Agent

An answered ask offers **Continue with the agent**. It adds the question and
answer to the Agent transcript as a compact **From Ask** card, switches to
Agent, and focuses the field. The next Agent message continues from that card.
The hand-off itself does not run another turn.

While Ask is still working, or when the answer says it needs the Agent, the
action becomes **Move to the agent**. It cancels the ask, adds the **From Ask**
card, and immediately runs the original question as an Agent turn. The
`needsAgent` explanation says what that turn is expected to do.

### Action chips

The `@answer` payload carries verified actions found in the answer:

| Kind | Detected as | Chip does |
|---|---|---|
| `file` | a path that exists and is a file | opens it in `nvim` in a kitty window |
| `dir` | a path that exists and is a directory | opens it in the file manager |
| `url` | an `http(s)` link | opens it in the browser |
| `cmd` | a backtick span whose first word is on `PATH` | copies the command |
| `color` | a hex color, shown with a live swatch | copies the hex |

The answer also offers Copy, the appropriate Agent hand-off, Open Rashin, and
Pin bubble. Nonexistent paths and non-runnable command spans are dropped.

### Model choices

Rashin stores one explicit model pick per harness. A pick made in the bar,
console, or any other Rashin surface applies to that harness everywhere:
Agent, Chat, every new or loaded session, respawns, and private Ask sessions.
The current pick is shown in every surface. If the harness no longer offers it,
Rashin keeps the preference, leaves the live session on its current model, and
shows **Saved model unavailable**.

Prowl's Active set remains one option when it has been added to the harness. It
does not replace the harness's other models. The Ask route picker is a different
choice: it selects Agent model or a Prowl route without changing the harness
model.

The Agent supports Hermes, Codex through `codex-acp`, Claude Code through
`claude-code-acp` or `claude-agent-acp`, omp, opencode, and Gemini CLI. **Open
in terminal** runs `ryoku-rashin terminal` and resumes the current Agent session
in the harness's own TUI when that harness supports resume. Hermes, omp, Claude
Code, and Codex currently do.

### The bar's drawers and notifications

The header names the current mode: `RASHIN // ASK`, `AGENT`, `TOOLS`, or `WEB`.
Ask carries a route pill under the field, then shows the answer's actual route
after it finishes. Ask history lists recent asks; Agent history lists Agent
sessions. The model drawer lists Ask routes in Ask mode and harness models in
Agent mode. `Ctrl+Shift+H` and `Ctrl+Shift+M` open the two drawers.

Buttons give press feedback, mode changes crossfade, transcript items enter in
order, and the live marker and orb report active work without making the bar
jump. The pinned Agent bubble stays clear of the bar. When an Agent turn ends
or needs approval while no Rashin surface is visible, Rashin sends a desktop
notification. Activating it opens the bar in Agent mode.

### `\resume`

Typing `\resume` lists recent asks from `rashin-asks.jsonl`, newest first.
Picking one recalls its stored answer and chips without a model call. The Ask
bar, launcher, console Ask sheet, and `rashin --resume` read the same Ask
history.

## The Rashin app

The quick bar is one surface; the companion window is another. `rashin-app`
(`ryoku/rashin/app/`, shipped by `ryoku-rashin`) is a small GTK3 + WebKitGTK
window that hosts the Rashin console the daemon serves. It opens like a normal
window from Super+Alt+Space, the launcher, `rashin-app`, or Open Rashin, and is
single-instance. It shows a dark boot page until `/api/ping` answers, then
loads `http://127.0.0.1:3600/#/agent`.

The host is a client of the daemon, never a second brain. The console is one
Svelte app (`ryoku/rashin/web/`) that a browser sees on the same port. Links to
other origins open in the default browser, notifications are granted, and the
window remembers its size. Its icon is Rashin's seal from
`ryoku/assets/brand/rashin-mark.svg`; the console favicon and bar header use
the same family.

| Sheet | What it holds |
|---|---|
| Agent | The persistent machine-agent workspace: sessions, transcript, thinking folds, tool rows, diffs, approvals, composer, images, model, usage, tools, commands, and Open in terminal |
| Ask | A read-only question field, streaming answer and route, recent Ask history, Ask route picker, cancel, and the Continue or Move to the agent hand-off |
| Chat | The same workspace shape on the plain Chat lane, without machine context |
| Wiki | The shipped guides for someone new to Ryoku, rendered with deep links |
| Overview | Wallpaper, clock, live vitals, code index summary, health, and vault state |
| System | Services, timers, cron, containers, sockets, processes, filesystems, Doctor, and deterministic tips |
| Vault | The grouped tree, rendered documents, deep links, and reindex |
| Memory | Provider tiles, vault graph, activity, and Hermes session history |
| Skills | One tab per harness, grouped and counted |
| Prowl | Prowl gateway, provider, route, activity, index, harness, and toolkit controls |
| About | What Rashin is, live system facts, and shortcuts |

Agent and Chat project their own `/ws/chat` streams through the shared reducer
used by the bar. Ask uses `/api/ask` and its separate history. A turn started in
one Agent surface is live in the other Agent surfaces, but Ask never appears
there without a hand-off. Console-only state covers layout details such as pane
collapse, drafts, and the last vault file. The console wears the desktop's live
palette from `GET /api/theme` and retints while open.

## In the terminal

The launcher's `\` ask has a sibling on the command line: the `rashin`
command. `rashin take me to the fastfetch config` answers from the same
read-only Ask system and can put a proposed command on the fish prompt.
`rashin scan Documents for pngs and move them to Pictures` returns a validated
one-liner informed by `habits.md`. Rashin does not run that proposal itself;
the command buffer is the confirmation and each command carries a
read/write/system/danger tier.

Terminal asks share the daemon, vault, model route, and `rashin-asks.jsonl`
history with the bar, launcher, and console Ask sheet. They do not share the
Agent transcript. `rashin -c` carries the previous terminal exchange into a
new read-only ask, and `rashin --resume` recalls a stored result. Repeated asks
can become `rr-<name>` shell recipes. Full behavior is in
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

Navigation is three floating islands, like QS Bar's islands form: the seal, the
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
| Prowl | A left rail for Overview, Providers, Routing, Activity, Projects, Harnesses, and Toolkit. Providers holds keys and sign-ins; Routing holds Prowl sets and Ask routes; Harnesses adds Prowl to detected CLIs and controls whether each defaults to Prowl auto |
| About | What Rashin is, the system manifest with live facts, quick start, a command crib (`hermes -h`, `prowl overview`), and the privacy note |

### Conversation workspaces

The console's Agent and Chat sheets are live views of their own harness
sessions. Thinking streams while the harness works and then folds; each tool
call is one row with its output or diff; approvals stay on the row that asked.
The searchable model picker changes the harness-wide saved pick, so Agent,
Chat, loaded sessions, respawns, and private Ask sessions converge on the same
choice. The Ask sheet has no approvals because every ask is strictly
read-only.

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
is the one `fix.agent` names in `~/.config/ryoku/rashin.json`, else the chat
agent picked in Rashin, when it can open a repair (Hermes, Oh My Pi, Claude
Code, or Codex), otherwise the first of those installed. For Codex, `fix.model`
and `fix.effort` set the model and reasoning effort of that session only, for
example `"fix": {"agent": "codex", "model": "gpt-6.1-sol", "effort": "low"}`;
Codex's own config stays as it is. Run without a terminal (the Updates page
button), `ryoku-rashin fix` opens the terminal at once and answers with the
harness it will start; the health check and any error then show in that
terminal, which stays open on a failure. It starts in the vault,
so the machine map is its working directory, with the Needle persona as extra
system prompt (Hermes and Codex, which take none, get it at the top of the message) and
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

Rashin is the app people open. Prowl is an external tool that plugs into
harnesses as a model gateway, code index, and skill source. Ryoku ships it as
the `prowl` package and `/usr/bin/prowl`. The two stay separate processes so
Prowl keeps its own gateway and index engine, while Rashin hosts its lifecycle
and control surface.

Both processes ship off, and Prowl starts only when something needs it: a quick
ask, the console's Prowl pages, or an agent that reaches its model through
Prowl. Rashin brings up `ryoku-prowl.service` (the runit service on Void) on
port 8788 for that and stops it once nothing has used it for 15 minutes, no
agent routed through it is running and no client holds a connection. **Keep
Prowl running** on the Hub's Rashin page (`ryoku-rashin prowl keep on`) keeps
it up for other apps that use the gateway directly. Stopping Rashin stops it.

The chat agent works the same way. It starts when a chat surface opens and
leaves two minutes after the last one closes and its turn ends, so an idle
desktop carries only the small Rashin daemon. The next open loads the same
session back.

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
provider keys and sign-ins, routing sets, request history, code indexes,
harness connections, and portable skills.

**Connect** adds Prowl's `auto` model and toolkit to a harness without changing
that harness's default. **Default to Prowl auto** is a separate switch for the
user who wants Prowl to become that harness's default. Disconnect removes the
integration. The model picker still lists the harness's own models alongside
one Prowl Active set choice.

Background wiring never makes that choice for the user. `ryoku-rashin wire`,
`ryoku update`, deploy, doctor, and daemon startup refresh credentials only for
harnesses the user already connected. They do not connect another harness,
change a default, replace the saved model pick, or restart the Agent. Choosing
which harness backs Rashin also does not connect it to Prowl.

The Ask route picker is independent. It offers Agent model first and available
Prowl routes after it. Changing an Ask route does not change the harness
default or its saved model.

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
3. **Connect:** add Prowl's `auto` model and toolkit to Hermes. Setup makes it
   the default only when Hermes has no model configured yet. Once
   `hermes acp --check` passes, setup skips Hermes's interactive provider step.
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

Reopen the console's Vault sheet and the new journal entry is there. The vault
is shared knowledge even though Ask, Agent, and Chat keep separate histories.

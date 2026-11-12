# Rashin

Rashin is Ryoku's optional AI system. It uses no process, socket, timer, watcher,
or login probe until the user turns it on. The Ryoku vault is separate and stays
available at `~/.local/share/ryoku/vault/` whether Rashin is installed or on.

## Ask, Agent, and Index

**Ask** answers system questions and general prompts through Rashin's built-in
free-model router. It starts read only: it can inspect the vault, system,
projects, wiki, and doctor results, and work that needs a change becomes a
proposed command or an explicit handoff to Agent. In the app a conversation can
be set to Ask first, where each command or file change waits for you, or to
YOLO, where changes run without asking and only a destructive command still
waits. An ask from the bar or a terminal never changes the machine.

**Agent** runs an external coding harness through ACP. Rashin owns the persistent
session, event log, files, terminal, approvals, and process lifecycle, so closing
the app does not lose the conversation. Each harness keeps its own model choice,
and its session shows the model, mode, and options it offers. Every agent
session is told where it runs before its first prompt: Ryoku, this vault,
Rashin, and the code search over its folder, which it also gets as tools from
Rashin's own MCP server.

An agent session also shows the subagents its harness runs, each in a box that
unfolds to what it did, and folds a finished turn's work behind one "Worked
for" line. `/goal <what>` sets a goal Rashin keeps the agent on, turn after
turn, until it reports the goal done or blocked or its turn budget runs out;
pause, stop, or clear it from the strip over the composer. On niri an agent
that needs a graphical app can be given a hidden screen of its own, once you
agree: you can watch it, take it over, and hand it back, and the agent hands it
to you for things like signing in. Nothing on it reaches your desktop.

**Index** covers the system and registered projects with the potion embedding
model, which is built into the daemon. A project added to Rashin and the
repository an agent session works in are indexed by themselves, C and C++
included. The command line exposes cited `search`, `find`, `def`, `outline`,
and `references` queries.

The full architecture and protocol contract is in `docs/rashin.md` in the Ryoku
source tree.

## Teams

Teams is the app's second mode, opened with the Teams button beside the search
field or Ctrl T. A team is a lead and up to five more agents, each on a harness
and model you pick, with one purpose and a folder of its own (`~/Teams/<name>`
by default). You give it a task, or a routine gives it one on a schedule, and
it hands back a report in the Inbox. Templates come in three levels: light
(System care, Mail desk), medium (Repo watch, Research desk) and advanced
(News desk, three members on two models with two routines), or start blank.
Repo watch asks for a `GITHUB_TOKEN` and News desk for a `NEWS_API_KEY` once
the team exists. Teams do not talk to each other.

- **Proposals**: when you ask an agent or Ask for a team, it proposes one with
  `propose_team` (or `rashin teams propose <file.json>` from a terminal). The
  proposal waits in the Inbox; Create team makes it and then asks for its
  credentials, Not now makes nothing. At most five wait at once.
- **Approvals**: a mail, a request carrying a credential for the first time on
  a host, or a credential the team asks for waits for you in the Inbox or on
  the task's page, showing exactly what will be sent. Always trusts that one
  host or recipient for that team. An approval nobody answers is denied after
  a day.
- **Routines** run on a cron schedule in local time, at most every 15
  minutes. Missed runs are skipped by default, and five failures in a row turn
  a routine off.
- **Budgets** cap a team's tokens a month. At the limit the team pauses and
  asks before it spends more.
- **Activity** records who did what, including what happened while no Rashin
  window was open.
- **Vault**: credentials for teams live sealed in Rashin's credential vault,
  whose key stays in your login keyring. Agents never see a value: Rashin adds
  it to the request or the mail itself, only for the hosts you allowed. This
  is not the Ryoku vault described below.
- **Sandbox**: every team agent runs in bubblewrap, with the filesystem
  read-only except its team folder and its harness's own files, and with
  Rashin's state, your keyrings, the other teams' folders and the Ryoku vault
  hidden. It reaches Rashin only through its own socket. Without the
  `bubblewrap` package team tasks do not start.

A program you run yourself outside a team still runs as you, the network is
shared, and a team agent can read your files like any program, so a team is
not a boundary against code you already run.

## Surfaces

- Alt+Space opens the thin Ask bar.
- Super+Alt+Space opens `rashin-app`, with separate Ask and Agent tabs and a
  Teams mode.
- `rashin <question>` asks from a terminal.
- Ryoku Settings > Apps > Rashin turns the system on or off.

All clients use the daemon's Unix socket. There is no web dashboard. The one
port Rashin can open is its local API, which serves the router to other
programs and stays closed until you turn it on in Rashin > Settings > Local API
or with `rashin api on`.

## Models and harnesses

Ask uses Rashin's built-in free-provider router. Provider credentials stay in
Rashin's local key store. Agent adapters discover supported ACP harnesses without
making one a required dependency. Turning Rashin off stops its daemon and any
agent process while keeping event-log sessions on disk for later resume.

## The Ryoku vault

The vault's generated maps describe hardware, the desktop, packages, ownership,
logs, user config differences, and the Ryoku source tree. `memory/` and
`journal/` belong to users and agents. The wiki is a shipped read-only mirror.
Refresh it independently of Rashin:

```bash
ryoku vault status
ryoku vault refresh
ryoku vault wire
```

Harness pointers tell installed coding tools to read the vault first, and the
linked `ryoku` skill carries the desktop safety and GUI-first rules. `ryoku vault
unwire` removes those pointers and records the user's opt-out.

## Terminal use

```bash
rashin ask "why did the shell stop?"
rashin search "where is the dock state written?"
rashin sessions
```

Ask can answer directly or propose a command for review. In the app, Run on a
proposed command runs it in the conversation: you watch its output, can stop
it, and Ask answers from what it printed. Agent work remains in a separate
persistent session and can be resumed from the app or command line.

Return to the [desktop tour](desktop.md), or read [Linux basics](linux-basics.md)
for services, logs, updates, and config ownership.

# Rashin

Rashin is Ryoku's optional AI system. It uses no process, socket, timer, watcher,
or login probe until the user turns it on. The Ryoku vault is separate and stays
available at `~/.local/share/ryoku/vault/` whether Rashin is installed or on.

## Ask, Agent, and Index

**Ask** answers system questions and general prompts with Rashin's native
engine. It starts read-only: it can inspect the vault, system, projects, wiki
and doctor results, then propose changes or move the conversation to Agent.
In the app Ask first requires permission before a change; YOLO still asks
for destructive work. An ask from the bar or terminal stays read-only.

**Agent** uses the same engine with file, shell, code and project tools.
The model and options belong to the session; approvals or Plan first decide
when it may act. Rashin owns the event log and transcript, so closing the
app does not lose the conversation. Every Agent prompt knows Ryoku, this
vault, Rashin and code search in its project. `task` runs child engines and
shows their work in expandable cards; `/goal` continues across turns until
done, blocked or the budget is spent. A screen can be requested with
permission for work in a hidden graphical session.

**Index** covers the system and registered projects with the potion embedding
model, which is built into the daemon. A project added to Rashin and the
repository an agent session works in are indexed by themselves, C and C++
included. The command line exposes cited `search`, `find`, `def`, `outline`,
and `references` queries.

The full architecture and protocol contract is in `docs/rashin.md` in the Ryoku
source tree.

## Teams

Teams is the app's second mode, opened with the Teams button or Ctrl T.
A team is a lead and up to five members, each on a model and options you
choose, with one purpose and a folder of its own (`~/Teams/<name>` by default).
You give it a task, or a routine gives it one on a schedule, and
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
- **Sandbox**: a team's file and shell tools run in a bubblewrap tool host
  child with a private scratch HOME. Only its team folder and allowed paths
  are writable; Rashin state, your keyrings, other teams and the Ryoku vault
  are hidden. Model calls and credentials stay in the daemon. Without
  `bubblewrap`, team tasks do not start.

A program you run yourself outside a team still runs as you, the network is
shared, and a team agent can read your files like any program, so a team is
not a boundary against code you already run.

## Surfaces

- Alt+Space opens the Rashin bar: Quick Ask, web search and downloads. Web
  and Downloads work with Rashin off.
- Super+Alt+Space opens `rashin-app` with Ask, Agents, Teams and Learning.
- `rashin <question>` asks from a terminal.
- Ryoku Settings > Apps > Rashin turns the system on or off.

All clients use the daemon's Unix socket. There is no web dashboard. The one
port Rashin can open is its local API, which serves the router to other
programs and stays closed until you turn it on in Rashin > Settings > Local API
or with `rashin api on`.

## Models and learning

Ask and Agents use Rashin's model router. Managed `rashin-models` runs
downloaded GGUF files through on-demand llama.cpp, then Rashin tries user
llama.cpp, Ollama and free remote providers; paid models run only when
picked. Model files and the runtime download only when you ask. The
Learning page shows optional managed skills, project decisions, rules
and local tool reports, all off by default. Turning Rashin off stops its
daemon and children while keeping sessions on disk.

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

`ryoku vault wire` also points other installed coding tools at the vault;
Rashin reads the vault directly. `ryoku vault unwire` removes those external
pointers and records the user's opt-out.

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

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

## Surfaces

- Alt+Space opens the thin Ask bar.
- Super+Alt+Space opens `rashin-app`, with separate Ask and Agent tabs.
- `rashin <question>` asks from a terminal.
- Ryoku Hub > Rashin turns the system on or off.

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

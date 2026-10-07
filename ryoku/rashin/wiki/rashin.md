# Rashin

Rashin is Ryoku's optional local assistant system. Press Alt+Space to open its
bar. The companion console, the `rashin` terminal command, and the knowledge
vault use one local daemon. Prowl is the model gateway and code index underneath,
so it appears as part of Rashin rather than as a second app.

## Two lanes, two purposes

**Ryoku** is the machine lane. The Needle works on this computer with the Ryoku
vault, shipped `ryoku` skill, this wiki, Prowl's source index, and the selected
approval mode. Use it to inspect, explain, troubleshoot, or change the machine.
A session starts in the vault so the harness loads its generated `AGENTS.md` and
machine contract.

**Chat** is a plain conversation with the selected harness. It has no machine
map, no Needle identity, and no quick path. Its session starts in a
neutral directory so a repository `AGENTS.md` is not loaded. Use it when you
want a normal conversation rather than work on Ryoku. The approval setting is
shared by both lanes.

Each lane keeps its own transcript and session history. Starting or loading a
session in one lane does not replace the other lane's conversation.

## Quick and Agent in the Ryoku lane

**Quick** sends a short question through Prowl's selected route, `auto` by
default, with a small set of read-only machine tools. It suits explanations and
lookups. If the question needs broader tools, edits, or a multi-step task, it
escalates into the Ryoku lane's full agent rather than pretending the work is
done.

**Agent** is the full harness session. It can use the vault, skill, wiki, Prowl,
and the harness tools allowed by the current approval mode. Use it for changes,
debugging, or work that must inspect several parts of the machine. Quick and
Agent write into the same Ryoku transcript, so an escalated answer remains in
context.

## Prowl inside Rashin

Open the console's **Prowl** section to connect providers and sign-ins, manage
keys and routing sets, inspect activity, index projects, connect harnesses, and
install their toolkit. Every harness connected through Rashin uses Prowl's
`auto` route, so provider and routing changes apply in one place.

Prowl ships with Ryoku but stays off until Rashin starts. It stops with Rashin
too. Its project indexes and gateway data remain available for the next start.

## Approval modes

The labels say what runs without another prompt:

- **Ask:** every tool call asks first, including reads.
- **Reads run:** read-only tools run automatically; writes and commands ask. This
  is the default.
- **All run:** every tool call runs automatically, including writes and commands.
  This is an explicit choice, never the default.

Choose the narrowest mode that fits the work. An approval response applies to
the live request; it does not turn a plain Chat session into the machine lane.

## The vault and the Needle

The vault is `~/.local/share/ryoku/rashin/`. Its generated maps describe the
system, desktop, packages, config ownership, logs, user differences, and source
tree. The `memory/` and `journal/` directories hold durable and dated agent
notes. This wiki lives under `wiki/` and is shipped read-only.

The Needle knows what the vault and live read-only tools can prove: this
machine's current hardware and desktop map, who owns a config, where logs live,
which Ryoku source implements a feature, and the safety rules in the `ryoku`
skill. It does not make a stale environment variable or an unverified guess into
machine truth. Reindexing refreshes the generated maps.

## Rashin in a terminal

Ask from a terminal with natural language:

```bash
rashin take me to the fastfetch config
```

The terminal command uses the Ryoku lane. It can answer directly or return a
ready-to-run command plan informed by the vault and current directory. It does
not execute the proposal by default; you review it and press Enter. The terminal
surface is useful over SSH and when the desktop bar is not visible.

For service state and maintenance, the daemon command lists its own verbs:

```bash
ryoku-rashin --help
ryoku-rashin status
ryoku-rashin index
```

Return to the [desktop tour](desktop.md), or read [Linux basics](linux-basics.md)
for services, logs, updates, and config ownership.

# Rashin

Rashin is Ryoku's optional local assistant system. Press Alt+Space to open its
bar, or Super+Alt+Space for the companion console. The `rashin` terminal
command and the knowledge vault use the same local daemon. Prowl is an external
model gateway, code index, and skill source that can plug into the harnesses
Rashin runs.

## Ask, Agent, and Chat

**Ask** gives quick read-only answers. It appears in the bar's Ask tab, the
launcher's `\` ask, the console's Ask sheet, and the `rashin` terminal command.
These surfaces share `rashin-asks.jsonl`, not the Agent transcript. A Prowl
route can answer quickly with bounded read-only tools, or the selected Agent
model can answer through a fresh private read-only harness session. Each
escalated ask gets a new session and never presents an approval. If the request
needs a change, the answer says the Agent is needed and explains what it would
do.

**Agent** is the persistent machine-aware conversation. It appears in the bar's
Agent tab, the pinned bubble, and the console's Agent sheet. Its harness starts
in the vault and can use the machine map, shipped `ryoku` skill, wiki, code
index, and the tools allowed by the current approval mode. Use it for changes,
debugging, and work that needs continuing context.

**Chat** is a plain harness conversation in the console. It has its own
transcript and sessions, no machine map, and no Needle identity. Starting or
loading a Chat session does not replace the Agent conversation.

## Hand an ask to the Agent

After Ask answers, **Continue with the agent** adds its question and answer to
the Agent transcript as a **From Ask** card. The next Agent message continues
from that context. While Ask is still working, or when the result needs the
Agent, **Move to the agent** cancels the ask and immediately runs its original
question as an Agent turn.

No Ask reaches Agent by itself. The hand-off is always explicit.

## Models and harnesses

Rashin stores one model pick per harness. A pick made in any surface applies
to Agent, Chat, every new or loaded session, respawns, and private Ask sessions
for that harness. Every surface shows the saved pick. **Saved model
unavailable** means the harness does not offer it right now; Rashin keeps the
preference and leaves that session on its current model.

The Ask route picker is separate. It offers **Agent model** first, followed by
available Prowl routes. Changing the Ask route never changes the harness model.

The Agent can run Hermes, Codex through `codex-acp`, Claude Code through
`claude-code-acp` or `claude-agent-acp`, omp, opencode, or Gemini CLI. **Open
in terminal** resumes the current session in the harness's own TUI where that
harness supports it.

## Prowl inside Rashin

The console's **Prowl** section manages providers and sign-ins, routing sets,
activity, project indexes, harness connections, and portable skills. Prowl
remains its own tool even though Rashin hosts this control surface.

**Connect** adds Prowl's `auto` choice to a harness without making it the
default. **Default to Prowl auto** is a separate switch. `ryoku update` and
`ryoku-rashin wire` only refresh credentials for harnesses that are already
connected. They never change a harness default, replace the user's model pick,
or restart the Agent.

Prowl ships with Ryoku but only runs while something needs it, and stops 15
minutes after its last use. The Hub's Rashin page has a switch to keep it
running for other apps. The chat agent also leaves two minutes after you close
the chat, and picks the same conversation back up when you open it again.

## Approval modes

Approval modes apply to Agent and Chat, not Ask:

- **Ask for approval:** every tool call asks first, including reads.
- **Reads run:** read-only tools run automatically; writes and commands ask.
  This is the default.
- **All run:** every tool call runs automatically, including writes and
  commands. This is an explicit choice, never the default.

Choose the narrowest mode that fits the work. An approval response applies to
the live request; it does not turn Chat into the machine-aware Agent.

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

The command uses the same read-only Ask system and history as the bar,
launcher, and console Ask sheet. It can answer directly or return a
ready-to-run command plan informed by the vault and current directory. It does
not execute the proposal by default; you review it and press Enter. It never
writes into the Agent conversation.

For service state and maintenance, the daemon command lists its own verbs:

```bash
ryoku-rashin --help
ryoku-rashin status
ryoku-rashin index
```

Return to the [desktop tour](desktop.md), or read [Linux basics](linux-basics.md)
for services, logs, updates, and config ownership.

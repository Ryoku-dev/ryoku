# Agents

The Agents widget is Nomarchy's usage panel. On Ryoku it displays usage
records without owning AI credentials or account state.

`Panel.qml` draws the bar button and popup. `Main.qml` discovers records,
refreshes them and handles optional cross-device aggregation. `Agent.qml`
watches one record.

## Ryoku data flow

`omarchy-agent-usage-update` runs the installed usage adapters and writes one
JSON record per provider under:

```text
${XDG_STATE_HOME:-~/.local/state}/omarchy/agents/usage/
```

Ryoku ships adapters for Claude and Codex. They translate the existing
`~/.cache/claude-usage.json` and `~/.cache/codex-usage.json` caches into the
record format consumed by this panel. They do not read credentials, contact a
provider or change an account. A stale source remains visible and is labelled
as last-known usage.

Rāshin owns AI credentials and accounts. The capability probe reports account
management as unavailable, so the panel does not show add, rename,
reauthentication, Use or Autoswitch controls. Ryoku Ask also owns prompt
entry; the Agents panel opens Ask rather than carrying a second prompt
surface.

The record boundary remains open to extensions. A marketplace plugin can
write another valid provider record, such as Grok usage, and the panel will
discover it without a code change. An optional `assets/<id>.svg` supplies its
mark; `assets/<id>-light.svg` can provide a light-surface variant.

## Panel

Each provider section can show:

- current limit windows and reset times;
- today, weekly and model usage summaries;
- stale or unavailable status;
- prepaid balance data when a record supplies it;
- account-shaped usage rows when an extension supplies them.

Account-shaped rows are display-only on Ryoku. They never offer credential or
account mutation controls.

The bar button interactions are:

- left click opens or closes the panel;
- middle click refreshes the records;
- right click opens Ryoku Ask.

The hero button also opens Ryoku Ask. Arrow keys or `h`, `j`, `k`, `l` move
through available panel actions. Ctrl+Up and Ctrl+Down reorder provider
sections. Dragging a provider mark does the same. `r` refreshes, Tab moves to
the next bar panel and Escape closes.

The provider order is stored at:

```text
${XDG_STATE_HOME:-~/.local/state}/omarchy/agents/order.json
```

IPC remains compatible with Nomarchy:

```text
omarchy-shell omarchy.agents <open|close|toggle|refresh>
```

## Settings

Settings live in the `omarchy.agents` entry of
`~/.config/omarchy/shell.json`.

| Key | Default | What it does |
|---|---|---|
| `refreshIntervalSec` | `900` | How often usage records regenerate |
| `syncMode` | `"Off"` | Writes this machine's snapshot and merges other snapshots when enabled |
| `syncDir` | `""` | Folder containing synchronized snapshots |
| `syncFileName` | `<hostname>.json` | This machine's snapshot filename |
| `syncDeviceId` | hostname | Stable device name stored in the snapshot |
| `providers` | `{}` | Per-provider visibility settings |

Examples:

```bash
omarchy bar set omarchy.agents refreshIntervalSec 300 --json
omarchy bar set omarchy.agents syncDir '~/Sync/agent-usage'
omarchy bar set omarchy.agents providers '{
  "claude": { "enabled": true },
  "codex": { "enabled": false }
}' --json
```

Provider enablement defaults to true. A disabled provider is hidden and
skipped during refresh.

With sync enabled, device-scoped totals are added across snapshots and active
days are merged by date. Rate limits and account-scoped values stay local.

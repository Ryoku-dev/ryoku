# AI usage dashboard

The AI usage module adds OpenCode, Codex and Claude usage to Shima's Island
desktop page. It shows local token history, sessions, model usage, trends,
activity, measured account limits and local API Value estimates.

## Enable and use

Open the Island's settings, then **Island > Desktop page > Your modules**, and
enable **AI usage**. Keep the **Modules** block enabled in the desktop page's
Blocks list. Open the Island's **Desktop** page to view the dashboard.

Use the agent chip to switch between agents with an available local cache.
Expand Trend, Models or Activity through their headers. The period selector
offers Today, Last 24 hours, Last 7 days, Last 30 days and All time. The All time
trend uses a fixed 45-day window; the activity card also shows lifetime totals
from the available local history.

Settings control card width, the number of top models and the default period.
The same module provides a compact desktop tile. Existing user-installed copies
take precedence over the built-in copy when they provide an Island entry.

## Collection and refresh

After the first module load in a shell session, one widget-owned worker collects
all available agents every 15 seconds, even after the page closes or unloads.
Opening the page requests an earlier collection. An active collection is shared
instead of starting overlapping processes; private-lock contention retries.
The worker ends with the shell and starts again when the module first loads in
the next session. It does not start a separate login daemon.

Each collector uses the files and authentication already available to its agent:

| Agent | Local history | Account limits |
| --- | --- | --- |
| OpenCode | Native SQLite session/message data, with legacy-store fallbacks | The dashboard does not show invented local token budgets as account limits. |
| Codex | Local rollout records and turn lifecycle | Existing CLI account-limit RPC, with measured rollout snapshots as fallback. |
| Claude | Local Claude Code transcripts and native session metadata | Matching Claude Desktop usage captures or saved CLI subscription OAuth. |

Claude usage and lifecycle share one transcript pass. Session activity uses
native state and process identity rather than treating an open app as a live
request. Usage is counted from local history regardless of who owns the login;
switching accounts does not erase previously recorded local history.

Account limit bars are account-wide, so use on another device can affect them.
Their source and authentication govern how fresh they can be. The 15-second
local collection interval does not bypass provider cooldowns or cause Desktop
to obtain a fresh account response. Stale captures remain marked as last known.

## API Value and storage

API Value estimates the local workload at supported model rates; it is not a
subscription charge, wallet balance or account bill. Missing pricing coverage
is marked as partial. Historical daily price snapshots remain frozen rather
than repricing earlier days with current rates.

The widget resolves its collectors from its own `bin/` directory. It does not
require Rashin's collectors or the optional shared AI usage service.

| Path | Purpose |
| --- | --- |
| `~/.cache/ai-usage/` | Private agent caches and collection locks. |
| `~/.local/state/ai-usage/` | Local pricing state and monthly API Value ledgers, unless overridden through the supported collector environment. |
| `~/.config/inir/widgets/ai-usage/` | A user-installed copy, when present. |

Caches and ledgers are published privately and atomically. Usage collection does
not launch paid model prompts, change authentication or upload transcript data.
Quota reads use the existing authenticated provider connection; pricing reads
use public model-rate metadata.

## Source map

The built-in payload lives in
`ryoku/shell/quickshell/inir/defaults/widgets/ai-usage/`: `IrisCompact.qml` is
the Island view, `AiUsageTile.qml` is the desktop tile, `UsageModel.qml` reads
the caches, `refresh/` owns collection timing, `components/` holds the cards,
and `bin/` holds collectors and pricing helpers.

The required shared wiring is in `inir/services/CustomWidgets.qml`,
`inir/modules/iris/settings/IrisOptions.qml` and
`inir/modules/iris/bar/IrisCustomModule.qml`, relative to
`ryoku/shell/quickshell/`. The existing source/package payload copy delivers the
module with the shell. Visible copy uses the shared translation runtime;
Ryoku's `docs/i18n.md` describes catalog extraction and translation.

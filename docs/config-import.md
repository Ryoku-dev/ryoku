# Config import

Bring an existing setup onto Ryoku without losing it or breaking the desktop. A
source may be a Hyprland tree, a niri KDL tree, a whole config home, an app
directory, or a dotfiles checkout. Ryoku maps supported desktop settings into
`desktop.json`, keeps provider-only values under `wm.<provider>`, shows keybind
collisions, preserves native nodes it cannot map, and can undo the complete
transaction.

Status: implemented (v1). This document is the feature's reference.

## Why this exists

People do not start from nothing. They come from a hand-built rice or another
distro with a config they like, and today Ryoku has no path to carry it over:
they either abandon their setup or hand-merge files and guess at what clashes
with Ryoku's shipped binds. The result is a wall right at the first impression.

The desktop already has the pieces this needs: apps load a user override last,
the neutral desktop store is shared by providers, and every provider owns its
native config syntax. Import joins them into one guided, reversible flow.

## The override model this builds on

Every Ryoku app has the same three tiers. Import writes into them; it never
edits the shipped base.

| Tier | What it is | Where | Who edits |
|---|---|---|---|
| Desktop settings | Neutral and provider-owned settings | `~/.config/ryoku/desktop.json` (`desktop.*` and `wm.<provider>.*`) | Hub or hand edit |
| Raw overrides | Hand-owned config loaded last | the provider's advertised user include, plus app user includes | you, in place |
| Shipped base | Ryoku defaults, replaced every update | provider and packaged app config trees | nobody |

The overlay (`~/.config/ryoku/user_edits/`) is a separate, advanced mechanism
for forking a whole shipped file; import does not use it.


## Goals and non-goals

Goals:
- Failsafe: nothing shipped is destroyed; every import is backed up and undoable.
- Drop-and-go: folder, drag-drop, an existing `~/.config`, or a git URL all work.
- Resolve collisions in place: keybind overlaps are shown and fixed in the Hub.
- Desktop binds, rules, input, appearance and other supported values become
  first-class Ryoku settings rather than opaque hand config.
- Provider-native values remain in that provider's `wm.*` namespace and are
  preserved while another provider is active.

Non-goals (v1):
- Guessing at a translation. A native node without an exact typed destination is
  listed and copied into the provider's marked user include.
- Deep ingest for ordinary apps. kitty, fish, fastfetch and other app configs
  already have late-loaded override files, so there is nothing to reconcile.
- Cloud sync, profile export, theme translation. Rices and `.ryoprofile` already
  cover those.

## User flow

Hub, Advanced on, Tools group, "Import config". A wizard:

1. Source. Four affordances, no dead ends: pick a folder, drop files or a
   folder, paste a git URL (cloned to a temp dir), or accept the banner shown
   when `~/.config` already holds non-Ryoku config from a previous setup.
2. Review. One card per detected app: what was found ("47 keybinds, 12 window
   rules, 30 raw settings" / "kitty: colors + 20 settings"), an include toggle,
   and a conflict badge. A running summary ("6 apps, 8 conflicts").
3. Resolve. A conflict table, keybinds first. Each row shows the combo, what
   Ryoku uses it for, what the imported binding does, and [Keep Ryoku's]
   [Use mine] [Remap...]. Same-combo duplicates within the source are flagged.
   The review also lists every native item kept in the provider user include and
   why it could not be typed.
4. Preview. The exact change set: files written, binds ingested into the GUI (N),
   unbinds and rebinds added (M), and the backup location. Apply or Cancel.
5. Done. Peak-end success ("Your keybinds are in the Keybinds page; kitty and
   fish are live"), with [Reload now] and [Undo this import].

## Scope (v1)

Three handling tiers are chosen per detected source:

- Deep: Hyprland and niri desktop trees. Supported values enter
  `desktop.json`; provider-exclusive values enter `wm.<provider>`; unknown
  native syntax is preserved in the provider's marked user include.
- Layer-on-top: kitty (`user.conf`), fish (`user.fish`) and fastfetch
  (`user.jsonc`).
- Generic drop: any other config directory is listed and copied only when the
  user includes it.

The niri reader follows `include` directives in place, resolves relative paths
against the including file, expands include globs, guards cycles, and also scans
unreferenced KDL files under `config.d/` and `cfg/`. Its parser handles line and
nested block comments, slash-dash comments, escaped and raw strings, properties,
and nested blocks.

If a provider recognizes its entry file but cannot parse it, Review shows the
parse error and Apply refuses that provider until it is excluded or corrected.


### niri mapping

| niri source | Ryoku destination | Notes |
|---|---|---|
| `input.keyboard`, `touchpad`, `mouse` | `desktop.input` | XKB, repeat, numlock, pointer acceleration, scrolling, tap and click settings |
| input focus, warp, workspace and modifier nodes | `wm.niri` | Keeps niri-only pointer and modifier behaviour out of neutral settings |
| `layout` gaps, frame and shadow | `desktop.appearance` | Border or focus-ring choice and niri-only frame details also populate `wm.niri` |
| column widths, centring, tabs, insert hint, struts, background | `wm.niri` | Proportions remain proportional; unsupported fixed widths are preserved |
| `animations` | `desktop.appearance.animations`, `wm.niri.anim` | Global off/slowdown plus all nine ease, spring or off animation kinds |
| `environment` | `desktop.env`, `desktop.apps` | `BROWSER` and `TERMINAL` become app roles; other variables remain environment rows |
| `spawn-at-startup`, `spawn-sh-at-startup` | `desktop.autostart` | Argument-form commands are shell-quoted before entering the neutral command field |
| `cursor` | `desktop.cursor` | Theme, size, typing hide and inactive timeout |
| `binds` | `desktop.keybinds`, `desktop.keybindRebinds` | Uses the provider's existing chord and action catalogue; lock/repeat/cooldown metadata is reported as a loss |
| `window-rule` | `desktop.windowRules` | Supported matches and actions become one neutral rule per action |
| `layer-rule` | `wm.niri.layerRules` | Namespace, opacity, radius, blur, shadow and screencast fields |
| outputs | marked provider user include | There is no persistent neutral output-layout store; output blocks remain native |
| hotkey overlay, gestures, overview, recent windows and other modelled niri nodes | `wm.niri` | The same fields the niri generator writes |

### Loss policy

Nothing disappears silently. Every unsupported node or unsupported part of a
rule is returned in `losses`, shown in Review, and copied into one timestamped
`ryoku-import` block in the provider's user include. Re-import replaces that
block instead of stacking another copy. A keybind action with no neutral mapping
is kept in a slash-dash-disabled `binds` block so it cannot silently replace a
shipped shortcut. Bind metadata that has no neutral field is the one intentional
lossy mapping: the chord and action are imported, the metadata loss is listed,
and the original bind is not guessed into unrelated fields. Output blocks remain
native for the same reason.

Importing a provider tree while another provider is running writes the neutral
keys and the target `wm.*` namespace but does not regenerate or reload either
desktop. The target provider applies those settings when it is active.

## Architecture

One engine, provider-owned syntax, no duplicated parser:

- `ryoku-hub import providers|detect` reads importer metadata without inferring
  support from a filename extension.
- `ryoku-hub import scan <path|url>` asks each installed provider to scan the
  source, then merges its app/items/conflicts/losses model with app config scans.
- `ryoku-hub import apply <decisions.json>` re-scans, merges typed patches into
  `desktop.json`, writes marked native preservation blocks, backs up every path,
  and applies only when the imported provider is the live provider.
- `ryoku-hub import undo [<ts>]` restores the manifest byte for byte.
- A provider exposes `import metadata` and `import scan <source>`. The niri
  implementation lives under `ryoku/wm/niri`; no Hub code parses KDL or knows
  native action names.
- The Hub page consumes the metadata and scan JSON. Provider ids are opaque in
  QML, so adding another importer needs no page branch.

## Data contracts

- scan output (stdout JSON):
  `{ source, apps: [ { id, name, present, path, summary, error?,
     items: [ { kind, raw, combo?, dispatcher?, ingestable } ],
     conflicts: [ { combo, norm, ryoku: { action, desc }, mine: { raw, desc }, kind } ],
     losses: [ { raw, reason } ] } ] }`
  where conflict `kind` is `shipped` or `duplicate`.
- decisions (Hub to `apply`, JSON):
  `{ source, apps: { <id>: { include: bool } },
     conflicts: { <norm>: "ryoku" | "mine" | { remap: "<combo>" } } }`
- backup manifest (`~/.config/ryoku/import-backups/<ts>/manifest.json`):
  `{ ts, snapshot?, files: [ { path, backup } ] }`, so undo is a pure restore.

## Failsafe and undo

Before writing anything: take a snapper snapshot when snapper is available, and
always copy every file that will be touched into
`~/.config/ryoku/import-backups/<ts>/` with the manifest above. Apply writes to
temporary files, validates format-specific generated content, then swaps. A live
target provider regenerates its config only after the store write succeeds. Any
failure rolls back from the backup. `ryoku import --undo` and the Hub's Undo
button restore the manifest byte for byte.

## Delivery

`ryoku-hub`, `ryoku`, the Hub QML and provider binaries already ship through the
desktop packages, so this reaches users through `ryoku update` with no new
seeded config. The backup directory is created on demand. No `shell.json` key is
added or removed, so no doctor reconciler is needed.

## Testing

- Provider tests parse a realistic KDL tree with includes, raw strings, comments,
  input, layout, animations, binds, rules, outputs and unknown nodes, then assert
  the exact neutral patch, conflicts, losses and preserved KDL.
- Hub tests scan a provider result, apply it in an isolated config home, and
  assert that undo restores every touched byte.
- Smoke proof uses the same fixture through the built provider and Hub binaries,
  then validates the generated niri tree with `niri validate`.

## Future (post v1)

Deep ingest for more apps (starship, yazi, nvim), private git auth, and an
export counterpart so a Ryoku box can hand its config to another.

# Ryoku vault

The Ryoku vault is the machine map shared by coding agents on a Ryoku desktop.
It belongs to Ryoku core and works without Rashin installed or enabled. Rashin
can read and index it when the user turns Rashin on, but it does not own the
vault.

## Paths

The live vault is `$XDG_DATA_HOME/ryoku/vault/`, normally
`~/.local/share/ryoku/vault/`. The desktop package ships its read-only source
assets under `/usr/share/ryoku/vault/`:

- `wiki/` is the plain-language Ryoku guide set.
- `skills/ryoku/` is the desktop operation skill linked into supported harnesses.
- `ryoku-repo.md` is a source-tree map generated while the package still has the
  checkout.

`ryoku vault path` prints the live path and `ryoku vault status` reports whether
it exists, matches this Ryoku version, is wired, or has a Rashin 1 vault left to
migrate.

## Generated and user-owned content

`ryoku vault refresh` creates `AGENTS.md` and a `CLAUDE.md` symlink to it,
mirrors the shipped wiki, and regenerates:

- `system.md`: CPU, memory, kernel, GPU, displays, and disks.
- `desktop.md`: compositor, subsystem owners, config paths, reload commands, GUI
  entry points, and the bar and dock map.
- `packages.md`: explicitly installed packages, the installed count, and
  pending updates.
- `ownership.md`: the same ownership rules exposed by `ryoku owner --map`.
- `logs.md`: exact journal, Quickshell, update, doctor, and crash commands.
- `user.md`: config differences from the shipped baseline.
- `ryoku-repo.md`: the shipped or checkout-generated source map.

Generated bodies sit between
`<!-- ryoku-vault:generated:begin -->` and
`<!-- ryoku-vault:generated:end -->`. Refresh replaces only that fenced body.
Prose outside the fence remains.

`memory/`, `journal/`, and any other user files are not generated. Agents and
users own them. `journal/` uses dated files such as `2026-11-10.md`; `memory/`
holds durable facts worth carrying between sessions. The wiki mirror is
read-only and follows the package on refresh.

## Harness pointers and skill links

`ryoku vault wire` adds one fenced pointer to the instruction file of each
installed harness:

- `~/.claude/CLAUDE.md`
- `~/.codex/AGENTS.md`
- `~/.config/opencode/AGENTS.md`
- `~/.omp/agent/AGENTS.md`

A harness counts as installed only when its parent directory already exists.
Wiring does not create a new harness home. The block is fenced by
`<!-- ryoku-vault:begin -->` and `<!-- ryoku-vault:end -->`, so repeated wiring
is stable and user prose remains untouched. Wiring also removes the old
`ryoku-rashin` pointer block.

The shipped `ryoku` skill is linked into the existing Claude, Codex, omp,
`~/.agents`, and `~/.hermes` skill parents. An old skill symlink is repointed to
the core-owned source. A real file or foreign skill directory is never replaced.

`ryoku vault unwire` removes the pointer blocks and owned skill links, then
writes `$XDG_STATE_HOME/ryoku/vault-unwired`. The doctor and update refresh keep
wiring off while that marker exists. `ryoku vault wire` removes the marker.

## Refresh and repair

`ryoku doctor` checks that the vault exists, has the current generated maps, is
wired unless the user opted out, and has no pending Rashin 1 migration. Its fix
is the same in-process refresh used by both update paths. A vault error is
reported but never blocks `ryoku update`.

Use `ryoku vault repo-index <checkout> <output>` to generate the same source map
from an installed CLI. Data package recipes do not build that CLI, so they call
the small `ryoku/vault/repo-index` build helper while the checkout is available,
then ship the result with the wiki and skill.

## Migration from Rashin 1

A Rashin 1 vault lived at `$XDG_DATA_HOME/ryoku/rashin/`. On refresh, Ryoku
recognizes its fenced `AGENTS.md`, merges `memory/`, `journal/`, and other
non-generated user files into the new vault, and removes the old directory.
Existing destination files are never overwritten. A conflicting migrated file
gets a `.rashin-1` suffix, with a number when needed.

The old generated maps, wiki mirror, source mirror, and instruction contract are
not copied. Ryoku regenerates their current core-owned forms. Migration is
idempotent and also runs directly with `ryoku vault migrate`.

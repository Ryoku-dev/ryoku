# Development

The loop, the gates, and how to add things without breaking the rules.

## The loop

Edit the repo, deploy, test on the running system.

- **Shell (QML + daemon):** `ryoku/shell/dev-run.sh` builds `ryoku-shell` and
  runs it from the checkout (`qs -p`, hot-reload). `ryoku/hyprland/dev-binds.sh on` binds the
  shell keys for the session; `dev-stop.sh` stops it. Your own `~/.config` is not
  touched.
- **Configs:** `ryoku deploy` builds the binaries and lays the repo into
  `~/.config` from a checkout (it runs `ryoku/shell/deploy.sh`); on an installed
  system `ryoku materialize` copies the base config in. Never edit `~/.config`
  and copy back.
- **Shaders:** a `.frag` needs `qsb --qt6 -o <name>.frag.qsb <name>.frag` beside
  it, and the compiled `.qsb` is committed. Quickshell reads a shader file once
  per process, so a rebuilt `.qsb` needs the surface's process restarted; a QML
  hot-reload keeps serving the old one and the change looks like it did nothing.

## Verify before committing

- Lua: `luac -p <file>` parses every changed Lua file.
- Shell scripts: `bash -n <file>`; the pre-commit hook also checks staged scripts.
- Installer: exercise the whole flow without a disk. The dry-run matrix runs the
  backend across every strategy and profile (from the repo root):

  ```
  for s in whole alongside; do for p in vm amd intel amd-nvidia; do \
    RYOKU_DRYRUN=1 RYOKU_DISK=/dev/vda RYOKU_PASSWORD_HASH=x \
    RYOKU_DISK_STRATEGY=$s RYOKU_PROFILE=$p RYOKU_REPO=$PWD \
    installation/backend/ryoku-install >/dev/null || echo "FAIL $s/$p"; done; done
  ```

  Then the focused checks for what you touched:
  - `tests/install-*.sh` mocked fixtures (no real device unless noted): the
    whole-disk partition plan, the free-space sizer, the Secure Boot
    preflight gate, the clock-skew heal, the dry-run step/sentinel matrix, the
    disk teardown, and the DNS, mirror, and chroot-safety gates. The `alongside`
    partitioner (`install-partition-alongside.sh`) is a real loop-device test and
    needs root.
  - `installation/tui`: `go test ./...` (layout math + safety gates).
  - `installation/tests/iso-stage-check.sh` stages the ISO twice and diffs, so the
    build stays byte-reproducible (skips cleanly without `go`/`cmake`/`ninja`).
  - `installation/tests/iso-preflight.sh` runs the whole root-free installer gate
    in one command (syntax, ShellCheck, package lists, offline install, boot menu,
    contract suite, TUI, delivery). Build ISO runs it as a blocking job before
    mkarchiso; run it yourself before dispatching a build.
- VM green is not metal green. A clean VM install still misses the real-hardware
  classes (Intel VMD, Secure Boot, NVIDIA modeset, Windows dual-boot, Broadcom,
  clock skew, NVRAM, USB media). Before calling an installer change done, walk
  the matching entry in `docs/installation-hardware.md`.
- QML: `qmllint` when available.
- Test behavior, not just that it parses. Exercise the actual change on the
  running system.

## Adding things

- **A package:** the right set in `system/packages/` (`base` for everyone,
  `dev` for toolchains, `hardware` per profile, `aur` for the AUR). Prefer the
  official repos over the AUR when both have it.
- **A keybind:** `ryoku/hyprland/modules/binds.lua`.
- **A Hyprland concern:** a new module under `ryoku/hyprland/modules/` plus one
  `require` in `hyprland.lua`. Do not grow an unrelated module.
- **A shell surface:** a new component under `ryoku/shell/quickshell/`, with any
  state wired through `ryoku-shell` (`ryoku/shell/ipc/`).
- **A QML plugin (C++):** build it against the installed Qt and rebuild it on
  every Qt update. Qt's private API carries no cross-version promise, so a stale
  module (or a stale `quickshell` from the AUR) fails to load and takes the whole
  surface down to a black screen. `deploy.sh` stamps `Ryoku.Blobs` with the Qt it
  built against and rebuilds when that changes; `ryoku doctor` reports a renderer
  or module that cannot load.
- **A system helper:** a `ryoku-<thing>` script under `system/hardware/.../`,
  shipped to `/usr/bin` by the `ryoku-desktop` package (its PKGBUILD installs
  every `system/hardware/*/ryoku-*`), and invoked by name from Lua autostart or
  a keybind.

## How a change reaches users

Where a change lives decides whether, and how, it reaches an installed machine.

- **Desktop config and binaries (`ryoku/`)** reach users through `ryoku update`:
  config is re-laid by `ryoku materialize` (override-safe), binaries come from the
  signed `[ryoku]` repo. They reach the testing channel on every push to
  `unstable-dev` and stable when a release is tagged (`docs/updates.md`,
  "Publishing: releases and channels").
- **Push, or work on a branch that is not the channel.** `ryoku update` on a
  checkout reconciles the branch it is ON onto `origin/<channel>`: a clean
  fast-forward when it can, and a `git reset --hard` when the branch has diverged
  (`ryoku/cli/internal/updater/channel.go`, `syncChannel`). The channel branch is
  meant to mirror upstream, so **commits sitting unpushed on `unstable-dev` are
  dropped by the next `ryoku update`** (they survive only in the reflog). Push
  them, or keep them on any other branch. Local edits to tracked files are never
  touched: the deploy runs on what is checked out.
- **The installer (`installation/`)** runs once from the ISO. Fixes here reach
  only new installs from a new ISO, never an existing machine.
- **Package-set additions (`system/packages/`)** are pacstrapped at install.
  `ryoku update` upgrades installed packages; it does not pacstrap newly listed
  ones, so a new package reaches only fresh installs.
- **Stateful drift** the declarative layers cannot express (disk layout,
  subvolumes, swap) is healed by an idempotent `ryoku doctor` reconciler that runs
  inside `ryoku update`.

There is no ordered migration ledger. Config is reconciled declaratively by
`materialize`; stateful drift is reconciled by `ryoku doctor`. Reach for a
reconciler only when a fix must change an existing machine's structure and neither
a package nor `materialize` can do it.

### Adding a doctor reconciler

A reconciler is one entry in `reconcilers()` in `ryoku/cli/doctor.go`. It must be
idempotent: report `ok` when the machine already matches the desired state,
otherwise converge (or, under `--check`, report what it would do). It runs on
every `ryoku update`, so keep the check cheap and the fix safe to repeat; auto-fix
only the exact known-safe case and warn on anything unexpected. Retire it once
every supported install has run it, so the set stays small instead of piling up.

## Binaries and package managers

- The desktop ships as signed pacman packages from the `[ryoku]` repo
  (`release/packages/`): `ryoku-shell`, `ryoku-hub`, `ryoku`,
  `ryoku-blobs`, and `ryoku-sidebarfx` build from source via their PKGBUILDs.
  The live ISO still prebuilds the installer TUI (`installation/iso/build.sh`);
  the installed desktop's binaries come from the repo, so never assume `go` at
  install time.
- AUR packages install in the post-install step (`installation/backend/lib/
  aur.sh`), not via pacstrap.
- User-level package managers install without root, into `~/.local/bin` (`npm`,
  `pip --user`, `go install`, `cargo install`, `pipx`, `mise`). Do not
  reintroduce root-global installs or assume `sudo`.

## Commit gates

Every commit passes the hooks in `.githooks/`; never use `--no-verify`.

- `commit-msg`: subject is `[area] scope: summary` with area in
  `global | installation | system | ryoku | docs | test | tooling | release`
  (shell uses `[global]`). No em-dash, no authorship/attribution trailer.
- `pre-commit`: no em-dash in text files, valid bash syntax on staged scripts,
  no filler comment lines.
- `pre-push`: shellcheck when installed.

One logical change per commit. Update the matching `CHANGELOG.md` in the area you
touched, and keep the change documented where future readers will look.

## Research

When something is unfamiliar, look it up against primary sources (the Arch Wiki,
the Hyprland wiki, Quickshell and Qt docs, each tool's own docs), cross-check
anything load-bearing, and confirm the result on the running system. Match
existing patterns in the repo over introducing a new one.

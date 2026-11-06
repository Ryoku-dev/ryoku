# Changelog: system/extras/

## Unreleased

### Added
- **`ryoku-pkg-blackarch` manages the repository used by profiles.** It pins
  accepted signing keys, installs the signed mirror list, keeps BlackArch last,
  and refreshes only its own package database.
- **`install bundle <id> --only name1,name2,...` installs a chosen subset.** The
  store's bundle preview passes the user's manual selection; without `--only`
  the whole-bundle install is unchanged. `bundle_items` filters to the
  comma-separated allowlist.

### Fixed
- **Bundles install on Fedora and Void.** `ryostore-install` routes package
  items through `ryoku-host` with the catalogue's edition table laid under
  Ryoku's, asks `ryoku-host pkg status` once per bundle, marks a package the
  edition does not ship unavailable with the table's reason, and honours a
  `packageManagers` gate on a bundle or an item. `ryoku-pkg-add` and
  `ryoku-pkg-remove` carry the table across `sudo` as `--overlay`.
- **The CachyOS kernel toggle keeps CachyOS repositories below Arch, not above
  `[core]`.** `ryoku-pkg-cachyos` had drifted to add
  `[cachyos-core-v3]`, `[cachyos-extra-v3]`, and the baseline `[cachyos]` above
  Arch's repositories, pulling CachyOS's forked pacman and letting a later
  `pacman -Syu` convert the Arch userland, which is exactly what the toggle is
  documented to avoid (`docs/kernels.md`). It now inserts `[cachyos-v3]` and
  `[cachyos]` below every Arch repository and `[ryoku]`. The full-CachyOS
  install variant keeps its native stack via a separate path
  (`installation/backend/lib/cachyos.sh`). Existing v3-only above-`[core]`
  layouts are migrated the next time the helper runs.
- **CachyOS Gaming no longer risks converting an Arch userland.** RyoStore's
  renamed 1.1.0 bundle keeps id `cachyos-layer` and now installs the same
  kernel, tuning, scheduler, Steam, and CachyOS gaming set as the Arch installer.
  The helper leaves a CachyOS edition's native configuration untouched and
  falls back from keyservers to a checksum- and fingerprint-verified
  `cachyos-keyring` package.
- **A failed multilib enable now aborts the bundle instead of mis-routing its
  packages.** `ensure_requires` ran `ryoku-pkg-multilib || true`, so when the
  repo could not be enabled (sed no-op on a hand-edited pacman.conf, a failed
  `-Sy`) the bundle's lib32/Steam items no longer resolved as official and were
  silently routed to the AUR as source builds, failing late or building the
  wrong thing. It now dies with the real cause, matching the `cachyos`
  requirement's fail-closed behavior.
- **`ryoku-pkg-cachyos` and `ryoku-pkg-multilib` serialize their pacman.conf
  edits.** Both check-then-edit the file; two racing invocations (a double-fired
  Hub install in two floating terminals) could each pass the idempotence check
  and insert the repo stanza twice, leaving a pacman.conf pacman rejects. Both
  scripts now take the same `$XDG_RUNTIME_DIR/ryoku-pkg-conf.lock` flock for
  their whole body; the second holder just waits and exits on the idempotence
  check.
- **`ryoku-pkg-cachyos` hardens the `Architecture = x86_64_v3` edit.** The old
  sed only matched one exact spacing of an active `Architecture =` line and
  reported nothing when it matched nothing, so a nonstandard or hand-edited
  pacman.conf (spacing variants, a commented or absent line) kept rejecting
  every [cachyos-v3] package with "architecture x86_64_v3 not found" after the
  repo was added. Any active line now gets the value appended whatever its
  spacing; a config without one gets the line inserted under `[options]`
  (never by rewriting a commented line, which could sit inside a repo section
  and break pacman's parse); and the result is verified, so a remaining no-op
  exits loudly instead of stranding the repo.
- `ryoku-pkg-aur-add` refuses to run as root with one clear message. makepkg
  cannot build as root, so a root invocation used to die per package with
  yay's cryptic refusal instead of saying what to do (run it as your user).
- **Removing a bundle no longer dies on a provided package name, taking the
  whole batch with it.** A `package` item can be satisfied by a provider (the
  Influencer bundle's `whisper.cpp` item is provided by an installed
  `whisper.cpp-cuda`): `pacman -Q` resolves that, so the item read as
  installed, but `pacman -Rs` only accepts real installed names, aborted the
  entire transaction on "target not found", and every package in the bundle
  stayed put. `remove` now resolves each item to the package(s) that actually
  own it and hands pacman those; if the batch still fails (one target another
  package requires), the stragglers are retried one by one so a lone blocker
  costs itself, not the whole bundle.

### Added
- `ryoku-extras-install`: satisfy `"requires": ["gpu-lib32"]` by running
  `ryoku-gpu-lib32`, which installs the 32-bit GPU userspace matching the
  machine's hardware. The Gaming bundle declares it after `multilib` (enable the
  repo first, then pull the lib32 drivers), so a fresh Steam/Proton install
  renders on the real GPU instead of software. Best-effort like `multilib`: a
  driver hiccup never aborts the rest of the bundle.
- The extras subsystem that backs the Hub's Extras section: helpers shipped to
  `/usr/bin` by `ryoku-desktop` that install, remove, and report the optional
  bundles defined in the `ryoku-extras` catalogue.
- `ryoku-extras-install`: the actuator. Reads a bundle definition from the
  catalogue cache, detects each item, routes `package` items to the official
  repos (`ryoku-pkg-add`) or the AUR (`ryoku-pkg-aur-add`), runs `script` items'
  installers from the catalogue, defers `plugin` items to the shell, and publishes
  a per-bundle JSON report under `$XDG_RUNTIME_DIR/ryoku-extras/` that the Hub
  watches. `status` queries without changing anything; a bundle's
  `"requires": ["multilib"]` is satisfied before its packages are routed.
  `RYOKU_EXTRAS_DRYRUN=1` prints the plan and touches nothing.
- `ryoku-pkg-add`, `ryoku-pkg-aur-add`, `ryoku-pkg-remove`: thin pacman and
  AUR-helper wrappers for repo installs, AUR installs, and removals.
- `ryoku-pkg-multilib`: idempotently enables the `[multilib]` repo for bundles
  that need 32-bit packages (Gaming, for Steam and the lib32 libraries).
- `ryoku-pkg-cachyos`: idempotently adds `[cachyos-v3]` and `[cachyos]`
  (CachyOS key, x86-64-v3 only) below Arch's repositories and `[ryoku]`, so the
  CachyOS Gaming bundle can install `linux-cachyos` and its generic-repository
  packages through pacman without replacing the Arch userland.
- `ryoku-extras-install`: satisfy `"requires": ["cachyos"]` via
  `ryoku-pkg-cachyos`, and abort with a "run ryoku update" message on any
  unrecognized requirement instead of skipping it (which would mis-route the
  bundle's packages to an AUR source build).
- `ryoku-cmd-present`: the single command-presence test shared by the actuator
  and the catalogue's installer scripts.

# system/extras/

The extras subsystem: the helpers that install, remove, and report the optional
**bundles** offered under Ryoku Settings > Apps > Add-ons. They ship to `/usr/bin` with
`ryoku-desktop` and are driven by Ryoku Settings; nothing here runs at boot.

A bundle is a curated set of tools (packages, small installer scripts, and shell
plugins) defined in the `ryostore` catalogue
(`https://github.com/ryoku-dev/ryostore`, under `bundles/`). `ryoku-hub`
fetches and caches that catalogue; the helpers here do the work.

## The helpers

- `ryostore-install` the actuator. Reads a bundle definition from the
  catalogue cache (`ryoku-hub extras cache`), then installs, removes, or reports
  its items. Routes each `package` item through `ryoku-host` (see below), runs
  each `script` item's installer from the catalogue, and leaves `plugin` items to
  the shell's plugin path. It publishes a per-bundle JSON report under
  `$XDG_RUNTIME_DIR/ryostore/<id>.json` that the Hub watches for live state.
  `status` reports presence without changing anything; `install`/`remove` mutate
  and so run from the Hub's floating terminal, where `sudo` and the AUR helper
  have a TTY. `RYOSTORE_DRYRUN=1` prints the plan and changes nothing.
- `ryoku-pkg-add` install repository packages through `ryoku-host pkg install`
  (pacman, dnf or xbps), carrying the catalogue's package table across `sudo`.
- `ryoku-pkg-aur-add` install AUR packages with the system AUR helper (yay/paru),
  on Arch and CachyOS only.
- `ryoku-pkg-remove` remove packages: `pacman -Rs` on Arch and CachyOS, so
  orphaned dependencies go too, and `ryoku-host pkg remove` elsewhere.
- `ryoku-pkg-multilib` enable the `[multilib]` repo, for bundles that declare
  `"requires": ["multilib"]` (Gaming needs it for Steam and the lib32 libraries).
- `ryoku-pkg-cachyos` add the `[cachyos-v3]` repo (CachyOS key, x86-64-v3 only)
  so `linux-cachyos` installs through pacman, for bundles that declare
  `"requires": ["cachyos"]` (CachyOS Kernel). Additive and idempotent: it never
  touches `[core]`/`[extra]` or the stock kernel, and leaves out the baseline
  `[cachyos]` repo and its forked pacman.
- `ryoku-pkg-blackarch` enable, disable, refresh, or report the signed
  `[blackarch]` repository used by installable profiles. It verifies the keyring
  against pinned BlackArch fingerprints, installs the packaged mirror list, and
  refreshes only BlackArch metadata.
- `ryoku-cmd-present` the one presence test (`command -v`) shared by the actuator
  and the catalogue's installer scripts.

## Detection and routing

Bundle packages keep their Arch names. On Fedora and Void, `ryoku-host` names
each one for the edition from Ryoku's own package table with the catalogue's
`bundles/packages/<edition>.tsv` laid under it; `ryostore internal
package-table` hands the cached table to the actuator, which exports it as
`RYOKU_HOST_PKG_OVERLAY`. A bundle author only ever writes the Arch name.

Detection: on Arch and CachyOS a `package` item is present when `pacman -Qq`
finds it; elsewhere one `ryoku-host pkg status` call reports every package in
the bundle as installed, available, or unavailable with the table's reason. A
`script` item is present when its `detect` command is on `PATH`.

Routing: on Arch and CachyOS a package that resolves with `pacman -Si` is an
official-repo package, otherwise it is built from the AUR. Fedora and Void have
no AUR, so a package installs from the edition's repositories under its
translated name, or the item is unavailable and the Store shows why. A
`packageManagers` gate on a bundle or an item marks it unavailable outright on
the other editions, with the catalogue's sentence as the reason.

## Boundaries

`ryoku-hub` owns all network and disk for the catalogue (fetch, cache, installer
resolution); the catalogue helpers never fetch it themselves. Bundle package
transactions go through `ryoku-host` and, on Arch and CachyOS, the AUR helper.
The BlackArch helper fetches only its database, keyring package, and detached
signature over HTTPS; it accepts the keyring only from pinned signers, and its
mirror list comes from the repository as a signed package.

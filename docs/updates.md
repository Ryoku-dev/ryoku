# Updates and delivery

How a change in this repo reaches a running machine, and the contract that keeps
a user's install a mirror of a dev checkout. Read this before adding a config
file, a `shell.json` key, or anything a user must receive.

## Two lanes, and why

Ryoku owns its own layer and nothing under it. Two package lanes, deliberately
separate:

| Lane | Command | What moves |
|---|---|---|
| **Ryoku** | `ryoku update` | packages from Ryoku's signed pacman, XBPS, or RPM repository, then config and doctor |
| **Your distribution** | `sudo pacman -Syu` on Arch/CachyOS; `sudo xbps-install -Syu` on Void; `sudo dnf5 upgrade --refresh` on Fedora | the base system and its kernel, from the installed distribution |

On packaged Arch and CachyOS installs, `ryoku update` upgrades the installed
`[ryoku]` packages by name (`pacman -Sy`, then `pacman -S --needed
ryoku/<pkg>...`) and never runs a system upgrade. The reasons are the design:

- **The kernel is not ours to move.** Ryoku ships a plain Arch variant and a
  CachyOS variant and publishes neither kernel. A Ryoku release must not decide
  when your box changes kernel, rebuilds its DKMS modules, or rewrites its boot
  image.
- **A release has to be reversible.** `ryoku rollback` puts the Ryoku set back;
  it does not put the Arch, CachyOS, Void, or Fedora base back. Snapshots cover
  that layer where the edition supports them. An update that moved both was
  never fully reversible as a release.
- **The lanes fail apart.** A box that cannot take an Arch upgrade today (a
  mirror out of sync, a full boot partition) must still be able to take a Ryoku
  fix, and the reverse.

So a plain distribution update remains supported and is the only thing that
moves the kernel unless `ryoku update --system` is used. The direct commands are
`sudo pacman -Syu` on Arch or CachyOS, `sudo xbps-install -Syu` on Void, and
`sudo dnf5 upgrade --refresh` on Fedora. XBPS has no transaction snapshot
hooks, so Void users should use `ryoku update --system` when they want a base
update covered by Ryoku's pre/post snapshots. Fedora snapshots every dnf
transaction through a libdnf5 actions hook; `--system` still runs the Ryoku and
Fedora package lanes together, under one pair. On the
packaged lane, every `ryoku update` reports what the distribution lane is
holding (`N system package(s) waiting`), `ryoku status` prints it as `system:`,
and the Hub lists it under SYSTEM PACKAGES.

The desktop package's post-transaction power cutover also makes a direct
package update safe while graphical sessions are live. One temporary login1
sleep block covers the systemd reload; then each Ryoku user session (never the
SDDM greeter or another desktop) stops the old lid, idle, and shell owners and
must report its new sleep guard ready before the block is released. Pacman uses
its transaction hook, Fedora uses the RPM `%posttrans` script, and the Void
XBPS `INSTALL` action provides the same cutover. A managed `ryoku update` marks
its transaction so the package hook skips prepare and commit and stage two
remains the single cutover owner.

- A **dev box** runs the checkout: `ryoku deploy` builds the binaries and lays
  `ryoku/` into `~/.config`. `ryoku update` on it tracks `origin/main` (the git
  channel) and redeploys.
- A **user box** runs signed packages: `ryoku update` moves the Ryoku set from
  the native pacman, XBPS, or DNF repository, then `ryoku materialize`, then
  `ryoku doctor`.

They must converge. A change that lands on one but not the other is the bug this
page exists to prevent.

### Void

Void packages come from Ryoku's signed XBPS repositories. Stable releases use
`https://repo.ryoku.dev/stable/void/x86_64`; every `unstable-dev` push publishes
testing at
`https://repo.ryoku.dev/stable/void/channels/testing/x86_64`. The
`ryoku-keyring` package carries the trust key and stable repository
configuration. `ryoku update` refreshes the selected repository, updates the
installed Ryoku set, then runs the same stage-two cutover, materialize, and
doctor flow as Arch. `ryoku update --system` also updates the Void base system
inside the same snapper pre/post pair.

ISO installs create `@snapshots` at `/.snapshots`, use the shared snapper root
policy (number cleanup, 10 kept, no timeline), and run daily cleanup through the
`snapper-cleanup` runit service. Ryoku's Void package of
`limine-snapper-sync` runs its watcher as a runit service and maintains the
**Ryoku Linux -> Snapshots** menu. XBPS has no transaction hooks, so a direct
`xbps-install -Su` is not snapshotted.

The shell installer does not provision a bootloader or snapshot layout on an
existing Void system. Snapshot restore works there when the btrfs root and
snapper stack already exist; `ryoku doctor` converges the root config.

### Fedora

Fedora packages come from a signed RPM repository for each release listed at
`https://repo.ryoku.dev/stable/fedora/releases`, currently Fedora 44 and 45.
Release `<N>` uses `https://repo.ryoku.dev/stable/fedora/<N>/x86_64`; testing
uses `https://repo.ryoku.dev/stable/fedora/<N>/channels/testing/x86_64`. Before
sudo or any system change, the installer requires repository metadata for the
running release and selected channel. It refuses an unpublished release and
prints the supported list. Atomic and other rpm-ostree systems are refused.

The installer verifies and imports Ryoku's release key and writes
`/etc/yum.repos.d/ryoku.repo`. From then on the `ryoku-keyring` RPM, which
`ryoku-desktop` requires, owns the key file that `ryoku.repo` names as its
`gpgkey`, so dnf imports a rotated key on the next update. rpm holds its
transaction lock while scriptlets run, so the package cannot import the key
itself. The repository file stays installer-written because its URL follows the
channel `ryoku track` selects. Separate package-restricted COPR files supply the
Hyprland stack, starship, lazygit, and yazi packages Fedora lacks. Ryoku-owned
packages, including Quickshell, come from Ryoku's signed repository.

`ryoku update` refreshes the selected repository, moves the installed Ryoku RPM
set in one DNF transaction, then runs the same stage-two cutover, materialize,
and doctor flow. `ryoku update --system` also upgrades the Fedora base. On a
Fedora release Ryoku has not published yet, update and doctor explain that the
Ryoku packages remain at their current versions. Fedora keeps GRUB and uses
snapper on a btrfs root. Its **Ryoku snapshots** submenu can boot a read-only
snapshot, and `sudo ryoku boot-menu restore` makes that snapshot the system
again. Package rollback and the boot guard still use DNF downgrades for the
Ryoku set without moving the Fedora base or kernel.

### NixOS

Ryoku on NixOS lives in
[Ryoku-dev/Ryoku-on-NixOS](https://github.com/Ryoku-dev/Ryoku-on-NixOS). It
carries its own copy of this tree plus the NixOS module, installer and ISO, and
installed systems follow one of its branches through their `/etc/nixos` flake:
`unstable-dev` for unstable, `main` for stable. `ryoku update` there bumps that
flake input, builds the new system generation and switches to it, rolling back
the lock and the generation if any step fails.

The port's **Sync Ryoku** workflow keeps it on the same two lanes as every
other edition. Every 20 minutes it opens or updates one pull request per lane:
the newest `unstable-dev` commit whose Testing gate passed, and the newest `v*`
tag (which only exists after Stable Release). Nothing reaches NixOS users until
a NixOS maintainer merges that pull request; after the merge, the port's cache
workflow pushes the built system to the binary cache installed systems trust.
How a sync merges and how to resolve a conflicted one is in the port's
`docs/nixos.md`.

### Ryotunes by edition

Ryotunes releases independently on
[ryoku-dev/ryotunes](https://github.com/ryoku-dev/ryotunes). Arch and CachyOS
import the prebuilt package from that release lane. Void and Fedora package the
same application in their signed Ryoku repositories, so XBPS and DNF install
and update it with the rest of the native Ryoku set.

On Arch and CachyOS:

- **`ryoku update` installs a newer build.** It re-reads the latest release
  fresh, verifies the download by sha256 and by its own pacman metadata (name,
  version, `x86_64`), installs it with `pacman -U`, and only ever moves the
  version forward. A build that is not strictly newer is left alone, so an
  external build is never downgraded, and a box without Ryotunes installed gets
  nothing.
- **`ryoku doctor --check` reports a pending release** without installing
  anything. Advisory findings also appear with `--verbose` and `--json`; plain
  doctor remains quiet for advisory notes. Failed release lookups are reported
  as unavailable rather than "up to date".
- Ryotunes is **excluded from the `[ryoku]` update set** so the repo's base
  build can never overwrite a newer external one. Only the trusted
  `ryoku-dev/ryotunes` release path is used. The package lands through
  `pacman -U`, which honours pacman's signature policy and file ownership.

## `ryoku update`

On hosts that provide Ryoku's snapshot stack, a snapper pre-snapshot starts the
run and a post-snapshot closes it. On hosts without that capability, the
snapshot step emits one warning and is skipped; no snapper command or snapshot
helper offer runs.

Before a packaged run creates a snapshot or changes packages, it reads the
channel's `status.json`. A build still marked `building` refuses the run and
prints the Actions run URL. `ryoku update --wait` polls until that build
finishes, while `ryoku update --force` proceeds without waiting. If the last
build failed, update warns you and takes the previous good build, because a
failed publisher never moved the repository. The Hub reads the same state and
holds its Update button while the channel is building.

The channel follows (git fast-forward, or the host's signed package repository),
then stage2 runs through the just-installed binary. Package hooks first adopt
every live Ryoku session; on the hook's first release, stage2 invokes the same
all-session helper synchronously. For the invoking active session, stage2 holds
a durable login1 sleep block, stops the old shell, idle and clamshell owners,
materializes config, binds `ryoku-session.target` to the exact login1 session,
and requires the compositor's power bindings to reload. It starts the new
shell, waits for `sleep-ready`, verifies idle and clamshell, and restores a
previously running Ryogami before releasing protection. Any earlier failure
leaves the durable block active until retry or reboot. Longer `ryoku doctor`
and index work follows.

Each stage publishes to `$XDG_RUNTIME_DIR/ryoku-update.json` (the ordered steps
and their timings, snapshot capability and its reason, the current label, the
line the running work last printed, a heartbeat with the run's own watchdog
verdict, and, on failure, the error and any pre-update snapshot), so the Hub's
Updates page renders a determinate run and notices a stalled or vanished one.
The Hub offers one-click rollback only when snapshots are supported and the run
created one. It drives its own runs (`ryoku update --gui`) without a terminal;
the terminal shows a curated console and keeps the raw output in
`~/.local/state/ryoku/update-log.txt` (`ryoku update -v` streams it instead).

On the source lane, **Deploying the desktop** opens into the pieces it builds.
The terminal gives each piece its own progress bar, using progress reported by
the compiler when it is available or the duration remembered from the last run
in `~/.local/state/ryoku/deploy-pieces.json`. The first build on a box explains
that every piece is compiling from scratch and that later runs reuse the build
caches. The Hub shows the same list beneath the deploy step.

The database refresh happens before the set is read, so a move onto a frozen
release asks only for packages that release served. Pacman targets are
repo-qualified (`ryoku/<name>`); XBPS targets use their native names and `-f`
for an explicit channel downgrade; DNF targets the exact EVR served by the
selected repository.

After the desktop is back, the update refreshes Rashin's knowledge when it is
enabled: `ryoku-rashin index` regenerates the vault and reindexes the config
mirror with Prowl. Arch, CachyOS, and Void receive Prowl as its own signed
package; Fedora carries it inside `ryoku-rashin`. A dev box whose Prowl binary
is not managed by the native package manager runs `prowl update` instead. Both
refreshes are best effort and never fail the desktop update.

### Filtered DNS

The Ryoku mirror has no geographic restriction. If `repo.ryoku.dev` resolves to
a private or loopback address, the network's DNS path is filtering the domain;
the resulting pacman or installer TLS error is not a mirror outage.

Prefer a VPN or an encrypted resolver. On a system where systemd-resolved owns
DNS, configure a DNS-over-TLS provider that is reachable from the current
network, for example:

```ini
# /etc/systemd/resolved.conf.d/encrypted-dns.conf
[Resolve]
DNS=1.1.1.1#cloudflare-dns.com
FallbackDNS=9.9.9.9#dns.quad9.net
DNSOverTLS=yes
```

Restart `systemd-resolved`, then verify the answer:

```bash
sudo systemctl restart systemd-resolved
getent hosts repo.ryoku.dev
```

The result must be a public address. If encrypted DNS is also blocked, connect
through a VPN. As a last resort, resolve the host on a trusted connection and
temporarily pin that public address for `repo.ryoku.dev` in `/etc/hosts`.
Cloudflare addresses rotate, so remove the pin afterward and prefer encrypted
DNS for a lasting fix.

If the name resolves to a public address but HTTPS connections still time out
or reset, changing DNS will not help. On Arch or CachyOS, route both parts of a
terminal update through a local HTTP proxy:

```bash
export https_proxy=http://127.0.0.1:7890
export HTTPS_PROXY=$https_proxy
ryoku update
```

The environment covers the requests `ryoku` makes itself, but `sudo` normally
drops it before pacman runs. To route pacman's downloads too, add this under
`[options]` in `/etc/pacman.conf`, replacing the address with the local proxy:

```ini
XferCommand = /usr/bin/curl --proxy http://127.0.0.1:7890 --location --continue-at - --fail --output %o %u
```

Run the update from the same terminal. `XferCommand` applies to every pacman
repository, so remove it when the proxy is no longer needed. It changes only
the download route: leave the `[ryoku]` repository's `SigLevel = Required` in
place so pacman continues to verify the signed database and packages.

### The boot guard

A packaged update that moves the box to another tagged release arms a boot
guard. Stage two writes `/var/lib/ryoku/update-pending.json` with the previous
release, new release, pre-update snapshot, and current boot. The shell daemon
records a good boot once the shell has stayed up for 45 seconds.

The early-boot service is `ryoku-boot-guard.service` on Arch/CachyOS and
Fedora, and the `ryoku-boot-guard` runit service on Void. After two boots where
the desktop never came up, it pins the previous tagged release, moves the whole
Ryoku set back in one native package transaction, re-materializes every user's
config, and leaves the distribution base and kernel untouched. On
snapshot-capable Arch and Void installs, a third failed boot selects the full
pre-update snapshot path and clears Limine's remembered EFI entry so the
fallback wins. Fedora keeps the package downgrade and sends only the next boot
to the pre-update snapshot's read-only GRUB entry; `sudo ryoku boot-menu
restore` keeps it.
`sudo ryoku boot-guard --disarm` clears a pending marker by hand.

Doctor enables the host's service and prepares its state directory, so older
installs adopt the guard on update.

## materialize: the config a user receives

`ryoku materialize` lays the package's base config (`/usr/share/ryoku/config`,
mirrored by `ryoku/shell/deploy.sh` on a dev box) into `~/.config`:

- Every shipped file is copied over on every update (the previous Ryoku copy is
  clobbered) and files dropped from a release are pruned; `~/.config/quickshell`
  is converged wholesale.
- A short **seed list** (`generatedSeed` in
  `ryoku/cli/internal/updater/materialize.go`) is copied only when absent, never
  clobbered. It covers user-edited starting points such as Fastfetch and
  Starship, Matugen palette siblings, kitty, ghostty and nvim, the Vencord
  settings that turn on the Ryoku Discord theme in a Vesktop or Equibop profile
  that has none yet, plus every provider's per-machine files from `wm.ConfigSeeds`.
  Per-machine or user-owned state survives updates for every installed
  provider, not just the active one.
- The user overlay (`~/.config/ryoku/user_edits`, mirroring `~/.config`) is laid
  on top last, so a file there wins at its mirrored path; see below. Anything the
  package never ships (`hypr/user.lua`, `kitty/user.conf`, a forked module) is
  left alone regardless.

So the QML and the `Config.qml` defaults reach users on every update. A **new**
`shell.json` key is safe: the user's file lacks it, and the shell reads the new
`Config.qml` default.

Who runs it: the installer for the first user, `ryoku update` on every box that
has a config, and every graphical login whose home has no tree yet. The SDDM
session wrapper (`ryoku/lockscreen/sddm/ryoku-wayland-session`) lays the tree
as the user before the compositor reads its config, on systemd and runit alike,
so a user account created later or a package install never boots a bare
compositor; `ryoku-bootstrap` (a systemd user unit ordered before niri, a runit
user service on Void) covers the same condition for sessions that start without
the wrapper.

`/etc/skel` does not need a copy of the desktop tree. A newly-created home may
contain only the edition's skeleton files: the session wrapper runs
`materialize` under that account's UID, so every path it creates in the home is
user-owned. Installer-created extra accounts and accounts added later in Ryoku
Hub therefore take the same first-login path on every edition and compositor.

## user_edits: your edits, kept apart

Ryoku-owned config and user edits live in separate trees, so an update refreshes
the base freely while your edits stand. The base is the restore point; the
overlay is yours.

- **base** `/usr/share/ryoku/config` (the checkout on a dev box): pristine,
  re-laid in full on every update, so every fix and addition lands first.
- **user_edits** `~/.config/ryoku/user_edits`, mirroring `~/.config`, sparse:
  only what you changed. `materialize` overlays it last, so a file here wins at
  its mirrored path. Empty means pure base and the overlay is a no-op.

Two ways to override, neither of which blocks a fix:

- **Overlay (default).** The tool's own last-wins include: Hyprland loads the
  base modules, then `settings.lua` and `user.lua` last; kitty `globinclude`s
  `user.conf`. The base loads underneath, so a new upstream keybind still arrives
  while your file wins on what it sets.
- **Fork (opt-in).** A whole copy of a shipped file shadows the base one. You own
  it now, so an upstream fix to that file will not reach you automatically. Your
  forks are the files you see in the overlay; `ryoku reset <path>` takes the new
  base.

Ryoku Settings writes its generated `hypr/settings.lua` and `hypr/rebinds.lua`
into the overlay (authored under `user_edits`, reflected live). Its other state
(bar, colours, launcher, device lighting) it keeps under `~/.config/ryoku`,
GUI-managed and update-safe: the package ships no file there, so `materialize`
never clobbers or prunes it and a keyboard keeps the look you gave it across an
update. `ryoku reset` drops an override; `ryoku recovery` is the last
resort, wiping the overlay and that state back to shipped defaults.

## doctor: converging what materialize can't

`ryoku doctor` runs convergent reconcilers for the stateful drift materialize
can't state declaratively (disk, boot, session, and the user-owned
`~/.config/ryoku/*.json` materialize never rewrites). Reconcilers stand in for a
migration ledger: each is idempotent and safe on every update, and is retired
once every supported install has run it. `reconcileShellConfig` migrates a stale
`shell.json` (drops retired keys, revives the bar, clamps geometry).

### 0.75.5 shell migration gate

`TestShellMigrationFrom0755Fixture` runs the shipped 0.75.5 shell fixture
through every registered shell-store migration and then runs the chain again.
Every key the chain removes must be named in the test's documented-drop table;
an unlisted removal fails with the key name. This gate covers `shell.json`
only. It does not prove per-provider config trees, `user_edits` forks, or the
rollback path across the whole release jump.

`reconcileLauncherLocalFrostDefault` moves only the launcher's retired shipped
`bgBlur: 12` to the new 2 px local-frost default, then records a marker so a
later deliberate 12 remains a user choice.
`reconcileUserEdits` seeds the how-to guide and, for boxes upgraded from the
retired adopt step, moves the tool's own user files (`hypr/user.lua`,
`hypr/monitors_user.lua`, `kitty/user.conf`) back OUT of the overlay. Those are
edited in place; a frozen overlay copy of one used to be re-laid over the live
file on every update, wiping edits made afterward. Idempotent.
`reconcileMimeDefaults` clears the default-app map an older release froze into
`~/.config/mimeapps.list`: entries that only copy Ryoku's shipped values are
dropped (the file goes if that is all it held), and anything the user chose
stays. Ryoku's map ships to `/usr/local/share/applications/mimeapps.list`, the
site layer of the XDG mimeapps chain, so it sets the defaults without ever
outranking a user's pick and without colliding with a distro package that
owns `/usr/share/applications/mimeapps.list` (#295).
`reconcileShellInstances` clears a desktop that is running twice: a shell surface
orphaned by a daemon that was killed keeps drawing, and Quickshell allows a second
instance of one config, so the replacement draws over it. It keeps the instance
the supervising daemon started and stops the rest.
`reconcileShellLoad` gets a black screen back. Every surface is one Quickshell
instance, so a single QML file that cannot load takes the whole desktop, at login
and after an update alike. It reads the load failure from the shell daemon's
surface log (or loads the config once when there is no log), scopes the repair to
the module the loader blamed, moves a user override that breaks the desktop aside
as `.broken`, puts back every shipped file the live tree no longer matches, and
restarts the shell. When the shipped file is itself at fault it says so and names
`ryoku update` and `ryoku rollback`, the two things that help.
`reconcileWmPlugins` keeps a compositor's enabled plugins loading across a
compositor bump: a plugin is ABI-locked to the exact build and every copy Ryoku
builds carries an ABI receipt, so after an update it rebuilds each enabled
plugin whose receipts no longer match the installed headers through the provider
(`ryoku-hub desktop plugins rebuild --stale`, the Plugins page's builder) before
the next login, and names the toolchain to install when a box has none. Gated on
`CapPlugins`, so a compositor with no plugin system (niri) is a no-op. See
`docs/hyprland-plugins.md`.
`reconcileManifest` converges the box's package set to the release's control
manifest. It reads the channel's `manifest.json`, diffs it against the baseline
the box last converged to (saved with the names installed at that moment), and
installs what the release wants that this box never received, which is what
makes a package added to a set reach every box on the next update without a hard
depend, and heals a box that has been missing one all along. A name present at
the baseline and gone now was deleted by the user and stays gone; a name the
release retired is reported, never uninstalled. A name another installed
package provides (rustup for rust, tuned-ppd for power-profiles-daemon) counts
as present, so the box's own choice is never overwritten, and the opt-in names
(`ryokumanifest.OptIn`: hardware packages, the Limine hook and snapshot sync,
and prowl, which only ever arrives as ryoku-rashin's dependency)
belong to their gated reconcilers and are never converged. On a CachyOS box
(os-release `ID=cachyos`) the manifest is read without the names its layer
replaces (`cachyosReplaces`, from `system/packages/cachyos.replaces`: the stock
`linux` kernel and `yay-bin`), so a box converted in place, which has no ledger
entries for them, is never offered them either. The deliver-once
apps stay `reconcileShippedApps'` lane, so one update never runs two
transactions over the same names. Best-effort: a box with no mirror or no
network reports what did not land and the update moves on. `ryoku verify`
answers the same diff read-only, so two machines can be compared line by line.

## More than one compositor

A box can have every compositor installed and switch between them. Update, doctor
and recovery reach the compositor only through the seam (`ryoku/wm/`), so none of
them names one. Update and doctor keep the inactive compositor's config
untouched; recovery deliberately resets every installed one.

- **`ryoku update`** re-lays the base config, then reloads the active compositor
  through `wm.Open()` (`update.go` `pauseConfigAutoreload`/`reloadConfig` call
  `Act(ActionConfigReload)`; a compositor that watches its own file no-ops the
  reload). The seed list folds in every provider's per-machine files from
  `wm.ConfigSeeds` (`ryoku/cli/internal/updater/materialize.go`), so an update
  while niri is active never clobbers or prunes `hypr/*` seeds, and the reverse.
- **`ryoku doctor`** repairs compositor state through the seam and only for the
  running provider: it points xdg-desktop-portal at that provider's
  `Caps.PortalBackend` (`reconcilePortalRouting`), rebuilds stale window manager
  plugins only when the provider declares `CapPlugins` (`reconcileWmPlugins`; a
  compositor with no plugin system reports no plugin support), and writes the
  overlay how-to guide against the active provider's own config files
  (`reconcileUserEdits`, so it names `niri/user.kdl` on niri and `hypr/` paths on
  Hyprland). It does not touch the inactive compositor.
- **`ryoku recovery`** clears the `user_edits` overlay and the neutral Hub
  stores, then removes every path that `ryoku wm reset-paths` prints: each
  provider's generated config plus its hand-edit files (`wm.ResetPaths` over
  `wm.Providers`), for every installed compositor. It runs that
  command from the freshly fetched checkout first (`go run . wm reset-paths`), so
  a broken installed build cannot skew the list, then redeploys the shipped
  defaults. The per-machine seeds (monitors, gpu, keyboard) and saved rices are
  not in that set and survive. `--no-packages` skips the native package step; it
  refuses on a machine that is not Ryoku.
- **Switching** (`ryoku wm use <name> [--keep-previous|--remove-previous]`)
  installs the target's package; removing the old compositor reclaims its
  packages. `wm.Reclaim` computes the free set from the outgoing provider's
  `Caps.Packages`, and the package and byte counts shown come from the native
  package manager's own removal plan, re-checked immediately before the
  transaction. The full switch contract is in `docs/compositors.md`.

## Publishing: releases and channels

The package repositories and ISOs live in the `ryoku-iso` bucket below
`stable/`, served at `https://repo.ryoku.dev/stable` and, for ISOs,
`https://iso.ryoku.dev/stable`. Arch and CachyOS share the bucket root and one
`[ryoku]` repository. Void uses the same layout below `void/`; Fedora release
`N` uses it below `fedora/N/`. These paths are relative to an edition root:

| Directory | Purpose | Written when |
|---|---|---|
| `x86_64/` | moving stable head | a release tag is published |
| `channels/testing/x86_64/` | moving packaged testing head | every push to `unstable-dev` |
| `x86_64/status.json`, `channels/testing/x86_64/status.json` | state of the publisher moving that channel | when its build starts, publishes, or fails |
| `channels/testing/checked.json` | source-lane Testing gate state and last passing commit, shared by every edition | on every push to `unstable-dev` |
| `releases/<tag>/x86_64/` | frozen stable release | the tag is published |
| `releases/index.json` | stable release ledger, newest first, with each release's ISO per variant | after each stable publish or ledger rebuild |
| `channels/testing/builds/<build>/x86_64/` | frozen tested unstable build | every successful `unstable-dev` publish |
| `channels/testing/index.json` | unstable build ledger, newest first | after each unstable publish or ledger rebuild |

The unstable directory name replaces `+` in the release name with `_`, so
`v1.0.0.dev.12+g1234567` is stored below
`builds/v1.0.0.dev.12_g1234567/`. The ISO files sit at the bucket
root with their `.sha256`, `.sig`, `.json`, and `.js` sidecars. The
`latest*.json` files point to current images.

A box can move between the ten retained frozen targets in its channel, on any
edition. The stable ledger's `images` map names the Arch, CachyOS, or Void
image for each release, derived from the per-ISO manifests in the bucket.
Fedora ledger entries carry packages only. Each build carries a strictly
increasing package version (`core.r<commit-count>.g<sha>` for pacman and the
corresponding native version for XBPS or RPM) that the Ryoku upgrade moves to.
The `ryoku-desktop` package writes `/etc/ryoku-release` (`RELEASE=`,
`CHANNEL=`, `VERSION=`, `COMMIT=`) so a box can say which release or build it
runs.
`release.json` beside each moving or frozen repository says what it serves.
The pacman repository also publishes `manifest.json`, which lists every package
by lane (base, dev, hardware, AUR, first-party, compositor, provisioned).
`build-repo.sh` generates both files; they are never hand-edited. The Fedora
repository publishes its own `manifest.json`, generated by `build-rpm-repo.sh`
with `ryoku-manifest -edition fedora`: the same sets translated through
`fedora/packages/translations.tsv`, with the converge lanes limited to the
table's `desktop` lane, the set the Fedora installer installs.

`status.json` is the channel publisher's live marker. Its state is `building`,
`published`, or `failed`, and it names the commit, release, run URL, start, and
finish. Each edition writes it with `bin/ryoku-channel-status`; terminal states
keep the matching start time.

The source lane follows `channels/testing/checked.json`, written by
`bin/ryoku-checked-pointer`. Its state is `checking`, `passed`, or `failed`.
Every write carries the most recent passing commit, so a running or failed gate
cannot move a tester past the last commit that passed all required checks.

A release is a tag: `main` advances only by fast-forward from `unstable-dev`
(`RYOKU_RELEASE_PUSH=1 git push origin unstable-dev:main`; the `pre-push` hook
refuses every other push to `main`, see `docs/development.md`, "Branches and
pushing"), and publishing nothing on that push. The maintainer runs **Stable
Release** (`bump_type: none` tags the `VERSION` main already carries; a bump
rewrites it first). Because a tag pushed with the workflow token cannot trigger
tag-push workflows, Stable Release explicitly dispatches the pacman, Void, and
Fedora publishers on that tag. Each publisher writes its frozen
`releases/<tag>/` repository, moves its stable pointer, and rebuilds its ledger.
The pacman publisher then dispatches the plain Arch and CachyOS ISOs; the Void
publisher dispatches the Void ISO from its frozen repository. Arch itself keeps
rolling between releases; only the Ryoku set is frozen.

**Work on `unstable-dev` reaches source testers when the Testing gate passes,
and stable only when a release is tagged.** The packaged testing repository is
also rebuilt on every push, after its own build and install checks.

On a packaged box the channel is the native repository selection: the `Server`
line in pacman.conf on Arch/CachyOS,
`/etc/xbps.d/20-ryoku.conf` overriding the shipped XBPS stable repository on
Void, or the `baseurl` in `/etc/yum.repos.d/ryoku.repo` on Fedora. There is no
second channel state to drift from it:

- `ryoku track unstable` turns a box into a **source testing box**. It checks
  out `unstable-dev` from `https://github.com/ryoku-dev/ryoku.git` under
  `~/ryoku-arch`; `bin/ryoku-track` installs the host toolchain, and
  `ryoku/shell/deploy.sh` builds and deploys the checkout. Updates fast-forward
  only to the last commit the Testing gate passed. A bot-authored
  `release: bump unstable to ...` commit on top does not hold the source lane.
  When the gate cannot be read, `ryoku update` keeps the desktop that is
  running and deploys nothing, and it never lays down a checked commit older
  than the one already deployed. `ryoku update --wait` polls a running gate,
  while `ryoku update --force` deploys the current tip without waiting.
  `testing` remains a quiet synonym for `unstable`.
- `ryoku track stable` returns the box to signed stable packages. `ryoku track
  stable --source` follows `main` from a checkout. `ryoku track <version>`
  selects a frozen packaged release or unstable build and pins the box until it
  is tracked away. Pacman uses repo-qualified targets; XBPS force-refreshes its
  repository and uses `-f` when moving to an older frozen target; DNF installs
  the exact RPM versions served by the selected repository.
- The packaged testing channel still publishes every `unstable-dev` push and
  freezes the successful repository. Those builds remain available to `ryoku
  track <build>` and `ryoku rollback`, even though `ryoku track unstable` now
  selects the source lane.
- `ryoku rollback --to <version>` makes a frozen-target move in one native
  package transaction while the Arch, CachyOS, Void, or Fedora base stays
  current. A box pinned to an unstable build sees the retained unstable builds
  in `ryoku rollback`.
- `ryoku status` reports `release` (this box) and `channelRelease` (what the
  channel serves); `ryoku version` prints the release tag.
- The doctor names the channel it finds and warns, without touching it, when
  the configured repository is a mirror Ryoku does not publish.

### Release ledgers

`bin/ryoku-release-ledger <remote> [arch|void|fedora/<N>] [stable|testing]`
rebuilds one edition and channel from its frozen repository
directories. Arch writes `releases/index.json` and
`channels/testing/index.json`; Void writes the same paths below `void/`;
Fedora release `N` writes them below `fedora/N/`. Stable Arch and Void ledgers
include matching images; Fedora ledgers contain packages only. The publish
workflows upload `release.json` last as the frozen build's completion marker,
then rebuild the matching ledger. `release-ledger.yml` rebuilds every Arch,
Void, and supported Fedora ledger on demand. `RYOKU_RELEASE_BASE` overrides the
bucket base used for ledger and repository URLs, which keeps mirrors and tests
on one origin.

### Release-day checklist

1. Bring `main` up to the green `unstable-dev` tip (green means
   `channels/testing/checked.json` names that commit with `state: passed`).
   `main` takes changes only through a pull request: open one from
   `unstable-dev` and merge it with a merge commit, then fast-forward
   `unstable-dev` to that merge commit so `main` stays its ancestor. Then
   dispatch **Stable Release** on `main`. It creates the `v*` tag and dispatches
   `publish-repo.yml`, `publish-repo-void.yml`, and
   `publish-repo-fedora.yml` on that tag. Each publisher's gate job waits for
   the tagged commit's required checks before it uploads, including every check
   the merge into `main` started, so wait for those to pass before dispatching.
   A `VERSION` bump commit the workflow made on `main` has none, so the gate
   passes it after its grace period and the stable head writes `status.json` as
   `building` until the publish lands.
2. Wait for all three publishers. Their stable `release.json` files must name
   the tag at `stable/x86_64/release.json`,
   `stable/void/x86_64/release.json`, and
   `stable/fedora/<N>/x86_64/release.json`. Check the matching frozen files
   below each edition's `releases/<tag>/x86_64/`. Confirm
   `stable/fedora/releases` lists every release in the tagged
   `fedora/releases` file.
3. Wait for **Build ISO**, **Build ISO (CachyOS)**, and **Build Void ISO**. Check
   `https://iso.ryoku.dev/stable/latest.json`,
   `latest-cachyos.json`, and `latest-void.json`: each `installer_ref` must be
   the tag and every named ISO, checksum, signature, and manifest URL must
   return HTTP 200.
4. Dispatch `release-ledger.yml` on `main` after all three images upload. Verify
   `stable/releases/index.json` has the tag as `latest` with `images.plain` and
   `images.cachyos`; `stable/void/releases/index.json` has `images.void`; and
   every `stable/fedora/<N>/releases/index.json` has the tag as `latest` and no
   image map.
5. Dispatch `install-test.yml` on `main` manually. It gates Arch and CachyOS
   package installs and the plain Arch ISO VM path. Void's publisher separately
   installs its exact XBPS artifact in a clean Void container, and Fedora's
   publisher installs both desktop variants for every supported Fedora release
   and power-profile scenario. The Void ISO workflow runs its installer
   preflight and image-content checks, but release day has no automated Void VM
   install.
6. Verify `release-notes.yml` published the tag's GitHub release. It is
   dispatched by Stable Release and does not prove that repositories or images
   finished, so the endpoint checks above remain required.
7. Check `#updates` on the Discord. Once the Arch and CachyOS stable channel
   serves the tag, `publish-repo.yml` dispatches `release-announcement.yml`,
   which posts the release and pings the role in the `DISCORD_RELEASE_ROLE_ID`
   repository variable. It posts each tag once (`refs/notify/announced/<tag>`);
   if it never arrived, dispatch it on the tag by hand, with `force` to repeat
   one that did.
8. NixOS: within 20 minutes of the tag, Ryoku-on-NixOS opens a
   `sync: Ryoku <version> (stable)` pull request into its `main`. Stable NixOS
   users get the release once a NixOS maintainer merges it.
9. Publish the script installer the site shows: upload `main`'s
   `ryoku-shell-installer/install.sh` to the `ryoku-iso` R2 bucket as
   `install.sh` (`wrangler r2 object put ryoku-iso/install.sh --remote --file
   ryoku-shell-installer/install.sh --content-type application/x-shellscript`)
   and check that `https://shell.ryoku.dev/install.sh` matches it byte for byte.

The scheduled Cloudflare Worker in `release/r2-retention/` runs every six hours.
It keeps the newest ten stable releases and unstable builds per package edition
and Fedora release. For editions that publish images, it also keeps the newest
ten ISOs. An ISO and all its sidecars are deleted together. Incomplete frozen
directories without `release.json` are removed after 48 hours.

Retention never touches the moving repository heads, `latest*.json` pointers or
the ISOs they name, the ledgers except to remove deleted entries, the signing
key, or the bucket-root `install.sh`. Each run logs one JSON line per group and
an `r2_retention_total` line.

`ryoku track unstable` and `ryoku track stable --source` build and track a git
checkout instead of packages (see `docs/development.md`). An explicitly
recorded checkout selects this source lane on Arch, Void, or Fedora; packaged
installs simply never create that record.

### Release names

Every release line has a name from the creation stories Ryoku draws on (the
Kojiki and the Theogony), in the order those stories tell them; `CODENAME`
holds the current one and `release/names.md` tells each name's story. The
name changes when a line begins (the pre-1.0 line is Onogoro, the first
island; 1.0 is Amaterasu) and every release inside the line keeps it. It
travels with the release: `build-repo.sh` writes it into `release.json` and
the ryoku-desktop package into `/etc/ryoku-release` (`NAME=`), the publish
copies it into `releases/index.json`, the Stable Release and Release Notes
workflows title the tag and the GitHub release with it (a line's first release
opens with its story), and a box shows it in `ryoku version --pretty` (which
fastfetch's OS line uses), `ryoku status`, `ryoku rollback`, the update
island (when the channel serves the next line) and the Hub's Updates page.

### Discord

CI talks to the Ryoku Discord through one script, `bin/ryoku-discord`, so every
message shares one look, fits Discord's limits and pings no one unless it is a
release. Each channel has its own webhook secret:

| Secret | Channel | What arrives |
|---|---|---|
| `DISCORD_UPDATES_WEBHOOK_URL` | `#updates` | stable release announcements (pinging `DISCORD_RELEASE_ROLE_ID`), and the unstable roundup the Testing gate posts about once a day from `Note:` trailers |
| `DISCORD_FEED_WEBHOOK_URL` | `#repo` | new issues, new and merged pull requests, and batches of five or more `unstable-dev` commits (the release bot's bumps do not count) |
| `DISCORD_CI_WEBHOOK_URL` | `#ci-alerts` | failed publishes for every edition, failed Testing gates, and every ISO build result |

A missing secret skips the message, which is why forks and the NixOS mirror stay
quiet. `tests/ryoku-discord.sh` builds every kind of message offline.

## The contract

- **Ryoku moves through the native signed repository.** On Arch/CachyOS,
  `ryoku update` upgrades the installed `[ryoku]` packages by name and nothing
  else: no sysupgrade or kernel move. On Void it sends the installed,
  repository-served Ryoku set to `xbps-install -Syu`, the native safe-upgrade
  transaction required by XBPS. On Fedora it gives DNF the exact versions in
  the selected signed RPM repository, which supports upgrades and downgrades.
  Dependencies belong in package metadata, not source-build fallbacks. Nothing
  blocks a user's direct `pacman -Syu`, `xbps-install -Syu`, or `dnf5 upgrade`.
- **What a surface shows about the system must be read from the system.** No
  kernel, variant, or OS name is hardcoded into a menu, a default, or a report:
  boot entries come from the installed kernels
  (`/usr/lib/modules/*/pkgbase`), the boot default from
  `/etc/ryoku/default-kernel` (what the install chose) then the running kernel,
  and an entry whose image is not on the boot partition is removed. A box that
  never had the CachyOS kernel must never be offered it.
- **A user-facing config file must be delivered by a path a user runs**: shipped
  in a package (then materialized) or seeded by the installer. A file only
  `deploy.sh` lays, or one no path lays, reaches no user. `ryoku-dev-verify-delivery`
  fails the commit on such an orphan.
- **A package the release is made of must reach every box on update.** A name in
  a `system/packages/*.packages` set, the AUR set, or `release/packages/` is
  carried to a packaged box by the control manifest: `build-repo.sh` (and
  `build-rpm-repo.sh` for Fedora, which also needs a `desktop`-lane row in
  `translations.tsv`) generates `manifest.json` at publish time and the
  doctor's `reconcileManifest` converges
  each box to it on `ryoku update`, so adding a package to a set needs no hard
  depend and no per-box install step. Per-box picks from the installer never
  appear in the control manifest's convergence lanes, so convergence cannot
  undo them. A box's own removals are respected (a name present when its
  baseline was saved and gone now stays gone), and `ryoku verify` reports the
  box-vs-release diff so two machines can be proved the same one. The
  container-install gate (and the Fedora publish) fails a publish whose channel
  serves no manifest, because a reconciler with nothing to converge to is inert,
  not delivered.
- **A removed or renamed `shell.json` key, or a changed default that must reach
  existing users, needs a `doctor` reconciler** (materialize never edits a user's
  `shell.json`). An additive key needs nothing.
- **Never ship into a path the user's own tools write, and never write a tool's
  output into a shipped path.** `materialize` clobbers every shipped file, so
  laying Ryoku's defaults where an app writes the user's choice resets that
  choice on each update: `~/.config/mimeapps.list` did exactly that to default
  apps. Ship such defaults one layer down where the format provides one
  (`/usr/local/share/applications/mimeapps.list` for mime defaults), or make the file
  a `generatedSeed` if it has no layering. The same rule read the other way:
  a rice used to copy its emblem over the shipped
  `fastfetch/fastfetch-emblem.png`, and every update put the brand mark back.
  Anything Ryoku writes on the user's behalf (an imported logo, a rice asset)
  goes to a user-owned name the package never ships (`fastfetch/ryoku-logo.*`).
- **A user override belongs in `~/.config/ryoku/user_edits`, never in a shipped
  path.** The base still ships every file (the delivery check stays green) and
  the overlay wins on top. A whole-file fork opts out of upstream fixes for that
  one file, so prefer an overlay for anything additive. An edit made to a
  shipped file in place is not lost either: `materialize` notices bytes that
  match neither what it laid last time nor what it ships now, copies them into
  the overlay as a fork, lays the base, and lists the files it kept. Hyprland
  additions (rules, binds) belong in `hypr/user.lua` or the Hub, which are
  never re-laid.
- **Everything a user runs must converge on update, wherever it lives.**
  `materialize` covers `~/.config`; a payload installed elsewhere (the lock
  bundle under `~/.local/share`, the SDDM greeter skin under
  `/usr/share/sddm/themes`) needs a `doctor` reconciler that compares content
  with the shipped copy and re-lays it on drift, not one that only checks it
  exists. An install-once path silently pins every existing box to the release
  it was installed with: the lock shipped fixes for weeks that no updated box
  ever received.
- **A system path a package owns may exist unowned first, and the update adopts
  it.** The ISO installer and `ryoku/shell/deploy.sh` seed some paths before a
  package owns them: the privileged helpers and their polkit rules, the Plymouth
  theme, the shipped boot configs and lockscreen bundle, Wayfire's baseline
  config, portal routing, window-decoration plugin and plugin metadata (a
  checkout box has no Wayfire variant package), the boot hook, the modules-load
  and udev rules, the default-apps mimeapps map, and the logind lid-switch
  drop-in `/etc/systemd/logind.conf.d/10-ryoku-lid.conf`. An unowned
  copy collides with the package on the next transaction ("exists in
  filesystem") and aborts the whole atomic `-Syu`, so every ryoku pacman
  transaction -- `ryoku update`, the channel move, and the compositor switch
  (`ryoku wm use`) -- passes `--overwrite` for the seeded globs
  (`updater.RyokuOverwriteGlob`, fed by `unownedFiles`) and the doctor clears
  the same paths on a box already wedged (`reconcileConflictingRyokuFiles`,
  `ryokuSystemGlobs` -- the glob itself, split). Either way the package adopts
  the path and later updates own it normally; `deploy.sh` keeps its own copy of
  the list (`_rovw`) in sync.
- **Generated power policy and long-running helpers must be adopted as one live
  transaction.** `hypridle.conf` is rendered state, not materialized payload,
  and a running shell-script daemon keeps executing its old file after the
  package replaces it. Paired libalpm hooks preserve the pre-transaction
  executor and synchronously adopt every live Ryoku user after install,
  downgrade, or removal. The shared lifecycle helper selects one confirmed
  active Ryoku session per user, watches login1 activity/logout, and keeps its
  watcher retryable through D-Bus outages. Package stage two, login, and
  checkout deploy use the same durable login1 block while activating logind's
  sessionless fallback, proving old owners stopped, requiring the compositor
  bindings to reload, and replacing shell, idle, clamshell and wallpaper
  owners. Doctor instead stages a complete qylock repair under the generation
  guard for the next managed shell activation. Live-cutover protection releases
  only after the new shell reports its inhibitor state and service ownership is
  verified; failure remains blocked until a successful retry or reboot. The
  first package release cannot load its
  newly extracted hook: an updater-owned transaction suppresses the deferred
  install-script job and stage two invokes the all-session helper synchronously,
  while direct pacman installs schedule an unlimited-retry transient after the
  database lock drops. A `--no-reload` checkout deploy stages the drop-in
  without changing live logind, so the running session never mixes old and new
  halves.
- **One master per setting.** Two stores that both claim a value drift, and the
  next sync of either undoes the other: the colour master is `shell.json`
  `theme.theme` (the daemon shadows it into `theme.json` `followWallpaper` on
  every load), so the Hub's scheme cards and a rice select the theme through
  `ryoku-shell theme` instead of writing the shadow. A new setting gets one
  writer; every other surface reads.
- **Shipped QML must load, not just exist.** One file that cannot instantiate
  blanks its whole surface (a Hub page, a shell root), and Quickshell reports it
  only in the instance log. `bin/ryoku-dev-lint-qml` fails on the qmllint
  classes that mean "will not load", resolved against the installed modules;
  the publish gate runs it over the materialized tree.
- **Login restarts the session daemons.** The user manager can outlive a
  session (linger, a relogin after a compositor crash) and still hold a
  `ryoku-shell` bound to the dead compositor; `start` then does nothing and the
  login lands on bare Hyprland. The autostart reloads units, clears a start
  limit, and `restart`s the shell and wallpaper daemons every session.
- **Nothing reaches a channel while a required check is red.** Every publisher
  runs `bin/ryoku-ci-gate` before it uploads. New checks are required by
  default; the checks that do not gate delivery are the documented advisory set
  in that script.
- **A change reaches stable only when a release is tagged.** A source tester
  advances after the Testing gate passes; the packaged testing build follows
  its publisher on each `unstable-dev` push.
- **Displayed English must be wrapped where it is displayed, or it never
  translates.** `I18n.tr("...")` in QML, a Hub schema `label`/`desc`, `i18n.T`
  in Go, `log 'fmt %s'` in the installer's shell. The `i18n` workflow extracts
  only what is wrapped, so an unwrapped string ships English in all 35
  languages and no later pass finds it. Keep a sentence whole with `%1`/`%s`
  placeholders rather than concatenating fragments: a fragment freezes the word
  order to English. `docs/i18n.md` has the rules; a new language is one row in
  `ryoku/i18n/langs.json` and nothing else.

## Checks

- `.github/workflows/testing-gate.yml` runs on every `unstable-dev` push and
  records `checking`, `passed`, or `failed` in
  `channels/testing/checked.json`.
- `bin/ryoku-ci-gate` waits for every required GitHub check on a commit and is
  shared by the Testing gate and all three package publishers. Its advisory
  workflow set is the single list of checks that do not block delivery.
- `bin/ryoku-dev-verify-delivery` flags orphan configs (hard fail) and reports
  the publish lag. It runs in `pre-commit`, `post-commit`, and the Delivery check
  workflow.
- `bin/ryoku-edition-parity` keeps the Arch, Void, and Fedora package catalogues
  in step. It fails when an Arch package has neither a native build nor a
  translation-table row for either edition, and when an unavailable row has no
  reason. The Delivery check runs it on every push.
- `install-test.yml` installs the pacman artifact on Arch and CachyOS, then runs
  the plain Arch ISO installer in a VM when KVM is available. It must be
  dispatched manually after tag-dispatched ISO builds.
- `publish-repo.yml` builds and signs the pacman repository once, installs
  `ryoku-desktop` from that exact artifact on Arch and CachyOS with the release
  key verified, and only then uploads it. `publish-repo-void.yml` does the same
  with the XBPS artifact in a clean Void container.
- `publish-repo-fedora.yml` installs each exact RPM artifact on every supported
  Fedora release, for both desktops and all supported power-profile providers,
  before publishing. The Void ISO workflow has preflight and image-content
  gates but no full VM-install job.
- `bin/ryoku-dev-lint-qml <config-root>...` fails on QML that cannot load. The
  publish gate (`installation/tests/container-install.sh`) runs it over the
  materialized shell and Hub trees against the installed Qt modules, the same
  import path a user's session resolves; run it on a dev box after
  `ryoku deploy` before pushing a QML change.

# Updates and delivery

How a change in this repo reaches a running machine, and the contract that keeps
a user's install a mirror of a dev checkout. Read this before adding a config
file, a `shell.json` key, or anything a user must receive.

## Two lanes, and why

Ryoku owns its own layer and nothing under it. Two package lanes, deliberately
separate:

| Lane | Command | What moves |
|---|---|---|
| **Ryoku** | `ryoku update` | the packages the signed `[ryoku]` repo serves, the config, the doctor |
| **Your distribution** | `sudo pacman -Syu` | the base system and its kernel, from Arch or CachyOS, whichever you installed |

`ryoku update` upgrades the installed `[ryoku]` packages by name
(`pacman -Sy`, then `pacman -S --needed ryoku/<pkg>...`) and never runs a
sysupgrade. The reasons are the design:

- **The kernel is not ours to move.** Ryoku ships a plain Arch variant and a
  CachyOS variant and publishes neither kernel. A Ryoku release must not decide
  when your box changes kernel, rebuilds its DKMS modules, or rewrites its boot
  image.
- **A release has to be reversible.** `ryoku rollback` puts the Ryoku set back;
  it cannot put Arch back. An update that moved both was never fully
  reversible.
- **The lanes fail apart.** A box that cannot take an Arch upgrade today (a
  mirror out of sync, a full boot partition) must still be able to take a Ryoku
  fix, and the reverse.

So a plain `sudo pacman -Syu` is expected, supported, and the only thing that
moves your kernel. Ryoku ships no hook that blocks it. Every `ryoku update`
reports what that lane is holding (`N system package(s) waiting`), `ryoku
status` prints it as `system:`, and the Hub lists it under SYSTEM PACKAGES;
`ryoku update --system` runs both lanes in one command for those who want that.

The desktop package's post-transaction power cutover also makes a direct
`pacman -Syu` safe while graphical sessions are live. One temporary login1 sleep
block covers the logind reload; then each Hyprland or niri user session (never
the SDDM greeter or another desktop) stops the old lid/idle/shell owners,
must report its new sleep guard ready before the block is released. The first
release that introduces the hook schedules the same adoption after pacman drops
its database lock, because libalpm discovers hooks before extracting packages.

- A **dev box** runs the checkout: `ryoku deploy` builds the binaries and lays
  `ryoku/` into `~/.config`. `ryoku update` on it tracks `origin/main` (the git
  channel) and redeploys.
- A **user box** runs signed packages: `ryoku update` moves the `[ryoku]` set,
  then `ryoku materialize`, then `ryoku doctor`.

They must converge. A change that lands on one but not the other is the bug this
page exists to prevent.

### Ryotunes: an external app on its own channel

Ryotunes updates are released independently as prebuilt Arch packages on
[ryoku-dev/ryotunes](https://github.com/ryoku-dev/ryotunes)' GitHub
releases (`ryotunes-<ver>-1-x86_64.pkg.tar.zst`, with a `.sha256` beside it), so
it is a third channel a Ryoku box tracks directly rather than through the
`[ryoku]` repo. `ryoku update` runs the check on every channel (dev checkout and
packaged) and outside the `[ryoku]` set, so a box with no other changes still
picks up a new Ryotunes (`internal/ryotunesrelease`, `internal/updater/ryotunes.go`):

- **`ryoku update` installs a newer build.** It re-reads the latest release
  fresh, verifies the download by sha256 and by its own pacman metadata (name,
  version, `x86_64`), installs it with `pacman -U`, and only ever moves the
  version forward. A build that is not strictly newer is left alone, so an
  external build is never downgraded, and a box without Ryotunes installed gets
  nothing (a removal stays removed).
- **`ryoku doctor --check` reports a pending release** without installing
  anything. Advisory findings also appear with `--verbose` and `--json`; plain
  doctor remains quiet for advisory notes. Failed release lookups are reported
  as unavailable rather than "up to date".
- Ryotunes is **excluded from the `[ryoku]` update set** so the repo's base
  build can never overwrite a newer external one. Only the download origin Ryoku
  trusts (the `ryoku-dev/ryotunes` GitHub release path) is used, and the package
  lands through `pacman -U`, which honours pacman's signature policy and file
  ownership -- never a raw `/usr/bin` replacement. The `[ryoku]` repo still
  builds the `ryotunes` package (a sha256-pinned source tarball) for the initial
  install.

## `ryoku update`

Snapper pre-snapshot, then the channel (git fast-forward, or the `[ryoku]`
package set), then stage2 through the just-installed binary. Package hooks first
adopt every live Ryoku session; on the hook's first release, stage2 invokes the
same all-session helper synchronously. For the invoking active session, stage2
holds a durable login1 sleep block, stops the old shell, idle and clamshell
owners, materializes config, binds `ryoku-session.target` to the exact login1
session, and requires the compositor's power bindings to reload. It starts the
new shell, waits for `sleep-ready`, verifies idle and clamshell, and restores a
previously running Ryogami before releasing protection. Any earlier failure
leaves the durable block active until retry or reboot. Longer `ryoku doctor` and
index work follows. A snapper post-snapshot closes the run. Each stage
publishes to `$XDG_RUNTIME_DIR/ryoku-update.json` (the ordered steps, the current
label, a live log tail, and, on failure, the error and the pre-update snapshot),
so the update island and the Hub's Updates page render a determinate run and a
one-click rollback.

The database refresh happens before the set is read, so a rollback onto a frozen
release only ever asks for packages that release actually served; targets are
repo-qualified (`ryoku/<name>`), so pacman takes our build of a name that also
exists in `extra`, and moves it down as readily as up.

After the desktop is back, the update refreshes the agent OS when it is present:
`ryoku-rashin index` regenerates the vault and re-indexes the config mirror with
Prowl, then `prowl` is brought current. On a dev box (Prowl on PATH but
not owned by a pacman package) it runs `prowl update`; a packaged box
already got the new build from the `[ryoku]` set, so the step just logs that the
binary is managed by pacman. Both are best effort and never fail an update.

### The boot guard

A packaged update that moves the box to another release arms a boot guard:
stage2 writes `/var/lib/ryoku/update-pending.json` (previous release, new
release, the pre-update snapshot, the boot it ran in). `ryoku-boot-guard.service`
runs `ryoku boot-guard` as root early in every boot, before the display
manager, and only while that marker exists. The shell daemon records a good
boot once the shell has stayed up 45 s (`/var/lib/ryoku/boot/ok-<uid>`, the boot
id); a record from any boot other than the one the update ran in disarms the
guard. Without one, the boot counts: on the second, the guard tracks the
previous release back (`ryoku track <from>`, then an explicit
`pacman -S ryoku-desktop`; the Ryoku set only, Arch untouched),
re-materializes every user's config from it, and leaves
a notice `ryoku doctor` shows once. On a third it points the Limine boot menu
at the pre-update snapshot entry, for the case where the packages were not what
broke. `sudo ryoku boot-guard --disarm` clears a marker by hand. The `ryoku`
package ships the unit and its tmpfiles entry; the doctor enables the unit and
prepares the record directory on every update, so boxes installed before it get
it on their next update.

## materialize: the config a user receives

`ryoku materialize` lays the package's base config (`/usr/share/ryoku/config`,
mirrored by `ryoku/shell/deploy.sh` on a dev box) into `~/.config`:

- Every shipped file is copied over on every update (the previous Ryoku copy is
  clobbered) and files dropped from a release are pruned; `~/.config/quickshell`
  is converged wholesale.
- A short **seed list** (`generatedSeed` in
  `ryoku/cli/internal/updater/materialize.go`: `fastfetch/config.jsonc`,
  `kitty/current-theme.conf`, the ghostty and nvim starting points, plus every
  provider's per-machine files from `wm.ConfigSeeds`, e.g. `hypr/monitors.lua`
  and `niri/monitors_user.kdl`) is copied only when absent, never clobbered:
  per-machine or user-owned state an update must keep, for every installed
  compositor, not just the active one.
- The user overlay (`~/.config/ryoku/user_edits`, mirroring `~/.config`) is laid
  on top last, so a file there wins at its mirrored path; see below. Anything the
  package never ships (`hypr/user.lua`, `kitty/user.conf`, a forked module) is
  left alone regardless.

So the QML and the `Config.qml` defaults reach users on every update. A **new**
`shell.json` key is safe: the user's file lacks it, and the shell reads the new
`Config.qml` default.

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
stays. Ryoku's map ships to `/usr/share/applications/mimeapps.list` now, the
bottom of the XDG mimeapps chain, so it sets the defaults without ever
outranking a user's pick.
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
release retired is reported, never uninstalled. The deliver-once apps stay
`reconcileShippedApps'` lane, so one update never runs two transactions over the
same names. Best-effort: a box with no mirror or no network reports what did not
land and the update moves on. `ryoku verify` answers the same diff read-only, so
two machines can be compared line by line.

## Two compositors

A box can have both compositors installed and switch between them. Update, doctor
and recovery reach the compositor only through the seam (`ryoku/wm/`), so none of
them names one. Update and doctor keep the inactive compositor's config
untouched; recovery deliberately resets both when both are installed.

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
  `wm.Providers`), for both compositors when both are installed. It runs that
  command from the freshly fetched checkout first (`go run . wm reset-paths`), so
  a broken installed build cannot skew the list, then redeploys the shipped
  defaults. The per-machine seeds (monitors, gpu, keyboard) and saved rices are
  not in that set and survive. `--no-packages` skips pacman; it refuses on a
  machine that is not Ryoku.
- **Switching** (`ryoku wm use <name> [--keep-previous|--remove-previous]`)
  installs the target's package; removing the old compositor reclaims its
  packages. `wm.Reclaim` computes the free set from the outgoing provider's
  `Caps.Packages`, and the package and byte counts shown come from pacman's own
  removal plan, re-checked immediately before the transaction. Full switch
  contract in `docs/compositors.md`.

## Publishing: releases and channels

The `[ryoku]` repo is published into named states, all under the one bucket
mount the repo domain serves (`repo.ryoku.dev/stable/<key>` is bucket object
`<key>`; the `stable` path segment is the mount, not the channel):

| Directory | Channel | Written when |
|---|---|---|
| `x86_64/` | **stable**: the URL every installed box has | a release tag is published: a byte copy of that release |
| `releases/<tag>/x86_64/` | one frozen release; never rewritten | the tag is published (`publish-repo.yml` refuses an existing directory) |
| `releases/index.json` | the release ledger, newest first, with each release's ISO per variant (`images.plain`, `images.cachyos`) | after each release |
| `channels/testing/x86_64/` | **testing** | every push to `unstable-dev` |

So a box on stable moves between named releases, and can be put back on any
earlier one, on either variant: the `[ryoku]` packages are one `x86_64` build
that both variants install, the frozen release directories are never pruned,
and the ledger's `images` map names the Arch and CachyOS ISO of each release
(derived from the per-ISO manifests in the bucket, so it heals on every
rebuild) for a reinstall of an older release. Each build carries a strictly
increasing package version (`core.r<commit-count>.g<sha>`) that the Ryoku
upgrade moves to, and the `ryoku-desktop` package writes `/etc/ryoku-release`
(`RELEASE=`, `CHANNEL=`, `VERSION=`, `COMMIT=`) so a box can say which release
it runs; `release.json` beside each channel's db says which one the channel
serves, and `manifest.json` beside it lists every package the release is made
of, by lane (base, dev, hardware, AUR, first-party, compositor, provisioned),
generated from the checkout by `build-repo.sh` and never hand-edited.

A release is a tag: `main` advances only by fast-forward from `unstable-dev`,
and publishing nothing on that push. The maintainer runs **Stable Release**
(`bump_type: none` tags the `VERSION` main already carries; a bump rewrites it
first), which tags `main`, publishes `releases/<tag>/`, moves the stable
pointer onto it, records the ledger entry, and dispatches both release ISOs
(plain Arch and CachyOS) from that frozen directory, so an ISO named for a
release installs exactly that release. Arch itself keeps rolling between
releases; only the Ryoku set is frozen.

**Work on `unstable-dev` reaches testing on every push, and stable only when a
release is tagged.**

On a packaged box the channel is nothing but the `Server` line of the `[ryoku]`
stanza, so there is no second state to drift from it:

- `ryoku track unstable-dev` turns any box into a **testing box**: it follows the
  `testing` channel, rebuilt on every push to `unstable-dev`, so a tester gets
  each push as signed packages through `ryoku update`. `ryoku track main` returns
  it to **stable** (named releases). The two are aliases for `ryoku track testing`
  and `ryoku track stable`.
- `ryoku track stable | testing | v<tag>` rewrites that line and runs an update
  that moves the Ryoku set to what the channel serves, down as well as up: the
  databases are force-refreshed (a frozen release's db is older than the
  channel's, so pacman would keep the cached one), then the installed
  `ryoku/<pkg>` set is installed at the versions that channel publishes. A tag
  pins the box to that release until it is tracked away.
- `ryoku rollback --to v<tag>` is `track` onto a frozen release: the Ryoku set
  goes back in one pacman transaction while Arch stays current. Bare
  `ryoku rollback` lists the ledger and the snapshots.
- `ryoku status` reports `release` (this box) and `channelRelease` (what the
  channel serves); `ryoku version` prints the release tag.
- The doctor names the channel it finds and warns, without touching it, when
  `[ryoku]` points at a mirror Ryoku does not publish.

`ryoku track main | unstable-dev --source` is the developer path: it builds and
tracks a git checkout instead of packages (see `docs/development.md`). A box
already on a checkout is migrated onto packages by a plain track without
`--source`: the checkout is retired as the update source (the `~/ryoku-arch`
clone stays on disk) and `ryoku update` runs `pacman` from then on.

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

## The contract

- **Ryoku moves only what Ryoku publishes.** `ryoku update` upgrades the
  installed `[ryoku]` packages by name and nothing else: no sysupgrade, no
  kernel, no `--ignore` list to maintain. Anything Ryoku needs from the base
  system belongs in a package dependency, where pacman resolves it, not in a
  transaction that quietly upgrades the machine. A user must never have to
  choose between a Ryoku fix and their own upgrade schedule, and nothing may
  block `sudo pacman -Syu`.
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
  carried to a packaged box by the control manifest: `build-repo.sh` generates
  `manifest.json` at publish time and the doctor's `reconcileManifest` converges
  each box to it on `ryoku update`, so adding a package to a set needs no hard
  depend and no per-box install step. A box's own removals are respected (a name
  present when its baseline was saved and gone now stays gone), and `ryoku
  verify` reports the box-vs-release diff so two machines can be proved the same
  one. The container-install gate fails a publish whose channel serves no
  manifest, because a reconciler with nothing to converge to is inert, not
  delivered.
- **A removed or renamed `shell.json` key, or a changed default that must reach
  existing users, needs a `doctor` reconciler** (materialize never edits a user's
  `shell.json`). An additive key needs nothing.
- **Never ship into a path the user's own tools write, and never write a tool's
  output into a shipped path.** `materialize` clobbers every shipped file, so
  laying Ryoku's defaults where an app writes the user's choice resets that
  choice on each update: `~/.config/mimeapps.list` did exactly that to default
  apps. Ship such defaults one layer down where the format provides one
  (`/usr/share/applications/mimeapps.list` for mime defaults), or make the file
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
  theme, the shipped boot configs, and the logind lid-switch drop-in
  `/etc/systemd/logind.conf.d/10-ryoku-lid.conf`. An unowned copy collides with
  the package on the next transaction ("exists in filesystem") and aborts the
  whole atomic `-Syu`, so `ryoku update` passes `--overwrite` for the seeded
  globs (`updater.ryokuOverwriteGlob`, fed by `unownedFiles`) and the doctor
  clears the same paths on a box already wedged
  (`reconcileConflictingRyokuFiles`, `ryokuSystemGlobs`). Either way the package
  adopts the path and later updates own it normally; `deploy.sh` keeps its own
  copy of the list (`_rovw`) in sync.
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
- **A change reaches stable only when a release is tagged**, and testing on
  every `unstable-dev` push. Keep the gap small; the delivery check reports it
  on every push.
- **Displayed English must be wrapped where it is displayed, or it never
  translates.** `I18n.tr("...")` in QML, a Hub schema `label`/`desc`, `i18n.T`
  in Go, `log 'fmt %s'` in the installer's shell. The `i18n` workflow extracts
  only what is wrapped, so an unwrapped string ships English in all 35
  languages and no later pass finds it. Keep a sentence whole with `%1`/`%s`
  placeholders rather than concatenating fragments: a fragment freezes the word
  order to English. `docs/i18n.md` has the rules; a new language is one row in
  `ryoku/i18n/langs.json` and nothing else.

## Checks

- `bin/ryoku-dev-verify-delivery` flags orphan configs (hard fail) and reports
  the publish lag. It runs in `pre-commit`, `post-commit`, and the Delivery check
  workflow.
- The install-test workflow builds the ISO and runs a real, unattended install in
  a VM, then verifies the desktop comes up, so a broken install or a missing
  package is caught before a user hits it.
- The publish (`publish-repo.yml`) builds and signs the repo once, keeps it as
  a workflow artifact, installs ryoku-desktop from that artifact on Arch and
  CachyOS with the release key verified (`installation/tests/container-install.sh`
  with `RYOKU_PREBUILT_REPO=1`), and uploads the same artifact. What was tested
  is byte-for-byte what ships; a run that fails the gate publishes nothing.
- `bin/ryoku-dev-lint-qml <config-root>...` fails on QML that cannot load. The
  publish gate (`installation/tests/container-install.sh`) runs it over the
  materialized shell and Hub trees against the installed Qt modules, the same
  import path a user's session resolves; run it on a dev box after
  `ryoku deploy` before pushing a QML change.

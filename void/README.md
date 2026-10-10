# Void Linux

Everything Ryoku needs to run on Void Linux lives under this directory, one
concern per subdirectory. Void differs from the Arch base at two host
boundaries: XBPS replaces pacman, and runit replaces systemd. Package
translation, init integration, and ISO inputs stay here; runtime callers use
`ryoku-host` instead of carrying distro checks.

The validated target profile matches the one the ecosystem converged on
(iNiR's Void port, DankMaterialShell): **x86_64 glibc + elogind + runit +
Turnstile**. musl and seatd-only sessions are out of scope until the glibc
profile ships.

## Port index

The Arch and CachyOS paths remain the source definitions. Their Void
counterparts translate those definitions rather than duplicating the desktop.

| artifact | Arch/CachyOS path | Void counterpart | status |
|---|---|---|---|
| Package sets | `system/packages/` | `void/packages/translations.tsv`, `void/packages/sets/`, `void/packages/resolve` | Done |
| Package recipes | `release/packages/` | `void/packages/srcpkgs/` in the Ryoku XBPS repository | Done |
| Repository build | `release/repo/` | `void/packages/repo/` | Done |
| Live ISO | `installation/iso/` | `void/iso/` | Done; see [`iso/README.md`](iso/README.md) |
| Init services | systemd unit trees across the repository | `void/init/` | Done |
| Runtime host seam | direct systemd and pacman calls | `ryoku/cli/cmd/ryoku-host` and `ryoku/cli/internal/host` | Done |

`packages/translations.tsv` is the package-name and lane map for the complete
Arch package closure. `packages/resolve` selects lanes and emits Void package
names, while `packages/sets/` supplies the Void session and source-build
packages that have no Arch package-set row. `packages/README.md` describes the
signed XBPS repository and how to build it.

`init/` is the systemd-to-runit translation:

- `translations.tsv` is the completeness manifest: every systemd unit, target,
  timer, drop-in and environment generator in the repo maps to its runit
  translation or an explicit accepted-loss verdict. `tests/void-init.sh`
  fails when a unit exists without a manifest row, so a new unit cannot
  silently skip Void.
- `system/` holds system-level runit services, installed to `/etc/sv/<name>`
  and enabled from `/etc/runit/runsvdir/default/<name>`.
- `user/` holds per-user session services installed to
  `~/.config/service/<name>` for Turnstile.
- `env/xdg-dirs` renders the localized XDG directories into Turnstile's envdir
  format.
- `polkit/` records the policy differences whose systemd action ids do not
  transfer to runit.
- `lib/wait-for` provides dependency and environment waits shared by every
  translated service.
- `session/session-start` publishes the compositor environment and restarts
  the services listed in `session-services`.

`iso/README.md` specifies the Void live image, offline XBPS closure, shared
installer routing, and release workflow.

## Snapshots

Ryoku's snapshot stack is not available on Void Linux. Updates skip snapper and
show one warning instead, so they cannot be rolled back from the boot menu.

## Runtime layout

The packages place `wait-for`, `xdg-dirs`, `session-start`, and
`session-services` in `/usr/lib/ryoku/runit/`. System services are copied to
`/etc/sv/`; their enablement symlinks live in
`/etc/runit/runsvdir/default/`, including distro services that already exist
under `/etc/sv/`. Packaged user service definitions live under
`/usr/lib/ryoku/runit/user/`.

Turnstile supervises each user's services from `~/.config/service/` and reads
one environment variable per file from `~/.config/service-env/`. Before the
session roster starts, `ryoku-host session ensure --user` creates missing
services from the packaged definitions and refreshes changed run, finish, conf,
and log scripts. It leaves `supervise/` state and user disables alone. A dev
deploy installs its checkout definitions to the same packaged-source directory
and calls the same provisioning command, so there is only one copy path.

PipeWire, pipewire-pulse, and WirePlumber run as login services without `down`
markers because Void has no systemd user units to start them. Every other Ryoku
user service carries a `down` marker and waits for `session-start` after the
compositor exists. `ryoku-host svc disable --user` records `.ryoku-disabled`
beside the runit marker, so login preserves the user's choice. `svc enable`
removes both files.

Both compositor login entries call `ryoku-host session start`. On runit,
`ryoku-host` dispatches to `/usr/lib/ryoku/runit/session-start`, which publishes
the compositor's exported environment, renders the localized XDG directories,
updates D-Bus activation, and restarts the session roster in order. A stale
service from a previous login therefore reconnects to the new display instead
of remaining attached to the old one.

Turnstile uses elogind's `/run/user` ownership, so
`/etc/turnstile/turnstiled.conf` sets `manage_rundir = no`. Its per-user D-Bus
service links to the packaged `dbus.run` and `dbus.check` examples, while
`turnstile-ready/conf` declares `core_services="dbus"`.

Every translated run script sources
`/usr/lib/ryoku/runit/wait-for` and reloads the Turnstile envdir before reading
session values. `wait_env` polls the variable's envdir file when Turnstile is
active, then the run script reloads the envdir before executing its daemon.

## Turnstile and the session bus

Yes. A stock Void setup normally leaves the administrator to enable
`turnstiled`, add `pam_turnstile` to the login PAM stack, disable
`manage_rundir` when elogind owns `/run/user`, and make the per-user D-Bus
service a `turnstile-ready` core service so the bus exists before login. The
session must then publish its display environment to supervised services and
D-Bus activation. Ryoku does not wrap the compositor in `dbus-run-session`;
PipeWire and the other login services use Turnstile's existing user bus.

Ryoku performs that setup automatically. Installation and dev deploys run
`ryoku-host session ensure`; `session-start` also runs the user half at each
login so a fresh user and scripts from a package update converge before the
roster starts. Installation runs `fix-wrappers` with backups, then the verify
step checks the result. The doctor's Turnstile check repairs later drift. The
audio login services start PipeWire, pipewire-pulse, and WirePlumber after the
bus is ready, and `session-start` publishes `WAYLAND_DISPLAY` and the rest of
the compositor environment to both the Turnstile envdir and D-Bus activation.

Polkit prompts also work from the session. The shell's agent registers with the
graphical session even though Turnstile supervises it.

## Translation rules

How a systemd unit becomes a runit service, applied uniformly:

| systemd concept | runit translation |
|---|---|
| `ExecStart` daemon | `run` script ending in `exec`; runsv re-runs it after ~1 s whenever it exits (verified on Void runit: the `finish` exit code does NOT gate restarts, only an `sv down` request does) |
| `Restart=always` | the default: `finish` exits non-zero, no `sv down`; an operator `sv down` still parks it (the down flag wins) |
| `Restart=on-failure` | `finish` calls `exec sv down .` when the run child exited 0 (clean stop parks), else exits non-zero so runsv restarts the crash |
| `RestartSec=2` | not translatable; runsv enforces a ~1 s minimum between restarts |
| `StartLimitBurst` | no runit equivalent; documented per service, doctor-side detection instead |
| `Type=oneshot` | `run` does the work then `exec sv down .`; `finish` also `exec sv down .` so a FAILURE parks too (systemd leaves a failed oneshot in `failed`, it does not retry) |
| `ExecStartPre`/`ExecStartPost` | sequential commands in `run` before `exec`; best-effort post-start hooks run backgrounded before `exec` (runit has no post-start phase) |
| `ExecStop` | `finish` script; it runs on every exit, so a stop-only hook is gated on the down flag, not the exit status |
| `Environment=KEY=V` | exported in `run` before `exec` |
| `ConditionEnvironment=` / `ConditionPathExists=` / `After=` / `Wants=` | bounded waits via `init/lib/wait-for` (dbus name, file, env var, or a sibling service's supervise state); a failed wait `exec sv down .`, never crash-loops |
| `PartOf=`/`BindsTo=` | the parent's `finish` downs the child (`sv down <child>`) |
| `Slice=`, cgroup keys | dropped; runit has no cgroup delegation |
| timer unit | long-running `run` loop with sleep, delays overridable by env for tests |
| `.target` | no process; `init/session-services` roster names the services and their order |
| `KillMode=process` | natural: runsv signals only the run child |

Two systemd behaviors have no runit analogue and are accepted losses, recorded
here so nobody re-litigates them per service: per-service restart rate limits,
and declarative cross-service ordering (runit starts everything in parallel;
ordering is the `wait-for` guards' job).

Binaries resolve through `${RYOKU_BIN_DIR:-/usr/bin}` so a dev checkout can
point the same service files at `~/.local/bin` without a second copy, the
same job the systemd drop-ins do on Arch.

## Deliberate exceptions

The host seam covers runtime behavior. These provisioning or upstream-owned
paths remain init- or distribution-specific:

- `ryoku/shell/deploy.sh`, the installer, and PKGBUILD install hooks provision
  the host and keep their explicit init branches.
- `system/hardware/power/ryoku-power-cutover` is systemd orchestration. Its
  runit counterpart is `void/init/session/session-start` together with
  `deploy.sh`'s runit lane.
- `ryoku-boot-apply` and the NVIDIA pacman hook are Arch boot and package
  provisioning.
- iNiR upstream files that already branch for the host stay as upstream wrote
  them.
- Doctor and the packaged update lane move to `ryoku-host` in a later phase.

## Testing

- `tests/void-init.sh` (host, no container): manifest completeness, script
  hygiene (shellcheck, exec bits, shebang), and parity assertions between each
  unit and its translation (env vars, binaries, conditions all present).
- `tests/void-init-runit.sh` (docker, `ghcr.io/void-linux/void-glibc`): runs
  the translated services under a real `runsvdir` against stub binaries and
  asserts oneshot and restart semantics, ordering guards, finish hooks,
  envdir waits, down markers, and session roster startup.
- `.github/workflows/void.yml` runs both; the live job needs docker.

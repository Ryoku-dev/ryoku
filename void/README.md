# Void Linux

Everything Ryoku needs to run on Void Linux lives under this directory, one
concern per subdirectory. Void differs from the Arch base in exactly two ways
that reach into this repo: XBPS replaces pacman, and runit replaces systemd.
This area holds the translations; the installer, the ISO and the package repo
are deliberately NOT wired to it yet.

The validated target profile matches the one the ecosystem converged on
(iNiR's Void port, DankMaterialShell): **x86_64 glibc + elogind + runit +
Turnstile**. musl and seatd-only sessions are out of scope until the glibc
profile ships.

## Layout

- `init/` the systemd to runit translation, the only implemented part today.
  - `translations.tsv` the completeness manifest: every systemd unit, target,
    timer, drop-in and environment generator in the repo maps to its runit
    translation or an explicit accepted-loss verdict. `tests/void-init.sh`
    fails when a unit exists without a manifest row, so a new unit can never
    silently skip Void.
  - `system/` system-level runit services (installed to `/etc/sv/<name>`,
    enabled by symlink into `/var/service/`).
  - `user/` per-user session services (installed to `~/.config/service/<name>`
    under Turnstile, or any `runsvdir`-supervised directory).
  - `env/` session environment translation for the systemd user-environment
    generator (Turnstile envdir format).
  - `polkit/` the polkit rules whose systemd-specific action ids do not
    transfer to runit.
  - `lib/` shared run-script helpers (dependency waiting).
  - `session-services` the roster that replaces `ryoku-session.target`: the
    services a session brings up, in order.
- `iso/` (future) the Void live ISO profile; only a README marking the slot.
  XBPS repo baking and the live-image runit services belong there.
- `packages/` (future) xbps-src templates for a signed Ryoku XBPS repository,
  the counterpart of `release/packages/`; only a README marking the slot.

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

## Testing

- `tests/void-init.sh` (host, no container): manifest completeness, script
  hygiene (shellcheck, exec bits, shebang), and parity assertions between each
  unit and its translation (env vars, binaries, conditions all present).
- `tests/void-init-runit.sh` (docker, `ghcr.io/void-linux/void-glibc`): runs
  the translated services under a real `runsvdir` against stub binaries and
  asserts the behaviors that matter: oneshot parking, ordering guards,
  restart-on-crash, finish hooks, down-propagation, timer loop.
- `.github/workflows/void-init.yml` runs both; the live job needs docker.

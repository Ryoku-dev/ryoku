# lockscreen/

The login screen and the in-session lock. Both render the same qylock skin
(clockwork/orbital by default): a plain Qt6 QML greeter that needs no Ryoku
desktop shell, so it works on a clean session under either supported
compositor. The in-session lock is compositor-independent: it is a Quickshell
`WlSessionLock` surface, the Wayland session-lock protocol both Hyprland and
niri serve, so the same lock runs on both.

## What's here

- `qylock/` The vendored qylock bundle (clockwork theme + Quickshell
  lockscreen), trimmed to just what Ryoku ships. Copied verbatim from upstream;
  see `qylock/README.ryoku.md` for the source commit and license.
- `install-qylock` Installs the greeter and prepares the in-session lock on the
  target machine: the default skin goes to `/usr/share/sddm/themes/ryoku`, the
  SDDM selection to `/etc/sddm.conf.d/99-ryoku.conf`, and the Quickshell client
  to the user's home. An existing managed session stages the user generation;
  its service activates client and core theme together after the old daemon
  stops. A fresh offline install activates it directly.
- `ryoku-qylock-lock` is the stable launcher. It leases the selected generation
  before entering replaceable client code.
- `ryoku-qylock-unlock-prepare` restores the daemon's sleep block before
  unlock. If the daemon is between generations, it holds a transient login1
  block until the replacement daemon publishes equivalent protection.
- `sddm/setup` The install-time SDDM wiring: enable the service, default to the
  graphical target, drop `pam_gnome_keyring` from the SDDM PAM stack, and make
  sure a Hyprland wayland session exists.
- `README.md`, `CHANGELOG.md` This file and the change log.

## Two pieces, one theme

The greeter you see at boot is SDDM rendering the selected skin (clockwork/orbital
by default). After you log in, locking the session -- the lock keybind, or an idle
timeout -- enters `ryoku-qylock-lock`, which selects and runs the active
Quickshell `WlSessionLock` generation with the same skin. The greeter reads
`/etc/sddm.conf.d/99-ryoku.conf`; the in-session lock reads
`~/.config/qylock/theme`.

The skin is chosen in Ryoku Settings (**Lockscreen**), which browses the full qylock
catalogue live from upstream with looping previews. Selecting a skin rewrites
`~/.config/qylock/theme` (the in-session lock) and reinstalls it as the SDDM greeter
under `/usr/share/sddm/themes/ryoku`. The greeter half lives on a system path, so the
Hub escalates it with pkexec (`ryoku-hub lock apply-greeter`); skins not yet present
download into `~/.local/share/qylock/themes` first. Only the theme changes; the
login/auth flow is untouched.

## What owns the lock, the lid and sleep

qylock draws the lock surface and nothing more: `lock_shell.qml` uses
`WlSessionLock` and publishes a token to `qylock.<session>.locked` only once the
compositor confirms every output is covered. The shell accepts it only when the
expected generation matches a live qylock client from that login1 session.
Everything around the lock belongs to the shell daemon:

- the daemon bound to the foreground session owns login1's delay inhibitor and
  hard sleep block. Before a same-user session handoff, it locks and confirms
  the outgoing session first;
- every other online session gets its own qylock client and login1 observer, so
  it is already secure without pretending that a second user daemon owns
  another delay FD;
- `ryoku-shell suspend` releases the active block only after compositor-secure
  qylock, then completes the bounded delay handshake;
- unlock names the qylock session explicitly to both the daemon and login1.
  The daemon restores the block and confirms that exact session is active. If
  the daemon generation is restarting, the stable unlock helper substitutes a
  durable transient login1 block until the replacement is ready;
- `PrepareForSleep` starts output recovery immediately on resume, bounds every
  provider attempt, and reasserts power across the panel retrain window while
  lighting is restored;
- ordinary hypridle screen wake uses the same bounded provider calls and retries
  until outputs return; a newer idle-off generation cancels stale wake retries.

hypridle owns the idle timers only -- dim, lock, screen-off, suspend -- and sends
the suspend timer to `ryoku-shell suspend`. `ryoku-clamshell` owns lid events
only while its login1 session is active. Live docked mode stays awake and every
other close uses the same secure transaction. Hyprland handles the panel
handoff; niri sends both native close and open switch events and keeps its
topology. See `system/hardware/README.md` and `docs/compositors.md`.

## Installing by hand

```
sudo ryoku/lockscreen/sddm/setup        # service + session wiring
ryoku/lockscreen/install-qylock         # greeter theme + in-session lock
```

Both take `--dry-run` (or `RYOKU_DRYRUN=1`) to print the plan without changing
anything. `install-qylock` chooses staged activation when the managed activation
unit is present and direct activation on a fresh offline home. Internal package
and deployment callers set `RYOKU_QYLOCK_MODE=stage|live` explicitly. The
install backend runs both scripts for you on a normal install.

## Packages it needs

The greeter and lock are Qt6 QML and depend on these (install them from the
package set, not from these scripts):

```
qt6-declarative qt6-5compat qt6-svg qt6-multimedia qt6-multimedia-ffmpeg quickshell
```

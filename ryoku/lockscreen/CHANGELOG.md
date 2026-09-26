# Changelog: lockscreen/

## Unreleased

### Changed
- **The login pointer is visible again.** On a hybrid laptop whose panel is on
  the iGPU, the greeter still forced weston's Pixman renderer -- a workaround
  only NVIDIA-panel machines need -- and under Pixman weston draws no cursor at
  all (weston #375), so the login pointer vanished (issue #191). The greeter now
  uses Pixman only when NVIDIA actually drives the connected panel; everywhere
  else it keeps the GL renderer whose hardware cursor works. And because the
  login compositor cannot always be trusted to draw a pointer at all, the
  clockwork login theme now paints its own cursor in the scene, following the
  mouse, so a pointer is always shown no matter what the compositor supports.
- **The login pointer shows on NVIDIA machines.** The greeter client pushes a
  themed cursor surface and weston's kiosk shell has no cursor of its own; on
  NVIDIA the DRM backend hands that surface to the hardware cursor plane and
  the driver accepts it without ever displaying it, so the login screen had a
  working pointer with nothing drawn (`ryoku/lockscreen/sddm/ryoku-greeter`).
  The greeter now detects an NVIDIA DRM card and runs weston on the pixman
  renderer, which skips plane assignment entirely: the sprite is composited
  into the scanout and always shows. A login screen is transient, so software
  rendering there costs nothing worth missing; every other GPU keeps the GL
  path. The plain-weston fallbacks (`sddm/setup`, the doctor reconciler) make
  the same call for the boxes that have not landed the wrapper yet.
- **`sddm/setup` ships unlock-on-login by default instead of stripping the
  keyring.** The old wiring unconditionally deleted `pam_gnome_keyring` from
  `/etc/pam.d/sddm`, citing a "passwordless Default_keyring" that nothing in the
  repo ever created, so browsers prompted for the keyring on every launch. A
  fresh install now wires `pam_gnome_keyring` (auth + session) so the login
  keyring unlocks with the login password at sign-in -- unless autologin is
  configured, where there is no password to reuse and it ships never-ask (lines
  stripped; the blank default keyring is seeded lazily by the Hub/CLI, never by
  this root installer). Honors `RYOKU_DRYRUN`; `ryoku keyring` changes it later.

### Fixed
- **Lock and unlock are serialized per login1 session and qylock generation.**
  Lid, idle and manual requests share a session-scoped launch guard. A stable
  launcher leases the selected client generation before entering replaceable
  code, and updates stage the wrapper with its matching core theme before an
  atomic service handoff. The secure marker carries a fresh token and counts
  only while the matching session's qylock process carries that token; a
  delayed callback from an old client cannot bless its replacement. The
  long-lived wrapper survives a shell-daemon restart, retries three times
  quickly, then uses a capped backoff so a recovering compositor regains its
  unlock UI without a hot crash loop. Unlock passes the explicit session ID to
  the stable preparation helper and login1. If the owning daemon is between
  generations, that helper substitutes a durable login1 sleep block until the
  replacement daemon publishes its own, so neither unlock nor an update opens
  an unprotected suspend window. Inactive locks leave the global fingerprint
  reader to the foreground session while keeping password authentication ready
  (`install-qylock`, `ryoku-qylock-lock`,
  `ryoku-qylock-unlock-prepare`, `qylock/quickshell-lockscreen/lock.sh`,
  `qylock/quickshell-lockscreen/proof.sh`,
  `qylock/quickshell-lockscreen/unlock.sh`).
- **The lockscreen's suspend action cannot bypass the secure handshake.**
  qylock's in-session SDDM shim used `systemctl suspend` directly; it now calls
  `ryoku-shell suspend`, which keeps the session sleep block held unless the
  compositor has confirmed a live qylock client covers every output
  (`qylock/quickshell-lockscreen/shim/SddmShim.qml`).
- **The in-session lock shows a usable mouse cursor with any theme.** The lock
  is spawned by the shell daemon, whose imported env can predate the login-time
  `hyprctl setcursor` (autostart.lua), and a downloaded theme carries no cursor
  workaround of its own, so the lock could come up with no visible pointer.
  `lock.sh` now re-asserts the provider's compositor cursor action from the
  same theme/size the lock client uses before launching
  (`qylock/quickshell-lockscreen/lock.sh`).
- **A downloaded lockscreen theme now shows its preview in the Hub.** The Hub
  looked for `preview.gif` only at the skin root, where shipped themes keep it,
  but a RyoStore download lands it under `assets/preview.gif` (its product
  manifest maps it there), so downloaded skins showed a blank tile with no
  Preview button. `lockSkinFor` now checks both paths (`hub/backend/lock.go`).
- **The login screen lands on a chosen monitor instead of whichever trained
  first.** With two displays weston's kiosk shell dropped the greeter on the
  connector that came up first (a DP a beat before an HDMI), so the login
  appeared on the wrong screen, or only on one. `ryoku-greeter` now pins the
  greeter (`sddm-greeter-qt6`) to a resolved output via kiosk-shell `app-ids`:
  an internal laptop panel (`eDP`/`LVDS`/`DSI`) when present, or the connector
  set in `/etc/ryoku/greeter.conf` (`PRIMARY=<name>`, `APPID=<id>`) or the
  `RYOKU_GREETER_PRIMARY` / `RYOKU_GREETER_APPID` env. Idle blanking is disabled
  in the generated config so a display no longer powers off at the login screen
  (`sddm/ryoku-greeter`).
- **The SDDM greeter stops logging a Quickshell plugin error on every boot
  (#162).** Its theme imported the shell's `Ryoku.Ui.Singletons`, whose
  singletons load `Quickshell.Io`, a plugin the plain `sddm-greeter-qt6` process
  cannot load, so the greeter logged "quickshell-coreplugin not found" each boot.
  The theme now carries its own Quickshell-free `I18n` and reads the shipped
  catalog directly, so it localises without the shell singletons
  (`themes/clockwork/orbital/i18n`, greeter env gains `QML_XHR_ALLOW_FILE_READ`).
- **The login screen waits for a slow second monitor before it starts.** On
  boards that bring one connector up a beat before another (DP before HDMI), the
  greeter probed once and lit only the fast panel, so login landed on the wrong
  screen or just one of two. It now polls until the connected set holds steady,
  then lights every output, bounded so login always comes up and tunable with
  `RYOKU_GREETER_SETTLE` and `RYOKU_GREETER_DEADLINE` (`sddm/ryoku-greeter`).
- **Pressing Enter on an empty password no longer strands the in-session lock
  on a white screen, taking the reboot and shutdown buttons with it.** The
  clockwork/orbital submit runs a windup that ends in a full-screen blast (white
  in the shipped dark theme) held up until PAM answers, and the shell drops the
  whole HUD -- clock, password field, reboot and shutdown -- while it is raised.
  An empty submit reached `sddm.login(user, "")`, but the shim never feeds an
  empty key to PAM (the sensor keeps scanning instead), so the conversation
  never completed, `loginFailed` never fired, and the blast never fell: a white
  surface with no controls, reported both as "stuck on a white screen" and as
  "the reboot and shutdown buttons are white on white." An empty Enter is now a
  no-op that keeps the field; the shim will not start a response-less
  conversation; a conversation that dies mid-auth now reports failure; and a
  watchdog clears the flash if auth ever goes silent, so no submit -- empty,
  wrong, or hung -- can hold the lock (`themes/clockwork/orbital/Main.qml`,
  `quickshell-lockscreen/shim/SddmShim.qml`).
- **The login screen uses the same keyboard layout as the session, so a non-US
  password is accepted.** Moving the greeter to a Wayland weston kiosk left it on
  weston's built-in `us` map: `/etc/X11/xorg.conf.d/00-keyboard.conf` (which the
  installer and `localectl` write for the session and console) is X11-only, and a
  bare weston reads neither it nor the console keymap. An AZERTY password typed at
  the login field then failed on a QWERTY map while the same password worked on a
  tty, which has the vconsole keymap. `sddm/ryoku-greeter` now reads that file and
  hands weston the primary layout through the libxkbcommon defaults
  (`XKB_DEFAULT_LAYOUT`/`_VARIANT`/`_OPTIONS`), which weston honours when its own
  config names no keymap. Delivered by the package to fresh and existing boxes.
- **The login screen always shows a mouse pointer.** Pinning
  `XCURSOR_THEME=Bibata-Modern-Ice` in `GreeterEnvironment` only helps clients
  that honor it; SDDM's Wayland greeter ignores it and, like weston's own
  pointer, falls back to the cursor theme literally named `default`. Ryoku
  shipped no `/usr/share/icons/default`, so that fallback resolved to nothing and
  the greeter drew no pointer at all at boot and after logout (the in-session
  lock, drawn by the running session, was unaffected). `sddm/setup` now points
  the `default` theme at the shipped Bibata set (only when the box has no default
  of its own); `ryoku doctor`'s `reconcileGreeterCursor` converges existing boxes.
- **The greeter and the in-session lock always have a cursor theme.**
  `sddm/setup` and the doctor pin `XCURSOR_THEME=Bibata-Modern-Ice` and
  `XCURSOR_SIZE=24` in SDDM's `GreeterEnvironment`, `sddm/ryoku-greeter` exports
  the same for weston's own pointer, and `lock.sh` defaults them when the shell
  daemon that spawns it carries none, so the pointer is drawn from the shipped
  set instead of whatever the "default" theme chain resolves to (or nothing).
- **The SDDM greeter no longer lingers after login, draining power.** SDDM ran
  the greeter on X11 while the Hyprland session is Wayland; at login SDDM's
  `sddm-helper` died mid-teardown without reaping `sddm-greeter-qt6`, so the
  greeter was orphaned onto a leftover Xorg and kept rendering forever. A video
  greeter skin (e.g. Store "forest") decoded a 1440p60 clip on a screen nobody
  was looking at, and even the default clockwork clock woke the CPU ~60x/s,
  blocking deep idle on a laptop. `sddm/setup` now writes
  `/etc/sddm.conf.d/10-ryoku-wayland.conf` (`DisplayServer=wayland`,
  `GreeterEnvironment=QT_QPA_PLATFORM=wayland`,
  `CompositorCommand=weston --shell=kiosk`), so the Qt greeter connects to
  Weston instead of selecting xcb with no X server. The session tears it down
  cleanly at login. Validated headlessly:
  weston hosts the greeter as a separate client, the exact separate-process
  model SDDM uses. `ryoku doctor` backports it to existing boxes; `base.packages`
  and `ryoku-desktop` ship weston. Honors `RYOKU_DRYRUN`.
- **The clockwork/orbital greeter clock stops animating off screen.** Its 60fps
  smooth-second-hand timer ran unconditionally; it now gates on `Window.active`,
  so a backgrounded or orphaned greeter never wakes the CPU for a clock no one
  can see. The in-session lock (which alone sets `sddm.fingerprintHint`) keeps
  animating -- its surface exists only while shown.
- **Suspend now waits for the lock to actually cover the screen.** qylock's
  `lock_shell.qml` touches `$XDG_RUNTIME_DIR/qylock.locked` the moment the
  compositor confirms every output is covered (`WlSessionLock.secure`) and
  removes it on unlock, giving `ryoku-shell lock` a real "locked" signal to
  block on. Before, the lock was fire-and-forgotten and the sleep delay was
  released with Quickshell still loading QML, so opening the lid showed your
  windows for a beat before the lock painted; a lock that never confirms now
  reports a failure rather than passing a desktop left uncovered.
- **A missing lock theme can no longer lock you out.** `lock.sh` defaulted to
  `nier-automata`, a theme the shipped bundle does not contain, and never
  checked the resolved theme path: with `~/.config/qylock/theme` lost (or an
  uninstalled skin still named there) the theme Loader errored and the session
  locked into a plain black surface that absorbed every keypress with no
  password field. The default is now the shipped `clockwork/orbital` (also in
  `lock_shell.qml` and the QtMultimedia shims), and `lock.sh` verifies
  `Main.qml` exists, falling back to the stock theme before launching.
- `lock.sh` resolves the session type from `$XDG_SESSION_ID` (or the user's
  first logind session) instead of `loginctl | grep $(whoami)`, which matched
  whichever of several sessions happened to sort first (re-login, a second
  seat) and could misread wayland as tty.
- The desktop no longer strands itself on a black lock screen after sleep. The
  ext-session-lock protocol wedges the whole session if the locker crashes while
  locked, which a GPU glitch on resume can trigger: the machine wakes to a black
  screen that eats every keypress and can't be dismissed (reported as "slept and
  won't wake up" and "keybinds don't register on the lock screen"). Hyprland now
  ships with `misc:allow_session_lock_restore` on from boot, so it accepts a
  fresh locker instead of stranding the session and `ryoku-shell lock` can relock
  and take the password. qylock only enabled it after a successful unlock, which
  is too late for the crash that happens before one.

### Added
- **Every lock skin shows a smooth fingerprint scan, with no per-theme code.** A
  shared `FingerprintScan` reader rides above whatever theme is loaded, in both
  the Wayland and X11 lock surfaces, driven by the shim's fingerprint state: a
  calm breath while armed, the ridges filling as a finger is read, and a ring
  completing on unlock. Because it sits above the theme Loader it covers the
  current clockwork/orbital skin and any future one automatically
  (`quickshell-lockscreen/lock_shell.qml`,
  `quickshell-lockscreen/FingerprintScan.qml`).
- **Fingerprint touch-to-unlock at the lock screen, sudo, and the SDDM greeter.**
  The qylock lock unlocks with the same `pam_fprintd_grosshack` mechanism as the
  greeter, through a self-contained `ryoku-lock` PAM service loaded via
  `PamContext.configDirectory` (no root edit of `/etc/pam.d`): grosshack paired
  with `pam_unix`, ending in `pam_deny`, so a touch or a typed password unlocks
  in one conversation and neither matching fails closed. A Fingerprint card in
  Ryoku Settings (**Lockscreen**) enrolls, verifies, names, and removes fingers
  (fprintd over its own bus, never root), and toggles fingerprint for `sudo` and
  the sign-in screen by injecting one `auth sufficient pam_fprintd_grosshack.so`
  line at the top of `/etc/pam.d/{sudo,sddm}` through `pkexec` (backed up first,
  removal deletes only that line). Arming waits for `WlSessionLock.secure`,
  clears orphaned `fprintd-verify` before each arm, and never lets a dead
  conversation fake a live sensor. A `~/.config/qylock/fingerprint` toggle gates
  the lock's sensor.
- Vendored the qylock clockwork theme (orbital and tape variants) and the
  Quickshell lockscreen under `qylock/`, trimmed to only what Ryoku ships.
- Per-skin `preview.gif` for the Lockscreen section in Ryoku Settings: orbital
  reuses qylock's own clockwork preview (its dark-mode segment, to match the
  shipped `themeMode=dark`); tape is rendered from the skin itself. They deploy
  inside the themes dir, and `ryoku-hub lock list` reports their paths.
- `install-qylock`: offline installer for the SDDM greeter and the in-session
  lock. Installs the default skin under the fixed `/usr/share/sddm/themes/ryoku`
  name (the one the Hub overwrites when a skin is chosen) and writes
  `/etc/sddm.conf.d/99-ryoku.conf` (Current=ryoku), installs the Quickshell
  lockscreen to the user's home, links `themes_link`, and sets
  `~/.config/qylock/theme`. Resolves the login user under sudo and pkexec.
  Honors `RYOKU_DRYRUN=1` and `--dry-run`.
- `sddm/setup`: install-time SDDM wiring (enable sddm.service, default to
  graphical.target, strip pam_gnome_keyring from the SDDM PAM stack, ensure a
  Hyprland wayland session exists). Honors `RYOKU_DRYRUN=1` and `--dry-run`.

### Fixed
- In-session lock: skins that gate login and power behind `!isQuickshell`
  (notably `material-you` and `nothing`) left the password field, reboot, and
  shutdown dead under the Quickshell lock, since the shim omitted `sddm.hostName`
  and `isQuickshell` was always true. The shim now reports a real `sddm.hostName`
  (so `isQuickshell` is false), implements `sddm.suspend()`, and exposes SDDM's
  `keyboard` object, so every catalogue skin authenticates and powers off under
  the lock as it does under the SDDM greeter.

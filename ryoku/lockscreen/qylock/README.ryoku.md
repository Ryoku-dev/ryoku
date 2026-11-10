# Vendored qylock (trimmed)

Upstream: https://github.com/Darkkal44/qylock
Vendored at commit: cde4d11e9e3d385620becdc877a0521e40a55e47

Only the assets Ryoku ships are kept here so the greeter installs **offline**
(no git clone at install time): the `clockwork` SDDM theme and the
`quickshell-lockscreen`. The full upstream repo carries ~35 themes with large
video backgrounds (1.2G) which Ryoku does not use. Licensed under the upstream
LICENSE in this directory.

The vendored core skin carries `themes/clockwork/orbital/preview.gif`, the
dark-mode segment of upstream `Assets/clockwork.gif`. Optional skins and their
catalogue previews are owned by `ryostore`; they are not duplicated here.

The in-session shim diverges from upstream in three places:

1. **Skin compatibility** — `quickshell-lockscreen/shim/SddmShim.qml` (plus the
   matching `keyboard` export in `lock_shell.qml`). Upstream omits
   `sddm.hostName`, so every skin's `isQuickshell` test is true; skins like
   `material-you` and `nothing` gate login and power behind `!isQuickshell`,
   leaving their password field, reboot, and shutdown dead under the in-session
   lock. The shim now reports a real `hostName` (making `isQuickshell` false),
   implements `sddm.suspend()`, routes `reboot()` and `powerOff()` through
   `ryoku-host power`, and exposes SDDM's `keyboard` object (skins assign
   `keyboard.numLock`).

2. **Parallel PAM conversations** — once the compositor-secure lock proof is
   published for the foreground login1 session,
   `shim/SddmShim.qml` runs two `PamContext`s backed by services in
   `assets/pam/`: `ryoku-lock` (fingerprint, stock
   `pam_fprintd.so timeout=-1`) and `ryoku-lock-pw` (password); the first
   `Success` unlocks and aborts the other. A single conversation cannot host
   both: the PAM stack is serialised by design (see the LIMITATIONS section of
   `pam_fprintd(8)`), so the password prompt starves the sensor — the stock 30s
   scan timeout falls through to `pam_unix`, the conversation stays alive
   waiting for a typed key, the scan never re-arms, and a password typed
   mid-scan is only flushed when the prompt finally arrives.
   A password conversation that ends before PAM ever prompts is a stack that
   cannot run, not a wrong password, so it re-arms on a doubling delay (1s to
   30s) instead of restarting on the same tick.

3. **Security key conversation**: when `ryoku security-key set lock on` has
   written `~/.config/qylock/pam/ryoku-lock-key`, the shim runs a third
   `PamContext` (pam_u2f) from it, armed when the lock secures and on a
   `hidraw` plug-in. The shim reports `sddm.keyState` (`off`, `pin`,
   `verifying`, `touch`) and `sddm.usePassword()` / `sddm.useKey()`. A PIN
   request reaches the field only if the skin sets `sddm.keyPrompts`; the
   Orbital skin does, and draws its hint and USE PASSWORD / USE SECURITY KEY
   button from that state, handing the PIN over without its unlock wind-up.

## Optional authentication retry

The in-session lock can restart a password PAM conversation that is waiting
after an optional authentication method falls through. This is disabled by
default. To enable it, create `$XDG_CONFIG_HOME/qylock/auth-retry`, or
`~/.config/qylock/auth-retry` when `XDG_CONFIG_HOME` is unset, with exactly:

    on

Remove the file or give it any other content to disable the feature. Missing,
non-regular, and unreadable files also disable it. Changes apply to the next
lock.

When enabled, an empty Return, keypad Enter, settled pointer motion, or a
background click can abort one waiting password conversation and start one
replacement after a cooldown. A nonempty typed, pending, or submitted password
always wins, including a hidden password after focus moves. Account, session,
power, password-field, and other theme controls keep their native input.

The retry remains inhibited while a security-key conversation or explicit key
mode owns input. It never responds to the key conversation, submits a PIN, or
moves password text between channels. PAM remains the only source of
authentication success. The common input layer is exercised with the shipped
skin and its generic control rules; it is not a compatibility promise for every
third-party qylock theme.

Everything else is upstream verbatim.

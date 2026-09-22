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

The in-session shim diverges from upstream in two places:

1. **Skin compatibility** — `quickshell-lockscreen/shim/SddmShim.qml` (plus the
   matching `keyboard` export in `lock_shell.qml`). Upstream omits
   `sddm.hostName`, so every skin's `isQuickshell` test is true; skins like
   `material-you` and `nothing` gate login and power behind `!isQuickshell`,
   leaving their password field, reboot, and shutdown dead under the in-session
   lock. The shim now reports a real `hostName` (making `isQuickshell` false),
   implements `sddm.suspend()`, and exposes SDDM's `keyboard` object (skins
   assign `keyboard.numLock`).

2. **Parallel PAM conversations** — `shim/SddmShim.qml` runs two `PamContext`s,
   backed by two services in `assets/pam/`: `ryoku-lock` (fingerprint, stock
   `pam_fprintd.so timeout=-1`) and `ryoku-lock-pw` (password, prompt pending
   from lock time); the first `Success` unlocks and aborts the other. A single
   conversation cannot host both: the PAM stack is serialised by design (see
   the LIMITATIONS section of `pam_fprintd(8)`), so the password prompt starves
   the sensor — the stock 30s scan timeout falls through to `pam_unix`, the
   conversation stays alive waiting for a typed key, the scan never re-arms, and
   a password typed mid-sit is only flushed when the prompt finally arrives.

Everything else is upstream verbatim.

# Polkit translations

The only polkit rule in the repo tied to the init system is
`system/hardware/bluetooth/54-ryoku-bluetooth-a2dp.rules`, which grants a
wheel-group user session the passwordless
`org.freedesktop.systemd1.manage-units` action on `bluetooth.service` so
`ryoku-bluetooth-reset.service` can restart BlueZ from the session.

Under runit that action id does not exist: `sv restart bluetoothd` is a
direct write to the service's `supervise/control` pipe, gated by filesystem
permissions on `/var/service/bluetoothd`, not by polkit. There is no
unprivileged-user restart to authorize, so there is nothing to translate:

- the Void reset path runs from the session bootstrap with sufficient
  privilege (see `void/init/user/ryoku-bluetooth-reset/run`);
- no polkit rule ships for Void; this directory exists so the future rule
  surface (e.g. a privileged helper for user-initiated service restarts)
  has its home.

The rest of Ryoku's polkit surface (the `ryoku-docker` fixed-argument door,
the network kill switch) is action-id based on Ryoku's own helpers and
transfers to Void unchanged; those rules stay where they live.

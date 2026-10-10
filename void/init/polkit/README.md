# Polkit translations

Void has no equivalent of systemd's
`org.freedesktop.systemd1.manage-units` action. Runit controls services through
the `supervise/control` pipe and its filesystem permissions, so
`system/hardware/bluetooth/54-ryoku-bluetooth-a2dp.rules` is not translated.

Other Ryoku polkit rules use Ryoku's own action IDs and ship unchanged.

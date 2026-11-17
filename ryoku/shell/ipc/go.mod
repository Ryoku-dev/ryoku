module ryoku-shell

go 1.26

toolchain go1.26.5

require github.com/godbus/dbus/v5 v5.2.2

require golang.org/x/sys v0.44.0

require ryoku-wm v0.0.0

replace ryoku-wm => ../../wm

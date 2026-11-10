module ryoku-rashin

go 1.26

require (
	gioui.org v0.10.3
	github.com/ryoku-dev/ryoku/docs/lithic/library/go v0.0.0
	go.etcd.io/bbolt v1.4.3
	golang.org/x/exp/shiny v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/text v0.32.0
	ryoku-i18n v0.0.0
)

require (
	gioui.org/shader v1.0.9 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	golang.org/x/image v0.26.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/ryoku-dev/ryoku/docs/lithic/library/go => ../../docs/lithic/library/go

replace ryoku-i18n => ../i18n

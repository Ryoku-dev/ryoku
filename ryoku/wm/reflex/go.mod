module ryoku-rashin/internal/reflex/wm

go 1.26

require (
	ryoku-rashin v0.0.0
	ryoku-rashin/internal/oni/window v0.0.0
)

require ryoku-i18n v0.0.0 // indirect

replace ryoku-rashin => ../../hyakki

replace ryoku-rashin/internal/oni/window => ../oniwindow

replace github.com/ryoku-dev/ryoku/docs/lithic/library/go => ../../../docs/lithic/library/go

replace ryoku-i18n => ../../i18n

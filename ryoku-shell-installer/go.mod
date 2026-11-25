module ryoku-shell-install

go 1.26.4

require (
	charm.land/bubbletea/v2 v2.0.2
	charm.land/lipgloss/v2 v2.0.2
)

require (
	github.com/charmbracelet/colorprofile v0.4.2 // indirect
	github.com/charmbracelet/x/ansi v0.11.6 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

// the shared translation runtime + the catalogs compiled in: this binary is
// downloaded onto a foreign distro with no Ryoku catalog on disk.
require ryoku-i18n v0.0.0

replace ryoku-i18n => ../ryoku/i18n

require ryoku-wm v0.0.0

replace ryoku-wm => ../ryoku/wm

require (
	github.com/charmbracelet/ultraviolet v0.0.0-20260205113103-524a6607adb8
	ryoku-installerplan v0.0.0
)

replace ryoku-installerplan => ../installation/installerplan

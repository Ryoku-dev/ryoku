module ryoku-tui

go 1.26.4

toolchain go1.26.5

require (
	charm.land/bubbletea/v2 v2.0.2
	charm.land/lipgloss/v2 v2.0.2
	github.com/charmbracelet/harmonica v0.2.0
	github.com/charmbracelet/x/ansi v0.11.6
	github.com/mdp/qrterminal/v3 v3.2.1
	github.com/sahilm/fuzzy v0.1.1
)

require (
	github.com/charmbracelet/colorprofile v0.4.2 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260205113103-524a6607adb8 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.21 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	rsc.io/qr v0.2.0 // indirect
)

// the shared translation runtime + the catalogs compiled in: the ISO's TUI has
// no installed Ryoku to read /usr/share/ryoku/i18n from.
require ryoku-i18n v0.0.0

replace ryoku-i18n => ../../ryoku/i18n

require ryoku-wm v0.0.0

replace ryoku-wm => ../../ryoku/wm

require ryoku-installerplan v0.0.0

replace ryoku-installerplan => ../installerplan

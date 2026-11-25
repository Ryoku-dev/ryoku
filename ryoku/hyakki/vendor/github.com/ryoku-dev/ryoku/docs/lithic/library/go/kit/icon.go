package kit

import (
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/symbols"
)

// Icon decodes IconVG data, such as a symbols value, or panics when data is invalid.
func Icon(data []byte) *widget.Icon {
	icon, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return icon
}

var (
	iconHome     = Icon(symbols.Home)
	iconSearch   = Icon(symbols.Search)
	iconSettings = Icon(symbols.Settings)
	iconClose    = Icon(symbols.Close)
)

// IconByName returns the foundation gallery icon with name, or nil.
func IconByName(name string) *widget.Icon {
	switch name {
	case "home":
		return iconHome
	case "search":
		return iconSearch
	case "settings":
		return iconSettings
	case "close":
		return iconClose
	default:
		return nil
	}
}

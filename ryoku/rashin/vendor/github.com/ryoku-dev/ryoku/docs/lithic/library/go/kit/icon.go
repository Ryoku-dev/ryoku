package kit

import (
	"gioui.org/widget"
	material "golang.org/x/exp/shiny/materialdesign/icons"
)

// Icon decodes Material Design icon data or panics when data is invalid.
func Icon(data []byte) *widget.Icon {
	icon, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return icon
}

var (
	iconHome     = Icon(material.ActionHome)
	iconSearch   = Icon(material.ActionSearch)
	iconSettings = Icon(material.ActionSettings)
	iconClose    = Icon(material.NavigationClose)
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

package palette

import "fmt"

// Mode selects where an app's colours come from. The zero value is Dark: every
// Lithic app opens in the signature, the ryoku.dev stock. Following the
// wallpaper is a choice the reader makes in the app, never the default.
type Mode uint8

const (
	// Dark paints the dark signature table and takes nothing from the wallpaper.
	Dark Mode = iota
	// Light paints the light signature table and takes nothing from the wallpaper.
	Light
	// Follow reads the desktop palette files and dyes the roles from them.
	Follow
)

// String returns the mode's name as ParseMode reads it.
func (m Mode) String() string {
	switch m {
	case Light:
		return "light"
	case Follow:
		return "follow"
	default:
		return "dark"
	}
}

// ParseMode reads "dark", "light" or "follow". An empty name is Dark; an
// unknown name returns Dark and an error.
func ParseMode(name string) (Mode, error) {
	switch name {
	case "", "dark":
		return Dark, nil
	case "light":
		return Light, nil
	case "follow":
		return Follow, nil
	default:
		return Dark, fmt.Errorf("unknown palette mode %q: use dark, light or follow", name)
	}
}

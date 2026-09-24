package wm

// The cursor store may name a role instead of a concrete theme. "DYNAMIC" is
// the Hub's "Follow the wallpaper" pick: the pointer set matugen recolours to
// the live accent on every palette change. Every surface that wears or emits
// the cursor resolves the sentinel, so it never reaches a compositor as a
// theme name that does not exist on disk.

// CursorThemeDynamic is the stored sentinel for the wallpaper-following pick.
const CursorThemeDynamic = "DYNAMIC"

// CursorThemeMaterial is the concrete theme behind the sentinel. The
// ryoku-cursor-material package installs it and its recolor tool.
const CursorThemeMaterial = "Bibata-Material-Ryoku"

// ResolveCursorTheme maps the stored theme onto one installed on disk.
// Anything that is not the sentinel passes through untouched.
func ResolveCursorTheme(theme string) string {
	if theme == CursorThemeDynamic {
		return CursorThemeMaterial
	}
	return theme
}

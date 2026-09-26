package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: follow-wallpaper default ------------------------------------
//
// The colour master (~/.config/ryoku/theme.json) follows the wallpaper unless a
// curated static palette is locked, and a missing file reads as following: that
// is the shipped look on every compositor. Boxes that carry the mono era's
// followWallpaper=false with nothing locked are incoherent instead: the desktop
// sits on a static ramp while every surface claims to follow the wallpaper,
// which on a niri box (no Hyprland decoration regen to mask it) reads as
// "follow wallpaper colour is not the default". Restore the default once; a
// locked palette, or follow turned off afterwards, is a choice and stands.

// themeFollowMarker records that the one-time heal has run, so turning follow
// off later (with no lock) is never quietly undone.
func themeFollowMarker() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "migrations", "theme-follow-default")
}

// themeFollowHeal decides whether one theme.json needs the default restored.
// ok is false for a document that cannot be read as an object.
func themeFollowHeal(raw string) (heal, ok bool) {
	var doc map[string]any
	if json.Unmarshal([]byte(raw), &doc) != nil {
		return false, false
	}
	if f, isBool := doc["followWallpaper"].(bool); !isBool || f {
		return false, true // absent (defaults to on) or already following
	}
	for _, key := range []string{"theme", "scheme"} {
		if name, isStr := doc[key].(string); isStr && staticPaletteName(name) {
			return false, true // a locked palette is a deliberate look
		}
	}
	return true, true
}

// staticPaletteName reports whether a palette name locks a curated scheme, as
// opposed to the two dynamic variants that follow the wallpaper.
func staticPaletteName(name string) bool {
	switch name {
	case "", "mono", "Default", "Wallpaper":
		return false
	}
	return true
}

func reconcileThemeFollowDefault(checkOnly bool) recResult {
	marker := themeFollowMarker()
	if sys.Exists(marker) {
		return okRes(i18n.T("follow-wallpaper default already reconciled"))
	}
	mark := func() {
		if checkOnly {
			return
		}
		_ = os.MkdirAll(filepath.Dir(marker), 0o755)
		_ = os.WriteFile(marker, []byte("done\n"), 0o644)
	}
	store := filepath.Join(sys.ConfigHome(), "ryoku", "theme.json")
	if !sys.Exists(store) {
		mark() // no master on disk: a missing file already follows the wallpaper
		return okRes(i18n.T("no theme master; colours follow the wallpaper by default"))
	}
	raw := readFileSafe(store)
	heal, ok := themeFollowHeal(raw)
	if !ok || !heal {
		mark()
		return okRes(i18n.T("colours follow the wallpaper, or a palette is locked by choice"))
	}
	if checkOnly {
		return wouldRes(i18n.T("colours are pinned off the wallpaper with no palette locked; the default would follow it")).
			withFix("ryoku doctor")
	}
	var doc map[string]any
	if json.Unmarshal([]byte(raw), &doc) != nil {
		return failRes(i18n.T("could not read %s"), store)
	}
	doc["followWallpaper"] = true
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return failRes(i18n.T("could not update the theme master: %v"), err)
	}
	if err := writeStore(store, append(out, '\n')); err != nil {
		return failRes(i18n.T("could not save the theme master: %v"), err).withFix("ryoku doctor")
	}
	mark()
	return fixedRes(i18n.T("colours follow the wallpaper again"))
}

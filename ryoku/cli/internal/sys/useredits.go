package sys

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	wm "ryoku-wm"
)

// UserEditFiles lists the layable files in the overlay: regular files under
// UserEditsDir, slash-relative and sorted, skipping symlinks, .md notes (the
// guide and anything a user keeps beside their edits), and the overlay's own
// nested path. The overlay lays these over ~/.config; doctor reads the same set
// to spot forks; reset walks it to clear everything. Absent overlay -> no files.
func UserEditFiles() ([]string, error) {
	root := UserEditsDir()
	if _, err := os.Stat(root); err != nil {
		return nil, nil
	}
	var rels []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 || strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "ryoku/user_edits/") {
			return nil // never mirror the overlay tree into itself
		}
		rels = append(rels, rel)
		return nil
	})
	sort.Strings(rels)
	return rels, err
}

// LiveOwnedConfig are the files edited at their normal ~/.config path and loaded
// there directly: the tool's own user-include files, Hub-owned Starship and
// Fastfetch choices, Matugen palette siblings, and each provider's display, GPU
// and keyboard state. They must NEVER live in the overlay: overlayUserEdits
// would re-lay a frozen copy over the live file on every update and silently
// wipe edits made afterward.
var LiveOwnedConfig = liveOwnedConfig()

// compositorLiveOwned are the live-owned files under a provider's config dir.
func liveOwnedConfig() []string {
	files := []string{
		"kitty/user.conf",
		"fastfetch/config.jsonc",
		"fastfetch/ryoku-colors.json",
		"starship.toml",
		"starship/ryoku-colors.toml",
		"kitty/current-theme.conf",
		"ghostty/ryoku-colors",
	}
	// Every provider's files are kept regardless of which one is running, so an
	// update under Hyprland never prunes a niri box's seeds and vice versa. The
	// names come from the provider because they are its own config format.
	for _, name := range wm.Providers() {
		files = append(files, wm.ConfigUserOwned(name)...)
	}
	return files
}

// IsLiveOwnedConfig reports whether rel (a slash path relative to ~/.config) is
// one of the live-owned user files the overlay must never lay. The nvim tree
// counts too: it seeds once (updater.isSeed) and is then the user's, so a frozen
// overlay copy must never be re-laid over their live LazyVim config.
func IsLiveOwnedConfig(rel string) bool {
	if strings.HasPrefix(rel, "nvim/") {
		return true
	}
	for _, r := range LiveOwnedConfig {
		if r == rel {
			return true
		}
	}
	return false
}

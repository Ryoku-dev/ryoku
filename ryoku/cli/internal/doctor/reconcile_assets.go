package doctor

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

func reconcileCheckoutAssets(checkOnly bool, repo, root string) recResult {
	var stray []string
	assets := map[string]string{
		"ryoku/assets/brand":   "usr/share/ryoku/brand",
		"ryoku/apps/npm/npmrc": "usr/share/ryoku/npmrc",
	}
	for source, target := range assets {
		base := filepath.Join(repo, source)
		_ = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || !entry.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(base, path)
			if err != nil {
				return nil
			}
			dest := filepath.Join(root, target, rel)
			info, err := os.Lstat(dest)
			if err != nil || !info.Mode().IsRegular() || pkgOwnsFile(dest) {
				return nil
			}
			want, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			got, err := os.ReadFile(dest)
			if err == nil && bytes.Equal(got, want) {
				stray = append(stray, dest)
			}
			return nil
		})
	}
	if len(stray) == 0 {
		return okRes(i18n.T("source checkout; no identical unowned assets blocking pacman"))
	}
	if checkOnly {
		return wouldRes(i18n.T("%d identical unowned assets block a package upgrade"), len(stray)).withFix("ryoku doctor")
	}
	for _, path := range stray {
		if err := sys.Sudo("rm", "-f", "--", path); err != nil {
			return failRes(i18n.T("could not remove %s: %v"), path, err).withFix("ryoku doctor")
		}
	}
	return fixedRes(i18n.T("removed %d identical unowned assets for the next package upgrade"), len(stray))
}

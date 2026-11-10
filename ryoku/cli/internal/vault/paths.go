package vault

import (
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"
)

func Path() string {
	return filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "vault")
}

func legacyPath() string {
	return filepath.Join(sys.Xdg("XDG_DATA_HOME", ".local/share"), "ryoku", "rashin")
}

func OptOutPath() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "vault-unwired")
}

func versionPath() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "vault-version")
}

func sourceRoots() []string {
	if root := os.Getenv("RYOKU_VAULT_SHARE"); root != "" {
		return []string{root}
	}
	var roots []string
	explicitRepo := os.Getenv("RYOKU_REPO")
	if explicitRepo != "" {
		roots = append(roots, filepath.Join(explicitRepo, "ryoku", "vault"))
	}
	if repo := sys.ResolveRepo(); repo != "" && repo != explicitRepo {
		roots = append(roots, filepath.Join(repo, "ryoku", "vault"))
	}
	return append(roots, "/usr/share/ryoku/vault")
}

func sourceDir(name string) string {
	for _, root := range sourceRoots() {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

func home() string {
	return sys.Home()
}

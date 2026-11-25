package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// LegacyRashinConfig is read only by the explicit namespace migration.
func LegacyRashinConfig() string { return filepath.Join(ConfigHome(), "ryoku", "rashin.json") }

func LegacyRashinState() string    { return filepath.Join(StateHome(), "ryoku", "rashin") }
func LegacyRashinData() string     { return filepath.Join(DataHome(), "ryoku", "rashin") }
func LegacyRashinCache() string    { return filepath.Join(CacheHome(), "ryoku", "rashin") }
func LegacyRashinAppState() string { return filepath.Join(StateHome(), "ryoku", "rashin-app.json") }

func LegacyRashinRuntime() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" || !filepath.IsAbs(base) {
		return filepath.Join(os.TempDir(), fmt.Sprintf("rashin-%d", os.Getuid()))
	}
	return filepath.Join(base, "rashin")
}

// MigrationBackups is outside either app state tree, so a backup never contains itself.
func MigrationBackups() string { return filepath.Join(StateHome(), "hyakki-migration-backups") }

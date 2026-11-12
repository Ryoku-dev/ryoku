package hindsight

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/paths"
)

// Layout is where a local Hindsight keeps its pieces. Data holds the Python
// environment, State its database (as the child's home, where pg0 keeps its
// instance) and the private folder it runs in, Cache the models and uv's
// downloads.
type Layout struct {
	Data  string
	State string
	Cache string
}

// DefaultLayout keeps Hindsight beside Rashin's own data, state and cache.
func DefaultLayout() Layout {
	return Layout{
		Data:  filepath.Join(paths.DataHome(), "ryoku", "rashin", "hindsight"),
		State: filepath.Join(paths.StateDir(), "hindsight"),
		Cache: filepath.Join(paths.CacheDir(), "hindsight"),
	}
}

func (l Layout) venv() string      { return filepath.Join(l.Data, "venv") }
func (l Layout) pythons() string   { return filepath.Join(l.Data, "python") }
func (l Layout) program() string   { return filepath.Join(l.venv(), "bin", "hindsight-api") }
func (l Layout) python() string    { return filepath.Join(l.venv(), "bin", "python") }
func (l Layout) installed() string { return filepath.Join(l.Data, "installed") }
func (l Layout) home() string      { return filepath.Join(l.State, "home") }
func (l Layout) run() string       { return filepath.Join(l.State, "run") }
func (l Layout) log() string       { return filepath.Join(l.run(), "hindsight.log") }
func (l Layout) models() string    { return filepath.Join(l.Cache, "hf") }
func (l Layout) uvCache() string   { return filepath.Join(l.Cache, "uv") }

// postmaster is the file Postgres leaves while it runs; one left after
// Hindsight stopped means pg0's Postgres outlived it.
func (l Layout) postmaster() string {
	return filepath.Join(l.home(), ".pg0", "instances", instance, "data", "postmaster.pid")
}

// Installed reports whether setup finished with the lock this Rashin ships.
func (l Layout) Installed() bool {
	raw, err := os.ReadFile(l.installed())
	if err != nil {
		return false
	}
	if _, err := os.Stat(l.program()); err != nil {
		return false
	}
	return strings.TrimSpace(string(raw)) == lockSum()
}

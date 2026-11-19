package ytdlp

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain keeps the tests off the machine running them: every XDG directory
// and HOME point into a throwaway root.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryoget-ytdlp-test-")
	if err != nil {
		panic(err)
	}
	home := filepath.Join(root, "home")
	for key, value := range map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "runtime"),
	} {
		os.MkdirAll(value, 0o700)
		os.Setenv(key, value)
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

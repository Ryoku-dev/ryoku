package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain keeps the suite off the machine running it: every XDG directory
// points into a throwaway root.
func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	root, err := os.MkdirTemp("", "ryoget-services-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "XDG_") || strings.HasPrefix(key, "RYOKU_") {
			os.Unsetenv(key)
		}
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
		os.Setenv(key, value)
	}
	return m.Run()
}

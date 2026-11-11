package updater

import (
	"os"
	"testing"
)

// Every test in this package runs against a throwaway runtime and state dir:
// the run-state, the answer and secret channels and the update log all live
// there, and a test reaching the developer's real ones leaves the Hub showing
// a phantom update.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ryoku-updater-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_RUNTIME_DIR", dir)
	os.Setenv("XDG_STATE_HOME", dir)
	os.Setenv("RYOKU_UPDATE_UI", "")
	// Materialize also seeds the account, and HOME is the developer's here
	os.Setenv("RYOKU_SHARE_ROOT", dir)
	os.Unsetenv("RYOKU_SYSTEM_RICES_DIR")
	accountRepo = func() string { return "" }
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

package updater

import (
	"os"
	"testing"

	"ryoku-cli/internal/testenv"
)

// Every test in this package runs off this machine (see testenv.Run): the
// run-state, the answer and secret channels and the update log all live in its
// throwaway runtime and state dirs, and a test reaching the developer's real
// ones leaves the Hub showing a phantom update.
func TestMain(m *testing.M) {
	os.Setenv("RYOKU_UPDATE_UI", "")
	// Materialize also seeds the account: its sources stay off the machine too.
	share, err := os.MkdirTemp("", "ryoku-updater-share-")
	if err != nil {
		panic(err)
	}
	os.Setenv("RYOKU_SHARE_ROOT", share)
	os.Unsetenv("RYOKU_SYSTEM_RICES_DIR")
	accountRepo = func() string { return "" }
	code := testenv.Run(m)
	os.RemoveAll(share)
	os.Exit(code)
}
